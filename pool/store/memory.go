package store

import (
	"errors"
	"sync"
)

// MemoryStore implements an ephemeral in-memory store.
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

func (s *memoryStore) CheckAndSaveNonce(nodeID NodeID, nonce int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nonces[nodeID] >= nonce {
		return ErrInvalidNonce
	}
	s.nonces[nodeID] = nonce
	return nil
}

func (s *memoryStore) GetBalance(account Account) Balance {
	// XXX: ...
	return Balance{}
}
func (s *memoryStore) AddBalance(account Account, credit Amount) error {
	return errors.New("not implemented")
}
func (s *memoryStore) AddHostNode(n HostNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostnodes[n.ID] = n
	return nil
}
func (s *memoryStore) RemoveHostNode(nodeID NodeID) error {
	return errors.New("not implemented")
}

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
