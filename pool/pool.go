package pool

import (
	"context"

	"github.com/vipnode/vipnode/pool/store"
)

type HostRequest struct {
	Kind    string `json:"kind"`
	Payout  string `json:"payout"`
	NodeURI string `json:"node_uri"`
}

type ConnectRequest struct {
	Kind string `json:"kind"`
}

type ConnectResponse struct {
	Hosts []store.Node `json:"hosts"`
}

type UpdateRequest struct {
	Peers []string `json:"peers"`
}

type UpdateResponse struct {
	Balance      *store.Balance `json:"balance,omitempty"`
	InvalidPeers []string       `json:"invalid_peers"`
}

// Pool represents a vipnode pool for coordinating between clients and hosts.
type Pool interface {
	// Host subscribes a host to receive vipnode_whitelist instructions.
	Host(ctx context.Context, req HostRequest) error

	// Connect requests for available hosts to connect to as a client.
	Connect(ctx context.Context, req ConnectRequest) (*ConnectResponse, error)

	// Disconnect stops tracking the connection and billing, will prompt a
	// disconnect from both ends.
	Disconnect(ctx context.Context) error

	// Update is a keep-alive for sharing the node's peering info. It returns
	// a list of peers that are no longer corroborated by the pool, and current
	// balance for the node (if relevant).
	Update(ctx context.Context, req UpdateRequest) (*UpdateResponse, error)

	// Withdraw prompts a request to settle the node's balance.
	Withdraw(ctx context.Context) error
}
