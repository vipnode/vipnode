package payment

import (
	"context"
	"errors"

	"github.com/vipnode/vipnode/pool/store"
)

type AddNodeRequest struct {
	Request struct {
		Account       string `json:"account"`
		AuthorizeNode string `json:"authorize_node"`
		Timestamp     int    `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"sig"`
}

type RemoveNodeRequest struct {
	Request struct {
		Account         string `json:"account"`
		UnauthorizeNode string `json:"unauthorize_node"`
		Timestamp       int    `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"sig"`
}

type GetNodesRequest struct {
	Account string `json:"account"`
}

type GetNodesResponse struct {
	NodeShortIDs []string `json:"node_shortids"`
}

type WithdrawRequest struct {
	Request struct {
		Account   string `json:"account"`
		Timestamp int    `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"sig"`
}

type PaymentService struct {
	Store store.Store
}

func (p *PaymentService) GetNodes(ctx context.Context, req GetNodesRequest) (*GetNodesResponse, error) {
	return nil, errors.New("GetNodes: not implemented yet")
}

// AddNode authorizes a nodeID to be spent by a wallet account.
func (p *PaymentService) AddNode(ctx context.Context, req AddNodeRequest) error {
	// XXX: Verify sig
	return errors.New("AddNode: not implemented yet")
}

func (p *PaymentService) RemoveNode(ctx context.Context, req RemoveNodeRequest) error {
	// XXX: Verify sig
	return errors.New("RemoveNode: not implemented yet")
}

// Withdraw schedules a balance withdraw for an account
func (p *PaymentService) Withdraw(ctx context.Context, req WithdrawRequest) error {
	// XXX: Verify sig
	return errors.New("WithdrawRequest: not implemented yet")
}
