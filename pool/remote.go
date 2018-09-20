package pool

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/request"
)

// Remote returns a RemotePool abstraction which proxies an RPC pool client but
// takes care of all the request signing.
func Remote(client jsonrpc2.Service, privkey *ecdsa.PrivateKey) *RemotePool {
	return &RemotePool{
		client:  client,
		privkey: privkey,
		nodeID:  discv5.PubkeyID(&privkey.PublicKey).String(),
	}
}

// Type assert for Pool implementation.
var _ Pool = &RemotePool{}

// RemotePool wraps a Pool with an RPC service and handles all the signging.
type RemotePool struct {
	client  jsonrpc2.Service
	privkey *ecdsa.PrivateKey
	nodeID  string
}

func (p *RemotePool) getNonce() int64 {
	return time.Now().UnixNano()
}

func (p *RemotePool) Host(ctx context.Context, req HostRequest) (*HostResponse, error) {
	signedReq := request.Request{
		Method:    "vipnode_host",
		NodeID:    p.nodeID,
		Nonce:     p.getNonce(),
		ExtraArgs: []interface{}{req},
	}

	args, err := signedReq.SignedArgs(p.privkey)
	if err != nil {
		return nil, err
	}
	var resp HostResponse
	if err := p.client.Call(ctx, &resp, signedReq.Method, args...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *RemotePool) Client(ctx context.Context, req ClientRequest) (*ClientResponse, error) {
	signedReq := request.Request{
		Method:    "vipnode_client",
		NodeID:    p.nodeID,
		Nonce:     p.getNonce(),
		ExtraArgs: []interface{}{req},
	}

	args, err := signedReq.SignedArgs(p.privkey)
	if err != nil {
		return nil, err
	}
	var resp ClientResponse
	if err := p.client.Call(ctx, &resp, signedReq.Method, args...); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (p *RemotePool) Disconnect(ctx context.Context) error {
	signedReq := request.Request{
		Method: "vipnode_disconnect",
		NodeID: p.nodeID,
		Nonce:  p.getNonce(),
	}

	args, err := signedReq.SignedArgs(p.privkey)
	if err != nil {
		return err
	}
	var result interface{}
	return p.client.Call(ctx, &result, signedReq.Method, args...)
}

func (p *RemotePool) Update(ctx context.Context, req UpdateRequest) (*UpdateResponse, error) {
	signedReq := request.Request{
		Method:    "vipnode_update",
		NodeID:    p.nodeID,
		Nonce:     p.getNonce(),
		ExtraArgs: []interface{}{req},
	}

	args, err := signedReq.SignedArgs(p.privkey)
	if err != nil {
		return nil, err
	}

	var result UpdateResponse
	if err := p.client.Call(ctx, &result, signedReq.Method, args...); err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *RemotePool) Withdraw(ctx context.Context) error {
	signedReq := request.Request{
		Method: "vipnode_withdraw",
		NodeID: p.nodeID,
		Nonce:  p.getNonce(),
	}

	args, err := signedReq.SignedArgs(p.privkey)
	if err != nil {
		return err
	}
	var result interface{}
	return p.client.Call(ctx, &result, signedReq.Method, args...)
}
