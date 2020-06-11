package memory

import (
	"math/big"
	"sync"
	"time"

	"github.com/vipnode/vipnode/v2/pool/store"
)

// New implements an ephemeral in-memory store. It may not be a
// complete implementation but it's useful for testing.
func New() *memoryStore {
	return &memoryStore{
		balances: map[store.Account]store.Balance{},
		nodes:    map[store.NodeID]memNode{},
		accounts: map[store.NodeID]store.Account{},
		trials:   map[store.NodeID]store.Balance{},
		nonces:   map[string]int64{},
	}
}

type memNode struct {
	store.Node

	peers map[store.NodeID]time.Time // Last seen (only for vipnode-registered peers)
}

// Assert Store implementation
var _ store.Store = &memoryStore{}

type memoryStore struct {
	mu sync.Mutex

	// Registered balances
	balances map[store.Account]store.Balance

	// Connected nodes
	nodes map[store.NodeID]memNode

	// Node to balance mapping
	accounts map[store.NodeID]store.Account

	// Trial balances to be migrated once registered
	trials map[store.NodeID]store.Balance

	nonces map[string]int64
}

// CheckAndSaveNonce asserts that this is the highest nonce seen for this NodeID.
func (s *memoryStore) CheckAndSaveNonce(ID string, nonce int64) error {
	if store.ExpireNonce > 0 && nonce <= time.Now().Add(-store.ExpireNonce).UnixNano() {
		// Nonce is too old
		return store.ErrInvalidNonce
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nonces[ID] >= nonce {
		return store.ErrInvalidNonce
	}
	s.nonces[ID] = nonce
	return nil
}

// GetNodeBalance returns the current account balance for a node.
func (s *memoryStore) GetNodeBalance(nodeID store.NodeID) (store.Balance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.nodes[nodeID]
	if !ok {
		return store.Balance{}, store.ErrUnregisteredNode
	}

	account, ok := s.accounts[nodeID]
	if !ok {
		return s.trials[nodeID], nil
	}
	return s.balances[account], nil
}

// AddNodeBalance adds some credit amount to a node's account balance. (Can be negative)
// If only a node is provided which doesn't have an account registered to
// it, it should retain a balance, such as through temporary trial accounts
// that get migrated later.
//
// This driver only supports mapping a nodeID to one account, so remapping it
// will move the nodeID to the other account.
func (s *memoryStore) AddNodeBalance(nodeID store.NodeID, credit *big.Int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.nodes[nodeID]
	if !ok {
		return store.ErrUnregisteredNode
	}
	account, ok := s.accounts[nodeID]
	if ok {
		balance := s.balances[account]
		balance.Credit.Add(&balance.Credit, credit)
		s.balances[account] = balance
	} else {
		balance := s.trials[nodeID]
		balance.Credit.Add(&balance.Credit, credit)
		s.trials[nodeID] = balance
	}
	return nil
}

// GetAccountBalance returns an account's balance.
func (s *memoryStore) GetAccountBalance(account store.Account) (store.Balance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.balances[account], nil
}

// AddNodeBalance adds credit to an account balance. (Can be negative)
func (s *memoryStore) AddAccountBalance(account store.Account, credit *big.Int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	balance := s.balances[account]
	balance.Credit.Add(&balance.Credit, credit)
	s.balances[account] = balance
	return nil
}

// AddAccountNode authorizes a nodeID to be a spender of an account's
// balance. This should migrate any existing node's balance credit to the
// account.
func (s *memoryStore) AddAccountNode(account store.Account, nodeID store.NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.nodes[nodeID]
	if !ok {
		return store.ErrUnregisteredNode
	}

	// Migrate any trial balance
	balance := s.balances[account]
	s.accounts[nodeID] = account
	trialBalance := s.trials[nodeID]
	balance.Credit.Add(&balance.Credit, &trialBalance.Credit)
	balance.Account = account
	delete(s.trials, nodeID)
	s.balances[account] = balance
	return nil
}

// AddAccountNode authorizes a nodeID to be a spender of an account's
// balance.
func (s *memoryStore) IsAccountNode(account store.Account, nodeID store.NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	nodeAccount, ok := s.accounts[nodeID]
	if !ok || nodeAccount != account {
		return store.ErrNotAuthorized
	}
	return nil
}

// GetSpenders returns the authorized nodeIDs for this account, these are
// nodes that were added to accounts through AddAccountNode.
func (s *memoryStore) GetAccountNodes(account store.Account) ([]store.NodeID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := []store.NodeID{}
	for nodeID, a := range s.accounts {
		if account == a {
			r = append(r, nodeID)
		}
	}
	return r, nil
}

// GetNode returns the node with the given ID.
func (s *memoryStore) GetNode(id store.NodeID) (*store.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[id]
	if !ok {
		return nil, store.ErrUnregisteredNode
	}
	return &node.Node, nil
}

// SetNode saves a node.
func (s *memoryStore) SetNode(n store.Node) error {
	if n.ID.IsZero() {
		return store.ErrMalformedNode
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node := memNode{Node: n}
	if node.peers == nil {
		node.peers = map[store.NodeID]time.Time{}
	}
	s.nodes[n.ID] = node
	return nil
}

// RemoveNode removes a HostNode.
func (s *memoryStore) RemoveNode(nodeID store.NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, nodeID)
	return nil
}

// ActiveHosts returns `limit`-number of `kind` nodes. This could be an
// empty list, if none are available.
func (s *memoryStore) ActiveHosts(kind string, limit int) ([]store.Node, error) {
	seenSince := time.Now().Add(-store.ExpireInterval)
	r := make([]store.Node, 0, limit)

	s.mu.Lock()
	defer s.mu.Unlock()
	// TODO: Do something other than random, such as by availability?
	for _, n := range s.nodes {
		// Ranging over a map is implicitly random, so
		// results are shuffled as is desireable.
		if !n.IsHost {
			continue
		}
		if kind != "" && n.Kind != kind {
			continue
		}
		if !n.LastSeen.After(seenSince) {
			continue
		}
		r = append(r, n.Node)
		limit -= 1
		if limit == 0 {
			// If limit is originally 0, then limit is effectively ignored
			// since it will be <0.
			break
		}
	}
	return r, nil
}

// NodePeers returns a list of active connected peers that this pool knows
// about for this NodeID.
func (s *memoryStore) NodePeers(nodeID store.NodeID) ([]store.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, store.ErrUnregisteredNode
	}
	peers := []store.Node{}
	for nodeID, _ := range node.peers {
		if node, ok := s.nodes[nodeID]; ok {
			peers = append(peers, node.Node)
		}
	}
	return peers, nil
}

// UpdateNodePeers updates the Node.peers lookup with the current timestamp
// of nodes we know about. This is used as a keepalive, and to keep track of
// which client is connected to which host.
// Node's LastSeen should only be updated when it calls UpdateNodePeers.
// Node's LastSeen should be counted whether it's inactive only when it's
// included as a peer by another node in an UpdateNodePeers call.
func (s *memoryStore) UpdateNodePeers(nodeID store.NodeID, peers []string, blockNumber uint64) (inactive []store.NodeID, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		err = store.ErrUnregisteredNode
		return
	}
	now := time.Now()
	node.LastSeen = now
	node.BlockNumber = blockNumber

	for _, peer := range peers {
		// Only update peers we already know about
		// FIXME: If symmetric peers disappear at the same time, then reappear, will it be a problem if they never become inactive? (Okay if the balance manager caps the update interval?)
		peerID, err := store.ParseNodeID(peer)
		if err != nil {
			// Skip bad peers
			continue
		}
		if peer, ok := s.nodes[peerID]; ok {
			node.peers[peerID] = peer.LastSeen
		}
	}

	inactiveDeadline := now.Add(-store.ExpireInterval)
	for nodeID, timestamp := range node.peers {
		if timestamp.After(inactiveDeadline) {
			continue
		}
		delete(node.peers, nodeID)
		inactive = append(inactive, nodeID)
	}

	s.nodes[nodeID] = node
	return
}

// Stats returns aggregate statistics about the store state.
func (s *memoryStore) Stats() (*store.Stats, error) {
	stats := store.Stats{}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, n := range s.nodes {
		stats.CountNode(n.Node)
	}
	for _, b := range s.balances {
		stats.CountBalance(b)
	}
	for _, b := range s.trials {
		stats.CountBalance(b)
	}
	return &stats, nil
}

func (s *memoryStore) Close() error {
	return nil
}
