package store

import (
	"reflect"
	"sort"
	"testing"
)

// TestSuite runs a suite of tests against a store implementation.
func TestSuite(t *testing.T, newStore func() Store) {
	t.Helper()
	t.Run("Nonce", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		nodeID := NodeID("abc")
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

		node := Node{
			ID: NodeID("abc"),
		}
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

		// Unregistered
		if err := s.AddBalance(NodeID("abc"), 42); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}
		if _, err := s.GetBalance(NodeID("abc")); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}

		// Init node
		if err := s.SetNode(Node{ID: "abc"}); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		// Test balance adding
		if err := s.AddBalance(NodeID("abc"), 42); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if err := s.AddBalance(NodeID("abc"), 3); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if b, err := s.GetBalance(NodeID("abc")); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if b.Credit != 45 {
			t.Errorf("returned nwrong balance: %v", b)
		}

		// Test subtracting and negative
		if err := s.AddBalance(NodeID("abc"), -50); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if b, err := s.GetBalance(NodeID("abc")); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if b.Credit != -5 {
			t.Errorf("returned nwrong balance: %v", b)
		}

		if b, err := s.GetBalance(NodeID("def")); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		} else if b.Credit != 0 {
			t.Errorf("returned non-empty balance: %v", b)
		}
	})

	t.Run("NodePeers", func(t *testing.T) {
		s := newStore()
		defer s.Close()

		// Unregistered
		if _, err := s.NodePeers(NodeID("abc")); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}
		if _, err := s.UpdateNodePeers(NodeID("abc"), []string{"def"}); err != ErrUnregisteredNode {
			t.Errorf("expected unregistered error, got: %s", err)
		}

		// Init node
		node := Node{ID: "abc"}
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
		peers := []string{"peer1"}
		if inactive, err := s.UpdateNodePeers(node.ID, peers); err != nil {
			t.Errorf("unexpected error: %s", err)
		} else if len(inactive) != 0 {
			t.Errorf("unexpected peers: %v", inactive)
		}

		// Inactives only qualify after ExpireInterval
		newPeers := []string{"peer2", "peer3"}
		for _, peerID := range newPeers {
			if err := s.SetNode(Node{ID: NodeID(peerID)}); err != nil {
				t.Errorf("unexpected error: %s", err)
			}
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
}

func nodeIDs(nodes []Node) []string {
	r := make([]string, 0, len(nodes))
	for _, n := range nodes {
		r = append(r, string(n.ID))
	}
	sort.Strings(r)
	return r
}
