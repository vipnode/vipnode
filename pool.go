package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/OpenPeeDeeP/xdg"
	"github.com/dgraph-io/badger"
	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

// findDataDir returns a valid data dir, will create it if it doesn't
// exist.
func findDataDir(overridePath string) (string, error) {
	path := overridePath
	if path == "" {
		path = xdg.New("vipnode", "pool").DataHome()
	}
	err := os.MkdirAll(path, 0700)
	return path, err
}

func runPool(options Options) error {
	var storeDriver store.Store
	switch options.Pool.Store {
	case "memory":
		storeDriver = store.MemoryStore()
		defer storeDriver.Close()
	case "persist":
		fallthrough
	case "badger":
		dir, err := findDataDir(options.Pool.DataDir)
		if err != nil {
			return err
		}
		badgerOpts := badger.DefaultOptions
		badgerOpts.Dir = dir
		badgerOpts.ValueDir = dir
		storeDriver, err := badger.Open(badgerOpts)
		if err != nil {
			return err
		}
		defer storeDriver.Close()
		logger.Infof("Persistent store using badger backend: %s", dir)
	default:
		return errors.New("storage driver not implemented")
	}
	p := pool.New(storeDriver)
	p.Version = fmt.Sprintf("vipnode/pool/%s", Version)
	handler := &server{
		ws: &ws.Upgrader{},
	}
	if err := handler.Register("vipnode_", p); err != nil {
		return err
	}
	logger.Infof("Starting pool (version %s), listening on: %s", Version, options.Pool.Bind)
	// TODO: Add TLS support using autocert
	return http.ListenAndServe(options.Pool.Bind, handler)
}
