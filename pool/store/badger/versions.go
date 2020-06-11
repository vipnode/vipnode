package badger

import (
	"github.com/dgraph-io/badger/v2"
)

const dbVersion = 2

var migrations = [dbVersion]MigrationStep{
	// Version 0 -> 1
	func(txn *badger.Txn) error {
		if err := checkVersion(txn, 0); err != nil {
			return err
		}
		return setVersion(txn, 1)
	},

	// Version 1 -> 2 (added TTL to nonces, so we just nuke the table)
	func(txn *badger.Txn) error {
		if err := checkVersion(txn, 1); err != nil {
			return err
		}

		prefix := []byte("vip:nonce:")
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			txn.Delete(key)
		}

		return setVersion(txn, 2)
	},
}
