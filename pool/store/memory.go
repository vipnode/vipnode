package store

import (
	"errors"
	"sync"
	"time"
)

// MemoryStore implements an ephemeral in-memory store. It may not be a
// complete implementation but it's useful for testing.
func MemoryStore() *memoryStore {
	return &memoryStore{
		balances: map[Account]Balance{},
		nodes:    map[NodeID]Node{},
		nonces:   map[string]int64{},
	}
}

// Assert Store implementation
var _ Store = &memoryStore{}

type memoryStore struct {
	mu sync.Mutex

	// Registered balances
	balances map[Account]Balance

	// Connected nodes
	nodes map[NodeID]Node

	nonces map[string]int64
}

// CheckAndSaveNonce asserts that this is the highest nonce seen for this NodeID.
func (s *memoryStore) CheckAndSaveNonce(ID string, nonce int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nonces[ID] >= nonce {
		return ErrInvalidNonce
	}
	s.nonces[ID] = nonce
	return nil
}

// GetBalance returns the current balance for an account.
func (s *memoryStore) GetBalance(nodeID NodeID) (Balance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return Balance{}, ErrUnregisteredNode
	}
	if node.balance == nil {
		return Balance{}, nil
	}
	return *node.balance, nil
}

// AddBalance adds some credit amount to that account balance.
func (s *memoryStore) AddBalance(nodeID NodeID, credit Amount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return ErrUnregisteredNode
	}
	if node.balance == nil {
		node.balance = &Balance{}
	}
	node.balance.Credit += credit
	s.nodes[nodeID] = node
	return nil
}

// GetSpendable returns the balance for an account only if nodeID is
// authorized to spend it.
func (s *memoryStore) GetSpendable(account Account, nodeID NodeID) (Balance, error) {
	return Balance{}, errors.New("not implemented")
}

// SetSpendable authorizes nodeID to spend the balance (ie. allows nodeID
// to access GetSpendable for that account).
func (s *memoryStore) SetSpendable(account Account, nodeID NodeID) error {
	return errors.New("not implemented")
}

// GetNode returns the node with the given ID.
func (s *memoryStore) GetNode(id NodeID) (*Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[id]
	if !ok {
		return nil, ErrUnregisteredNode
	}
	return &node, nil
}

// SetNode saves a node.
func (s *memoryStore) SetNode(n Node) error {
	if n.ID == "" {
		return ErrMalformedNode
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.balance == nil {
		// Use existing balance?
		if existing, ok := s.nodes[n.ID]; ok {
			n.balance = existing.balance
		}
	}
	if n.peers == nil {
		n.peers = map[NodeID]time.Time{}
	}
	s.nodes[n.ID] = n
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
		r = append(r, n)
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
			peers = append(peers, node)
		}
	}
	return peers, nil
}

// UpdateNodePeers updates the Node.peers lookup with the current timestamp
// of nodes we know about. This is used as a keepalive, and to keep track of
// which client is connected to which host.
func (s *memoryStore) UpdateNodePeers(nodeID NodeID, peers []string) ([]NodeID, error) {
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
		if _, ok := s.nodes[NodeID(peer)]; ok {
			node.peers[NodeID(peer)] = now
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

func (s *memoryStore) Close() error {
	return nil
}
