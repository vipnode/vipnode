package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/OpenPeeDeeP/xdg"
	"github.com/dgraph-io/badger/v2"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/vipnode/vipnode/v2/ethnode"
	"github.com/vipnode/vipnode/v2/internal/pretty"
	ws "github.com/vipnode/vipnode/v2/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/v2/pool"
	"github.com/vipnode/vipnode/v2/pool/balance"
	"github.com/vipnode/vipnode/v2/pool/payment"
	"github.com/vipnode/vipnode/v2/pool/status"
	"github.com/vipnode/vipnode/v2/pool/store"
	badgerStore "github.com/vipnode/vipnode/v2/pool/store/badger"
	memoryStore "github.com/vipnode/vipnode/v2/pool/store/memory"
	"golang.org/x/crypto/acme/autocert"
)

const healthTimeout = time.Second * 5

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
		storeDriver = memoryStore.New()
		defer storeDriver.Close()
	case "persist":
		fallthrough
	case "badger":
		dir, err := findDataDir(options.Pool.DataDir)
		if err != nil {
			return err
		}
		badgerOpts := badger.DefaultOptions(dir)
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
	var settleHandler payment.SettleHandler
	var depositGetter func(ctx context.Context) (*big.Int, error)
	if options.Pool.Contract.Addr != "" {
		// Payment contract implements NodeBalanceStore used by the balance
		// manager, but with contract awareness.
		contractPath, err := url.Parse(options.Pool.Contract.Addr)
		if err != nil {
			return err
		}

		contractAddr := common.HexToAddress(contractPath.Hostname())
		network := contractPath.Scheme
		ethclient, err := ethclient.Dial(options.Pool.Contract.RPC)
		if err != nil {
			return err
		}

		// Confirm we're on the right network.
		// Note: The contract network/node can be independent of the --restrict-network setting
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

		var transactOpts *bind.TransactOpts
		if options.Pool.Contract.KeyStore != "" {
			transactOpts, err = unlockTransactor(options.Pool.Contract.KeyStore)
			if err != nil {
				return ErrExplain{
					err,
					"Failed to unlock the keystore for the contract operator wallet. Make sure the path is correct and the decryption password is set in the `KEYSTORE_PASSPHRASE` environment variable.",
				}
			}
		}

		if transactOpts == nil {
			logger.Warningf("Contract payment starting in read-only mode because --contract-keystore was not set. Withdraw and settlement attempts will fail.")
		}

		contract, err := payment.ContractPayment(storeDriver, contractAddr, ethclient, transactOpts)
		if err != nil {
			if err, ok := err.(payment.AddressMismatchError); ok {
				return ErrExplain{
					err,
					"Contract keystore must match the wallet of the contract operator. Make sure you're providing the correct keystore.",
				}
			}
			return err
		}
		balanceStore = contract
		settleHandler = contract.OpSettle

		depositGetter = func(ctx context.Context) (*big.Int, error) {
			r, err := ethclient.PendingBalanceAt(ctx, contractAddr)
			if err != nil {
				// Try again in case the connection dropped
				logger.Warningf("PoolStatus: ethclient.PendingBalanceAt failed, retrying: %s", err)
				r, err = ethclient.PendingBalanceAt(ctx, contractAddr)
			}
			if err != nil {
				logger.Errorf("PoolStatus: ethclient.PendingBalanceAt failed twice: %s", err)
			}
			return r, err
		}
	}

	// Setup balance manager
	creditPerInterval, err := pretty.ParseEther(options.Pool.Contract.Price)
	if err != nil {
		return fmt.Errorf("failed to parse contract price: %s", err)
	}
	balanceManager := balance.PayPerInterval(
		balanceStore,
		time.Minute*1, // Interval
		creditPerInterval,
	)

	if options.Pool.Contract.MinBalance != "off" {
		minBalance, err := pretty.ParseEther(options.Pool.Contract.MinBalance)
		if err != nil {
			return fmt.Errorf("failed to parse contract minimum balance: %s", err)
		}

		balanceManager.MinBalance = minBalance
	}

	// Setup welcome message template
	var welcomeTmpl *template.Template
	if welcomeMsg := options.Pool.Contract.Welcome; welcomeMsg != "" {
		welcomeTmpl, err = template.New("vipnode_welcome").Parse(welcomeMsg)
		if err != nil {
			return err
		}
	}

	var networkID ethnode.NetworkID
	if options.Pool.RestrictNetwork != "" {
		networkID = ethnode.ParseNetwork(options.Pool.RestrictNetwork)
		if networkID == ethnode.UnknownNetwork {
			return ErrExplain{errors.New("unknown network"), `Failed to parse network ID provided to --restrict-network`}
		}
	}

	p := pool.New(storeDriver, balanceManager)
	p.MaxRequestHosts = options.Pool.MaxRequestHosts
	p.Version = fmt.Sprintf("vipnode/pool/%s", Version)

	if welcomeTmpl != nil {
		p.ClientMessager = func(nodeID string) string {
			var buf bytes.Buffer
			err := welcomeTmpl.Execute(&buf, struct {
				NodeID string
			}{
				NodeID: nodeID,
			})
			if err != nil {
				// TODO: Should this be recoverable? What conditions would cause this?
				logger.Errorf("ClientMessager failed: %s", err)
			}
			return buf.String()
		}
	}

	p.RestrictNetwork = networkID
	p.BlockNumberProvider = func(network ethnode.NetworkID) (uint64, error) {
		// TODO: Does it make sense also fetching this from an external service? Eg: Infura's eth_blockNumber?
		if network != p.RestrictNetwork {
			return 0, fmt.Errorf("block number provider does not support network: %s", network)
		}
		stats, err := p.Store.Stats()
		if err != nil {
			return 0, err
		}
		return stats.LatestBlockNumber, nil
	}

	if options.Pool.InvalidNode != "" {
		re, err := regexp.Compile(options.Pool.InvalidNode)
		if err != nil {
			return ErrExplain{err, `Failed to compile --invalid-nodes regular expression`}
		}
		p.CheckPeer = func(peer ethnode.PeerInfo) bool {
			if re.MatchString(peer.Name) {
				logger.Debugf("Invalid peer matched %q: %s", peer.Name, peer.EnodeURI())
				return false
			}
			return true
		}
	}

	handler := &server{
		ws:           &ws.Upgrader{},
		header:       http.Header{},
		onDisconnect: p.CloseRemote,
	}
	if options.Pool.AllowOrigin != "" {
		handler.header.Set("Access-Control-Allow-Origin", options.Pool.AllowOrigin)
	}

	if err := handler.Register("vipnode_", p, "connect", "disconnect", "ping", "update", "peer", "client", "host"); err != nil {
		return err
	}

	// Pool payment management API (optional)
	payment := &payment.PaymentService{
		NonceStore:   storeDriver,
		AccountStore: storeDriver,
		BalanceStore: balanceStore, // Proxy smart contract store if available

		WithdrawFee: func(amount *big.Int) *big.Int {
			// TODO: Adjust fee dynamically based on gas price?
			fee := big.NewInt(2500000000000000) // 0.0025 ETH
			return amount.Sub(amount, fee)
		},
		WithdrawMin: big.NewInt(5000000000000000), // 0.005 ETH
		Settle:      settleHandler,
	}
	if err := handler.Register("pool_", payment); err != nil {
		return err
	}

	// Pool status dashboard API
	dashboard := &status.PoolStatus{
		Store:           storeDriver,
		GetTotalDeposit: depositGetter,
		TimeStarted:     time.Now(),
		Version:         Version,
		CacheDuration:   time.Minute * 1,
	}
	if err := handler.Register("pool_", dashboard); err != nil {
		return err
	}

	// PoolStatus-based healthcheck for our HTTP handler
	handler.healthCheck = func(w io.Writer) error {
		ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
		status, err := dashboard.Status(ctx)
		defer cancel()
		if err != nil {
			return err
		}
		return json.NewEncoder(w).Encode(status)
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

func unlockTransactor(keystorePath string) (*bind.TransactOpts, error) {
	pw := os.Getenv("KEYSTORE_PASSPHRASE")
	r, err := os.Open(keystorePath)
	if err != nil {
		return nil, err
	}
	return bind.NewTransactor(r, pw)
}
