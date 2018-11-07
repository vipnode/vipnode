package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/vipnode/vipnode-contract/go/vipnodepool"
	"github.com/vipnode/vipnode/pool/store"
)

// AddressMismatchError is used when we require a specific address but the wrong one was provided.
type AddressMismatchError struct {
	Prelude string
	Want    common.Address
	Got     common.Address
}

func (e AddressMismatchError) Error() string {
	return fmt.Sprintf("%s: got %q; want %q", e.Prelude, e.Got.Hex(), e.Want.Hex())
}

var zeroInt = &big.Int{}

// ErrDepositTimelocked is returned when a balance is checked but the deposit
// is timelocked.
var ErrDepositTimelocked = errors.New("deposit is timelocked")

// ContractPayment returns an abstraction around a vipnode pool payment
// contract. Contract implements store.NodeBalanceStore.
func ContractPayment(storeDriver store.AccountStore, address common.Address, backend bind.ContractBackend, transactOpts *bind.TransactOpts) (*contractPayment, error) {
	contract, err := vipnodepool.NewVipnodePool(address, backend)
	if err != nil {
		return nil, err
	}
	p := &contractPayment{
		store:        storeDriver,
		contract:     contract,
		backend:      backend,
		transactOpts: transactOpts,
	}

	if transactOpts != nil {
		// Check that the transactor matches the contract operator
		opAddr, err := contract.Operator(nil)
		if err != nil {
			return nil, err
		}
		if opAddr != transactOpts.From {
			return nil, AddressMismatchError{
				Prelude: "incorrect wallet provided for contract operator",
				Want:    opAddr,
				Got:     transactOpts.From,
			}
		}
	}
	// Setup cache getter and subscribe to the event-based value fill
	p.balanceCache.Getter = p.GetBalance
	if err := p.SubscribeBalance(context.Background(), p.balanceCache.Set); err != nil {
		return nil, err
	}
	return p, nil
}

var _ store.BalanceStore = &contractPayment{}

// ContractPayment uses the github.com/vipnode/vipnode-contract smart contract for payment.
type contractPayment struct {
	store        store.AccountStore
	contract     *vipnodepool.VipnodePool
	backend      bind.ContractBackend
	balanceCache balanceCache
	transactOpts *bind.TransactOpts
}

// GetNodeBalance proxies the normal store implementation
// by adding the contract deposit to the resulting balance.
func (p *contractPayment) GetNodeBalance(nodeID store.NodeID) (store.Balance, error) {
	balance, err := p.store.GetNodeBalance(nodeID)
	if err != nil {
		return balance, err
	}

	if len(balance.Account) == 0 {
		// No account associated, probably on trial
		return balance, nil
	}

	// FIXME: Cache this, since it's pretty slow. Use SubscribeBalance to update the cache.
	deposit, err := p.balanceCache.Get(balance.Account)
	if err != nil {
		return balance, err
	}
	balance.Deposit = *deposit
	return balance, nil
}

// AddNodeBalance proxies to the underlying store.BalanceStore
func (p *contractPayment) AddNodeBalance(nodeID store.NodeID, credit *big.Int) error {
	return p.store.AddNodeBalance(nodeID, credit)
}

// GetAccountBalance returns an account's balance, which includes the contract deposit.
func (p *contractPayment) GetAccountBalance(account store.Account) (store.Balance, error) {
	balance, err := p.store.GetAccountBalance(account)
	if err != nil {
		return balance, err
	}

	// FIXME: Cache this, since it's pretty slow. Use SubscribeBalance to update the cache.
	deposit, err := p.balanceCache.Get(account)
	if err != nil {
		return balance, err
	}
	balance.Deposit = *deposit
	return balance, nil
}

// AddAccountBalance proxies to the underlying store.BalanceStore
func (p *contractPayment) AddAccountBalance(account store.Account, credit *big.Int) error {
	return p.store.AddAccountBalance(account, credit)
}

func (p *contractPayment) SubscribeBalance(ctx context.Context, handler func(account store.Account, amount *big.Int)) error {
	sink := make(chan *vipnodepool.VipnodePoolBalance, 1)
	sub, err := p.contract.WatchBalance(&bind.WatchOpts{
		Context: ctx,
	}, sink)
	if err != nil {
		return err
	}
	eventHandler := func() error {
		for {
			select {
			case balanceEvent := <-sink:
				account := store.Account(balanceEvent.Client.Hex())
				logger.Printf("SubscribeBalance: Processing event for account: %s", account)
				go handler(account, balanceEvent.Balance)
			case err := <-sub.Err():
				return err
			case <-ctx.Done():
				sub.Unsubscribe()
				return ctx.Err()
			}
		}
	}
	go func() {
		// FIXME: This is a hacky retry loop because we don't have a convenient
		// way to bubble up errors. Need to refactor someday.
		retryTimeout := time.Minute * 10 // Abort if we fail more often than once in 10min
		var lastErr time.Time
		for {
			err := eventHandler()
			if err == nil {
				// Clean exit
				return
			}
			logger.Print("SubscribeBalance event handler loop failed: %s", err)
			if lastErr.Add(retryTimeout).After(time.Now()) {
				break
			}
		}
		logger.Printf("SubscribeBalance event loop aborted, falling back to expiration cache.")
		p.balanceCache.Reset(time.Minute * 10)
	}()
	return nil
}

func (p *contractPayment) GetBalance(account store.Account) (*big.Int, error) {
	if account == store.Account("") {
		return nil, errors.New("failed to get balance: empty account")
	}
	timer := time.Now()
	r, err := p.contract.Clients(&bind.CallOpts{Pending: true}, common.HexToAddress(string(account)))
	if err != nil {
		return nil, err
	}
	if r.TimeLocked.Cmp(zeroInt) != 0 {
		return nil, ErrDepositTimelocked
	}
	logger.Printf("Retrieved contract balance for %q in %s: %d", account, time.Now().Sub(timer), r.Balance)
	return r.Balance, nil
}

func (p *contractPayment) OpSettle(account store.Account, amount *big.Int) (tx string, err error) {
	if p.transactOpts == nil {
		return "", errors.New("OpSettle contract write failed: Payment provider is in read-only mode.")
	}
	addr := common.HexToAddress(string(account))
	txn, err := p.contract.OpSettle(p.transactOpts, addr, amount, true)
	if err != nil {
		return "", err
	}
	return txn.Hash().Hex(), nil
}
