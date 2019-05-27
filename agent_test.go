package main

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/vipnode/vipnode/agent"
	"github.com/vipnode/vipnode/ethnode"
	"golang.org/x/sync/errgroup"
)

func TestAgentRunner(t *testing.T) {
	options := Options{}
	options.Agent.Args.Coordinator = ":memory:"
	options.Agent.NodeURI = "enode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1@127.0.0.1:30303?discport=0"
	options.Agent.RPC = "fakenode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1?fakepeers=3&fullnode=1"
	options.Agent.UpdateInterval = "60s"

	var out bytes.Buffer
	agent.SetLogger(&out)
	//SetLogger(golog.New(&out, log.Debug))

	runner := agentRunner{}
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
		"Connected to local node:",
		"Registered on pool: Version vipnode/memory-pool/dev",
		"Sent update: 3 peers. Pool response:",
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
