package badger

import (
	"io/ioutil"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/options"
	"github.com/vipnode/vipnode/pool/store"
)

type badgerTemp struct {
	*badgerStore
	Dir string
}

func (s badgerTemp) Close() error {
	defer os.RemoveAll(s.Dir)
	return s.badgerStore.Close()
}

// badgerTesting is a wrapper that retains but clears the db on Close().
type badgerTesting struct {
	*badgerTemp
}

func (s badgerTesting) Close() error {
	// Seems to be faster to just delete all keys between tests than to make a
	// fresh db each time.
	opt := badger.DefaultIteratorOptions
	return s.badgerTemp.badgerStore.db.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(opt)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			if err := txn.Delete(it.Item().Key()); err != nil {
				return err
			}
		}
		return nil
	})
}

func OpenTemp() (*badgerTemp, error) {
	dir, err := ioutil.TempDir("", "vipnodetest")
	if err != nil {
		return nil, err
	}

	// Memory-only settings from here: https://github.com/dgraph-io/badger/issues/377#issuecomment-424422144
	opts := badger.LSMOnlyOptions
	opts.TableLoadingMode = options.LoadToRAM
	opts.ValueLogLoadingMode = options.MemoryMap
	opts.Dir = dir
	opts.ValueDir = dir

	s, err := Open(opts)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return &badgerTemp{s, dir}, nil
}

func TestBadgerHelpers(t *testing.T) {
	store, err := OpenTemp()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	type Foo struct {
		Amount big.Int
	}
	a := Foo{
		Amount: *big.NewInt(42),
	}
	if err := store.db.Update(func(txn *badger.Txn) error {
		return setItem(txn, []byte("someprefix:a"), &a)
	}); err != nil {
		t.Fatal(err)
	}
	aa := Foo{}
	if err := store.db.View(func(txn *badger.Txn) error {
		return getItem(txn, []byte("someprefix:a"), &aa)
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, aa) {
		t.Errorf("got: %v; want %v", aa, a)
	}

	numItems := 0
	aaa := Foo{}
	if err := store.db.View(func(txn *badger.Txn) error {
		return loopItem(txn, []byte("someprefix:"), &aaa, func() error {
			numItems += 1
			if !reflect.DeepEqual(a, aaa) {
				t.Errorf("got: %v; want %v", aaa, a)
			}
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}

	if numItems != 1 {
		t.Error("loopItem failed to find prefix")
	}
}

func TestBadgerStore(t *testing.T) {
	s, err := OpenTemp()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	t.Run("BadgerStore", func(t *testing.T) {
		store.TestSuite(t, func() store.Store {
			return badgerTesting{s}
		})
	})
}
