package jsonrpc2

import (
	"encoding/json"
	"sync/atomic"
)

type Client struct {
	id int32
}

func (c *Client) NextID() int {
	return int(atomic.AddInt32(&c.id, 1))
}

func (c *Client) Request(method string, params ...interface{}) (*Request, error) {
	req := &Request{
		Version: Version,
		Method:  method,
	}
	var err error
	if req.ID, err = json.Marshal(c.NextID()); err != nil {
		return nil, err
	}
	if req.Params, err = json.Marshal(params); err != nil {
		return nil, err
	}
	return req, nil
}
