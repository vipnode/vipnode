# Vipnode: Advanced Details

## How to run your own pool

Most users (clients and hosts) don't need to do this.

If you want to start your own pool of hosts:

1. `vipnode pool -vv --bind "0.0.0.0:8080"`

This will start a basic pool with no payment mechanism. To setup your own payment DApp,
you can provide `--contract.*` flags to configure it. If you'd like to use a different
payment mechanism, you'll need to define a payment structure like the one in `pool/payment`. 


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
