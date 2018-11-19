package badger

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/vipnode/vipnode/pool/store"
)

const dbVersion = 1

// MigrationError is returned when the database is opened with an outdated
// version and migation fails.
type MigrationError struct {
	OldVersion int
	NewVersion int
	Path       string
	Cause      error
}

func (err MigrationError) Error() string {
	return fmt.Sprintf("badgerdb migration error: Failed to migrate from version %d to %d at path %q: %s", err.OldVersion, err.NewVersion, err.Path, err.Cause)
}

// TODO: Set reasonable expiration values?

type peers map[store.NodeID]time.Time

// Open returns a store.Store implementation using Badger as the storage
// driver. The store should be (*badgerStore).Close()'d after use.
func Open(opts badger.Options) (*badgerStore, error) {
	// TODO: Add versioning and automatic migration support
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	s := &badgerStore{db: db}
	// Attempt to migrate
	var oldVersion int
	err = s.db.Update(func(txn *badger.Txn) error {
		versionKey := []byte("vip:version")
		if len(db.Tables()) == 0 {
			// New database, set current version and continue
			return s.setItem(txn, versionKey, dbVersion)
		}

		if err := s.getItem(txn, versionKey, &oldVersion); err != nil {
			return MigrationError{
				OldVersion: oldVersion,
				NewVersion: dbVersion,
				Path:       opts.Dir,
				Cause:      err,
			}
		}

		// TODO: Implement migration
		if oldVersion != dbVersion {
			return MigrationError{
				OldVersion: oldVersion,
				NewVersion: dbVersion,
				Path:       opts.Dir,
				Cause:      errors.New("migration not implemented yet"),
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
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
	return item.Value(func(val []byte) error {
		return gob.NewDecoder(bytes.NewReader(val)).Decode(into)
	})
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
	// add TTL to nonce store. (Use SetWithTTL)
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

		return s.setItem(txn, key, &nonce)
	})
}

// GetNodeBalance returns the current account balance for a node.
func (s *badgerStore) GetNodeBalance(nodeID store.NodeID) (store.Balance, error) {
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

// AddNodeBalance adds some credit amount to a node's account balance. (Can be negative)
// If only a node is provided which doesn't have an account registered to
// it, it should retain a balance, such as through temporary trial accounts
// that get migrated later.
func (s *badgerStore) AddNodeBalance(nodeID store.NodeID, credit *big.Int) error {
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
		balance.Credit.Add(&balance.Credit, credit)

		return s.setItem(txn, balanceKey, &balance)
	})
}

// GetAccountBalance returns an account's balance.
func (s *badgerStore) GetAccountBalance(account store.Account) (store.Balance, error) {
	balanceKey := []byte(fmt.Sprintf("vip:balance:%s", account))
	var r store.Balance
	err := s.db.View(func(txn *badger.Txn) error {
		return s.getItem(txn, balanceKey, &r)
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
		if err := s.getItem(txn, balanceKey, &balance); err == badger.ErrKeyNotFound {
			// No balance = empty balance
		} else if err != nil {
			return err
		}
		balance.Credit.Add(&balance.Credit, credit)
		balance.Account = account

		return s.setItem(txn, balanceKey, &balance)
	})
}

// AddAccountNode authorizes a nodeID to be a spender of an account's
// balance. This should migrate any existing node's balance credit to the
// account.
func (s *badgerStore) AddAccountNode(account store.Account, nodeID store.NodeID) error {
	return s.db.Update(func(txn *badger.Txn) error {
		// Check nodeID
		nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
		if !s.hasKey(txn, nodeKey) {
			return store.ErrUnregisteredNode
		}

		// Load trial balance to migrate
		var trialBalance store.Balance
		trialKey := []byte(fmt.Sprintf("vip:trial:%s", nodeID))
		if err := s.getItem(txn, trialKey, &trialBalance); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Load existing balance
		balanceKey := []byte(fmt.Sprintf("vip:balance:%s", account))
		var balance store.Balance
		if err := s.getItem(txn, balanceKey, &balance); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Authorize node
		accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
		if err := s.setItem(txn, accountKey, &account); err != nil {
			return err
		}

		// Merge trial and save
		balance.Credit.Add(&balance.Credit, &trialBalance.Credit)
		balance.Account = account
		if err := s.setItem(txn, balanceKey, &balance); err != nil {
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
		if err := s.getItem(txn, accountKey, &nodeAccount); err == badger.ErrKeyNotFound {
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
		return s.setItem(txn, key, &n)
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
		if err := s.setItem(txn, nodeKey, &node); err != nil {
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
			return s.setItem(txn, peersKey, &nodePeers)
		}

		inactiveDeadline := now.Add(-store.ExpireInterval)
		for nodeID, timestamp := range nodePeers {
			if timestamp.Before(inactiveDeadline) {
				continue
			}
			delete(nodePeers, nodeID)
			inactive = append(inactive, nodeID)
		}
		return s.setItem(txn, peersKey, &nodePeers)
	})
	return
}
