package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/golog"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/gobwas/ws"
	flags "github.com/jessevdk/go-flags"
	"github.com/vipnode/vipnode/client"
	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/host"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"
)

// defaultClientNode is the value used when `vipnode client` is run without additional args.
var defaultClientNode string = "enode://19b5013d24243a659bda7f1df13933bb05820ab6c3ebf6b5e0854848b97e1f7e308f703466e72486c5bc7fe8ed402eb62f6303418e05d330a5df80738ac974f6@163.172.138.100:30303?discport=30301"

// Version of the binary, assigned during build.
var Version string = "dev"

// Options contains the flag options
type Options struct {
	Verbose []bool `short:"v" long:"verbose" description:"Show verbose logging."`
	Version bool   `long:"version" description:"Print version and exit."`

	Client struct {
		Args struct {
			VIPNode string `positional-arg-name:"vipnode" description:"vipnode pool URL or stand-alone vipnode enode string"`
		} `positional-args:"yes"`
		RPC     string `long:"rpc" description:"RPC path or URL of the client node."`
		NodeKey string `long:"nodekey" description:"Path to the client node's private key."`
	} `command:"client" description:"Connect to a vipnode as a client."`

	Host struct {
		Pool    string `long:"pool" description:"Pool to participate in." default:"https://pool.vipnode.org/"`
		RPC     string `long:"rpc" description:"RPC path or URL of the host node."`
		NodeKey string `long:"nodekey" description:"Path to the host node's private key."`
		NodeURI string `long:"enode" description:"Public enode://... URI for clients to connect to."`
	} `command:"host" description:"Host a vipnode."`

	Pool struct {
		Bind  string `long:"bind" description:"Address and port to listen on." default:"0.0.0.0:8080"`
		Store string `long:"store" description:"Storage driver. (persist|memory)" default:"persist"`
	} `command:"pool" description:"Start a vipnode pool coordinator."`
}

const clientUsage = `Examples:
* Connect to a stand-alone vipnode:
  $ vipnode client "enode://19b5013d24243a659bda7f1df13933bb05820ab6c3ebf6b5e0854848b97e1f7e308f703466e72486c5bc7fe8ed402eb62f6303418e05d330a5df80738ac974f6@163.172.138.100:30303?discport=30301"

* Connect to a vipnode pool:
  $ vipnode client "https://pool.vipnode.org/"
`

func findGethDir() string {
	// TODO: Search multiple places?
	// TODO: Search for parity?
	home := os.Getenv("HOME")
	if home == "" {
		if usr, err := user.Current(); err == nil {
			home = usr.HomeDir
		}
	}
	if home == "" {
		return ""
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Ethereum")
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "Ethereum")
	default:
		return filepath.Join(home, ".ethereum")
	}
}

var logLevels = []log.Level{
	log.Warning,
	log.Info,
	log.Debug,
}

func findNodeKey(nodeKeyPath string) (*ecdsa.PrivateKey, error) {
	if nodeKeyPath == "" {
		nodeKeyPath = findGethDir()
		if nodeKeyPath != "" {
			nodeKeyPath = filepath.Join(nodeKeyPath, "geth", "nodekey")
		}
	}

	return crypto.LoadECDSA(nodeKeyPath)
}

func findRPC(rpcPath string) (ethnode.EthNode, error) {
	if rpcPath == "" {
		rpcPath = findGethDir()
		if rpcPath != "" {
			rpcPath = filepath.Join(rpcPath, "geth.ipc")
		}
	}
	logger.Info("Connecting to RPC:", rpcPath)
	return ethnode.Dial(rpcPath)
}

func subcommand(cmd string, options Options) error {
	switch cmd {
	case "client":
		remoteNode, err := findRPC(options.Client.RPC)
		if err != nil {
			return err
		}
		poolURI := options.Client.Args.VIPNode
		if poolURI == "" {
			poolURI = defaultClientNode
		}
		u, err := url.Parse(poolURI)
		if err != nil {
			return err
		}
		var c client.Client
		if u.Scheme == "enode" {
			pool := &pool.StaticPool{}
			pool.AddNode(poolURI)
			logger.Infof("Connecting to a static node (bypassing pool): %s", poolURI)
			c = client.Client{
				EthNode: remoteNode,
				Pool:    pool,
			}
		}
		if err := c.Connect(); err != nil {
			return err
		}
		// TODO: Register c.Disconnect() on ctrl+c signal?
		return c.ServeUpdates()

	case "host":
		remoteNode, err := findRPC(options.Host.RPC)
		if err != nil {
			return err
		}
		privkey, err := findNodeKey(options.Host.NodeKey)
		if err != nil {
			return ErrExplain{err, "Failed to find node private key. Use --nodekey to specify the correct path. "}
		}
		// Confirm that nodeID matches the private key
		nodeID := discv5.PubkeyID(&privkey.PublicKey).String()
		u, err := url.Parse(options.Host.NodeURI)
		if err != nil {
			return fmt.Errorf("failed to parse enode URI: %s", err)
		}
		if u.User.Username() != nodeID {
			return ErrExplain{
				fmt.Errorf("enode URI does not match node private key"),
				"Make sure the --nodekey used is corresponding to the public node that is running."}
		}

		// Dial host to pool
		h := host.New(options.Host.NodeURI, remoteNode)
		ctx := context.TODO()
		conn, _, _, err := ws.Dial(ctx, options.Host.Pool)
		if err != nil {
			return err
		}
		rpcPool := jsonrpc2.Remote{
			Codec: jsonrpc2.WebSocketCodec(conn),
		}
		rpcPool.Register("vipnode_", h) // For bidirectional vipnode_whitelist
		remotePool := pool.Remote(&rpcPool, privkey)
		return h.ServeUpdates(ctx, remotePool)

	case "pool":
		if options.Pool.Store != "memory" {
			return errors.New("storage driver not implemented")
		}
		p := pool.New()
		server := jsonrpc2.Server{}
		server.Register("vipnode_", p)
		return http.ListenAndServe(options.Pool.Bind, http.HandlerFunc(server.WebsocketHandler))
	}

	return nil
}

func main() {
	options := Options{}
	parser := flags.NewParser(&options, flags.Default)
	p, err := parser.Parse()
	if err != nil {
		if p == nil {
			fmt.Println(err)
		}
		if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp && parser.Active != nil {
			// Print additional usage help when run with --help
			switch parser.Active.Name {
			case "client":
				exit(0, clientUsage)
			}
		}
		return
	}

	if options.Version {
		fmt.Println(Version)
		os.Exit(0)
	}

	// Figure out the log level
	numVerbose := len(options.Verbose)
	if numVerbose > len(logLevels) {
		numVerbose = len(logLevels) - 1
	}

	logLevel := logLevels[numVerbose]
	logWriter := os.Stderr

	SetLogger(golog.New(logWriter, logLevel))
	if logLevel == log.Debug {
		// Enable logging from subpackages
		client.SetLogger(logWriter)
		host.SetLogger(logWriter)
		ethnode.SetLogger(logWriter)
	}

	err = subcommand(parser.Active.Name, options)
	if err == nil {
		return
	}

	switch typedErr := err.(type) {
	case *net.OpError:
		err = ErrExplain{err, `Could not find the RPC path of the running Ethereum node (such as Geth or Parity). Make sure your node is running with RPC enabled. You can specify the path with the --rpc="..." flag.`}
	case interface{ ErrorCode() int }:
		switch typedErr.ErrorCode() {
		case -32601:
			err = ErrExplain{err, `Missing a required RPC method. Make sure your Ethereum node is up to date.`}
		default:
			err = ErrExplain{err, fmt.Sprintf(`Unexpected RPC error occurred: %T. Please open an issue at https://github.com/vipnode/vipnode`, typedErr)}
		}
	case ErrExplain:
		// All good.
	default:
		err = ErrExplain{err, fmt.Sprintf(`Error type %T is missing an explanation. Please open an issue at https://github.com/vipnode/vipnode`, err)}
	}

	if err != nil {
		exit(2, "failed to start %s: %s\n", parser.Active.Name, err)
	}
}

func exit(code int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(code)
}

// ErrExplain annotates an error with an explanation.
type ErrExplain struct {
	Cause       error
	Explanation string
}

func (err ErrExplain) Error() string {
	return fmt.Sprintf("%s\n -> %s", err.Cause, err.Explanation)
}
