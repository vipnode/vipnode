package pool

import "time"

// FIXME: placeholder types, replace with go-ethereum types
type account string
type nodeID string
type amount int

// Balance describes a node's account balance on the pool.
type Balance struct {
	Account      account   `json:"account"`
	Credit       amount    `json:"credit"`
	NextWithdraw time.Time `json:"next_withdraw"`
}

// ClientNode stores metadata for tracking VIPs
type ClientNode struct {
	LastSeen time.Time `json:"last_seen"`

	balance *Balance
	peers   map[nodeID]time.Time // Last seen
}

// HostNode stores metadata requires for tracking full nodes.
type HostNode struct {
	ID       string
	URI      string    `json:"uri"`
	LastSeen time.Time `json:"last_seen"`

	balance *Balance
	peers   map[nodeID]time.Time // Last seen
	inSync  bool                 // TODO: Do we need a penalty if a full node wants to accept peers while not in sync?
}
