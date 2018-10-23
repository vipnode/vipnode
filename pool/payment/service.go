package payment

import (
	"context"
	"errors"

	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

// StatusResponse is returned on RPC calls to pool_status
type StatusResponse struct {
	NodeShortIDs []string      `json:"node_short_ids"`
	Balance      store.Balance `json:"balance"`
}

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
func (p *PaymentService) Status(ctx context.Context, wallet string) (*StatusResponse, error) {
	nodeIDs, err := p.Store.GetAccountNodes(store.Account(wallet))
	if err != nil {
		return nil, err
	}
	balance, err := p.Store.GetAccountBalance(store.Account(wallet))
	if err != nil {
		return nil, err
	}
	r := &StatusResponse{
		Balance: balance,
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

	return p.Store.AddAccountNode(store.Account(wallet), store.NodeID(nodeID))
}

// Withdraw schedules a balance withdraw for an account
func (p *PaymentService) Withdraw(ctx context.Context, sig string, wallet string, nonce int64) error {
	if err := p.verify(sig, "pool_withdraw", wallet, nonce); err != nil {
		return err
	}
	return errors.New("WithdrawRequest: not implemented yet")
}
