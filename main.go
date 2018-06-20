package main

import (
	"fmt"
	"os"

	"github.com/alexcesaro/log"
	flags "github.com/jessevdk/go-flags"
	"github.com/vipnode/vipnode/host"
	"github.com/vipnode/vipnode/rpc"
)

// Version of the binary, assigned during build.
var Version string = "dev"

// Options contains the flag options
type Options struct {
	Verbose []bool `short:"v" long:"verbose" description:"Show verbose logging."`
	Version bool   `long:"version" description:"Print version and exit."`

	Client struct {
		Args struct {
			VIPNode string `required:"yes" positional-arg-name:"vipnode" description:"vipnode pool URL or stand-alone vipnode enode string"`
		} `positional-args:"yes" required:"yes"`
		RPC string `long:"rpc" description:"RPC path or URL of the client node." default:"~/.ethereum/geth.ipc"`
	} `command:"client" description:"Connect to a vipnode as a client."`

	Host struct {
		Pool string `long:"pool" description:"Pool to participate in. (Optional)"`
		RPC  string `long:"rpc" description:"RPC path or URL of the host node." default:"~/.ethereum/geth.ipc"`
	} `command:"host" description:"Host a vipnode."`

	Pool struct {
		Bind string `long:"bind" description:"Address and port to listen on." default:"0.0.0.0:8080"`
	} `command:"pool" description:"Start a vipnode pool coordinator."`
}

const clientUsage = `Examples:
* Connect to a stand-alone vipnode:
  $ vipnode client "enode://6f8a80d143…b39763a4c0@123.123.123.123:30303?discport=30301"

* Connect to a vipnode pool:
  $ vipnode client "https://pool.vipnode.org/"
`

var logLevels = []log.Level{
	log.Warning,
	log.Info,
	log.Debug,
}

func exit(code int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(code)
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
	if logLevel == log.Debug {
		// Enable logging from subpackages
		// TODO: ...
	}

	switch parser.Active.Name {
	case "client":
	case "host":
		log.Info("Connecting to RPC:", options.Host.RPC)
		remote, err := rpc.Dial(options.Host.RPC)
		if err != nil {
			break
		}

		nodetype, err := rpc.DetectClient(remote)
		if err != nil {
			break
		}

		fmt.Println("Node detected: ", nodetype)

		h := host.New()
		err = h.Start()
	case "pool":
	}

	if err != nil {
		exit(2, "failed to start %s: %s", parser.Active.Name, err)
	}
}
