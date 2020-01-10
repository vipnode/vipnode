package ethnode

import (
	"context"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// baseNode is the common functionality across node types.
type baseNode struct {
	client *rpc.Client
	agent  UserAgent
}

func (n *baseNode) NodeRPC() *rpc.Client {
	return n.client
}

func (n *baseNode) ContractBackend() bind.ContractBackend {
	return ethclient.NewClient(n.client)
}

func (n *baseNode) UserAgent() UserAgent {
	return n.agent
}

func (n *baseNode) BlockNumber(ctx context.Context) (uint64, error) {
	var result string
	if err := n.client.CallContext(ctx, &result, "eth_blockNumber"); err != nil {
		return 0, err
	}
	return strconv.ParseUint(result, 0, 64)
}
