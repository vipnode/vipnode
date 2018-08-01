# vipnode

**Status**: Pre-alpha. Big pieces are implemented but it's not fully functional
yet.

* `vipnode pool` - Run your own vipnode pool.
* `vipnode host` - Host a vipnode by connecting your full node to a pool.
* `vipnode client` - Connect to a vipnode with your light node (or full node).


## Design

![Diagram](https://raw.githubusercontent.com/vipnode/vipnode.org/master/docs/clientflow.png)

The vipnode system can be run in a few different configurations.

By default, it's designed for a client to connect to a set of hosts discovered
via the pool, but the client can also connect to a host directly as if it were a
dummy pool.

Additionally, pools are designed to provide an economic incentive between the
client and host. Clients provide a deposit of a spending balance to the pool,
and the pool keeps track of which hosts the client is connected to. At the end
of some period (e.g. a week), the pool withdraws the necessary balances from the
clients' deposits to settle the hosts' earnings.

The payment mechanism is managed by a smart contract maintained here:
https://github.com/vipnode/vipnode-contract

The goal is to keep the payment and pool registration optional and replaceable.


## License

MIT
