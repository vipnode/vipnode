package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/golog"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	flags "github.com/jessevdk/go-flags"
	"github.com/vipnode/vipnode/client"
	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/host"
	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/jsonrpc2/ws"
	"github.com/vipnode/vipnode/pool"
)

// defaultClientNode is the value used when `vipnode client` is run without additional args.
var defaultClientNode string = "enode://19b5013d24243a659bda7f1df13933bb05820ab6c3ebf6b5e0854848b97e1f7e308f703466e72486c5bc7fe8ed402eb62f6303418e05d330a5df80738ac974f6@163.172.138.100:30303?discport=30301"

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
		Pool    string `long:"pool" description:"Pool to participate in." default:"https://pool.vipnode.org/"`
		RPC     string `long:"rpc" description:"RPC path or URL of the host node."`
		NodeKey string `long:"nodekey" description:"Path to the host node's private key."`
		NodeURI string `long:"enode" description:"Public enode://... URI for clients to connect to."`
		Payout  string `long:"payout" description:"Ethereum wallet address to receive pool payments."`
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
		nodeID := rpcPath[len("fakenode://"):]
		return fakenode.Node(nodeID), nil
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
			fmt.Errorf("enode URI does not match node key; public key prefixes: %q != %q", shortID(enode), shortID(nodeID)),
			"Make sure the --nodekey used is corresponding to the public node that is running.",
		}
	}
	return nil
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

		errChan := make(chan error)
		c := client.New(remoteNode)
		if u.Scheme == "enode" {
			staticPool := &pool.StaticPool{}
			if err := staticPool.AddNode(poolURI); err != nil {
				return err
			}
			logger.Infof("Connecting to a static node (bypassing pool): %s", poolURI)
			if err := c.Start(staticPool); err != nil {
				return err
			}
			return c.Wait()
		}

		privkey, err := findNodeKey(options.Client.NodeKey)
		if err != nil {
			return ErrExplain{err, "Failed to find node private key. Use --nodekey to specify the correct path."}
		}
		// Confirm that nodeID matches the private key
		nodeID := discv5.PubkeyID(&privkey.PublicKey).String()
		remoteEnode, err := remoteNode.Enode(context.Background())
		if err != nil {
			return err
		}
		if err := matchEnode(remoteEnode, nodeID); err != nil {
			return err
		}

		logger.Infof("Connecting to pool: %s", poolURI)
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		poolCodec, err := ws.WebSocketDial(ctx, poolURI)
		cancel()
		if err != nil {
			return ErrExplain{err, "Failed to connect to the pool RPC API."}
		}
		rpcPool := &jsonrpc2.Remote{
			Codec: poolCodec,
		}
		p := pool.Remote(rpcPool, privkey)
		go func() {
			errChan <- rpcPool.Serve()
		}()
		if err := c.Start(p); err != nil {
			return err
		}
		logger.Info("Connected.")

		// Register c.Stop() on ctrl+c signal
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			for _ = range sigCh {
				logger.Info("Shutting down...")
				c.Stop()
			}
		}()

		go func() {
			errChan <- c.Wait()
		}()
		return <-errChan

	case "host":
		remoteNode, err := findRPC(options.Host.RPC)
		if err != nil {
			return err
		}
		privkey, err := findNodeKey(options.Host.NodeKey)
		if err != nil {
			return ErrExplain{err, "Failed to find node private key. Use --nodekey to specify the correct path."}
		}
		// Confirm that nodeID matches the private key
		nodeID := discv5.PubkeyID(&privkey.PublicKey).String()
		if err := matchEnode(options.Host.NodeURI, nodeID); err != nil {
			return err
		}
		remoteEnode, err := remoteNode.Enode(context.Background())
		if err != nil {
			return err
		}
		if err := matchEnode(remoteEnode, nodeID); err != nil {
			return err
		}

		if options.Host.Payout == "" {
			logger.Warning("No --payout address provided, will not receive pool payments.")
		}

		h := host.New(options.Host.NodeURI, remoteNode, options.Host.Payout)

		if options.Host.Pool == ":memory:" {
			// Support for in-memory pool. This is primarily for testing.
			logger.Infof("Starting in-memory vipnode pool.")
			p := pool.New()
			rpcPool := &jsonrpc2.Local{}
			if err := rpcPool.Server.Register("vipnode_", p); err != nil {
				return err
			}
			remotePool := pool.Remote(rpcPool, privkey)
			if err := h.Start(remotePool); err != nil {
				return err
			}
			return h.Wait()
		}

		// Dial host to pool
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		poolCodec, err := ws.WebSocketDial(ctx, options.Host.Pool)
		cancel()
		if err != nil {
			return ErrExplain{err, "Failed to connect to the pool RPC API."}
		}
		logger.Infof("Connected to vipnode pool: %s", options.Host.Pool)

		rpcServer := &jsonrpc2.Server{}
		if err := rpcServer.RegisterMethod("vipnode_whitelist", h, "Whitelist"); err != nil {
			return err
		}
		rpcPool := jsonrpc2.Remote{
			Client: &jsonrpc2.Client{},
			Server: rpcServer,
			Codec:  poolCodec,
		}

		errChan := make(chan error)
		go func() {
			errChan <- rpcPool.Serve()
		}()
		remotePool := pool.Remote(&rpcPool, privkey)
		if err := h.Start(remotePool); err != nil {
			return err
		}
		go func() {
			errChan <- h.Wait()
		}()
		err = <-errChan
		go h.Stop()
		return err

	case "pool":
		if options.Pool.Store != "memory" {
			return errors.New("storage driver not implemented")
		}
		p := pool.New()
		srv := jsonrpc2.Server{}
		if err := srv.Register("vipnode_", p); err != nil {
			return err
		}
		handler := ws.WebsocketHandler(&srv)
		logger.Infof("Starting pool (version %s), listening on: ws://%s", Version, options.Pool.Bind)
		return http.ListenAndServe(options.Pool.Bind, handler)
	}

	return nil
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

type shortID string

func (s shortID) String() string {
	if len(s) > 12 {
		return fmt.Sprintf("%s…", s[:8])
	}
	return string(s)
}
