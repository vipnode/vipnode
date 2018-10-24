package store

import (
	"math/big"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// TestSuite runs a suite of tests against a store implementation.
func TestSuite(t *testing.T, newStore func() Store) {
	nodes := []Node{
		{ID: NodeID([64]byte{'a'})},
		{ID: NodeID([64]byte{'b'})},
		{ID: NodeID([64]byte{'c'})},
		{ID: NodeID([64]byte{'d'})},
		{ID: NodeID([64]byte{'e'})},
		{ID: NodeID([64]byte{'f'})},
		{ID: NodeID([64]byte{'g'})},
		{ID: NodeID([64]byte{'h'})},
		{ID: NodeID([64]byte{'i'})},
		{ID: NodeID([64]byte{'j'})},
	}
	accounts := []Account{
		Account(common.HexToAddress("abcd")),
		Account(common.HexToAddress("efgh")),
	}

	t.Helper()
	t.Run("Nonce", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		nodeID := "abc"
		if err := s.CheckAndSaveNonce(nodeID, 42); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if err := s.CheckAndSaveNonce(nodeID, 45); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if err := s.CheckAndSaveNonce(nodeID, 43); err != ErrInvalidNonce {
			t.Errorf("missing invalid nonce error: %s", err)
		}
		if err := s.CheckAndSaveNonce(nodeID, 45); err != ErrInvalidNonce {
			t.Errorf("missing invalid nonce error: %s", err)
		}
		if err := s.CheckAndSaveNonce("def", 42); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	})

	t.Run("Node", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		node := nodes[0]
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

		node := nodes[0]
		othernode := nodes[1]

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
			t.Errorf("returned nwrong balance: %v", b)
		}

		// Test subtracting and negative
		if err := s.AddNodeBalance(node.ID, big.NewInt(-50)); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if b, err := s.GetNodeBalance(node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if b.Credit.Cmp(big.NewInt(-5)) != 0 {
			t.Errorf("returned nwrong balance: %v", b)
		}

		if b, err := s.GetNodeBalance(othernode.ID); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		} else if b.Credit.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("returned non-empty balance: %v", b)
		}
	})

	t.Run("NodePeers", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		node := nodes[0]

		// Unregistered
		if _, err := s.NodePeers(node.ID); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}
		if _, err := s.UpdateNodePeers(node.ID, []string{"def"}); err != ErrUnregisteredNode {
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

		// peer1 is not a known node, so it will be ignored
		peers := []string{nodes[1].ID.String()}
		if inactive, err := s.UpdateNodePeers(node.ID, peers); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if len(inactive) != 0 {
			t.Errorf("unexpected peers: %v", inactive)
		}

		// Inactives only qualify after ExpireInterval
		newPeers := []string{nodes[2].ID.String(), nodes[3].ID.String()}
		if err := s.SetNode(nodes[2]); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if err := s.SetNode(nodes[3]); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		if inactive, err := s.UpdateNodePeers(node.ID, newPeers); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if len(inactive) != 0 {
			t.Errorf("unexpected peers: %v", inactive)
		}
		if peers, err := s.NodePeers(node.ID); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if peerIDs := nodeIDs(peers); !reflect.DeepEqual(peerIDs, newPeers) {
			t.Errorf("got: %+v; want: %+v", peerIDs, newPeers)
		}
	})

	t.Run("Node", func(t *testing.T) {
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
				ID:     node.ID,
				IsHost: i > 3,
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
		} else if got, want := nodeIDs(hosts), []string{nodes[6].ID.String(), nodes[7].ID.String(), nodes[8].ID.String(), nodes[9].ID.String()}; !reflect.DeepEqual(got, want) {
			t.Errorf("got: %v; want: %v", got, want)
		}

		if hosts, err := s.ActiveHosts("", 2); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if len(hosts) != 2 {
			t.Errorf("wrong number of hosts: %d", len(hosts))
		}
	})

	t.Run("Spender", func(t *testing.T) {
		s := newStore()
		defer s.Close()

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

		if spenders, err := s.GetAccountNodes(account); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if !reflect.DeepEqual(spenders, []NodeID{node.ID}) {
			t.Errorf("invalid spenders: %q", spenders)
		}
	})

	t.Run("SpenderBalance", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		node := nodes[0]
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

		node2 := nodes[1]
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

func nodeIDs(nodes []Node) []string {
	r := make([]string, 0, len(nodes))
	for _, n := range nodes {
		r = append(r, string(n.ID.String()))
	}
	sort.Strings(r)
	return r
}
