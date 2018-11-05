package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/OpenPeeDeeP/xdg"
	"github.com/dgraph-io/badger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/vipnode/vipnode/ethnode"
	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/balance"
	"github.com/vipnode/vipnode/pool/payment"
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

	balanceStore := store.BalanceStore(storeDriver)
	if options.Pool.ContractAddr != "" {
		// Payment contract implements NodeBalanceStore used by the balance
		// manager, but with contract awareness.
		contractPath, err := url.Parse(options.Pool.ContractAddr)
		if err != nil {
			return err
		}

		contractAddr := common.HexToAddress(contractPath.Hostname())
		network := contractPath.Scheme
		ethclient, err := ethclient.Dial(options.Pool.ContractRPC)
		if err != nil {
			return err
		}

		// Confirm we're on the right network
		gotNetwork, err := ethclient.NetworkID(context.Background())
		if err != nil {
			return err
		}
		if networkID := ethnode.NetworkID(int(gotNetwork.Int64())); !networkID.Is(network) {
			return ErrExplain{
				errors.New("ethereum network mismatch for payment contract"),
				fmt.Sprintf("Contract is on %q while the Contact RPC is a %q node. Please provide a Contract RPC on the same network as the contract.", network, networkID),
			}
		}

		contract, err := payment.ContractPayment(storeDriver, contractAddr, ethclient)
		if err != nil {
			return err
		}
		balanceStore = contract
	}

	balanceManager := balance.PayPerInterval(
		balanceStore,
		time.Minute*1,    // Interval
		big.NewInt(1000), // Credit per interval
	)

	p := pool.New(storeDriver, balanceManager)
	p.Version = fmt.Sprintf("vipnode/pool/%s", Version)
	handler := &server{
		ws:     &ws.Upgrader{},
		header: http.Header{},
	}
	if options.Pool.AllowOrigin != "" {
		handler.header.Set("Access-Control-Allow-Origin", options.Pool.AllowOrigin)
	}

	if err := handler.Register("vipnode_", p); err != nil {
		return err
	}

	// Pool payment management API (optional)
	payment := &payment.PaymentService{
		NonceStore:   storeDriver,
		AccountStore: storeDriver,
		BalanceStore: balanceStore, // Proxy smart contract store if available
	}
	if err := handler.Register("pool_", payment); err != nil {
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
