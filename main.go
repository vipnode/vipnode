package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
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
	"github.com/vipnode/vipnode/client"
	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/host"
	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/internal/pretty"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"
)

// defaultClientNode is the value used when `vipnode client` is run without additional args.
//var defaultClientNode string = "enode://19b5013d24243a659bda7f1df13933bb05820ab6c3ebf6b5e0854848b97e1f7e308f703466e72486c5bc7fe8ed402eb62f6303418e05d330a5df80738ac974f6@163.172.138.100:30303?discport=30301"
var defaultClientNode string = "ws://pool.vipnode.org:8080/" // TODO: Update this for prod release

// Version of the binary, assigned during build.
var Version string = "dev"

var rpcTimeout = time.Second * 5

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
		Pool    string `long:"pool" description:"Pool to participate in." default:"ws://pool.vipnode.org:8080/"` // TODO: Update this for prod release
		RPC     string `long:"rpc" description:"RPC path or URL of the host node."`
		NodeKey string `long:"nodekey" description:"Path to the host node's private key."`
		NodeURI string `long:"enode" description:"Public enode://... URI for clients to connect to."`
		Payout  string `long:"payout" description:"Ethereum wallet address to receive pool payments."`
	} `command:"host" description:"Host a vipnode."`

	Pool struct {
		Bind    string `long:"bind" description:"Address and port to listen on." default:"0.0.0.0:8080"`
		Store   string `long:"store" description:"Storage driver. (persist|memory)" default:"persist"`
		DataDir string `long:"datadir" description:"Path for storing the persistent database."`
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
		if numpeers, err := strconv.Atoi(u.Query().Get("fakepeers")); err == nil {
			node.FakePeers = fakenode.FakePeers(numpeers)
		}
		logger.Warningf("Using a *fake* Ethereum node (only use for testing) with %d peers and nodeID: %q", len(node.FakePeers), pretty.Abbrev(node.NodeID))
		return node, nil
	}
	logger.Info("Connecting to Ethereum node:", rpcPath)
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	node, err := ethnode.Dial(ctx, rpcPath)
	cancel()
	if err != nil {
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

	backoff := []int{5, 30, 60, 90, 300} // Backoff in sequence in seconds.
	clearTimeout := time.Second * 300    // Time between attempts before we reset the backoff
	var err error
	for i := 0; ; i++ {
		since := time.Now()
		switch cmd {
		case "client":
			err = runClient(options)
		case "host":
			err = runHost(options)
		}

		b := i
		if b >= len(backoff) {
			// Keep trying at the max interval
			b = len(backoff) - 1
		}

		waitTime := time.Duration(backoff[b]) * time.Second
		if err == io.EOF {
			logger.Warningf("Connection closed, retrying in %s...", waitTime)
		} else if errRetry, ok := err.(ErrExplainRetry); ok {
			logger.Warningf("Failed to connect, retrying in %s: %s", waitTime, errRetry.Cause)
		} else if _, ok := err.(net.Error); ok {
			logger.Warningf("Failed to connect, retrying in %s: %s", waitTime, err)
		} else if err.Error() == (pool.ErrNoHostNodes{}).Error() {
			logger.Warningf("Pool does not have available hosts, retrying in %s...", waitTime)
		} else {
			return err
		}

		if time.Now().After(since.Add(clearTimeout)) {
			// Reset backoff if run ran for at least clearTimeout
			i = 0
		}

		time.Sleep(waitTime)
	}
}

func main() {
	options := Options{}
	parser := flags.NewParser(&options, flags.Default)
	parser.SubcommandsOptional = true
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
		pool.SetLogger(logWriter)
		client.SetLogger(logWriter)
		host.SetLogger(logWriter)
		ethnode.SetLogger(logWriter)
		jsonrpc2.SetLogger(logWriter)
	}

	if !strings.HasPrefix(Version, "v") || strings.HasPrefix(Version, "v0.") {
		logger.Warningf("This is a pre-release version (%s). It can stop working at any time.", Version)
	}

	cmd := "client"
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

	switch typedErr := err.(type) {
	case net.Error:
		err = ErrExplain{err, `Disconnected from server unexpectedly. Could be a connectivity issue or the server is down. Try again?`}
	case interface{ ErrorCode() int }:
		switch typedErr.ErrorCode() {
		case -32601:
			err = ErrExplain{err, `Missing a required RPC method. Make sure your Ethereum node is up to date.`}
		case -32603:
			if err.Error() == (pool.ErrNoHostNodes{}).Error() {
				err = ErrExplain{err, `The pool does not have any hosts who are ready to serve your kind of client right now. Try again later or contact the pool operator for help.`}
				break
			}
			fallthrough
		default:
			err = ErrExplain{err, fmt.Sprintf(`Unexpected RPC error occurred: %T (code %d). Please open an issue at https://github.com/vipnode/vipnode`, typedErr, typedErr.ErrorCode())}
		}
	case ErrExplain:
		// All good.
	default:
		err = ErrExplain{err, fmt.Sprintf(`Error type %T is missing an explanation. Please open an issue at https://github.com/vipnode/vipnode`, err)}
	}

	if err != nil {
		exit(2, "%s failed: %s\n", cmd, err)
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

// ErrExplainRetry is the same as ErrExplain except it can be retried
type ErrExplainRetry struct {
	ErrExplain
}
