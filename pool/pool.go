package pool

import (
	"context"

	"github.com/vipnode/vipnode/pool/store"
)

type UpdateResponse struct {
	Balance      *store.Balance `json:"balance,omitempty"`
	InvalidPeers []string       `json:"invalid_peers"`
}

// Pool represents a vipnode pool for coordinating between clients and hosts.
type Pool interface {
	// Host subscribes a host to receive vipnode_whitelist instructions.
	Host(ctx context.Context, kind string, payout string, nodeURI string) error

	// TODO: Do we need a Ready(ctx context.Context, peer string) to receive Whitelist responses?
	// Or would it be easier to fork ethereum's rpc to allow bidirectional services?

	// Connect requests for available hosts to connect to as a client.
	Connect(ctx context.Context, kind string) ([]store.Node, error)

	// Disconnect stops tracking the connection and billing, will prompt a
	// disconnect from both ends.
	Disconnect(ctx context.Context) error

	// Update is a keep-alive for sharing the node's peering info. It returns
	// a list of peers that are no longer corroborated by the pool, and current
	// balance for the node (if relevant).
	Update(ctx context.Context, peers []string) (*UpdateResponse, error)

	// Withdraw prompts a request to settle the node's balance.
	Withdraw(ctx context.Context) error
}
