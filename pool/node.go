package pool

import "time"

// FIXME: placeholder types, replace with go-ethereum types
type account string
type nodeID string
type amount int

// Balance describes a node's account balance on the pool.
type Balance struct {
	Account      account
	Credit       amount
	NextWithdraw time.Time
}

// ClientNode stores metadata for tracking VIPs
type ClientNode struct {
	LastSeen time.Time

	balance *Balance
	peers   map[nodeID]time.Time // Last seen
}

// HostNode stores metadata requires for tracking full nodes.
type HostNode struct {
	URI      string
	LastSeen time.Time

	balance *Balance
	peers   map[nodeID]time.Time // Last seen
	inSync  bool                 // TODO: Do we need a penalty if a full node wants to accept peers while not in sync?
}
