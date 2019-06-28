package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"math/big"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/agent"
	"github.com/vipnode/vipnode/internal/fakecluster"
	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/internal/pretty"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/balance"
	"github.com/vipnode/vipnode/pool/status"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/pool/store/memory"
)

func TestPoolHostClient(t *testing.T) {
	privkey := keygen.HardcodedKeyIdx(t, 0)
	payout := ""

	db := memory.New()
	p := pool.New(db, nil)
	rpcPool2Host, rpcHost2Pool := jsonrpc2.ServePipe()
	defer rpcPool2Host.Close()
	defer rpcHost2Pool.Close()
	if err := rpcPool2Host.Server.Register("vipnode_", p); err != nil {
		t.Fatalf("failed to register vipnode_ rpc for pool: %s", err)
	}
	hostNodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	hostNode := fakenode.Node(hostNodeID)
	hostNodeURI := fmt.Sprintf("enode://%s@127.0.0.1:30303", hostNodeID)
	h := agent.Agent{EthNode: hostNode, Payout: payout}
	if err := rpcHost2Pool.Server.RegisterMethod("vipnode_whitelist", &h, "Whitelist"); err != nil {
		t.Fatalf("failed to register vipnode_ rpc for host: %s", err)
	}
	h.NodeURI = hostNodeURI
	hostPool := pool.Remote(rpcHost2Pool, privkey)

	if err := h.Start(hostPool); err != nil {
		t.Fatalf("failed to start host: %s", err)
	}
	defer h.Stop()

	if stats, err := db.Stats(); err != nil {
		t.Fatal(err)
	} else if stats.NumActiveHosts != 1 {
		t.Errorf("wrong number of active hosts: %+v", stats)
	}

	rpcPool2Client, rpcClient2Pool := jsonrpc2.ServePipe()
	defer rpcPool2Client.Close()
	defer rpcClient2Pool.Close()
	rpcPool2Client.Server.Register("vipnode_", p)

	clientPrivkey := keygen.HardcodedKeyIdx(t, 1)
	clientNodeID := discv5.PubkeyID(&clientPrivkey.PublicKey).String()
	clientNode := fakenode.Node(clientNodeID)
	clientNode.IsFullNode = false
	c := agent.Agent{
		EthNode:  clientNode,
		NumHosts: 3,
	}
	clientPool := pool.Remote(rpcClient2Pool, clientPrivkey)
	if err := c.Start(clientPool); err != nil {
		t.Fatalf("failed to start client: %s", err)
	}

	want := fakenode.Calls{
		fakenode.Call("AddTrustedPeer", clientNodeID),
	}
	if got := hostNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("hostNode.Calls:\n  got %q;\n want %q", got, want)
	}

	want = fakenode.Calls{fakenode.Call("ConnectPeer", hostNodeURI)}
	if got := clientNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("clientNode.Calls:\n  got %q;\n want %q", got, want)
	}

	c.Stop()
	if err := c.Wait(); err != nil {
		t.Errorf("client.Stop() failed: %s", err)
	}
	want = fakenode.Calls{
		fakenode.Call("ConnectPeer", hostNodeURI),
	}
	if got := clientNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("clientNode.Calls:\n  got %q;\n want %q", got, want)
	}

	// Start again!
	if err := c.Start(clientPool); err != nil {
		t.Fatalf("failed to start client again: %s", err)
	}
	want = fakenode.Calls{
		fakenode.Call("ConnectPeer", hostNodeURI),
		fakenode.Call("ConnectPeer", hostNodeURI),
	}
	if got := clientNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("clientNode.Calls:\n  got %q;\n want %q", got, want)
	}

	want = fakenode.Calls{
		fakenode.Call("AddTrustedPeer", clientNodeID),
		fakenode.Call("AddTrustedPeer", clientNodeID),
	}
	if got := hostNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("hostNode.Calls:\n  got %q;\n want %q", got, want)
	}
}

func TestCloseHost(t *testing.T) {
	privkey := keygen.HardcodedKeyIdx(t, 0)
	payout := ""

	p := pool.New(memory.New(), nil)
	c1, c2 := net.Pipe()
	rpcPool2Host := &jsonrpc2.Remote{
		Codec:  jsonrpc2.IOCodec(c1),
		Client: &jsonrpc2.Client{},
		Server: &jsonrpc2.Server{},
	}
	rpcHost2Pool := &jsonrpc2.Remote{
		Codec:  jsonrpc2.IOCodec(c2),
		Client: &jsonrpc2.Client{},
		Server: &jsonrpc2.Server{},
	}
	if err := rpcPool2Host.Server.Register("vipnode_", p); err != nil {
		t.Fatalf("failed to register vipnode_ rpc for pool: %s", err)
	}

	errChan := make(chan error)
	go func() {
		errChan <- rpcPool2Host.Serve()
	}()
	go func() {
		errChan <- rpcHost2Pool.Serve()
	}()

	hostNodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	hostNode := fakenode.Node(hostNodeID)
	hostNodeURI := fmt.Sprintf("enode://%s@127.0.0.1", hostNodeID)
	h := agent.Agent{EthNode: hostNode, Payout: payout}
	if err := rpcHost2Pool.Server.RegisterMethod("vipnode_whitelist", &h, "Whitelist"); err != nil {
		t.Fatalf("failed to register vipnode_ rpc for host: %s", err)
	}
	h.NodeURI = hostNodeURI
	hostPool := pool.Remote(rpcHost2Pool, privkey)

	if err := h.Start(hostPool); err != nil {
		t.Fatalf("failed to start host: %s", err)
	}
	rpcHost2Pool.Close()
	rpcPool2Host.Close()

	if err := <-errChan; err != io.EOF && err != io.ErrClosedPipe {
		t.Errorf("unexpected error: %s (%T)", err, err)
	}
	h.Stop()
	h.Wait()
}

func TestPoolHostConnectPeers(t *testing.T) {
	hostKeys := []*ecdsa.PrivateKey{}
	clientKeys := []*ecdsa.PrivateKey{}

	// We limit to 3 hosts for now, because clients autoconnect to 3 hosts and
	// it's easier to check 100% connectivity deterministically (otherwise
	// hosts are chosen randomly)
	i := 0
	for ; i < 3; i++ {
		hostKeys = append(hostKeys, keygen.HardcodedKeyIdx(t, i))
	}

	for ; i < 5; i++ {
		clientKeys = append(clientKeys, keygen.HardcodedKeyIdx(t, i))
	}

	poolStore := memory.New()
	balanceManager := balance.PayPerInterval(poolStore, time.Nanosecond*1, big.NewInt(10))
	p := pool.New(poolStore, balanceManager)

	cluster, err := fakecluster.New(p, hostKeys, clientKeys)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(cluster.Hosts), len(hostKeys); got != want {
		t.Errorf("wrong number of hosts: got %d; want %d", got, want)
	}

	if got, want := cluster.Pool.NumRemotes(), len(hostKeys); got != want {
		t.Errorf("wrong number of remotes: got %d; want %d", got, want)
	}

	stats, err := cluster.Pool.Store.Stats()
	if err != nil {
		t.Error(err)
	}

	if stats.NumActiveHosts != len(hostKeys) {
		t.Errorf("wrong stats: %+v", stats)
	}

	var blockNumber uint64
	numHosts := len(hostKeys)
	{
		// Connect a host to all the peers (numHosts = peers + 1, because it includes self)
		host := cluster.Hosts[0]
		if peers, err := host.Node.Peers(context.Background()); err != nil {
			t.Fatal(err)
		} else if len(peers) > 0 {
			t.Errorf("host has unexpected peers: %s", peers)
		}

		// Request peers
		if err := host.AddPeers(context.Background(), host.RemotePool, 10); err != nil {
			t.Fatal(err)
		}

		if peers, err := host.Node.Peers(context.Background()); err != nil {
			t.Fatal(err)
		} else if got, want := len(peers), numHosts-1; got != want {
			t.Errorf("host has wrong number of peers: got %d; want %d", got, want)
		}

		numCalls := 0
		for _, call := range host.Node.Calls {
			if call.Method != "ConnectPeer" {
				continue
			}
			numCalls++
		}
		if got, want := numCalls, numHosts-1; got != want {
			t.Errorf("wrong number of ConnectPeer calls: got %d; want %d", got, want)
		}

		if blockNumber, err = host.BlockNumber(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	{
		// Check a peer
		host := cluster.Hosts[1]
		peer := cluster.Hosts[0]
		if !host.Node.Calls.Has("AddTrustedPeer", peer.Node.NodeID) {
			t.Errorf("peer host calls missing AddTrustedPeer for %s, got:\n%s", peer.Node.NodeID, host.Node.Calls)
		}
		// We can't check the Peers of the host here because it's a FakeNode so
		// it does not actually initiate a connection on Connect.
	}

	// Run a couple of updates to make sure everyone who wants to connect has
	// connected.
	if err := cluster.Update(); err != nil {
		t.Fatal(err)
	}
	if err := cluster.Update(); err != nil {
		t.Fatal(err)
	}

	poolStatus := status.PoolStatus{Store: cluster.Pool.Store}
	s, err := poolStatus.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ActiveHosts) != len(cluster.Hosts) {
		t.Errorf("PoolStatus: wrong number of hosts: %d", len(s.ActiveHosts))
	}

	var firstHost status.Host
	for _, h := range s.ActiveHosts {
		if strings.HasPrefix(cluster.Hosts[0].Node.NodeID, h.ShortID) {
			firstHost = h
			break
		}
	}
	if firstHost.ShortID == "" {
		t.Errorf("PoolStatus: missing first host")
	} else if got, want := firstHost.BlockNumber, blockNumber; got != want {
		t.Errorf("PoolStatus: wrong block number for first host: got %d; want %d", got, want)
	} else if got, want := firstHost.NumPeers, len(cluster.Hosts)-1; got != want {
		t.Errorf("PoolStatus: wrong number of peers for first host: got %d; want %d", got, want)
	}

	{
		// Check balances
		total := new(big.Int)
		for _, a := range cluster.Hosts {
			nodeID := a.Node.NodeID
			b, err := poolStore.GetNodeBalance(store.NodeID(nodeID))
			if err != nil {
				t.Fatal(err)
			} else if b.Credit.Cmp(new(big.Int)) <= 0 {
				// Hosts should have a positive balance
				t.Errorf("wrong balance for host %s: %d", pretty.Abbrev(nodeID), &b.Credit)
			}
			t.Logf("balance for host %s: %d", pretty.Abbrev(nodeID), &b.Credit)
			total = total.Add(total, &b.Credit)
		}

		if total.Cmp(new(big.Int)) <= 0 {
			t.Errorf("Balance check: Total hosts balance should be positive: %d", total)
		}

		for _, a := range cluster.Clients {
			nodeID := a.Node.NodeID
			b, err := poolStore.GetNodeBalance(store.NodeID(nodeID))
			if err != nil {
				t.Fatal(err)
			} else if b.Credit.Cmp(new(big.Int)) >= 0 {
				// Clients should have a negative balance
				t.Errorf("wrong balance for client %s: %d", pretty.Abbrev(nodeID), &b.Credit)
			}
			t.Logf("balance for client %s: %d", pretty.Abbrev(nodeID), &b.Credit)
			total = total.Add(total, &b.Credit)
		}

		if total.Cmp(new(big.Int)) != 0 {
			t.Errorf("Balance check: Total balance should be zero: %d", total)
		}
	}

	err = cluster.Close()
	if err != nil {
		t.Error(err)
	}

	if got, want := cluster.Pool.NumRemotes(), 0; got != want {
		t.Errorf("wrong number of remotes: got %d; want %d", got, want)
	}
}
