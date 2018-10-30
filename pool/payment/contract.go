package payment

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/vipnode/vipnode-contract/go/vipnodepool"
	"github.com/vipnode/vipnode/pool/store"
)

var zeroInt = &big.Int{}

// ErrDepositTimelocked is returned when a balance is checked but the deposit
// is timelocked.
var ErrDepositTimelocked = errors.New("deposit is timelocked")

// ContractPayment returns an abstraction around a vipnode pool payment contract.
func ContractPayment(address common.Address, backend bind.ContractBackend) (*contractPayment, error) {
	contract, err := vipnodepool.NewVipnodePool(address, backend)
	if err != nil {
		return nil, err
	}
	return &contractPayment{
		contract: contract,
		backend:  backend,
	}, nil
}

// ContractPayment uses the github.com/vipnode/vipnode-contract smart contract for payment.
type contractPayment struct {
	contract *vipnodepool.VipnodePool
	backend  bind.ContractBackend
}

func (p *contractPayment) SubscribeBalance(ctx context.Context, handler func(account store.Account, amount *big.Int)) error {
	sink := make(chan *vipnodepool.VipnodePoolBalance, 1)
	sub, err := p.contract.WatchBalance(&bind.WatchOpts{
		Context: ctx,
	}, sink)
	if err != nil {
		return err
	}
	for {
		select {
		case balanceEvent := <-sink:
			account := store.Account(balanceEvent.Client.Hex())
			go handler(account, balanceEvent.Balance)
		case err := <-sub.Err():
			return err
		case <-ctx.Done():
			sub.Unsubscribe()
			return ctx.Err()
		}
	}
}

func (p *contractPayment) GetBalance(account store.Account) (*big.Int, error) {
	r, err := p.contract.Clients(&bind.CallOpts{Pending: true}, common.HexToAddress(string(account)))
	if err != nil {
		return nil, err
	}
	if r.TimeLocked.Cmp(zeroInt) != 0 {
		return nil, ErrDepositTimelocked
	}
	return r.Balance, nil
}

func (p *contractPayment) Withdraw(account store.Account, amount *big.Int) (tx string, err error) {
	return "", errors.New("ContractPayment has not implemented Withdraw")
}
