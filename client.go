package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/v2/agent"
	"github.com/vipnode/vipnode/v2/jsonrpc2"
	"github.com/vipnode/vipnode/v2/pool"

	ws "github.com/vipnode/vipnode/v2/jsonrpc2/ws/gorilla"
)

// defaultClientNode is the value used when `vipnode client` is run without additional args.
//var defaultClientNode string = "enode://19b5013d24243a659bda7f1df13933bb05820ab6c3ebf6b5e0854848b97e1f7e308f703466e72486c5bc7fe8ed402eb62f6303418e05d330a5df80738ac974f6@163.172.138.100:30303?discport=30301"
var defaultClientNode string = "https://pool.vipnode.org/"

func runClient(options Options) error {
	// Load input data from cli params
	remoteNode, err := findRPC(options.Client.RPC)
	if err != nil {
		return err
	}

	poolURI := options.Client.Args.VIPNode
	if poolURI == "" {
		poolURI = defaultClientNode
	}
	uri, err := url.Parse(poolURI)
	if err != nil {
		return err
	}

	errChan := make(chan error)
	c := agent.Agent{
		EthNode: remoteNode,
		Version: fmt.Sprintf("vipnode/client/%s", Version),
	}
	c.PoolMessageCallback = func(msg string) {
		logger.Alertf("Message from pool: %s", msg)
	}

	// If we want, we can connect to a vipnode host directly, bypassing the need for a pool.
	if uri.Scheme == "enode" {
		staticPool := &pool.StaticPool{}
		if err := staticPool.AddNode(poolURI); err != nil {
			return err
		}
		logger.Infof("Connecting to a static node (bypassing pool): %s", poolURI)
		if err := c.Start(staticPool); err != nil {
			return err
		}
		return c.Wait()
	}

	// Node key is required for signing requests from the NodeID to avoid forgery.
	privkey, err := findNodeKey(options.Client.NodeKey)
	if err != nil {
		return ErrExplain{err, "Failed to find node private key. Use --nodekey to specify the correct path."}
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

	logger.Infof("Connecting to pool: %s", poolURI)

	// The pool hosts an RPC API over websocket and HTTP. The host uses websocket by default
	// for persistent connectivity, but for the client it's probably best to stick with HTTP.
	// Especially if the client could be a mobile device, it's probably more battery-friendly.
	var rpcPool jsonrpc2.Service
	if uri.Scheme == "ws" || uri.Scheme == "wss" {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		poolCodec, err := ws.WebSocketDial(ctx, uri.String())
		cancel()
		if err != nil {
			return ErrExplain{err, "Failed to connect to the pool RPC API."}
		}
		remote := &jsonrpc2.Remote{
			Codec: poolCodec,
		}
		go func() {
			errChan <- remote.Serve()
		}()
		rpcPool = remote
	} else {
		// Assume HTTP by default
		rpcPool = &jsonrpc2.HTTPService{
			Endpoint: uri.String(),
		}
	}

	p := pool.Remote(rpcPool, privkey)

	// Send the vipnode_client handshake and start sending regular updates.
	if err := c.Start(p); err != nil {
		if jsonrpc2.IsErrorCode(err, jsonrpc2.ErrCodeMethodNotFound, jsonrpc2.ErrCodeInvalidParams) {
			err = ErrExplain{err, fmt.Sprintf(`Missing a required RPC method. Make sure your vipnode client is up to date. (Current version: %s)`, Version)}
		}
		return err
	}
	logger.Info("Connected.")
	go func() {
		errChan <- c.Wait()
	}()

	// Register c.Stop() on ctrl+c signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for _ = range sigCh {
			logger.Info("Shutting down...")
			c.Stop()
			// TODO: More aggressive abort if c.Stop() times out?
		}
	}()

	return <-errChan

}
