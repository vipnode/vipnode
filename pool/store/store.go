package store

import (
	"fmt"
	"math/big"
	"time"
)

// KeepaliveInterval is the rate of when clients and hosts are expected to send
// peering updates.
const KeepaliveInterval = 60 * time.Second

const ExpireInterval = KeepaliveInterval * 2

// ExpireNonce as non-zero forces nonces to be nanosecond unix timestamps
// within 15 minutes of now. This allows us to discard old nonces more
// aggressively. Skewed clocks will get invalid nonce errors.
const ExpireNonce = 15 * time.Minute

// FIXME: placeholder types, replace with go-ethereum types

type Account string // TODO: Switch to common.Address?
type NodeID string  // TODO: Switch to discv5.NodeID?

func (id NodeID) IsZero() bool {
	return id == ""
}

func (id NodeID) String() string {
	return string(id)
}

func ParseNodeID(s string) (NodeID, error) {
	return NodeID(s), nil
}

// Balance describes a node's account balance on the pool.
type Balance struct {
	Account      Account   `json:"account,omitempty"`
	Deposit      big.Int   `json:"deposit"`
	Credit       big.Int   `json:"credit"`
	NextWithdraw time.Time `json:"next_withdraw,omitempty"`
}

func (b *Balance) String() string {
	account := b.Account
	total := new(big.Int).Add(&b.Credit, &b.Deposit)
	if len(account) == 0 {
		return fmt.Sprintf("Balance(<null account>, %s)", total)
	}
	return fmt.Sprintf("Balance(%q, %s)", account, total)
}

// Node stores metadata requires for tracking full nodes.
type Node struct {
	ID          NodeID
	URI         string    `json:"uri"`
	LastSeen    time.Time `json:"last_seen"`
	Kind        string    `json:"kind"`
	IsHost      bool
	Payout      Account
	BlockNumber uint64 `json:"block_number"`
}

// Stats contains various aggregate stats of the store state, used for
// providing a dashboard.
type Stats struct {
	NumActiveHosts    int     `json:"num_active_hosts"`
	NumTotalHosts     int     `json:"num_total_hosts"`
	NumActiveClients  int     `json:"num_active_clients"`
	NumTotalClients   int     `json:"num_total_clients"`
	LatestBlockNumber uint64  `json:"latest_block_number"`
	TotalCredit       big.Int `json:"total_credit"`
	TotalDeposit      big.Int `json:"total_deposit"`
	NumTrialBalances  int     `json:"num_trial_balances"`

	activeSince time.Time
}

// CountNode is a helper for aggregating node-related stats. It is not
// goroutine-safe.
func (stats *Stats) CountNode(n Node) {
	if stats.activeSince.IsZero() {
		stats.activeSince = time.Now().Add(-ExpireInterval)
	}
	isActive := n.LastSeen.After(stats.activeSince)
	if n.IsHost {
		stats.NumTotalHosts += 1
		if isActive {
			stats.NumActiveHosts += 1
		}
	} else {
		stats.NumTotalClients = 1
		if isActive {
			stats.NumActiveClients += 1
		}
	}
	if n.BlockNumber > stats.LatestBlockNumber {
		stats.LatestBlockNumber = n.BlockNumber
	}
}

// CountBalance is a helper for aggregating balance-related stats. It is not
// goroutine-safe.
func (s *Stats) CountBalance(b Balance) {
	s.TotalCredit.Add(&s.TotalCredit, &b.Credit)
	s.TotalDeposit.Add(&s.TotalDeposit, &b.Deposit)
	if b.Account == "" {
		s.NumTrialBalances += 1
	}
}

// Store is the storage interface used by VipnodePool. It should be goroutine-safe.
type Store interface {
	NonceStore
	PoolStore
	AccountStore

	// Stats returns aggregate statistics about the store state.
	Stats() (*Stats, error)

	// Close shuts down or disconnects from the storage driver.
	Close() error
}

type NonceStore interface {
	// CheckAndSaveNonce asserts that this is the highest nonce seen for this ID (typically nodeID or wallet address).
	CheckAndSaveNonce(ID string, nonce int64) error
}

// TODO: Replace ActiveHosts params with HostQuery type?

type PoolStore interface {
	// GetNode returns the node from the set of active nods.
	GetNode(NodeID) (*Node, error)
	// SetNode adds a Node to the set of active nodes.
	SetNode(Node) error

	// ActiveHosts returns `limit`-number of `kind` nodes. This could be an
	// empty list, if none are available.
	ActiveHosts(kind string, limit int) ([]Node, error)

	// NodePeers returns a list of active connected peers that this pool knows
	// about for this NodeID.
	NodePeers(nodeID NodeID) ([]Node, error)
	// UpdateNodePeers updates the Node.peers lookup with the current timestamp
	// of nodes we know about. This is used as a keepalive, and to keep track
	// of which client is connected to which host. Any missing peer is removed
	// from the known peers and returned. It also updates nodeID's
	// LastSeen.
	UpdateNodePeers(nodeID NodeID, peers []string, blockNumber uint64) (inactive []NodeID, err error)
}

// AccountStore manages the accounts associated with nodes and their balances.
type AccountStore interface {
	BalanceStore

	// AddAccountNode authorizes a nodeID to be a spender of an account's
	// balance. This should migrate any existing node's balance credit to the
	// account.
	AddAccountNode(account Account, nodeID NodeID) error
	// IsAccountNode returns nil if node is a valid spender of the given
	// account.
	IsAccountNode(account Account, nodeID NodeID) error
	// GetSpenders returns the authorized nodeIDs for this account, these are
	// nodes that were added to accounts through AddAccountNode.
	GetAccountNodes(account Account) ([]NodeID, error)
}

// BalanceStore is a store subset required for the balance manager.
type BalanceStore interface {
	// GetNodeBalance returns the current account balance for a node.
	GetNodeBalance(nodeID NodeID) (Balance, error)
	// AddNodeBalance adds some credit amount to a node's account balance. (Can be negative)
	// If only a node is provided which doesn't have an account registered to
	// it, it should retain a balance, such as through temporary trial accounts
	// that get migrated later.
	AddNodeBalance(nodeID NodeID, credit *big.Int) error

	// GetAccountBalance returns an account's balance.
	GetAccountBalance(account Account) (Balance, error)
	// AddNodeBalance adds credit to an account balance. (Can be negative)
	AddAccountBalance(account Account, credit *big.Int) error
}
