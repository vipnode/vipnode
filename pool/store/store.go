package store

import (
	"time"
)

// FIXME: placeholder types, replace with go-ethereum types
type Account string
type NodeID string
type Amount int

// Balance describes a node's account balance on the pool.
type Balance struct {
	Account      Account   `json:"account"`
	Credit       Amount    `json:"credit"`
	NextWithdraw time.Time `json:"next_withdraw"`
}

// ClientNode stores metadata for tracking VIPs
type ClientNode struct {
	LastSeen time.Time `json:"last_seen"`
	Kind     string    `json:"kind"`

	balance *Balance
	peers   map[NodeID]time.Time // Last seen
}

// HostNode stores metadata requires for tracking full nodes.
type HostNode struct {
	ID       NodeID
	URI      string    `json:"uri"`
	LastSeen time.Time `json:"last_seen"`
	Kind     string    `json:"kind"`

	balance *Balance
	peers   map[NodeID]time.Time // Last seen
	inSync  bool                 // TODO: Do we need a penalty if a full node wants to accept peers while not in sync?
}

// Store is the storage interface used by VipnodePool. It should be goroutine-safe.
type Store interface {
	// CheckAndSaveNonce asserts that this is the highest nonce seen for this NodeID.
	CheckAndSaveNonce(nodeID NodeID, nonce int64) error

	// GetBalance returns the current balance for an account.
	GetBalance(account Account) Balance
	// AddBalance adds some credit amount to that account balance.
	AddBalance(account Account, credit Amount) error

	// GetHostNodes returns `limit`-number of `kind` nodes. This could be an
	// empty list, if none are available.
	GetHostNodes(kind string, limit int) []HostNode
	// SetHostNode adds a HostNode to the set of active host nodes.
	SetHostNode(HostNode) error
	// RemoveHostNode removes a HostNode.
	RemoveHostNode(nodeID NodeID) error
}
