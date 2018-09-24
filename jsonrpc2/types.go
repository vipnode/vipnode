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
	Version string          `json:"jsonrpc"` // TODO: Replace this with a null-type that encodes to 2.0, like https://go-review.googlesource.com/c/tools/+/136675/1/internal/jsonrpc2/jsonrpc2.go#221
}

type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrResponse    `json:"error,omitempty"`
}

// UnmarshalResult attempts to convert the message into a successful result
// unmarshal. If the message is not a success type (or if unmarshal fails), then
// an appropriate error will be returned.
func (resp *Response) UnmarshalResult(result interface{}) error {
	if resp.Error != nil {
		return resp.Error
	}
	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		// No result
		return nil
	}
	return json.Unmarshal(resp.Result, result)
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
