package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/OpenPeeDeeP/xdg"
	"github.com/dgraph-io/badger"
	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
	badgerStore "github.com/vipnode/vipnode/pool/store/badger"
	"golang.org/x/crypto/acme/autocert"
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
		storeDriver, err = badgerStore.Open(badgerOpts)
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
	if options.Pool.TLSHost != "" {
		if !strings.HasSuffix(":443", options.Pool.Bind) {
			logger.Warningf("Ignoring --bind value (%q) because it's not 443 and --tlshost is set.", options.Pool.Bind)
		}
		logger.Infof("Starting pool (version %s), acquiring ACME certificate and listening on: https://%s", Version, options.Pool.TLSHost)
		err := http.Serve(autocert.NewListener(options.Pool.TLSHost), handler)
		if strings.HasSuffix(err.Error(), "bind: permission denied") {
			err = ErrExplain{err, "Hosting a pool with autocert requires CAP_NET_BIND_SERVICE capability permission to bind on low-numbered ports. See: https://superuser.com/questions/710253/allow-non-root-process-to-bind-to-port-80-and-443/892391"}
		}
		return err
	}
	logger.Infof("Starting pool (version %s), listening on: %s", Version, options.Pool.Bind)
	return http.ListenAndServe(options.Pool.Bind, handler)
}
