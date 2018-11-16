package pool

import (
	"context"

	"github.com/vipnode/vipnode/pool/store"
)

// HostRequest is the request format for Host RPC calls.
type HostRequest struct {
	Kind    string `json:"kind"`
	Payout  string `json:"payout"`
	NodeURI string `json:"node_uri"`
}

type HostResponse struct {
	PoolVersion string `json:"pool_version"`
}

type ClientRequest struct {
	Kind string `json:"kind"`
}

type ClientResponse struct {
	// Hosts that have whitelisted the client NodeID and are ready for the
	// client to connect.
	Hosts []store.Node `json:"hosts"`
	// PoolVersion is the version of vipnode-pool that is running.
	PoolVersion string `json:"pool_version"`
	// Message contains a prompt for the client from the pool, possibly
	// instructions for interfacing with this pool. For example, a link to the
	// DApp for adding a balance deposit.
	Message string `json:"message,omitempty"`
}

type UpdateRequest struct {
	Peers       []string `json:"peers"`
	BlockNumber uint64   `json:"block_number"`
}

type UpdateResponse struct {
	Balance      *store.Balance `json:"balance,omitempty"`
	InvalidPeers []string       `json:"invalid_peers"`
}

// Pool represents a vipnode pool for coordinating between clients and hosts.
type Pool interface {
	// Host subscribes a host to receive vipnode_whitelist instructions.
	Host(ctx context.Context, req HostRequest) (*HostResponse, error)

	// Client requests for available hosts to connect to as a client.
	Client(ctx context.Context, req ClientRequest) (*ClientResponse, error)

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
