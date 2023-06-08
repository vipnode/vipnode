package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/golog"
	"github.com/ethereum/go-ethereum/crypto"
	flags "github.com/jessevdk/go-flags"
	"github.com/vipnode/vipnode/v2/agent"
	"github.com/vipnode/vipnode/v2/ethnode"
	"github.com/vipnode/vipnode/v2/internal/fakenode"
	"github.com/vipnode/vipnode/v2/internal/pretty"
	"github.com/vipnode/vipnode/v2/jsonrpc2"
	"github.com/vipnode/vipnode/v2/pool"
	"github.com/vipnode/vipnode/v2/pool/payment"
)

// Version of the binary, assigned during build.
var Version string = "dev"

var rpcTimeout = time.Second * 5

// Options contains the flag options
type Options struct {
	Config      string `long:"config" description:"Load configuration from file. (Use --print-config for an example)"`
	PrintConfig bool   `long:"print-config" description:"Print the current configuration to stdout."`
	Pprof       string `long:"pprof" description:"Bind pprof http server for profiling. (Example: localhost:6060)"`
	Verbose     []bool `short:"v" long:"verbose" description:"Show verbose logging."`
	Version     bool   `long:"version" description:"Print version and exit."`

	Agent struct {
		Args struct {
			Coordinator string `positional-arg-name:"coordinator" description:"vipnode pool URL or stand-alone vipnode enode string" default:"wss://pool.vipnode.org/"`
		} `positional-args:"yes"`
		RPC            string `long:"rpc" description:"RPC path or URL of the host node."`
		NodeKey        string `long:"nodekey" description:"Path to the host node's private key."`
		NodeURI        string `long:"enode" description:"Public enode://... URI for clients to connect to. (If node is on a different IP from the vipnode agent)"`
		NodeHost       string `long:"enode.host" description:"Override just the host component of reported enode:// URI. Useful for overriding network routing."`
		Payout         string `long:"payout" description:"Ethereum wallet address to associate pool credits."`
		MinPeers       int    `long:"min-peers" description:"Minimum number of peers to maintain." default:"3"`
		StrictPeers    bool   `long:"strict-peers" description:"Disconnect peers that were not provided by the pool."`
		UpdateInterval string `long:"update-interval" description:"Time between updates sent to pool, should be under 120s." default:"60s"`
	} `command:"agent" description:"Connect as a node to a pool or another vipnode."`

	Pool struct {
		Bind            string `long:"bind" description:"Address and port to listen on." default:"0.0.0.0:8080"`
		Store           string `long:"store" description:"Storage driver. (persist|memory)" default:"persist"`
		DataDir         string `long:"datadir" description:"Path for storing the persistent database."`
		TLSHost         string `long:"tlshost" description:"Acquire an ACME TLS cert for this host (forces bind to port :443)."`
		AllowOrigin     string `long:"allow-origin" description:"Include Access-Control-Allow-Origin header for CORS."`
		RestrictNetwork string `long:"restrict-network" description:"Restrict nodes to a single Ethereum network, such as: mainnet, rinkeby, goerli"`
		MaxRequestHosts int    `long:"max-request-hosts" description:"Maximum number of hosts a node is allowed to request."`
		InvalidNode     string `long:"invalid-node" description:"Regexp to mark peers as invalid if the node client matches."`
		Contract        struct {
			RPC        string `long:"rpc" description:"Path or URL of an Ethereum RPC provider for payment contract operations. Must match the network of the contract."`
			Addr       string `long:"address" description:"Deployed contract address, prefixed with network name scheme. (Example: \"rinkeby://0xb2f8987986259facdc539ac1745f7a0b395972b1\")"`
			KeyStore   string `long:"keystore" description:"Path to encrypted JSON wallet keystore for contract operator. (Password set in KEYSTORE_PASSPHRASE env)"`
			Price      string `long:"price" description:"Price per minute." default:"100 gwei"`
			MinBalance string `long:"min-balance" description:"Minimum balance required to join as a client, or 'off'." default:"off"`
			Welcome    string `long:"welcome" description:"Welcome message for clients. (Example: \"Welcome, {{.NodeID}}\")"`
		} `group:"contract" namespace:"contract"`
	} `command:"pool" description:"Start a vipnode pool coordinator."`

	// DEPRECATED
	Client struct {
		Args struct {
			VIPNode string `positional-arg-name:"vipnode" description:"vipnode pool URL or stand-alone vipnode enode string"`
		} `positional-args:"yes"`
		RPC     string `long:"rpc" description:"RPC path or URL of the client node."`
		NodeKey string `long:"nodekey" description:"Path to the client node's private key."`
	} `command:"client" description:"Connect to a vipnode as a client." hidden:"true"`

	// DEPRECATED
	Host struct {
		Pool    string `long:"pool" description:"Pool to participate in." default:"wss://pool.vipnode.org/"`
		RPC     string `long:"rpc" description:"RPC path or URL of the host node."`
		NodeKey string `long:"nodekey" description:"Path to the host node's private key."`
		NodeURI string `long:"enode" description:"Public enode://... URI for clients to connect to. (If node is on a different IP from the vipnode agent)"`
		Payout  string `long:"payout" description:"Ethereum wallet address to receive pool payments."`
	} `command:"host" description:"Host a vipnode." hidden:"true"`
}

const clientUsage = `Examples:
* Connect to a stand-alone vipnode:
  $ vipnode agent "enode://19b5013d24243a659bda7f1df13933bb05820ab6c3ebf6b5e0854848b97e1f7e308f703466e72486c5bc7fe8ed402eb62f6303418e05d330a5df80738ac974f6@163.172.138.100:30303?discport=30301"

* Connect to a vipnode pool:
  $ vipnode agent "wss://pool.vipnode.org/"
`

func findGethDir() string {
	// TODO: Search multiple places?
	// TODO: Search for parity?
	// TODO: Search CWD?
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
	} else if strings.HasPrefix(rpcPath, "fakenode://") {
		// Used for testing
		u, err := url.Parse(rpcPath)
		if err != nil {
			return nil, err
		}
		nodeID := u.User.Username()
		if nodeID == "" {
			nodeID = u.Hostname()
		}
		node := fakenode.Node(nodeID)
		if numPeers, err := strconv.Atoi(u.Query().Get("fakepeers")); err == nil {
			node.FakePeers = fakenode.FakePeers(numPeers)
		}
		if numBlock, err := strconv.Atoi(u.Query().Get("fakeblock")); err == nil {
			node.FakeBlockNumber = uint64(numBlock)
		}
		node.IsFullNode = u.Query().Get("fullnode") != ""

		logger.Warningf("Using a *fake* Ethereum node (only use for testing) with %d peers and nodeID: %q", len(node.FakePeers), pretty.Abbrev(node.NodeID))
		return node, nil
	}
	logger.Info("Connecting to Ethereum node:", rpcPath)
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	node, err := ethnode.Dial(ctx, rpcPath)
	cancel()
	if err != nil {
		if _, ok := err.(interface{ Explain() string }); ok {
			return nil, err
		}
		err = ErrExplain{
			err,
			fmt.Sprintf(`Could not find the RPC path of the running Ethereum node (such as Geth or Parity). Tried "%s". Make sure your node is running with RPC enabled. You can specify the path with the --rpc="..." flag.`, rpcPath),
		}
		return nil, err
	}
	return node, nil
}

func matchEnode(enode string, nodeID string) error {
	if strings.Contains(enode, "://") {
		u, err := url.Parse(enode)
		if err != nil {
			return fmt.Errorf("failed to parse enode URI: %s", err)
		}
		enode = u.User.Username()
	}
	if enode != nodeID {
		return ErrExplain{
			fmt.Errorf("enode URI does not match node key; public key prefixes: %q != %q", pretty.Abbrev(enode), pretty.Abbrev(nodeID)),
			"Make sure the --nodekey used is corresponding to the public node that is running.",
		}
	}
	return nil
}

func subcommand(cmd string, options Options) error {
	if cmd == "pool" {
		return runPool(options)
	}

	// Run with retries for host/client

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	backoff := []int{5, 30, 60, 90, 300} // Backoff sequence in seconds.
	clearTimeout := time.Second * 300    // Time between attempts before we reset the backoff
	var err error
	for i := 0; ; i++ {
		since := time.Now()
		switch cmd {
		case "agent":
			err = runAgent(options)
		case "client":
			logger.Warning("The `client` subcommand is deprecated, use `agent` instead.")
			err = runClient(options)
		case "host":
			logger.Warning("The `host` subcommand is deprecated, use `agent` instead.")
			err = runHost(options)
		}

		b := i
		if b >= len(backoff) {
			// Keep trying at the max interval
			b = len(backoff) - 1
		}

		waitTime := time.Duration(backoff[b]) * time.Second
		if err == nil {
			// Exit cleanly
			return nil
		} else if err == io.EOF {
			logger.Warningf("Connection closed, retrying in %s...", waitTime)
		} else if errRetry, ok := err.(ErrExplainRetry); ok {
			logger.Warningf("Failed to connect, retrying in %s: %s", waitTime, errRetry)
		} else if _, ok := err.(net.Error); ok {
			logger.Warningf("Failed to connect, retrying in %s: %s", waitTime, err)
		} else if err.Error() == (pool.NoHostNodesError{}).Error() {
			logger.Warningf("Pool does not have available hosts, retrying in %s...", waitTime)
		} else {
			return err
		}

		if time.Now().After(since.Add(clearTimeout)) {
			// Reset backoff if run ran for at least clearTimeout
			i = 0
		}

		select {
		case <-time.After(waitTime):
		case <-sigCh:
			return nil
		}
	}
}

func main() {
	options := Options{}
	parser := flags.NewParser(&options, flags.Default)
	parser.Groups()[0].ShortDescription = "vipnode"
	parser.SubcommandsOptional = true
	p, err := parser.ParseArgs(os.Args[1:])
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

	if options.Config != "" {
		ini := flags.NewIniParser(parser)
		if err := ini.ParseFile(options.Config); err != nil {
			exit(1, "Failed to parse config: %s", err)
		}
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
		pool.SetLogger(logWriter)
		agent.SetLogger(logWriter)
		payment.SetLogger(logWriter)
		ethnode.SetLogger(logWriter)
		jsonrpc2.SetLogger(logWriter)
	}

	if !strings.HasPrefix(Version, "v") || strings.HasPrefix(Version, "v0.") {
		logger.Warningf("This is a pre-release version (%s). It can stop working at any time.", Version)
	}

	if options.PrintConfig {
		options.PrintConfig = false // Silly to include this in the example config
		ini := flags.NewIniParser(parser)
		ini.Write(os.Stdout, flags.IniIncludeDefaults|flags.IniIncludeComments)
		return
	}

	if options.Pprof != "" {
		go func() {
			logger.Debugf("Starting pprof server: http://%s/debug/pprof", options.Pprof)
			if err := http.ListenAndServe(options.Pprof, nil); err != nil {
				logger.Errorf("Failed to serve pprof: %s", err)
			}
		}()
	}

	cmd := "agent"
	if parser.Active != nil {
		cmd = parser.Active.Name
	}
	err = subcommand(cmd, options)
	if err == nil {
		return
	}

	if err == io.EOF {
		exit(3, "Connection closed.\n")
	}

	if err != nil {
		exit(2, "%s failed: %s\n", cmd, explainError(err))
	}
}

// explainError cycles through known errors and annotates them with additional
// explanation and instructions for display. It will unwrap wrapped errors
// until it finds one that it knows about.
func explainError(err error) error {
	if err == nil {
		return nil
	}

	currentErr := err
	for {
		switch typedErr := currentErr.(type) {
		case ErrExplain:
			// All good, this is what we want.
			return typedErr
		case interface{ Explain() string }:
			return ErrExplain{err, typedErr.Explain()}
		case net.Error:
			return ErrExplain{err, `Disconnected from server unexpectedly. Could be a connectivity issue or the server is down. Try again?`}
		case interface{ ErrorCode() int }:
			switch typedErr.ErrorCode() {
			case jsonrpc2.ErrCodeMethodNotFound, jsonrpc2.ErrCodeInvalidParams:
				return ErrExplain{err, `Missing a required RPC method. Make sure your Ethereum node is up to date.`}
			case jsonrpc2.ErrCodeInternal:
				if err.Error() == (pool.NoHostNodesError{}).Error() {
					return ErrExplain{err, `The pool does not have any hosts who are ready to serve your kind of client right now. Try again later or contact the pool operator for help.`}
				}
				fallthrough
			default:
				return ErrExplain{err, fmt.Sprintf(`Unexpected RPC error occurred: %T (code %d). Please open an issue at https://github.com/vipnode/vipnode`, typedErr, typedErr.ErrorCode())}
			}
		case interface{ Unwrap() error }:
			currentErr = typedErr.Unwrap()
		default:
			return ErrExplain{err, fmt.Sprintf(`Error type %T is missing an explanation. Please open an issue at https://github.com/vipnode/vipnode`, err)}
		}
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
	cause := err.Cause
	if cause == nil {
		cause = errors.New("an error occurred")
	}
	return fmt.Sprintf("%s\n -> %s", cause, err.Explanation)
}

// ErrExplainRetry is the same as ErrExplain except it can be retried
type ErrExplainRetry struct {
	ErrExplain
}
