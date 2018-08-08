package jsonrpc2

import (
	"encoding/json"
	"fmt"
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

type Request struct {
	ID      json.RawMessage `json:"id,omitempty"`
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	ID      json.RawMessage `json:"id,omitempty"`
	Version string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrResponse    `json:"error,omitempty"`
}

type ErrResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (err *ErrResponse) Error() string {
	return fmt.Sprintf("%d: %s", err.Code, err.Message)
}
