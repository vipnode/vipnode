package store

import (
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"testing"
	"time"
)

// TestSuite runs a suite of tests against a store implementation.
func TestSuite(t *testing.T, newStore func() Store) {
	accounts := []Account{
		Account("abcd"),
		Account("efgh"),
	}

	t.Helper()
	t.Run("Nonce", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		nodeID := "abc"

		oldNonce := time.Now().Add(-2 * time.Hour).UnixNano()
		if err := s.CheckAndSaveNonce(nodeID, oldNonce); err != ErrInvalidNonce {
			t.Errorf("missing invalid nonce error: %s", err)
		}

		nonce := time.Now().UnixNano()
		if err := s.CheckAndSaveNonce(nodeID, nonce); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if err := s.CheckAndSaveNonce(nodeID, nonce+1); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if err := s.CheckAndSaveNonce(nodeID, nonce-1); err != ErrInvalidNonce {
			t.Errorf("missing invalid nonce error: %s", err)
		}
		if err := s.CheckAndSaveNonce(nodeID, nonce); err != ErrInvalidNonce {
			t.Errorf("missing invalid nonce error: %s", err)
		}
		if err := s.CheckAndSaveNonce("def", nonce+100); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	})

	t.Run("Node", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		node := makeNode(0)
		emptynode := Node{}
		if _, err := s.GetNode(node.ID); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}
		if err := s.SetNode(emptynode); err != ErrMalformedNode {
			t.Errorf("expected malformed error, got: %s", err)
		}
		if err := s.SetNode(node); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if r, err := s.GetNode(node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if r.ID != node.ID {
			t.Errorf("returned wrong node: %v", r)
		}
	})

	t.Run("Balance", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		node := makeNode(0)
		othernode := makeNode(1)

		// Unregistered
		if err := s.AddNodeBalance(node.ID, big.NewInt(42)); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}
		if _, err := s.GetNodeBalance(node.ID); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}

		// Init node
		if err := s.SetNode(node); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		// Test balance adding
		if err := s.AddNodeBalance(node.ID, big.NewInt(42)); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if err := s.AddNodeBalance(node.ID, big.NewInt(3)); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if b, err := s.GetNodeBalance(node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if b.Credit.Cmp(big.NewInt(45)) != 0 {
			t.Errorf("wrong balance: %v", b)
		}

		// Test subtracting and negative
		if err := s.AddNodeBalance(node.ID, big.NewInt(-50)); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if b, err := s.GetNodeBalance(node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if b.Credit.Cmp(big.NewInt(-5)) != 0 {
			t.Errorf("wrong balance: %v", b)
		}

		if b, err := s.GetNodeBalance(othernode.ID); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		} else if b.Credit.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("returned non-empty balance: %v", b)
		}

		gotStats, err := s.Stats()
		if err != nil {
			t.Error(err)
		}
		wantStats := &Stats{
			NumTotalClients:  1,
			TotalCredit:      *big.NewInt(-5),
			NumTrialBalances: 1,
		}
		wantStats.activeSince = gotStats.activeSince
		if !reflect.DeepEqual(gotStats, wantStats) {
			t.Errorf("wrong stats:\n got: %+v;\nwant: %+v", gotStats, wantStats)
		}
	})

	t.Run("NodePeers", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		nodes := makeNodes(0, 10)
		node := nodes[0]
		var blockNumber uint64 = 42

		// Unregistered
		if _, err := s.NodePeers(node.ID); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}
		if _, err := s.UpdateNodePeers(node.ID, []string{"def"}, blockNumber); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}
		// Init node
		if err := s.SetNode(node); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		// Test peers
		if peers, err := s.NodePeers(node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if len(peers) != 0 {
			t.Errorf("unexpected peers: %v", peers)
		}

		before := time.Now()

		// peer1 is not a known node, so it will be ignored
		{
			peers := nodes[1:2].IDs()
			if inactive, err := s.UpdateNodePeers(node.ID, peers, blockNumber); err != nil {
				t.Errorf("unexpected error: %s", err)
			} else if len(inactive) != 0 {
				t.Errorf("unexpected inactive peers:\n    peers: %s\n inactive: %s", peers, inactive)
			}
		}

		// Check BlockNumber and LastSet being set during update.
		if n, err := s.GetNode(node.ID); err != nil {
			t.Errorf("unexpected GetNode error: %s", err)
		} else if n.BlockNumber != blockNumber {
			t.Errorf("wrong block number: got %d; want %d", n.BlockNumber, blockNumber)
		} else if n.LastSeen.Before(before) {
			t.Errorf("node's last seen is not updated: %s", n.LastSeen)
		}

		// Check active vs inactive peers
		active := nodes[1:5]
		if err := addActiveNodes(s, active...); err != nil {
			t.Errorf("unexpected error adding active nodes: %s", err)
		}

		// One inactive node
		inactive := nodes[5:6]
		if err := s.SetNode(inactive[0]); err != nil {
			t.Errorf("unexpected error adding inactive node: %s", err)
		}

		// Confirm that inactive node is actually inactive
		if n, err := s.GetNode(inactive[0].ID); err != nil {
			t.Errorf("unexpected GetNode error: %s", err)
		} else if before.Add(-ExpireInterval).Before(n.LastSeen) {
			t.Errorf("inactive node's LastSeen is too recent: %s", n.LastSeen)
		}

		{
			// Submit both active and inactive peers
			peers := append(active.IDs(), inactive.IDs()...)
			if gotInactive, err := s.UpdateNodePeers(node.ID, peers, blockNumber); err != nil {
				t.Errorf("unexpected error: %s", err)
			} else if got, want := NodeIDs(gotInactive).Strings(), inactive.IDs(); !reflect.DeepEqual(got, want) {
				t.Errorf("wrong inactive peers: %d\n got: %s\nwant: %s", len(got), got, want)
			}
			if peers, err := s.NodePeers(node.ID); err != nil {
				t.Errorf("unexpected error: %s", err)
			} else if peerIDs := Nodes(peers).IDs(); !reflect.DeepEqual(peerIDs, active.IDs()) {
				t.Errorf("wrong active peers:\n got: %s\nwant: %s", peerIDs, active.IDs())
			}
		}
	})

	t.Run("Node", func(t *testing.T) {
		nodes := makeNodes(0, 10)
		s := newStore()
		defer s.Close()

		if hosts, err := s.ActiveHosts("", 3); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if len(hosts) != 0 {
			t.Errorf("unexpected hosts: %v", hosts)
		}

		// Add some hosts
		now := time.Now()
		for i, node := range nodes {
			node := Node{
				ID:          node.ID,
				IsHost:      i%2 == 0, // Half hosts, interlaced to check for insertion order bugs
				BlockNumber: uint64(100 + i),
			}
			if i > 5 {
				node.LastSeen = now
			}
			if err := s.SetNode(node); err != nil {
				t.Error(err)
			}
		}
		if hosts, err := s.ActiveHosts("", 10); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if got, want := Nodes(hosts).IDs(), []string{
			nodes[6].ID.String(),
			nodes[8].ID.String(),
		}; !reflect.DeepEqual(got, want) {
			t.Errorf("got: %v; want: %v", got, want)
		}

		if hosts, err := s.ActiveHosts("", 1); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if len(hosts) != 1 {
			t.Errorf("wrong number of hosts: %d", len(hosts))
		} else if hosts[0].BlockNumber < 100 {
			t.Errorf("wrong block number: %d", hosts[0].BlockNumber)
		}

		gotStats, err := s.Stats()
		if err != nil {
			t.Error(err)
		}
		wantStats := &Stats{
			NumTotalHosts:     5,
			NumActiveHosts:    2,
			NumTotalClients:   5,
			NumActiveClients:  2,
			LatestBlockNumber: 109,
		}
		wantStats.activeSince = gotStats.activeSince
		if !reflect.DeepEqual(gotStats, wantStats) {
			t.Errorf("wrong stats:\n got: %+v;\nwant: %+v", gotStats, wantStats)
		}
	})

	t.Run("Spender", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		nodes := makeNodes(0, 10)
		node := nodes[0]
		if err := s.SetNode(node); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		account := accounts[0]

		if err := s.IsAccountNode(account, node.ID); err != ErrNotAuthorized {
			t.Errorf("expected ErrNotAuthorized, got: %s", err)
		}

		if err := s.AddAccountNode(account, node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		if b, err := s.GetNodeBalance(node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if b.Account != account {
			t.Errorf("invalid balance account: %q", b.Account)
		}

		// Adding again should have no effect
		if err := s.AddAccountNode(account, node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		// Adding another account/node should have no effect
		if err := s.SetNode(nodes[1]); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if err := s.AddAccountNode(accounts[1], nodes[1].ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		if spenders, err := s.GetAccountNodes(account); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if !reflect.DeepEqual(spenders, []NodeID{node.ID}) {
			t.Errorf("invalid spenders: %q", spenders)
		}
	})

	t.Run("SpenderBalance", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		node := makeNode(0)
		if err := s.SetNode(node); err != nil {
			t.Error(err)
		}

		if err := s.AddNodeBalance(node.ID, big.NewInt(42)); err != nil {
			t.Error(err)
		}
		if b, err := s.GetNodeBalance(node.ID); err != err {
			t.Error(err)
		} else if b.Credit.Cmp(big.NewInt(42)) != 0 {
			t.Errorf("invalid balance credit: %d", &b.Credit)
		}

		node2 := makeNode(1)
		if err := s.SetNode(node2); err != nil {
			t.Error(err)
		}
		account := accounts[0]
		if err := s.AddAccountNode(account, node2.ID); err != nil {
			t.Error(err)
		}
		if err := s.AddNodeBalance(node2.ID, big.NewInt(69)); err != nil {
			t.Error(err)
		}
		if b, err := s.GetNodeBalance(node2.ID); err != nil {
			t.Error(err)
		} else if b.Credit.Cmp(big.NewInt(69)) != 0 {
			t.Errorf("invalid balance credit: %d", &b.Credit)
		}

		if err := s.AddAccountNode(account, node.ID); err != nil {
			t.Error(err)
		}
		if b, err := s.GetNodeBalance(node2.ID); err != nil {
			t.Error(err)
		} else if b.Credit.Cmp(big.NewInt(42+69)) != 0 {
			t.Errorf("invalid balance credit: %d", &b.Credit)
		} else if b.Account != account {
			t.Errorf("invalid account: %s", b.Account)
		}
		if b, err := s.GetNodeBalance(node.ID); err != nil {
			t.Error(err)
		} else if b.Credit.Cmp(big.NewInt(42+69)) != 0 {
			t.Errorf("invalid balance credit: %d", &b.Credit)
		} else if b.Account != account {
			t.Errorf("invalid account: %s", b.Account)
		}

	})
}

type Nodes []Node

func (nodes Nodes) IDs() []string {
	r := make([]string, 0, len(nodes))
	for _, n := range nodes {
		r = append(r, n.ID.String())
	}
	sort.Strings(r)
	return r
}

type NodeIDs []NodeID

func (nodes NodeIDs) Strings() []string {
	r := make([]string, 0, len(nodes))
	for _, n := range nodes {
		r = append(r, n.String())
	}
	sort.Strings(r)
	return r
}

func makeNode(offset int) Node {
	return Node{
		ID: NodeID(fmt.Sprintf("%0128x", offset)),
	}
}

func makeNodes(offset int, num int) Nodes {
	r := make([]Node, 0, num)
	for i := offset; i < num+offset; i++ {
		r = append(r, makeNode(i))
	}
	return r
}

func addActiveNodes(s Store, nodes ...Node) error {
	for _, n := range nodes {
		if err := s.SetNode(n); err != nil {
			return err
		}

		if _, err := s.UpdateNodePeers(n.ID, nil, 0); err != nil {
			return err
		}
	}
	return nil
}
