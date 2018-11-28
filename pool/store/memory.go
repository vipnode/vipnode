package store

import (
	"math/big"
	"sync"
	"time"
)

// MemoryStore implements an ephemeral in-memory store. It may not be a
// complete implementation but it's useful for testing.
func MemoryStore() *memoryStore {
	return &memoryStore{
		balances: map[Account]Balance{},
		nodes:    map[NodeID]memNode{},
		accounts: map[NodeID]Account{},
		trials:   map[NodeID]Balance{},
		nonces:   map[string]int64{},
	}
}

type memNode struct {
	Node

	peers map[NodeID]time.Time // Last seen (only for vipnode-registered peers)
}

// Assert Store implementation
var _ Store = &memoryStore{}

type memoryStore struct {
	mu sync.Mutex

	// Registered balances
	balances map[Account]Balance

	// Connected nodes
	nodes map[NodeID]memNode

	// Node to balance mapping
	accounts map[NodeID]Account

	// Trial balances to be migrated once registered
	trials map[NodeID]Balance

	nonces map[string]int64
}

// CheckAndSaveNonce asserts that this is the highest nonce seen for this NodeID.
func (s *memoryStore) CheckAndSaveNonce(ID string, nonce int64) error {
	if ExpireNonce > 0 && nonce <= time.Now().Add(-ExpireNonce).UnixNano() {
		// Nonce is too old
		return ErrInvalidNonce
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nonces[ID] >= nonce {
		return ErrInvalidNonce
	}
	s.nonces[ID] = nonce
	return nil
}

// GetNodeBalance returns the current account balance for a node.
func (s *memoryStore) GetNodeBalance(nodeID NodeID) (Balance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.nodes[nodeID]
	if !ok {
		return Balance{}, ErrUnregisteredNode
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
func (s *memoryStore) AddNodeBalance(nodeID NodeID, credit *big.Int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.nodes[nodeID]
	if !ok {
		return ErrUnregisteredNode
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
func (s *memoryStore) GetAccountBalance(account Account) (Balance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.balances[account], nil
}

// AddNodeBalance adds credit to an account balance. (Can be negative)
func (s *memoryStore) AddAccountBalance(account Account, credit *big.Int) error {
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
func (s *memoryStore) AddAccountNode(account Account, nodeID NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.nodes[nodeID]
	if !ok {
		return ErrUnregisteredNode
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
func (s *memoryStore) IsAccountNode(account Account, nodeID NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	nodeAccount, ok := s.accounts[nodeID]
	if !ok || nodeAccount != account {
		return ErrNotAuthorized
	}
	return nil
}

// GetSpenders returns the authorized nodeIDs for this account, these are
// nodes that were added to accounts through AddAccountNode.
func (s *memoryStore) GetAccountNodes(account Account) ([]NodeID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := []NodeID{}
	for nodeID, a := range s.accounts {
		if account == a {
			r = append(r, nodeID)
		}
	}
	return r, nil
}

// GetNode returns the node with the given ID.
func (s *memoryStore) GetNode(id NodeID) (*Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[id]
	if !ok {
		return nil, ErrUnregisteredNode
	}
	return &node.Node, nil
}

// SetNode saves a node.
func (s *memoryStore) SetNode(n Node) error {
	if n.ID.IsZero() {
		return ErrMalformedNode
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node := memNode{Node: n}
	if node.peers == nil {
		node.peers = map[NodeID]time.Time{}
	}
	s.nodes[n.ID] = node
	return nil
}

// RemoveNode removes a HostNode.
func (s *memoryStore) RemoveNode(nodeID NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, nodeID)
	return nil
}

// ActiveHosts returns `limit`-number of `kind` nodes. This could be an
// empty list, if none are available.
func (s *memoryStore) ActiveHosts(kind string, limit int) ([]Node, error) {
	seenSince := time.Now().Add(-ExpireInterval)
	r := make([]Node, 0, limit)

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
func (s *memoryStore) NodePeers(nodeID NodeID) ([]Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, ErrUnregisteredNode
	}
	peers := []Node{}
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
func (s *memoryStore) UpdateNodePeers(nodeID NodeID, peers []string, blockNumber uint64) ([]NodeID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, ErrUnregisteredNode
	}
	now := time.Now()
	node.LastSeen = now
	numUpdated := 0
	for _, peer := range peers {
		// Only update peers we already know about
		// FIXME: If symmetric peers disappear at the same time, then reappear, will it be a problem if they never become inactive? (Okay if the balance manager caps the update interval?)
		// FIXME: Should the timestamp update happen on the peerNode also? Or is it okay to leave it for the symmetric update call?
		peerID, err := ParseNodeID(peer)
		if err != nil {
			// Skip bad peers
			continue
		}
		if _, ok := s.nodes[peerID]; ok {
			node.peers[peerID] = now
			numUpdated += 1
		}
	}

	if numUpdated == len(node.peers) {
		s.nodes[nodeID] = node
		return nil, nil
	}
	inactive := []NodeID{}
	inactiveDeadline := now.Add(-ExpireInterval)
	for nodeID, timestamp := range node.peers {
		if timestamp.Before(inactiveDeadline) {
			continue
		}
		delete(node.peers, nodeID)
		inactive = append(inactive, nodeID)
	}

	s.nodes[nodeID] = node
	return inactive, nil
}

// Stats returns aggregate statistics about the store state.
func (s *memoryStore) Stats() (*Stats, error) {
	stats := Stats{}

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
