package main

import (
	"testing"

	"golang.org/x/sync/errgroup"
)

func TestAgentRunner(t *testing.T) {
	options := Options{}
	options.Agent.Args.Coordinator = ":memory:"
	options.Agent.NodeURI = "enode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1@127.0.0.1:30303?discport=0"
	options.Agent.RPC = "fakenode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1?fakepeers=3&fullnode=1"
	options.Agent.UpdateInterval = "60s"

	runner := agentRunner{}
	if err := runner.LoadAgent(options); err != nil {
		t.Fatal(err)
	}
	if err := runner.LoadPool(options); err != nil {
		t.Fatal(err)
	}

	errors := errgroup.Group{}
	errors.Go(runner.Run)

	runner.Agent.Stop()

	if err := errors.Wait(); err != nil {
		t.Error("failed to stop cleanly:", err)
	}
}
