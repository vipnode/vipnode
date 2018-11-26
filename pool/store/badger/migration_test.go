package badger

import (
	"testing"

	"github.com/dgraph-io/badger"
)

func TestMigration(t *testing.T) {
	store, err := OpenTemp()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.db

	err = db.View(func(txn *badger.Txn) error {
		version, err := getVersion(txn)
		if err != nil {
			return err
		}
		if version != dbVersion {
			t.Errorf("incorrect version on fresh database: %d", version)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
