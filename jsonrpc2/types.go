package jsonrpc2

import (
	"encoding/json"
)

const Version = "2.0"

const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
	ErrCodeServer         = -32000
)

type Message struct {
	*Request
	*Response
	ID      json.RawMessage `json:"id,omitempty"`
	Version string          `json:"jsonrpc"`
}

type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrResponse    `json:"error,omitempty"`
}

type ErrResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (err *ErrResponse) Error() string {
	return err.Message
}

func (err *ErrResponse) ErrorCode() int {
	return err.Code
}
