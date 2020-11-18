# Vipnode

[![GoDoc](https://godoc.org/github.com/vipnode/vipnode?status.svg)](http://godoc.org/github.com/vipnode/vipnode)
[![Build Status](https://travis-ci.org/vipnode/vipnode.svg?branch=master)](https://travis-ci.org/vipnode/vipnode)
[![Go Report Card](https://goreportcard.com/badge/github.com/vipnode/vipnode)](https://goreportcard.com/report/github.com/vipnode/vipnode)

Vipnode creates an economic incentive to run full nodes and serve light clients.

* Connect your light client to the Ethereum network instantly, with time-metered fees. 
* Hosting a full node? Join a vipnode pool and earn money for every vipnode client your node serves.

**Status**: Stable release. **Participation payout is currently using Rinkeby money,** [subscribe to the newsletter for updates](https://tinyletter.com/vipnode).

## Quickstart

You'll need to use a terminal and the [geth](https://geth.ethereum.org/downloads/) client for these examples.

### Installing

1. Grab the latest binary release for your platform from here: https://github.com/vipnode/vipnode/releases
   
   Or run this one-liner for `linux_amd64` to download and extract:
   
   ```
   curl -s https://api.github.com/repos/vipnode/vipnode/releases | grep -om1 "https://.*/vipnode-linux_amd64.tgz" | xargs curl -sL | tar vxz
   ```

2. Once you extract it, you'll have a `vipnode` directory. You can run the binary inside of it:
   
   ```
   $ tar xzf vipnode*.tgz
   $ tree vipnode/
   vipnode
   ├── LICENSE
   ├── README.md
   └── vipnode
   $ cd vipnode/
   $ ./vipnode --help
   ```

You can move the `vipnode` binary into your `$PATH` for convenience: `sudo mv vipnode /usr/local/bin/`.

While exploring, try using the `-vv` flag for extra verbose output.


### How to connect as a light client

Clients pay a small fee per minute of being connected to a vipnode host. When you connect to a pool for the first time, you'll get a welcome message with instructions.

1. Run a local geth in light mode, something like:
    `geth --syncmode=light --http --nodiscover --verbosity 7`
2. `vipnode agent -vv`

It should automatically find the RPC and nodekey. If it doesn't, it will fail with a useful error message for how to provide those paths.


### How to connect as a full node host

Hosts earn a small fee per minute of being connected to a vipnode client.

1. Run a local geth in full mode with lightserv enabled, something like:
    `geth --light.serve=60 --http`
2. `vipnode agent -vv --payout=$(MYWALLET)`


## Advanced Details

For high-level design and details on running your own pool, check [ADVANCED.md](https://github.com/vipnode/vipnode/blob/master/ADVANCED.md)


## License

MIT
