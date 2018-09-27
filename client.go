package main

import (
	"context"
	"net/url"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/client"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"

	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
)

func runClient(options Options) error {
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
	c := client.New(remoteNode)
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
	if err := c.Start(p); err != nil {
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
		}
	}()

	return <-errChan

}
