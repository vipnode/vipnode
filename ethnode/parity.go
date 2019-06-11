package ethnode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

var _ EthNode = &parityNode{}

type parityClient struct {
	CanHandleLargeRequests bool   `json:"can_handle_large_requests"`
	Compiler               string `json:"compiler"`
	Identity               string `json:"identity"`
	Name                   string `json:"name"`
	OS                     string `json:"os"`
	Semver                 string `json:"semver"`
}

func (c parityClient) String() string {
	return fmt.Sprintf("Parity/%s/%s/%s", c.Semver, c.OS, c.Compiler)
}

// parityPeerInfo is a clone of the PeerInfo type but with polymorphic parsing for Name
// to support both string and parityClient types.
type parityPeerInfo struct {
	ID        string                     `json:"id"`
	Name      json.RawMessage            `json:"name"`
	Caps      []string                   `json:"caps"`
	Protocols map[string]json.RawMessage `json:"protocols"`
	Network   struct {
		LocalAddress  string `json:"localAddress"`
		RemoteAddress string `json:"remoteAddress"`
	} `json:"network"`
}

// PeerInfo converts parityPeerInfo into PeerInfo
func (pi parityPeerInfo) PeerInfo() (PeerInfo, error) {
	r := PeerInfo{
		ID:        pi.ID,
		Caps:      pi.Caps,
		Protocols: pi.Protocols,
		Network:   pi.Network,
	}
	if bytes.HasPrefix(pi.Name, []byte(`"`)) {
		// Name is a string
		if err := json.Unmarshal(pi.Name, &r.Name); err != nil {
			return r, err
		}
	} else {
		// Name is a ParityClient thing
		var name struct {
			ParityClient parityClient `json:"ParityClient"`
		}
		if err := json.Unmarshal(pi.Name, &name); err != nil {
			return r, err
		}
		r.Name = name.ParityClient.String()
	}
	return r, nil
}

type parityPeers struct {
	Peers []parityPeerInfo `json:"peers"`
}

type parityNode struct {
	client *rpc.Client
}

func (n *parityNode) ContractBackend() bind.ContractBackend {
	return ethclient.NewClient(n.client)
}

func (n *parityNode) Kind() NodeKind {
	return Parity
}

func (n *parityNode) ConnectPeer(ctx context.Context, nodeURI string) error {
	// Parity doesn't have a way to just add peers, so we overload
	// addReservedPeer for this.
	return n.AddTrustedPeer(ctx, nodeURI)
}

func (n *parityNode) DisconnectPeer(ctx context.Context, nodeID string) error {
	// Parity doesn't have a way to drop a specific peer, so we overload
	// removeReservedPeer for this.
	return n.RemoveTrustedPeer(ctx, nodeID)
}

func (n *parityNode) AddTrustedPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "parity_addReservedPeer", nodeID)
}

func (n *parityNode) RemoveTrustedPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "parity_removeReservedPeer", nodeID)
}

func (n *parityNode) Peers(ctx context.Context) ([]PeerInfo, error) {
	var result parityPeers
	err := n.client.CallContext(ctx, &result, "parity_netPeers")
	if err != nil {
		return nil, err
	}
	return filterActivePeers(result.Peers)
}

func (n *parityNode) Enode(ctx context.Context) (string, error) {
	var result string
	if err := n.client.CallContext(ctx, &result, "parity_enode"); err != nil {
		return "", err
	}
	return result, nil
}

func (n *parityNode) BlockNumber(ctx context.Context) (uint64, error) {
	var result string
	if err := n.client.CallContext(ctx, &result, "eth_blockNumber"); err != nil {
		return 0, err
	}
	return strconv.ParseUint(result, 0, 64)
}

// filterActivePeers filters out any peers that have not completed the
// handshake yet. In Parity, these are peers without any specified Protocols.
func filterActivePeers(peers []parityPeerInfo) ([]PeerInfo, error) {
	activePeers := make([]PeerInfo, 0, len(peers))
	for _, peer := range peers {
		if len(peer.Protocols) == 0 {
			continue
		}

		peerInfo, err := peer.PeerInfo()
		if err != nil {
			return nil, err
		}
		activePeers = append(activePeers, peerInfo)
	}

	return activePeers, nil
}
