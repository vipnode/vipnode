package badger

import (
	"github.com/dgraph-io/badger"
	"github.com/vipnode/vipnode/pool/store"
)

// Open returns a store.Store implementation using Badger as the storage
// driver. The store should be .Close()'d after use.
func Open(opts badger.Options) (*badgerStore, error) {
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &badgerStore{db: db}, nil
}

var _ store.Store = &badgerStore{}

type badgerStore struct {
	db *badger.DB
}

func (s *badgerStore) Close() error {
	return s.db.Close()
}

func (s *badgerStore) CheckAndSaveNonce(nodeID store.NodeID, nonce int64) error {
	panic("not implemented")
}

func (s *badgerStore) GetBalance(nodeID store.NodeID) (store.Balance, error) {
	panic("not implemented")
}

func (s *badgerStore) AddBalance(nodeID store.NodeID, credit store.Amount) error {
	panic("not implemented")
}

func (s *badgerStore) GetSpendable(account store.Account, nodeID store.NodeID) (store.Balance, error) {
	panic("not implemented")
}

func (s *badgerStore) SetSpendable(account store.Account, nodeID store.NodeID) error {
	panic("not implemented")
}

func (s *badgerStore) ActiveHosts(kind string, limit int) []store.Node {
	panic("not implemented")
}

func (s *badgerStore) GetNode(store.NodeID) (*store.Node, error) {
	panic("not implemented")
}

func (s *badgerStore) SetNode(store.Node, store.Account) error {
	panic("not implemented")
}

func (s *badgerStore) NodePeers(nodeID store.NodeID) ([]store.Node, error) {
	panic("not implemented")
}

func (s *badgerStore) UpdateNodePeers(nodeID store.NodeID, peers []string) (inactive []store.Node, err error) {
	panic("not implemented")
}
