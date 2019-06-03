package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/agent"
	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/internal/keygen"
	"golang.org/x/sync/errgroup"
)

func TestAgentRunner(t *testing.T) {
	var out bytes.Buffer
	agent.SetLogger(&out)
	//SetLogger(golog.New(&out, log.Debug))

	privkey := keygen.HardcodedKeyIdx(t, 0)
	nodeID := discv5.PubkeyID(&privkey.PublicKey).String()

	options := Options{}
	options.Agent.Args.Coordinator = ":memory:"
	options.Agent.NodeURI = fmt.Sprintf("enode://%s@127.0.0.1:30303?discport=0", nodeID)
	options.Agent.RPC = fmt.Sprintf("fakenode://%s?fakepeers=3&fullnode=1", nodeID)
	options.Agent.UpdateInterval = "60s"

	runner := agentRunner{
		PrivateKey: privkey,
	}
	if err := runner.LoadAgent(options); err != nil {
		t.Fatal(err)
	}
	if err := runner.LoadPool(options); err != nil {
		t.Fatal(err)
	}

	var gotBlock, expectBlock uint64 = 0, 42
	runner.pool.BlockNumberProvider = func(_ ethnode.NetworkID) (uint64, error) {
		return expectBlock, nil
	}
	runner.Agent.BlockNumberCallback = func(nodeBlockNumber uint64, poolBlockNumber uint64) {
		gotBlock = poolBlockNumber
		if nodeBlockNumber != 0 {
			t.Errorf("BlockNumberBallback nodeBlockNumber: got %d; want %d", nodeBlockNumber, 0)
		}
	}

	errors := errgroup.Group{}
	errors.Go(runner.Run)

	runner.Agent.Stop()

	if err := errors.Wait(); err != nil {
		t.Error("failed to stop cleanly:", err)
	}

	hasLogPrefixes(t, &out, len("[agent] 2019/05/27 13:10:00 "), []string{
		"Connected to local",
		"Registered on pool: Version vipnode/memory-pool/dev",
		"Pool update: peers=3 active=0 invalid=0 block=0",
	})

	if gotBlock != expectBlock {
		t.Errorf("BlockNumberBallback poolBlockNumber: got %d; want %d", gotBlock, expectBlock)
	}
}

func hasLogPrefixes(t *testing.T, out io.Reader, skipChars int, prefixes []string) {
	t.Helper()

	buf := bufio.NewReader(out)
	for _, prefix := range prefixes {
		for {
			line, err := buf.ReadString('\n')
			if err == io.EOF {
				t.Errorf("failed to find log prefix: %q", prefix)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			line = line[skipChars:]
			t.Logf("Looking for prefix: %q ?= %q", prefix, line)
			if strings.HasPrefix(line, prefix) {
				break
			}
		}
	}
}
