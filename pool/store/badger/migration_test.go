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

	if err = db.View(func(txn *badger.Txn) error {
		return checkVersion(txn, dbVersion)
	}); err != nil {
		t.Fatal(err)
	}

	testNonceKey := []byte("vip:nonce:testtesttest")
	if err = db.Update(func(txn *badger.Txn) error {
		if err := setVersion(txn, 1); err != nil {
			return err
		}
		if err := setItem(txn, testNonceKey, 42); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Confirm that migration nukes vip:nonce
	if err := MigrateLatest(db, "testdb"); err != nil {
		t.Fatal(err)
	}

	if err = db.View(func(txn *badger.Txn) error {
		if err := checkVersion(txn, 2); err != nil {
			t.Error(err)
		}
		if hasKey(txn, testNonceKey) {
			t.Errorf("nonce key is present after migation")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
