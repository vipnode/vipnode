package badger

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/dgraph-io/badger"
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
	return txn.SetWithTTL(key, buf.Bytes(), expire)
}

func loopItem(txn *badger.Txn, prefix []byte, into interface{}, callback func() error) error {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		if err := it.Item().Value(func(val []byte) error {
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
