package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/agent"
	"github.com/vipnode/vipnode/jsonrpc2"
	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store/memory"
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

	h := agent.Agent{
		EthNode: remoteNode,
		Payout:  options.Host.Payout,
		Version: fmt.Sprintf("vipnode/host/%s", Version),
	}
	if options.Host.NodeURI != "" {
		if err := matchEnode(options.Host.NodeURI, nodeID); err != nil {
			return err
		}
		h.NodeURI = options.Host.NodeURI
	} else {
		// This will populate all the fields except the host, which is fine
		// because the pool will use the connection's ip as the host.
		h.NodeURI = remoteEnode
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for _ = range sigCh {
			logger.Info("Shutting down...")
			h.Stop()
		}
	}()

	if options.Host.Pool == ":memory:" {
		// Support for in-memory pool. This is primarily for testing.
		logger.Infof("Starting in-memory vipnode pool.")
		p := pool.New(memory.New(), nil)
		p.Version = fmt.Sprintf("vipnode/pool/%s", Version)
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
		return ErrExplainRetry{ErrExplain{err, "Failed to connect to the pool RPC API."}}
	}
	logger.Infof("Connected to vipnode pool: %s", options.Host.Pool)

	// Register reverse-directional RPC calls available on the host
	rpcServer := &jsonrpc2.Server{}
	if err := rpcServer.RegisterMethod("vipnode_whitelist", &h, "Whitelist"); err != nil {
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
		if jsonrpc2.IsErrorCode(err, jsonrpc2.ErrCodeMethodNotFound, jsonrpc2.ErrCodeInvalidParams) {
			err = ErrExplain{err, fmt.Sprintf(`Missing a required RPC method. Make sure your vipnode binary is up to date. (Current version: %s)`, Version)}
		}
		return err
	}
	go func() {
		errChan <- h.Wait()
	}()

	return <-errChan

}
