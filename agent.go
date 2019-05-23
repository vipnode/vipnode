package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/agent"
	"github.com/vipnode/vipnode/jsonrpc2"
	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store/memory"
)

func runAgent(options Options) error {
	remoteNode, err := findRPC(options.Agent.RPC)
	if err != nil {
		return err
	}

	// Node key is required for signing requests from the NodeID to avoid forgery.
	privkey, err := findNodeKey(options.Agent.NodeKey)
	if err != nil {
		return ErrExplain{err, "Failed to find node private key. RPC requests are signed by this key to avoid forgery. Use --nodekey to specify the correct path."}
	}
	// Confirm that nodeID matches the private key
	nodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	remoteEnode, err := remoteNode.Enode(context.Background())
	if err != nil {
		return err
	}
	if err := matchEnode(remoteEnode, nodeID); err != nil {
		return err
	}

	a := &agent.Agent{
		EthNode: remoteNode,
		Payout:  options.Agent.Payout,
		Version: fmt.Sprintf("vipnode/agent/%s", Version),
	}
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for _ = range sigCh {
			logger.Info("Shutting down...")
			a.Stop()
		}
	}()

	poolURI := options.Agent.Args.Coordinator
	uri, err := url.Parse(poolURI)
	if err != nil {
		return ErrExplain{err, `Failed to parse the pool coordinator URI. It should look something like: "wss://pool.vipnode.org/"`}
	}

	// If we want, we can connect to another vipnode agent directly, bypassing the need for a pool.
	if uri.Scheme == "enode" {
		staticPool := &pool.StaticPool{}
		if err := staticPool.AddNode(poolURI); err != nil {
			return err
		}
		logger.Infof("Connecting to a static node (bypassing pool): %s", poolURI)
		if err := a.Start(staticPool); err != nil {
			return err
		}
		return a.Wait()
	}

	if poolURI == ":memory:" {
		// Support for in-memory pool. This is primarily for testing.
		logger.Infof("Starting in-memory vipnode pool.")
		p := pool.New(memory.New(), nil)
		p.Version = fmt.Sprintf("vipnode/memory-pool/%s", Version)
		rpcPool := &jsonrpc2.Local{}
		if err := rpcPool.Server.Register("vipnode_", p); err != nil {
			return err
		}
		remotePool := pool.Remote(rpcPool, privkey)
		if err := a.Start(remotePool); err != nil {
			return err
		}
		return a.Wait()
	}

	errChan := make(chan error)

	// We support multiple kinds of transports for the pool RPC API. Websocket
	// is preferred, but we allow HTTP for low power scenarios like mobile
	// devices.
	var rpcPool jsonrpc2.Service
	scheme := uri.Scheme
	if scheme == "" {
		// No scheme provided, use websockets
		scheme = "wss"
	}
	switch scheme {
	case "ws", "wss":
		// Register reverse-directional RPC calls available on the host
		var reverseService agent.Service = a
		rpcServer := &jsonrpc2.Server{}
		if err := rpcServer.RegisterMethod("vipnode_whitelist", reverseService, "Whitelist"); err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		poolCodec, err := ws.WebSocketDial(ctx, uri.String())
		cancel()
		if err != nil {
			return ErrExplainRetry{ErrExplain{err, "Failed to connect to the pool RPC API."}}
		}

		remote := &jsonrpc2.Remote{
			Server: rpcServer,
			Client: &jsonrpc2.Client{},
			Codec:  poolCodec,
		}
		go func() {
			errChan <- remote.Serve()
		}()
		rpcPool = remote
	case "http", "https":
		if remoteNode.UserAgent().IsFullNode {
			revisedURI := uri
			revisedURI.Scheme = "wss"
			return ErrExplain{
				errors.New("full node coordination not supported over HTTP"),
				"Full nodes must connect over the websocket protocol in order to support bidirectional RPC that is required to coordinate host peers. Try using: " + revisedURI.String(),
			}
		}
		rpcPool = &jsonrpc2.HTTPService{
			Endpoint: uri.String(),
		}
	default:
		return ErrExplain{
			errors.New("invalid pool coordinator URI scheme"),
			`Pool coordinator URI must be one of: ws, wss, http, or https. For example: "wss://pool.vipnode.org/"`,
		}
	}

	remotePool := pool.Remote(rpcPool, privkey)
	if err := a.Start(remotePool); err != nil {
		if jsonrpc2.IsErrorCode(err, jsonrpc2.ErrCodeMethodNotFound, jsonrpc2.ErrCodeInvalidParams) {
			err = ErrExplain{err, fmt.Sprintf(`Missing a required RPC method. Make sure your vipnode binary is up to date. (Current version: %s)`, Version)}
		}
		return err
	}
	go func() {
		errChan <- a.Wait()
	}()

	return <-errChan

}
