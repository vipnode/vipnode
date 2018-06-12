# vipnode-pool

Coordination server for vipnode pools.


## JSON RPC 2.0 API

It will live on `https://pool.vipnode.org/api`.

- vipnode_peers(paramSig, nodeID, timestamp, peers) -> returns the current
    balance.
- vipnode_connect(paramSig, nodeID, timestamp) -> returns list of full nodes
    who are ready for the node to connect.
