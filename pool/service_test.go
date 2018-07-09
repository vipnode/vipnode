package pool

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/vipnode/ethboot/forked/discv5"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/request"
)

func TestPoolInstance(t *testing.T) {
	pool := New()

	r := pool.Ping(context.Background())
	if r != "pong" {
		t.Errorf("pool.Ping direct call failed: %s", r)
	}

	privkey := keygen.HardcodedKey(t)
	req := request.Request{
		Method: "vipnode_connect",
		NodeID: discv5.PubkeyID(&privkey.PublicKey).String(),
		Nonce:  42,
		ExtraArgs: []interface{}{
			"geth",
		},
	}
	sig, err := req.Sign(privkey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Connect(context.Background(), sig, req.NodeID, req.Nonce, req.ExtraArgs[0].(string))
	if err != ErrNoHostNodes {
		t.Errorf("pool.Connect direct call failed: %s", err)
	}
}

func TestPoolService(t *testing.T) {
	pool := New()
	server := rpc.NewServer()
	if err := server.RegisterName("vipnode", pool); err != nil {
		t.Fatal(err)
	}

	client := rpc.DialInProc(server)
	{
		var result interface{}
		if err := client.Call(&result, "vipnode_ping"); err != nil {
			t.Error(err)
		}
		if result.(string) != "pong" {
			t.Errorf("invalid ping result: %s", result)
		}
	}

	privkey := keygen.HardcodedKey(t)
	req := request.Request{
		Method: "vipnode_connect",
		NodeID: discv5.PubkeyID(&privkey.PublicKey).String(),
		Nonce:  42,
		ExtraArgs: []interface{}{
			"geth",
		},
	}

	{
		args, err := req.SignedArgs(privkey)
		if err != nil {
			t.Fatal(err)
		}
		var result interface{}
		if err := client.Call(&result, req.Method, args...); err.Error() != ErrNoHostNodes.Error() {
			t.Error(err)
		}
	}
}

/*

https://github.com/ethereum/go-ethereum/blob/master/rpc/server_test.go#L104

func testServerMethodExecution(t *testing.T, method string) {
	server := rpc.NewServer()
	service := new(Service)

	if err := server.RegisterName("test", service); err != nil {
		t.Fatalf("%v", err)
	}

	stringArg := "string arg"
	intArg := 1122
	argsArg := &Args{"abcde"}
	params := []interface{}{stringArg, intArg, argsArg}

	request := map[string]interface{}{
		"id":      12345,
		"method":  "test_" + method,
		"version": "2.0",
		"params":  params,
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	go server.ServeCodec(rpc.NewJSONCodec(serverConn), rpc.OptionMethodInvocation)

	out := json.NewEncoder(clientConn)
	in := json.NewDecoder(clientConn)

	if err := out.Encode(request); err != nil {
		t.Fatal(err)
	}

	var response struct {
		Version string `json:"jsonrpc"`
		Id      int    `json:"id,omitempty"`
		Result  Result `json:"result"`
	}
	if err := in.Decode(&response); err != nil {
		t.Fatal(err)
	}

	result := response.Result
	if result.String != stringArg {
		t.Errorf("expected %s, got : %s\n", stringArg, result.String)
	}
	if result.Int != intArg {
		t.Errorf("expected %d, got %d\n", intArg, result.Int)
	}
	if !reflect.DeepEqual(result.Args, argsArg) {
		t.Errorf("expected %v, got %v\n", argsArg, result)
	}
}

func TestServerMethodExecution(t *testing.T) {
	testServerMethodExecution(t, "echo")
}

func TestServerMethodWithCtx(t *testing.T) {
	testServerMethodExecution(t, "echoWithCtx")
}
*/
