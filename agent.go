package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/agent"
	"github.com/vipnode/vipnode/jsonrpc2"
	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/pool/store/memory"
)

var minUpdateInterval = time.Second * 5
var maxUpdateInterval = store.ExpireInterval
var maxBlockNumberDrift uint64 = 3
var defaultPoolURI string = "wss://pool.vipnode.org/"

// runAgent configures and starts a the agent. It blocks until the agent is
// stopped or fails. runAgent also takes care of wrapping known arounds with
// ErrExplain for user-friendliness.
func runAgent(options Options) error {
	runner := agentRunner{}
	if err := runner.LoadAgent(options); err != nil {
		return err
	}
	if err := runner.LoadPool(options); err != nil {
		return err
	}
	err := runner.Run()
	if timeoutErr, ok := err.(interface{ Timeout() bool }); ok && timeoutErr.Timeout() {
		return ErrExplainRetry{ErrExplain{err, "Agent timed out while coordinating with the pool. Hopefully this is a transient error."}}
	}
	if agentErr, ok := err.(agent.AgentPoolError); !ok {
		return err
	} else if jsonErr, ok := agentErr.Cause().(interface{ ErrorCode() int }); ok && jsonErr.ErrorCode() == jsonrpc2.ErrCodeMethodNotFound {
		return ErrExplain{err, `Pool is missing a required RPC method, make sure your agent version is compatible with the pool version.`}
	}
	return err
}

// agentRunner is a stateful object for loading configuration and running the
// agent along with other necessary services.
// Used for testing.
type agentRunner struct {
	Agent         *agent.Agent
	PrivateKey    *ecdsa.PrivateKey
	RemotePool    pool.Pool
	RemoteService *jsonrpc2.Remote
	pool          *pool.VipnodePool // Only exists in :memory: mode:w
}

func (runner *agentRunner) LoadAgent(options Options) error {
	remoteNode, err := findRPC(options.Agent.RPC)
	if err != nil {
		return err
	}

	if runner.PrivateKey == nil {
		// Node key is required for signing requests from the NodeID to avoid forgery.
		privkey, err := findNodeKey(options.Agent.NodeKey)
		if err != nil {
			return ErrExplain{err, "Failed to find node private key. RPC requests are signed by this key to avoid forgery. Use --nodekey to specify the correct path."}
		}
		runner.PrivateKey = privkey
	}

	// Confirm that nodeID matches the private key
	nodeID := discv5.PubkeyID(&runner.PrivateKey.PublicKey).String()
	remoteEnode, err := remoteNode.Enode(context.Background())
	if err != nil {
		if err.Error() == "Network is disabled or not yet up." {
			// Parity error when warming up, or when --mode=offline
			return ErrExplainRetry{ErrExplain{err, "Node is not ready yet. Make sure the node is set to online mode."}}
		}
		return err
	}
	if err := matchEnode(remoteEnode, nodeID); err != nil {
		return err
	}

	updateInterval, err := time.ParseDuration(options.Agent.UpdateInterval)
	if err != nil {
		return ErrExplain{err, `Failed to parse the agent --update-interval value. Try using a value like "1m" or "60s".`}
	}
	if updateInterval >= maxUpdateInterval {
		return ErrExplain{
			errors.New("update interval too large"),
			`Updates must be sent frequently enough so that the pool does not write off the node as inactive. Try a shorter --update-interval value, like "60s".`,
		}
	} else if updateInterval <= minUpdateInterval {
		return ErrExplain{
			errors.New("update interval too small"),
			`Please don't flood the pool with updates. Try a longer --update-interval value, like "60s".`,
		}
	}

	a := &agent.Agent{
		EthNode:        remoteNode,
		Payout:         options.Agent.Payout,
		Version:        fmt.Sprintf("vipnode/agent/%s", Version),
		UpdateInterval: updateInterval,
	}
	runner.Agent = a
	if options.Agent.NodeURI != "" {
		if err := matchEnode(options.Agent.NodeURI, nodeID); err != nil {
			return err
		}
		a.NodeURI = options.Agent.NodeURI
	} else {
		// This will populate all the fields except the host, which is fine
		// because the pool will use the connection's ip as the host.
		a.NodeURI = remoteEnode
	}
	a.PoolMessageCallback = func(msg string) {
		logger.Alertf("Message from pool: %s", msg)
	}

	drifting := false
	a.BlockNumberCallback = func(blockNumber uint64, latestBlockNumber uint64) {
		var delta uint64
		if latestBlockNumber > blockNumber {
			delta = latestBlockNumber - blockNumber
		}
		if delta == 0 {
			// Either the pool doesn't know of the latest block, or we're in sync.
			return
		}
		if delta > maxBlockNumberDrift {
			if !drifting {
				logger.Warningf("Local node is behind the latest known block number by %d blocks: %d", delta, latestBlockNumber)
				drifting = true
			}
		} else if drifting {
			logger.Infof("Local node caught up to within %d of latest known block number: %d", delta, latestBlockNumber)
			drifting = false
		}
	}
	return nil
}

func (runner *agentRunner) LoadPool(options Options) error {
	if runner.Agent == nil {
		return errors.New("runner: Agent must be set")
	}

	poolURI := options.Agent.Args.Coordinator
	if poolURI == "" {
		poolURI = defaultPoolURI
	} else if poolURI == ":memory:" {
		// Support for in-memory pool. This is primarily for testing.
		logger.Infof("Using an in-memory vipnode pool.")
		p := pool.New(memory.New(), nil)
		p.Version = fmt.Sprintf("vipnode/memory-pool/%s", Version)
		runner.pool = p
		rpcPool := &jsonrpc2.Local{}
		if err := rpcPool.Server.Register("vipnode_", p); err != nil {
			return err
		}
		runner.RemotePool = pool.Remote(rpcPool, runner.PrivateKey)
		return nil
	}

	uri, err := url.Parse(poolURI)
	if err != nil {
		return ErrExplain{err, `Failed to parse the pool coordinator URI. It should look something like: "wss://pool.vipnode.org/"`}
	}

	if uri.Scheme == "enode" {
		staticPool := &pool.StaticPool{}
		if err := staticPool.AddNode(poolURI); err != nil {
			return err
		}
		logger.Infof("Using a static vipnode as vipnode pool: %s", poolURI)
		runner.RemotePool = staticPool
		return nil
	}

	// We support multiple kinds of transports for the pool RPC API. Websocket
	// is preferred, but we allow HTTP for low power scenarios like mobile
	// devices.
	if uri.Scheme == "" {
		// No scheme provided, use websockets.
		uri.Scheme = "wss"
	}

	logger.Infof("Connecting to remote pool: %s", uri.String())

	switch uri.Scheme {
	case "ws", "wss":
		// Register reverse-directional RPC calls available on the host
		var reverseService agent.Service = runner.Agent
		rpcServer := &jsonrpc2.Server{}
		if err := rpcServer.RegisterMethod("vipnode_whitelist", reverseService, "Whitelist"); err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		poolCodec, err := ws.WebSocketDial(ctx, uri.String())
		cancel()
		if err != nil {
			return ErrExplainRetry{ErrExplain{err, fmt.Sprintf("Failed to connect to the pool RPC API: %q", uri.String())}}
		}

		rpcPool := &jsonrpc2.Remote{
			Server: rpcServer,
			Client: &jsonrpc2.Client{},
			Codec:  poolCodec,
		}
		runner.RemotePool = pool.Remote(rpcPool, runner.PrivateKey)
		runner.RemoteService = rpcPool
	case "http", "https":
		if runner.Agent.EthNode.UserAgent().IsFullNode {
			revisedURI := uri
			revisedURI.Scheme = "wss"
			return ErrExplain{
				errors.New("full node coordination not supported over HTTP"),
				"Full nodes must connect over the websocket protocol in order to support bidirectional RPC that is required to coordinate host peers. Try using: " + revisedURI.String(),
			}
		}
		rpcPool := &jsonrpc2.HTTPService{
			Endpoint: uri.String(),
		}
		runner.RemotePool = pool.Remote(rpcPool, runner.PrivateKey)
	default:
		return ErrExplain{
			errors.New("invalid pool coordinator URI scheme"),
			`Pool coordinator URI must be one of: ws, wss, http, or https. For example: "wss://pool.vipnode.org/"`,
		}
	}

	return nil
}

// Run will block until the agent and remote pool service finish or fail.
func (runner *agentRunner) Run() error {
	if runner.Agent == nil {
		return errors.New("runner: Agent must be set")
	}
	if runner.RemotePool == nil {
		return errors.New("runner: Pool must be set")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for range sigCh {
			logger.Info("Shutting down...")
			runner.Agent.Stop()
		}
	}()

	errChan := make(chan error)
	if runner.RemoteService != nil {
		// RemoteService must be serving before we Start the agent, in case the
		// Pool requires us to whitelist hosts on connect.
		go func() {
			errChan <- runner.RemoteService.Serve()
		}()
	}

	if err := runner.Agent.Start(runner.RemotePool); err != nil {
		return err
	}

	go func() {
		errChan <- runner.Agent.Wait()
	}()
	return <-errChan
}
