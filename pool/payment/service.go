package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

// ErrWithdrawDisabled is returned when the PaymentService is initialized in read-only mode.
var ErrWithdrawDisabled = errors.New("withdraw is disabled")

// WithdrawBalanceMinimumError is returned when the account balance is below
// the configured minimum to withdraw.
type WithdrawBalanceMinimumError struct {
	Balance *big.Int
	Minimum *big.Int
}

func (err WithdrawBalanceMinimumError) Error() string {
	return fmt.Sprintf("account balance (%d) is below the minimum required to withdraw (%d)", err.Balance, err.Minimum)
}

// AccountResponse is returned on RPC calls to pool_account
type AccountResponse struct {
	NodeShortIDs []string      `json:"node_short_ids"`
	Balance      store.Balance `json:"balance"`
}

// SettleHandler is a function that settles the balance of a given account by
// updating the internal balance to newBalance and disbursing paymentAmount.
type SettleHandler func(account store.Account, paymentAmount *big.Int, newBalance *big.Int) (txID string, err error)

// PaymentService is an RPC service for managing pool payment-relatd requests.
type PaymentService struct {
	NonceStore   store.NonceStore
	AccountStore store.AccountStore
	BalanceStore store.BalanceStore

	// Settle is a function that disburses the given paymentAmount and replaces
	// the current "on-chain" balance with newBalance. It returns a transaction
	// ID. If nil, then Withdraw calls will error with ErrWithdrawDisabled.
	Settle SettleHandler
	// WithdrawFee (optional) takes the withdraw total and returns the new total to withdraw (with any fees applied).
	WithdrawFee func(*big.Int) *big.Int
	// WithdrawMin (optional) is the minimum amount required to allow a withdraw.
	WithdrawMin *big.Int
}

func (p *PaymentService) verify(sig string, method string, wallet string, nonce int64, args ...interface{}) error {
	if err := p.NonceStore.CheckAndSaveNonce(wallet, nonce); err != nil {
		return pool.VerifyFailedError{Cause: err, Method: method}
	}

	if err := request.Verify(sig, method, wallet, nonce, args...); err != nil {
		return pool.VerifyFailedError{Cause: err, Method: method}
	}
	return nil
}

// Account is an *unverified* endpoint for retrieving the balance and list of
// node shortIDs associated with a wallet.
func (p *PaymentService) Account(ctx context.Context, wallet string) (*AccountResponse, error) {
	balance, err := p.BalanceStore.GetAccountBalance(store.Account(wallet))
	if err != nil {
		return nil, err
	}
	r := &AccountResponse{
		Balance: balance,
	}

	nodeIDs, err := p.AccountStore.GetAccountNodes(store.Account(wallet))
	if err != nil {
		return nil, err
	}
	r.NodeShortIDs = make([]string, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		r.NodeShortIDs = append(r.NodeShortIDs, string(nodeID)[:12])
	}
	return r, nil
}

// AddNode authorizes a nodeID to be spent by a wallet account.
func (p *PaymentService) AddNode(ctx context.Context, sig string, wallet string, nonce int64, nodeID string) error {
	if err := p.verify(sig, "pool_addNode", wallet, nonce, nodeID); err != nil {
		return err
	}

	return p.AccountStore.AddAccountNode(store.Account(wallet), store.NodeID(nodeID))
}

// Withdraw schedules a balance withdraw for an account
func (p *PaymentService) Withdraw(ctx context.Context, sig string, wallet string, nonce int64) error {
	if err := p.verify(sig, "pool_withdraw", wallet, nonce); err != nil {
		return err
	}

	if p.Settle == nil {
		return ErrWithdrawDisabled
	}

	account := store.Account(wallet)
	balance, err := p.BalanceStore.GetAccountBalance(account)
	if err != nil {
		return err
	}

	total := new(big.Int)
	total.Add(&balance.Deposit, &balance.Credit)

	if p.WithdrawMin != nil && total.Cmp(p.WithdrawMin) < 0 {
		return WithdrawBalanceMinimumError{
			Balance: total,
			Minimum: p.WithdrawMin,
		}
	}

	if p.WithdrawFee != nil {
		total = p.WithdrawFee(total)
	}

	newBalance := big.NewInt(0)
	txID, err := p.Settle(account, total, newBalance)
	if err != nil {
		return err
	}
	logger.Printf("Withdraw from account %q for %d: %s", account, total, txID)
	return nil
}
