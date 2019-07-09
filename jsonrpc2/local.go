package jsonrpc2

import (
	"context"
)

var _ Service = &Local{}

// Local is a Service implementation for a local Server. It's like a Remote, but
// without a Codec.
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

func (loc *Local) Notify(ctx context.Context, method string, params ...interface{}) error {
	msg, err := newNotification(method, params...)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, ctxService, loc)
	_ = loc.Server.Handle(ctx, msg.Request)
	return nil
}
