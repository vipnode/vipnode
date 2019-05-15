/*
	Package jsonrpc2 implements bidirectional JSONRPC 2.0. This implementation
	does not include Subscription/Notifications, as these features are
	extraneous when bidirectional RPC is available.

	Server is an RPC method registry. Given a receiver, it will expose callable

	Client is an RPC caller implementation.

	Codec is the transport and encoding. Once a Codec is established, it does
	not care which side initiated the connection. There is no inherent
	asymmetry between a server and a client, beyond the codec implementation.

	Remote is a Codec, Server, and Client. Note that it can be a Server and
	Client at the same time, which allows for bidirectional calls.

	When a Remote receives a call, it includes a context which contains a
	service value that can be acquired with CtxService(ctx). The service can be
	used to send calls back to the caller.
*/
package jsonrpc2
