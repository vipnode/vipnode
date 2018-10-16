package badger

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/vipnode/vipnode/pool/store"
)

// TODO: Set reasonable expiration values?

type peers map[store.NodeID]time.Time

// Open returns a store.Store implementation using Badger as the storage
// driver. The store should be (*badgerStore).Close()'d after use.
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

func (s *badgerStore) hasKey(txn *badger.Txn, key []byte) bool {
	_, err := txn.Get(key)
	return err == nil
}

func (s *badgerStore) getItem(txn *badger.Txn, key []byte, into interface{}) error {
	item, err := txn.Get(key)
	if err != nil {
		return err
	}
	err = item.Value(func(val []byte) {
		if decodeErr := gob.NewDecoder(bytes.NewReader(val)).Decode(into); decodeErr != nil {
			err = decodeErr
		}
	})
	return err
}

func (s *badgerStore) setItem(txn *badger.Txn, key []byte, val interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val); err != nil {
		return err
	}
	return txn.Set(key, buf.Bytes())
}

func (s *badgerStore) CheckAndSaveNonce(ID string, nonce int64) error {
	// TODO: Check nonce timestamp as timestamp, reject if beyond some age, and
	// add TTL to nonce store.
	key := []byte(fmt.Sprintf("vip:nonce:%s", ID))
	return s.db.Update(func(txn *badger.Txn) error {
		var lastNonce int64
		if err := s.getItem(txn, key, &lastNonce); err == nil {
			if lastNonce >= nonce {
				return store.ErrInvalidNonce
			}
		} else if err != badger.ErrKeyNotFound {
			return err
		}

		return s.setItem(txn, key, nonce)
	})
}

func (s *badgerStore) GetBalance(nodeID store.NodeID) (store.Balance, error) {
	accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
	var account store.Account
	var r store.Balance
	err := s.db.View(func(txn *badger.Txn) error {
		balanceKey := []byte(fmt.Sprintf("vip:trial:%s", nodeID))
		if err := s.getItem(txn, accountKey, &account); err == badger.ErrKeyNotFound {
			// No spendable account, use the trial account
		} else if err == nil {
			balanceKey = []byte(fmt.Sprintf("vip:balance:%s", account))
		} else {
			return err
		}
		if err := s.getItem(txn, balanceKey, &r); err == badger.ErrKeyNotFound {
			nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
			if !s.hasKey(txn, nodeKey) {
				return store.ErrUnregisteredNode
			}
			return nil
		} else if err != nil {
			return err
		}
		return nil
	})

	return r, err
}

func (s *badgerStore) AddBalance(nodeID store.NodeID, credit store.Amount) error {
	accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
	return s.db.Update(func(txn *badger.Txn) error {
		var account store.Account
		balanceKey := []byte(fmt.Sprintf("vip:trial:%s", nodeID))
		if err := s.getItem(txn, accountKey, &account); err == badger.ErrKeyNotFound {
			// No spendable account, use the trial account
		} else if err == nil {
			balanceKey = []byte(fmt.Sprintf("vip:balance:%s", account))
		} else {
			return err
		}
		var balance store.Balance
		if err := s.getItem(txn, balanceKey, &balance); err == badger.ErrKeyNotFound {
			nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
			if !s.hasKey(txn, nodeKey) {
				return store.ErrUnregisteredNode
			}
			// No balance = empty balance
		} else if err != nil {
			return err
		}
		balance.Credit += credit

		return s.setItem(txn, balanceKey, balance)
	})
}

func (s *badgerStore) GetSpendable(account store.Account, nodeID store.NodeID) (store.Balance, error) {
	return store.Balance{}, errors.New("not implemented")
}

func (s *badgerStore) SetSpendable(account store.Account, nodeID store.NodeID) error {
	// TODO: Migrate trial account if exists
	return errors.New("not implemented")
}

// ActiveHosts loads all nodes, then return a valid shuffled subset of size limit.
func (s *badgerStore) ActiveHosts(kind string, limit int) ([]store.Node, error) {
	seenSince := time.Now().Add(-store.ExpireInterval)
	var r []store.Node
	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte("vip:node:")
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var n store.Node
			var err error
			err = it.Item().Value(func(val []byte) {
				if decodeErr := gob.NewDecoder(bytes.NewReader(val)).Decode(&n); decodeErr != nil {
					err = decodeErr
				}
			})
			if err != nil {
				return err
			}
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
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(r) < limit {
		// Skip shuffle since it's a subset
		return r, nil
	}

	// FIXME: Is there a different sorting method we want to use? Maybe sort by last seen?
	rand.Shuffle(len(r), func(i, j int) {
		r[i], r[j] = r[j], r[i]
	})

	return r[:limit], nil
}

func (s *badgerStore) GetNode(nodeID store.NodeID) (*store.Node, error) {
	key := []byte(fmt.Sprintf("vip:node:%s", nodeID))
	var r store.Node
	err := s.db.View(func(txn *badger.Txn) error {
		return s.getItem(txn, key, &r)
	})
	if err == badger.ErrKeyNotFound {
		return nil, store.ErrUnregisteredNode
	} else if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *badgerStore) SetNode(n store.Node) error {
	if n.ID == "" {
		return store.ErrMalformedNode
	}
	key := []byte(fmt.Sprintf("vip:node:%s", n.ID))
	return s.db.Update(func(txn *badger.Txn) error {
		return s.setItem(txn, key, n)
	})
}

func (s *badgerStore) NodePeers(nodeID store.NodeID) ([]store.Node, error) {
	peersKey := []byte(fmt.Sprintf("vip:peers:%s", nodeID))
	var r []store.Node
	err := s.db.View(func(txn *badger.Txn) error {
		var nodePeers map[store.NodeID]time.Time
		if err := s.getItem(txn, peersKey, &nodePeers); err == badger.ErrKeyNotFound {
			// No peers
			nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
			if !s.hasKey(txn, nodeKey) {
				return store.ErrUnregisteredNode
			}
			return nil
		} else if err != nil {
			return err
		}
		r = make([]store.Node, 0, len(nodePeers))
		for peerID, _ := range nodePeers {
			var peerNode store.Node
			if err := s.getItem(txn, []byte(fmt.Sprintf("vip:node:%s", peerID)), &peerNode); err == badger.ErrKeyNotFound {
				// Skip peer nodes we no longer know about.
				continue
			} else if err != nil {
				return err
			}
			r = append(r, peerNode)
		}
		return nil
	})
	return r, err
}

func (s *badgerStore) UpdateNodePeers(nodeID store.NodeID, peers []string) (inactive []store.NodeID, err error) {
	nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
	peersKey := []byte(fmt.Sprintf("vip:peers:%s", nodeID))
	now := time.Now()
	var node store.Node
	nodePeers := map[store.NodeID]time.Time{}
	err = s.db.Update(func(txn *badger.Txn) error {
		// Update this node's LastSeen
		if err := s.getItem(txn, nodeKey, &node); err == badger.ErrKeyNotFound {
			return store.ErrUnregisteredNode
		} else if err != nil {
			return err
		}

		node.LastSeen = now
		if err := s.setItem(txn, nodeKey, node); err != nil {
			return err
		}

		// Update peers
		if err := s.getItem(txn, peersKey, &nodePeers); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		numUpdated := 0
		for _, peerID := range peers {
			// Only update peers we already know about
			if s.hasKey(txn, []byte(fmt.Sprintf("vip:node:%s", peerID))) {
				nodePeers[store.NodeID(peerID)] = now
				numUpdated += 1
			}
		}

		if numUpdated == len(nodePeers) {
			return s.setItem(txn, peersKey, nodePeers)
		}

		inactiveDeadline := now.Add(-store.ExpireInterval)
		for nodeID, timestamp := range nodePeers {
			if timestamp.Before(inactiveDeadline) {
				continue
			}
			delete(nodePeers, nodeID)
			inactive = append(inactive, nodeID)
		}
		return s.setItem(txn, peersKey, nodePeers)
	})
	return
}
