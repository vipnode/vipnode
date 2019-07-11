package ethnode

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/rpc"
)

// NodeKind represents the different kinds of node implementations we know about.
type NodeKind int

const (
	Unknown NodeKind = iota // We'll treat unknown as Geth, just in case.
	Geth
	Parity
	Pantheon
)

func ParseNodeKind(s string) NodeKind {
	switch strings.ToLower(s) {
	case "geth":
		return Geth
	case "parity":
		return Parity
	case "pantheon":
		return Pantheon
	default:
		return Unknown
	}
}

// NetworkID represents the Ethereum network chain.
type NetworkID int

const (
	UnknownNetwork NetworkID = 0

	Mainnet NetworkID = 1
	Morden  NetworkID = 2
	Ropsten NetworkID = 3
	Rinkeby NetworkID = 4
	Goerli  NetworkID = 5
	Kotti   NetworkID = 6
	Kovan   NetworkID = 42
)

func (id NetworkID) String() string {
	switch id {
	case Mainnet:
		return "mainnet"
	case Morden:
		return "morden"
	case Ropsten:
		return "ropsten"
	case Rinkeby:
		return "rinkeby"
	case Kovan:
		return "kovan"
	case Goerli:
		return "goerli"
	case Kotti:
		return "kotti"
	}
	return "unknown"
}

// Is compares the ID to a network name.
func (id NetworkID) Is(network string) bool {
	return id.String() == strings.ToLower(network)
}

// ParseNetwork takes a network name string and returns a corresponding NetworkID type.
func ParseNetwork(network string) NetworkID {
	switch strings.ToLower(network) {
	case "mainnet":
		return Mainnet
	case "morden":
		return Morden
	case "ropsten":
		return Ropsten
	case "rinkeby":
		return Rinkeby
	case "kovan":
		return Kovan
	case "goerli":
		return Goerli
	case "kotti":
		return Kotti
	}
	return UnknownNetwork
}

func (n NodeKind) String() string {
	switch n {
	case Geth:
		return "geth"
	case Parity:
		return "parity"
	case Pantheon:
		return "pantheon"
	default:
		return "unknown"
	}
}

// UserAgent is the metadata about node client.
type UserAgent struct {
	Version     string `json:"version"`      // Result of web3_clientVersion
	EthProtocol string `json:"eth_protocol"` // Result of eth_protocolVersion

	// Parsed/derived values
	Kind       NodeKind  `json:"kind"`         // Node implementation
	Network    NetworkID `json:"network"`      // Network ID
	IsFullNode bool      `json:"is_full_node"` // Is this a full node? (or a light client?)
}

// KindType returns the Kind of node it is, suffixed with -full or -light.
func (ua *UserAgent) KindType() string {
	if ua.IsFullNode {
		return ua.Kind.String() + "-full"
	}
	return ua.Kind.String() + "-light"
}

// ParseUserAgent takes string values as output from the web3 RPC for
// web3_clientVersion, eth_protocolVersion, and net_version. It returns a
// parsed user agent metadata.
func ParseUserAgent(clientVersion, protocolVersion, netVersion string) (*UserAgent, error) {
	networkID, err := strconv.Atoi(netVersion)
	if err != nil {
		return nil, err
	}
	agent := &UserAgent{
		Version:     clientVersion,
		EthProtocol: protocolVersion,
		Network:     NetworkID(networkID),
		IsFullNode:  true,
	}
	if strings.HasPrefix(agent.Version, "Geth/") {
		agent.Kind = Geth
	} else if strings.HasPrefix(agent.Version, "pantheon/") {
		agent.Kind = Pantheon
	} else if strings.HasPrefix(agent.Version, "Parity-Ethereum/") || strings.HasPrefix(agent.Version, "Parity/") {
		agent.Kind = Parity
	}

	protocol, err := strconv.ParseInt(protocolVersion, 0, 32)
	if err != nil {
		return nil, err
	}
	// FIXME: Can't find any docs on how this protocol value is supposed to be
	// parsed, so just using anecdotal values for now.
	if agent.Kind == Parity && protocol == 1 {
		agent.IsFullNode = false
	} else if agent.Kind == Geth && protocol == 10002 {
		agent.IsFullNode = false
	}
	return agent, nil
}

// Dial is a wrapper around go-ethereum/rpc.Dial with client detection.
func Dial(ctx context.Context, uri string) (EthNode, error) {
	client, err := rpc.DialContext(ctx, uri)
	if err != nil {
		return nil, err
	}

	return RemoteNode(client)
}

// DetectClient queries the RPC API to determine which kind of node is running.
func DetectClient(client *rpc.Client) (*UserAgent, error) {
	var clientVersion string
	if err := client.Call(&clientVersion, "web3_clientVersion"); err != nil {
		return nil, err
	}
	var protocolVersion string
	if err := client.Call(&protocolVersion, "eth_protocolVersion"); err != nil {
		return nil, err
	}
	var netVersion string
	if err := client.Call(&netVersion, "net_version"); err != nil {
		return nil, err
	}
	return ParseUserAgent(clientVersion, protocolVersion, netVersion)
}

// PeerInfo stores the node ID and client metadata about a peer.
type PeerInfo struct {
	ID        string                     `json:"id"`              // Unique node identifier (also the encryption pubkey if `enode` is not included)
	Name      string                     `json:"name"`            // Name of the node, including client type, version, OS, custom data
	Caps      []string                   `json:"caps"`            // Capabilities the node is advertising.
	Protocols map[string]json.RawMessage `json:"protocols"`       // Sub-protocol specific metadata fields
	Enode     string                     `json:"enode,omitempty"` // Enode connection string (includes the enode ID). If available, then the ID is not the pubkey but a hash of it.

	// FIXME: Do we want to include node-local network address state? Or is it unnecessary security leakage?
	Network struct {
		LocalAddress  string `json:"localAddress"`  // Local endpoint of the TCP data connection
		RemoteAddress string `json:"remoteAddress"` // Remote endpoint of the TCP data connection
	} `json:"network"`
}

func (p *PeerInfo) EnodeID() string {
	if len(p.Enode) <= 8+128 { // "enode://{128 ascii chars}@..."
		return p.ID
	}
	return p.Enode[8 : 8+128]
}

func (p *PeerInfo) IsFullNode() bool {
	if p.Protocols == nil {
		return false
	}
	_, ok := p.Protocols["eth"]
	return ok
}

// Peers is a list of PeerInfo
type Peers []PeerInfo

// IDs returns a list of string IDs of the peers.
func (peers Peers) IDs() []string {
	r := make([]string, 0, len(peers))
	for _, peer := range peers {
		r = append(r, peer.EnodeID())
	}
	return r
}

// EthNode is the normalized interface between different kinds of nodes.
type EthNode interface {
	NodeRPC() *rpc.Client
	ContractBackend() bind.ContractBackend

	// Kind returns the kind of node this is.
	Kind() NodeKind
	// UserAgent returns the versions of the client.
	UserAgent() UserAgent
	// Enode returns this node's enode://...
	Enode(ctx context.Context) (string, error)
	// AddTrustedPeer adds a nodeID to a set of nodes that can always connect, even
	// if the maximum number of connections is reached.
	AddTrustedPeer(ctx context.Context, nodeID string) error
	// RemoveTrustedPeer removes a nodeID from the trusted node set.
	RemoveTrustedPeer(ctx context.Context, nodeID string) error
	// ConnectPeer prompts a connection to the given nodeURI.
	ConnectPeer(ctx context.Context, nodeURI string) error
	// DisconnectPeer disconnects from the given nodeID, if connected.
	DisconnectPeer(ctx context.Context, nodeID string) error
	// Peers returns the list of connected peers
	Peers(ctx context.Context) ([]PeerInfo, error)
	// BlockNumber returns the current sync'd block number.
	BlockNumber(ctx context.Context) (uint64, error)
}

// RemoteNode autodetects the node kind and returns the appropriate EthNode
// implementation.
func RemoteNode(client *rpc.Client) (EthNode, error) {
	agent, err := DetectClient(client)
	if err != nil {
		return nil, err
	}
	node := baseNode{
		agent:  *agent,
		client: client,
	}
	switch agent.Kind {
	case Parity:
		return &parityNode{
			baseNode: node,
		}, nil
	case Pantheon:
		return &pantheonNode{
			baseNode: node,
		}, nil
	default:
		// Treat everything else as Geth
		// FIXME: Is this a bad idea?
		node := &gethNode{
			baseNode: node,
		}
		ctx := context.TODO()
		if err := node.CheckCompatible(ctx); err != nil {
			return nil, err
		}
		return node, nil
	}
}
