package pool

import (
	"errors"
	"sync"
)

// Store is the storage interface used by VipnodePool. It should be goroutine-safe.
type Store interface {
	CheckAndSaveNonce(nodeID string, nonce int64) error

	// GetBalance returns the current balance for an account.
	GetBalance(account account) Balance
	// AddBalance adds some credit amount to that account balance.
	AddBalance(account account, credit amount) error

	// GetHostNodes returns `limit`-number of `kind` nodes. This could be an
	// empty list, if none are available.
	GetHostNodes(kind string, limit int) []HostNode
	// AddHostNode adds a HostNode to the set of active host nodes.
	AddHostNode(HostNode) error
	// RemoveHostNode removes a HostNode.
	RemoveHostNode(nodeID string) error
}

func MemoryStore() *memoryStore {
	return &memoryStore{
		balances:    map[account]Balance{},
		clientnodes: map[string]ClientNode{},
		hostnodes:   map[string]HostNode{},
		nonces:      map[string]int64{},
	}
}

type memoryStore struct {
	mu sync.Mutex

	// Registered balances
	balances map[account]Balance

	// Connected nodes
	clientnodes map[string]ClientNode
	hostnodes   map[string]HostNode

	nonces map[string]int64
}

func (s *memoryStore) CheckAndSaveNonce(nodeID string, nonce int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nonces[nodeID] >= nonce {
		return ErrInvalidNonce
	}
	s.nonces[nodeID] = nonce
	return nil
}

func (s *memoryStore) GetBalance(account account) Balance {
	// XXX: ...
	return Balance{}
}
func (s *memoryStore) AddBalance(account account, credit amount) error {
	return errors.New("not implemented")
}
func (s *memoryStore) AddHostNode(n HostNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostnodes[n.ID] = n
	return nil
}
func (s *memoryStore) RemoveHostNode(nodeID string) error {
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
