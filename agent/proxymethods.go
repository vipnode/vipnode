package agent

import "fmt"

// ProxyMethodNotAllowedError is returned when a node RPC method is requested
// that is not allowed by the agent.
type ProxyMethodNotAllowedError struct {
	Method string
}

func (err ProxyMethodNotAllowedError) Error() string {
	return fmt.Sprintf("proxy method is not allowed: %q", err.Method)
}

var allowedProxyMethods = map[string]struct{}{
	// "eth_accounts":                            struct{}{}, // TODO: Return a hardcoded result?
	// FIXME: Any other methods we want to return hardcoded results for?
	"eth_blockNumber":                         struct{}{},
	"eth_call":                                struct{}{},
	"eth_chainId":                             struct{}{},
	"eth_estimateGas":                         struct{}{},
	"eth_gasPrice":                            struct{}{},
	"eth_getBalance":                          struct{}{},
	"eth_getBlockByHash":                      struct{}{},
	"eth_getBlockByNumber":                    struct{}{},
	"eth_getBlockTransactionCountByHash":      struct{}{},
	"eth_getBlockTransactionCountByNumber":    struct{}{},
	"eth_getCode":                             struct{}{},
	"eth_getLogs":                             struct{}{},
	"eth_getStorageAt":                        struct{}{},
	"eth_getTransactionByBlockHashAndIndex":   struct{}{},
	"eth_getTransactionByBlockNumberAndIndex": struct{}{},
	"eth_getTransactionByHash":                struct{}{},
	"eth_getTransactionCount":                 struct{}{},
	"eth_getTransactionReceipt":               struct{}{},
	"eth_getUncleByBlockHashAndIndex":         struct{}{},
	"eth_getUncleByBlockNumberAndIndex":       struct{}{},
	"eth_getUncleCountByBlockHash":            struct{}{},
	"eth_getUncleCountByBlockNumber":          struct{}{},
	"eth_getWork":                             struct{}{},
	"eth_hashrate":                            struct{}{},
	"eth_mining":                              struct{}{},
	"eth_protocolVersion":                     struct{}{},
	"eth_sendRawTransaction":                  struct{}{},
	"eth_submitWork":                          struct{}{},
	"eth_syncing":                             struct{}{},
	"net_listening":                           struct{}{},
	"net_peerCount":                           struct{}{},
	"net_version":                             struct{}{},
	"web3_clientVersion":                      struct{}{},
}
