package badger

import (
	"errors"

	"github.com/dgraph-io/badger"
)

const dbVersion = 1

var migrations = [dbVersion]MigrationStep{
	// Version 0 -> 1
	func(txn *badger.Txn) error {
		version, err := getVersion(txn)
		if err != nil {
			return err
		}
		if version != 0 {
			return errors.New("wrong version for migration")
		}
		return setVersion(txn, 1)
	},
}
