package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/host"
	"github.com/vipnode/vipnode/jsonrpc2"
	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/pool"
)

func runHost(options Options) error {
	remoteNode, err := findRPC(options.Host.RPC)
	if err != nil {
		return err
	}
	privkey, err := findNodeKey(options.Host.NodeKey)
	if err != nil {
		return ErrExplain{err, "Failed to find node private key. Use --nodekey to specify the correct path."}
	}
	// Confirm that nodeID matches the private key
	nodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	if err := matchEnode(options.Host.NodeURI, nodeID); err != nil {
		return err
	}
	remoteEnode, err := remoteNode.Enode(context.Background())
	if err != nil {
		return err
	}
	if err := matchEnode(remoteEnode, nodeID); err != nil {
		return err
	}

	if options.Host.Payout == "" {
		logger.Warning("No --payout address provided, will not receive pool payments.")
	}

	h := host.New(options.Host.NodeURI, remoteNode, options.Host.Payout)

	if options.Host.Pool == ":memory:" {
		// Support for in-memory pool. This is primarily for testing.
		logger.Infof("Starting in-memory vipnode pool.")
		p := pool.New()
		rpcPool := &jsonrpc2.Local{}
		if err := rpcPool.Server.Register("vipnode_", p); err != nil {
			return err
		}
		remotePool := pool.Remote(rpcPool, privkey)
		if err := h.Start(remotePool); err != nil {
			return err
		}
		return h.Wait()
	}

	// Dial host to pool
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	poolCodec, err := ws.WebSocketDial(ctx, options.Host.Pool)
	cancel()
	if err != nil {
		return ErrExplain{err, "Failed to connect to the pool RPC API."}
	}
	logger.Infof("Connected to vipnode pool: %s", options.Host.Pool)

	rpcServer := &jsonrpc2.Server{}
	if err := rpcServer.RegisterMethod("vipnode_whitelist", h, "Whitelist"); err != nil {
		return err
	}
	rpcPool := jsonrpc2.Remote{
		Client: &jsonrpc2.Client{},
		Server: rpcServer,
		Codec:  poolCodec,
	}

	errChan := make(chan error)
	go func() {
		errChan <- rpcPool.Serve()
	}()
	remotePool := pool.Remote(&rpcPool, privkey)
	if err := h.Start(remotePool); err != nil {
		return err
	}
	go func() {
		errChan <- h.Wait()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for _ = range sigCh {
			logger.Info("Shutting down...")
			h.Stop()
		}
	}()

	return <-errChan

}
