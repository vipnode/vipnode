package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"net"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/client"
	"github.com/vipnode/vipnode/host"
	"github.com/vipnode/vipnode/internal/fakecluster"
	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"
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
	h := host.New(hostNode, payout)
	if err := rpcHost2Pool.Server.RegisterMethod("vipnode_whitelist", h, "Whitelist"); err != nil {
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
	c := client.New(clientNode)
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
		fakenode.Call("DisconnectPeer", hostNodeID),
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
		fakenode.Call("DisconnectPeer", hostNodeID),
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
	h := host.New(hostNode, payout)
	if err := rpcHost2Pool.Server.RegisterMethod("vipnode_whitelist", h, "Whitelist"); err != nil {
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

	for i := 0; i < 4; i++ {
		hostKeys = append(hostKeys, keygen.HardcodedKeyIdx(t, i))
	}

	cluster, err := fakecluster.New(hostKeys, clientKeys)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(cluster.Hosts), 4; got != want {
		t.Errorf("wrong number of hosts: got %d; want %d", got, want)
	}

	if got, want := cluster.Pool.NumRemotes(), 4; got != want {
		t.Errorf("wrong number of remotes: got %d; want %d", got, want)
	}

	stats, err := cluster.Pool.Store.Stats()
	if err != nil {
		t.Error(err)
	}

	if stats.NumActiveHosts != 4 {
		t.Errorf("wrong stats: %+v", stats)
	}

	numHosts := len(hostKeys)
	{
		// Connect a host to all the peers (numHosts = peers + 1, because it includes self)
		host := cluster.Hosts[0]
		if peers, err := host.Node.Peers(context.Background()); err != nil {
			t.Fatal(err)
		} else if len(peers) > 0 {
			t.Errorf("host has unexpected peers: %s", peers)
		}
		hostPool := pool.Remote(host.Out, host.Key)

		if err := host.ConnectPeers(hostPool, numHosts); err != nil {
			t.Error(err)
		}

		if peers, err := host.Node.Peers(context.Background()); err != nil {
			t.Fatal(err)
		} else if got, want := len(peers), numHosts-1; got != want {
			t.Errorf("host has wrong number of peers: got %d; want %d", got, want)
		}

		numCalls := 0
		for _, call := range host.Node.Calls {
			if call.Method != "ConnectPeer" {
				t.Errorf("unexpected call to host: %s", call)
				continue
			}
			numCalls++
		}
		if got, want := numCalls, numHosts-1; got != want {
			t.Errorf("wrong number of ConnectPeer calls: got %d; want %d", got, want)
		}
	}

	{
		// Check a peer
		host := cluster.Hosts[1]
		peer := cluster.Hosts[0]
		want := fakenode.Calls{fakenode.Call("AddTrustedPeer", peer.Node.NodeID)}
		if got := host.Node.Calls; !reflect.DeepEqual(got, want) {
			t.Errorf("peer host calls do not match:\n  got: %s;\n want: %s", got, want)
		}
		// We can't check the Peers of the host here because it's a FakeNode so
		// it does not actually initiate a connection on Connect.
	}

	err = cluster.Close()
	if err != nil {
		t.Error(err)
	}

	if got, want := cluster.Pool.NumRemotes(), 0; got != want {
		t.Errorf("wrong number of remotes: got %d; want %d", got, want)
	}
}
