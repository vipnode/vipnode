package jsonrpc2

import (
	"context"
)

var _ Service = &Local{}

// Local is a Service implementation for a local Server. It's like Remote, but
// no Codec. There is no distinction of Notifications or btch requests for the
// Local service.
type Local struct {
	Client
	Server
}

func (loc *Local) Call(ctx context.Context, result interface{}, method string, params ...interface{}) error {
	msg, err := loc.Client.Request(method, params...)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, ctxService, loc)
	resp := loc.Server.Handle(ctx, msg.Request)
	return resp.UnmarshalResult(result)
}
