package store

import (
	"fmt"
	"time"
)

// KeepaliveInterval is the rate of when clients and hosts are expected to send
// peering updates.
const KeepaliveInterval = 60 * time.Second

const ExpireInterval = KeepaliveInterval * 2

// FIXME: placeholder types, replace with go-ethereum types

type Account string // TODO: Switch to common.Address?
type NodeID string  // TODO: Switch to discv5.NodeID?
type Amount int64   // TODO: Switch to big.Int?

// Balance describes a node's account balance on the pool.
type Balance struct {
	Account      Account   `json:"account"`
	Credit       Amount    `json:"credit"`
	NextWithdraw time.Time `json:"next_withdraw"`
}

func (b *Balance) String() string {
	account := b.Account
	if account == "" {
		account = "(null account)"
	}
	return fmt.Sprintf("Balance(%q, %d)", account, b.Credit)
}

// Node stores metadata requires for tracking full nodes.
type Node struct {
	ID       NodeID
	URI      string    `json:"uri"`
	LastSeen time.Time `json:"last_seen"`
	Kind     string    `json:"kind"`
	IsHost   bool
	Payout   Account
}

// Store is the storage interface used by VipnodePool. It should be goroutine-safe.
type Store interface {
	// Close shuts down or disconnects from the storage driver.
	Close() error

	// CheckAndSaveNonce asserts that this is the highest nonce seen for this ID (typically nodeID or wallet address).
	CheckAndSaveNonce(ID string, nonce int64) error

	// GetBalance returns the current account balance for a node or account. If
	// both are non-zero values, then nodeID must be a valid spender of the
	// account.
	GetBalance(account Account, nodeID NodeID) (Balance, error)
	// AddBalance adds some credit amount to a node's account balance. (Can be negative)
	// If both account and node is provided, then the node is implicitly added
	// as a valid spender for this account.
	// If only a node is provided which doesn't have an account registered to
	// it, it should retain a balance, such as through temporary trial accounts
	// that get migrated later.
	AddBalance(account Account, nodeID NodeID, credit Amount) error

	// GetSpenders returns the authorized nodeIDs for this account, these are
	// nodes that were added to accounts through AddBalance.
	GetSpenders(account Account) ([]NodeID, error)

	// ActiveHosts returns `limit`-number of `kind` nodes. This could be an
	// empty list, if none are available.
	ActiveHosts(kind string, limit int) ([]Node, error)

	// GetNode returns the node from the set of active nods.
	GetNode(NodeID) (*Node, error)
	// SetNode adds a Node to the set of active nodes.
	SetNode(Node) error

	// NodePeers returns a list of active connected peers that this pool knows
	// about for this NodeID.
	NodePeers(nodeID NodeID) ([]Node, error)
	// UpdateNodePeers updates the Node.peers lookup with the current timestamp
	// of nodes we know about. This is used as a keepalive, and to keep track
	// of which client is connected to which host. Any missing peer is removed
	// from the known peers and returned. It also updates nodeID's
	// LastSeen.
	UpdateNodePeers(nodeID NodeID, peers []string) (inactive []NodeID, err error)
}
