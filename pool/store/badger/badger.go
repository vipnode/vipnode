package badger

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math/big"
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

	if err := MigrateLatest(db, opts.Dir); err != nil {
		return nil, err
	}

	s := &badgerStore{
		db:          db,
		nonceExpire: store.ExpireNonce,
	}

	return s, nil
}

var _ store.Store = &badgerStore{}

type badgerStore struct {
	db *badger.DB

	nonceExpire time.Duration
}

func (s *badgerStore) Close() error {
	return s.db.Close()
}

func (s *badgerStore) CheckAndSaveNonce(ID string, nonce int64) error {
	// If nonceExpire is set, nonce should be within nonceExpire of now.
	if s.nonceExpire > 0 {
		if nonce <= time.Now().Add(-s.nonceExpire).UnixNano() {
			// Nonce is too old
			return store.ErrInvalidNonce
		}
	}
	key := []byte(fmt.Sprintf("vip:nonce:%s", ID))
	return s.db.Update(func(txn *badger.Txn) error {
		var lastNonce int64
		if err := getItem(txn, key, &lastNonce); err == nil {
			if lastNonce >= nonce {
				return store.ErrInvalidNonce
			}
		} else if err != badger.ErrKeyNotFound {
			return err
		}

		if s.nonceExpire > 0 {
			return setExpiringItem(txn, key, &nonce, s.nonceExpire)
		}
		return setItem(txn, key, &nonce)
	})
}

// GetNodeBalance returns the current account balance for a node.
func (s *badgerStore) GetNodeBalance(nodeID store.NodeID) (store.Balance, error) {
	accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
	var account store.Account
	var r store.Balance
	err := s.db.View(func(txn *badger.Txn) error {
		balanceKey := []byte(fmt.Sprintf("vip:trial:%s", nodeID))
		if err := getItem(txn, accountKey, &account); err == badger.ErrKeyNotFound {
			// No spendable account, use the trial account
		} else if err == nil {
			balanceKey = []byte(fmt.Sprintf("vip:balance:%s", account))
		} else {
			return err
		}
		if err := getItem(txn, balanceKey, &r); err == badger.ErrKeyNotFound {
			nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
			if !hasKey(txn, nodeKey) {
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

// AddNodeBalance adds some credit amount to a node's account balance. (Can be negative)
// If only a node is provided which doesn't have an account registered to
// it, it should retain a balance, such as through temporary trial accounts
// that get migrated later.
func (s *badgerStore) AddNodeBalance(nodeID store.NodeID, credit *big.Int) error {
	accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
	return s.db.Update(func(txn *badger.Txn) error {
		var account store.Account
		balanceKey := []byte(fmt.Sprintf("vip:trial:%s", nodeID))
		if err := getItem(txn, accountKey, &account); err == badger.ErrKeyNotFound {
			// No spendable account, use the trial account
		} else if err == nil {
			balanceKey = []byte(fmt.Sprintf("vip:balance:%s", account))
		} else {
			return err
		}
		var balance store.Balance
		if err := getItem(txn, balanceKey, &balance); err == badger.ErrKeyNotFound {
			nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
			if !hasKey(txn, nodeKey) {
				return store.ErrUnregisteredNode
			}
			// No balance = empty balance
		} else if err != nil {
			return err
		}
		balance.Credit.Add(&balance.Credit, credit)

		return setItem(txn, balanceKey, &balance)
	})
}

// GetAccountBalance returns an account's balance.
func (s *badgerStore) GetAccountBalance(account store.Account) (store.Balance, error) {
	balanceKey := []byte(fmt.Sprintf("vip:balance:%s", account))
	var r store.Balance
	err := s.db.View(func(txn *badger.Txn) error {
		return getItem(txn, balanceKey, &r)
	})
	if err == badger.ErrKeyNotFound {
		// Default to empty balance
		return r, nil
	}
	return r, err
}

// AddNodeBalance adds credit to an account balance. (Can be negative)
func (s *badgerStore) AddAccountBalance(account store.Account, credit *big.Int) error {
	return s.db.Update(func(txn *badger.Txn) error {
		balanceKey := []byte(fmt.Sprintf("vip:balance:%s", account))
		var balance store.Balance
		if err := getItem(txn, balanceKey, &balance); err == badger.ErrKeyNotFound {
			// No balance = empty balance
		} else if err != nil {
			return err
		}
		balance.Credit.Add(&balance.Credit, credit)
		balance.Account = account

		return setItem(txn, balanceKey, &balance)
	})
}

// AddAccountNode authorizes a nodeID to be a spender of an account's
// balance. This should migrate any existing node's balance credit to the
// account.
func (s *badgerStore) AddAccountNode(account store.Account, nodeID store.NodeID) error {
	return s.db.Update(func(txn *badger.Txn) error {
		// Check nodeID
		nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
		if !hasKey(txn, nodeKey) {
			return store.ErrUnregisteredNode
		}

		// Load trial balance to migrate
		var trialBalance store.Balance
		trialKey := []byte(fmt.Sprintf("vip:trial:%s", nodeID))
		if err := getItem(txn, trialKey, &trialBalance); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Load existing balance
		balanceKey := []byte(fmt.Sprintf("vip:balance:%s", account))
		var balance store.Balance
		if err := getItem(txn, balanceKey, &balance); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Authorize node
		accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
		if err := setItem(txn, accountKey, &account); err != nil {
			return err
		}

		// Merge trial and save
		balance.Credit.Add(&balance.Credit, &trialBalance.Credit)
		balance.Account = account
		if err := setItem(txn, balanceKey, &balance); err != nil {
			return err
		}
		if err := txn.Delete(trialKey); err != nil {
			return err
		}
		return nil
	})
}

// IsAccountNode returns nil if node is a valid spender of the given
// account.
func (s *badgerStore) IsAccountNode(account store.Account, nodeID store.NodeID) error {
	accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
	var nodeAccount store.Account
	return s.db.View(func(txn *badger.Txn) error {
		if err := getItem(txn, accountKey, &nodeAccount); err == badger.ErrKeyNotFound {
			return store.ErrNotAuthorized
		} else if err != nil {
			return err
		}
		if nodeAccount != account {
			return store.ErrNotAuthorized
		}
		return nil
	})
}

// GetSpenders returns the authorized nodeIDs for this account, these are
// nodes that were added to accounts through AddAccountNode.
func (s *badgerStore) GetAccountNodes(account store.Account) ([]store.NodeID, error) {
	// FIXME: This could be more efficient if we had an account -> nodeID index
	var r []store.NodeID
	if err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte("vip:account:")
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			r = append(r, store.NodeID(key[len(prefix):]))
		}
		return nil
	}); err == badger.ErrKeyNotFound {
		return r, nil
	} else if err != nil {
		return nil, err
	}
	return r, nil
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
			err = it.Item().Value(func(val []byte) error {
				return gob.NewDecoder(bytes.NewReader(val)).Decode(&n)
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
	if limit <= 0 || len(r) < limit {
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
		return getItem(txn, key, &r)
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
		return setItem(txn, key, &n)
	})
}

func (s *badgerStore) NodePeers(nodeID store.NodeID) ([]store.Node, error) {
	peersKey := []byte(fmt.Sprintf("vip:peers:%s", nodeID))
	var r []store.Node
	err := s.db.View(func(txn *badger.Txn) error {
		var nodePeers map[store.NodeID]time.Time
		if err := getItem(txn, peersKey, &nodePeers); err == badger.ErrKeyNotFound {
			// No peers
			nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
			if !hasKey(txn, nodeKey) {
				return store.ErrUnregisteredNode
			}
			return nil
		} else if err != nil {
			return err
		}
		r = make([]store.Node, 0, len(nodePeers))
		for peerID, _ := range nodePeers {
			var peerNode store.Node
			if err := getItem(txn, []byte(fmt.Sprintf("vip:node:%s", peerID)), &peerNode); err == badger.ErrKeyNotFound {
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

func (s *badgerStore) UpdateNodePeers(nodeID store.NodeID, peers []string, blockNumber uint64) (inactive []store.NodeID, err error) {
	nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
	peersKey := []byte(fmt.Sprintf("vip:peers:%s", nodeID))
	now := time.Now()
	var node store.Node
	nodePeers := map[store.NodeID]time.Time{}
	err = s.db.Update(func(txn *badger.Txn) error {
		// Update this node's LastSeen
		if err := getItem(txn, nodeKey, &node); err == badger.ErrKeyNotFound {
			return store.ErrUnregisteredNode
		} else if err != nil {
			return err
		}

		node.LastSeen = now
		node.BlockNumber = blockNumber
		if err := setItem(txn, nodeKey, &node); err != nil {
			return err
		}

		// Update peers
		if err := getItem(txn, peersKey, &nodePeers); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		numUpdated := 0
		for _, peerID := range peers {
			// Only update peers we already know about
			if hasKey(txn, []byte(fmt.Sprintf("vip:node:%s", peerID))) {
				nodePeers[store.NodeID(peerID)] = now
				numUpdated += 1
			}
		}

		if numUpdated == len(nodePeers) {
			return setItem(txn, peersKey, &nodePeers)
		}

		inactiveDeadline := now.Add(-store.ExpireInterval)
		for nodeID, timestamp := range nodePeers {
			if timestamp.Before(inactiveDeadline) {
				continue
			}
			delete(nodePeers, nodeID)
			inactive = append(inactive, nodeID)
		}
		return setItem(txn, peersKey, &nodePeers)
	})
	return
}

// Stats returns aggregate statistics about the store state.
func (s *badgerStore) Stats() (*store.Stats, error) {
	stats := store.Stats{}

	err := s.db.View(func(txn *badger.Txn) error {
		var n store.Node
		if err := loopItem(txn, []byte("vip:node:"), &n, func() error {
			stats.CountNode(n)
			return nil
		}); err != nil {
			return err
		}

		var b store.Balance
		countBalance := func() error { stats.CountBalance(b); return nil }
		if err := loopItem(txn, []byte("vip:balance:"), &b, countBalance); err != nil {
			return err
		}
		if err := loopItem(txn, []byte("vip:trial:"), &b, countBalance); err != nil {
			return err
		}

		return nil
	})

	return &stats, err
}
