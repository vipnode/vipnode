package pool

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
