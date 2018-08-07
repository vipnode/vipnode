package jsonrpc2

import (
	"encoding/json"
	"sync/atomic"
)

type Request struct {
	ID      json.RawMessage `json:"id,omitempty"`
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type SuccessResponse struct {
	ID      json.RawMessage `json:"id,omitempty"`
	Version string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
}

type ErrorResponse struct {
	ID      json.RawMessage `json:"id,omitempty"`
	Version string          `json:"jsonrpc"`
	Error   struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	} `json:"error"`
}

type Server struct {
}

type Client struct {
	id int32
}

func (c *Client) NextID() int {
	return int(atomic.AddInt32(&c.id, 1))
}
