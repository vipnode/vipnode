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

type Account string
type NodeID string
type Amount int64

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

	balance *Balance             // Multiple nodes can share a balance (account), so it's a reference.
	peers   map[NodeID]time.Time // Last seen (only for vipnode-registered peers)
	inSync  bool                 // TODO: Do we need a penalty if a full node wants to accept peers while not in sync?
}

// Store is the storage interface used by VipnodePool. It should be goroutine-safe.
type Store interface {
	// CheckAndSaveNonce asserts that this is the highest nonce seen for this NodeID.
	CheckAndSaveNonce(nodeID NodeID, nonce int64) error

	// GetBalance returns the current account balance for a node.
	GetBalance(nodeID NodeID) (Balance, error)
	// AddBalance adds some credit amount to a node's account balance. (Can be negative)
	AddBalance(nodeID NodeID, credit Amount) error

	// GetSpendable returns the balance for an account only if nodeID is
	// authorized to spend it.
	GetSpendable(account Account, nodeID NodeID) (Balance, error)
	// SetSpendable authorizes nodeID to spend the balance (ie. allows nodeID
	// to access GetSpendable for that account).
	SetSpendable(account Account, nodeID NodeID) error

	// ActiveHosts returns `limit`-number of `kind` nodes. This could be an
	// empty list, if none are available.
	ActiveHosts(kind string, limit int) []Node

	// GetNode returns the node from the set of active nods.
	GetNode(NodeID) (*Node, error)
	// SetNode adds a Node to the set of active nodes.
	SetNode(Node, Account) error
	// RemoveNode removes a Node.
	RemoveNode(nodeID NodeID) error

	// NodePeers returns a list of active connected peers that this pool knows
	// about for this NodeID.
	NodePeers(nodeID NodeID) ([]Node, error)
	// UpdateNodePeers updates the Node.peers lookup with the current timestamp
	// of nodes we know about. This is used as a keepalive, and to keep track
	// of which client is connected to which host. Any missing peer is removed
	// from the known peers and returned. It also updates nodeID's
	// LastSeen.
	UpdateNodePeers(nodeID NodeID, peers []string) (inactive []Node, err error)
}
