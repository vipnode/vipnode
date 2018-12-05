# Vipnode

Connect your light client to the Ethereum network instantly, or make money while hosting a full node.

**Status**: Beta. Fully functional, needs testing. Payment is currently using Rinkeby money, [subscribe to the newsletter for updates](https://tinyletter.com/vipnode).

## Quickstart

### Installing

1. Grab the latest binary release for your platform from here: https://github.com/vipnode/vipnode/releases
   
   Or run this one-liner for `linux_amd64` to download and extract:
   
   ```
   curl -s https://api.github.com/repos/vipnode/vipnode/releases | grep -o -m1 "https://.*/vipnode-linux_amd64.tgz" | xargs wget --quiet -O- | tar vxz
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

### How to connect as a client

1. Run a local geth in light mode, something like:
    `geth --syncmode=light --rpc --nodiscover --verbosity 7`
2. `vipnode client -vv`

It should automatically find the RPC and nodekey. If it doesn't, it will fail with a useful error message for how to provide those paths.

### How to connect as a full node host

1. Run a local geth in full mode with lightserv enabled, something like:
    `geth --lightserv=60 --rpc`
2. `vipnode host -vv --payout=$(MYWALLET)`


## Advanced Details

For high-level design and details on running your own pool, check [ADVANCED.md](https://github.com/vipnode/vipnode/blob/master/ADVANCED.md)

## License

MIT
