package store

import (
	"sync"
)

// MemoryStore implements an ephemeral in-memory store. It may not be a
// complete implementation but it's useful for testing.
func MemoryStore() *memoryStore {
	return &memoryStore{
		balances:    map[Account]Balance{},
		clientnodes: map[NodeID]ClientNode{},
		hostnodes:   map[NodeID]HostNode{},
		nonces:      map[NodeID]int64{},
	}
}

// Assert Store implementation
var _ Store = &memoryStore{}

type memoryStore struct {
	mu sync.Mutex

	// Registered balances
	balances map[Account]Balance

	// Connected nodes
	clientnodes map[NodeID]ClientNode
	hostnodes   map[NodeID]HostNode

	nonces map[NodeID]int64
}

// CheckAndSaveNonce asserts that this is the highest nonce seen for this NodeID.
func (s *memoryStore) CheckAndSaveNonce(nodeID NodeID, nonce int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nonces[nodeID] >= nonce {
		return ErrInvalidNonce
	}
	s.nonces[nodeID] = nonce
	return nil
}

// GetBalance returns the current balance for an account.
func (s *memoryStore) GetBalance(account Account) Balance {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.balances[account]
}

// AddBalance adds some credit amount to that account balance.
func (s *memoryStore) AddBalance(account Account, credit Amount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.balances[account]
	b.Credit += credit
	if !ok {
		s.balances[account] = b
	}
	return nil
}

// SetHostNode adds a HostNode to the set of active host nodes.
func (s *memoryStore) SetHostNode(n HostNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostnodes[n.ID] = n
	return nil
}

// RemoveHostNode removes a HostNode.
func (s *memoryStore) RemoveHostNode(nodeID NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.hostnodes, nodeID)
	return nil
}

// GetHostNodes returns `limit`-number of `kind` nodes. This could be an
// empty list, if none are available.
func (s *memoryStore) GetHostNodes(kind string, limit int) []HostNode {
	r := make([]HostNode, 0, limit)

	s.mu.Lock()
	// TODO: Filter by kind (geth vs parity?)
	// TODO: Do something other than random, such as by availability.
	// FIXME: lol implicitly random map iteration
	for _, n := range s.hostnodes {
		r = append(r, n)
		limit -= 1
		if limit == 0 {
			// If limit is originally 0, then limit is effectively ignored
			// since it will be <0.
			break
		}
	}
	s.mu.Unlock()
	return r
}
