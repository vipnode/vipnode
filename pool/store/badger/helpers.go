package badger

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"time"

	"github.com/dgraph-io/badger/v2"
)

func hasKey(txn *badger.Txn, key []byte) bool {
	_, err := txn.Get(key)
	return err == nil
}

func getItem(txn *badger.Txn, key []byte, into interface{}) error {
	item, err := txn.Get(key)
	if err != nil {
		return err
	}
	return item.Value(func(val []byte) error {
		return gob.NewDecoder(bytes.NewReader(val)).Decode(into)
	})
}

func setItem(txn *badger.Txn, key []byte, val interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val); err != nil {
		return err
	}
	return txn.Set(key, buf.Bytes())
}

func setExpiringItem(txn *badger.Txn, key []byte, val interface{}, expire time.Duration) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val); err != nil {
		return err

	}
	entry := badger.NewEntry(key, buf.Bytes()).WithTTL(expire)
	return txn.SetEntry(entry)
}

// loopItem iterates over a prefix and decodes into `into`
func loopItem(txn *badger.Txn, prefix []byte, into interface{}, callback func() error) error {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		if err := it.Item().Value(func(val []byte) error {
			// Reset `into` before decoding into it (can this be done better?)
			p := reflect.ValueOf(into).Elem()
			p.Set(reflect.Zero(p.Type()))

			return gob.NewDecoder(bytes.NewReader(val)).Decode(into)
		}); err != nil {
			return err
		}
		if err := callback(); err != nil {
			return err
		}
	}
	return nil
}
