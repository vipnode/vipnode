package payment

import (
	"context"
	"errors"

	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

type PaymentService struct {
	Store store.Store
}

func (p *PaymentService) verify(sig string, method string, wallet string, nonce int64, args ...interface{}) error {
	if err := p.Store.CheckAndSaveNonce(wallet, nonce); err != nil {
		return pool.ErrVerifyFailed{Cause: err, Method: method}
	}

	if err := request.Verify(sig, method, wallet, nonce, args...); err != nil {
		return pool.ErrVerifyFailed{Cause: err, Method: method}
	}
	return nil
}

// GetNodes is an *unverified* endpoint for retrieving a list of node shortIDs associated with a wallet.
func (p *PaymentService) GetNodes(ctx context.Context, wallet string) ([]string, error) {
	return nil, errors.New("GetNodes: not implemented yet")
}

// AddNode authorizes a nodeID to be spent by a wallet account.
func (p *PaymentService) AddNode(ctx context.Context, sig string, wallet string, nonce int64, nodeID string) error {
	if err := p.verify(sig, "pool_addNode", wallet, nonce, nodeID); err != nil {
		return err
	}
	return errors.New("AddNode: not implemented yet")
}

func (p *PaymentService) RemoveNode(ctx context.Context, sig string, wallet string, nonce int64, nodeID string) error {
	if err := p.verify(sig, "pool_removeNode", wallet, nonce, nodeID); err != nil {
		return err
	}
	return errors.New("RemoveNode: not implemented yet")
}

// Withdraw schedules a balance withdraw for an account
func (p *PaymentService) Withdraw(ctx context.Context, sig string, wallet string, nonce int64) error {
	if err := p.verify(sig, "pool_withdraw", wallet, nonce); err != nil {
		return err
	}
	return errors.New("WithdrawRequest: not implemented yet")
}
