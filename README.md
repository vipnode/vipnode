# vipnode

**Status**: Pre-alpha. Big pieces are implemented but it's not fully functional
yet.

* `vipnode pool` - Run your own vipnode pool.
* `vipnode host` - Host a vipnode by connecting your full node to a pool.
* `vipnode client` - Connect to a vipnode with your light node (or full node).

## Quickstart

This release has demo support for all commands, and defaults are hardcoded to the _temporary_ deployed pool.

**WARNING**: Everything here is temporary and unstable, don't rely on it in production. **Payment is not implemented.**

### Installing

1. Grab the latest binary release from here: https://github.com/vipnode/vipnode/releases

2. Extract it (`tar xvf vipnode-*.tgz`) and move the binary in your `$PATH` somewhere (`sudo mv vipnode/vipnode-* /usr/local/bin/vipnode`).

You can run `vipnode --help` to see the commands. While exploring, use the `-vv`
flag for extra verbose output.

### How to connect as a client

1. Run a local geth in light mode, something like:
    `geth --syncmode=light --rpc --nodiscover --verbosity 7`
2. `vipnode client -vv`

It should automatically find the RPC and nodekey. If it doesn't, it will fail with a useful error message for how to provide those paths.

### How to connect as a full node host

1. Run a local geth in full mode with lightserv enabled, something like:
    `geth --lightserv=60 --rpc`
2. `vipnode host -vv`

### How to run your own pool

1. `vipnode pool -vv --bind "0.0.0.0:8080"`


## Design

![Diagram](https://raw.githubusercontent.com/vipnode/vipnode.org/master/docs/clientflow.png)

Clients are designed to connect to a set of hosts discovered via the pool, but
the client can also connect to a host directly as if it were a dummy pool.

Pools are designed to provide an economic incentive between the
client and host. Clients provide a deposit of a spending balance to the pool,
and the pool keeps track of which hosts the client is connected to. At the end
of some period (e.g. a week), the pool withdraws the necessary balances from the
clients' deposits to settle the hosts' earnings.

The payment mechanism is managed by a smart contract maintained here:
https://github.com/vipnode/vipnode-contract

The goal is to keep the payment and pool registration optional and replaceable.


## License

MIT
