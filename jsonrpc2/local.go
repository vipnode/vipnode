package jsonrpc2

import (
	"context"
	"encoding/json"
)

var _ Service = &Local{}

// Local is a Service implementation for a local Server. It's like Remote, but
// no Codec.
type Local struct {
	Client
	Server
}

func (loc *Local) Call(ctx context.Context, result interface{}, method string, params ...interface{}) error {
	req, err := loc.Client.Request(method, params...)
	if err != nil {
		return err
	}
	resp := loc.Server.Handle(ctx, req)
	if resp.Error != nil {
		return resp.Error
	}
	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		// No result
		return nil
	}
	return json.Unmarshal(resp.Result, result)
}
