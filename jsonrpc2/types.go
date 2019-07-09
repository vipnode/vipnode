package jsonrpc2

import (
	"encoding/json"
	"fmt"
)

// Version of the JSONRPC protocol that we're speaking. This is included with
// every RPC message.
const Version = "2.0"

const (
	ErrCodeParse          int = -32700
	ErrCodeInvalidRequest     = -32600
	ErrCodeMethodNotFound     = -32601
	ErrCodeInvalidParams      = -32602
	ErrCodeInternal           = -32603
	ErrCodeServer             = -32000
)

// Message is a single RPC message, can be a request or a response.
type Message struct {
	*Request
	*Response

	ID      json.RawMessage `json:"id,omitempty"`
	Version string          `json:"jsonrpc,omitempty"`
}

// IsNotification returns whether this message is a notification (has no ID,
// thus not expecting a response).
// https://www.jsonrpc.org/specification#notification
func (m Message) IsNotification() bool {
	return len(m.ID) == 0 || string(m.ID) == "null"
}

func (m Message) String() string {
	// This method is here to satisfy vet
	b, err := json.Marshal(m)
	if err != nil {
		// This shouldn't happen. Might even be worth panic'ing?
		return fmt.Sprintf("failed to marshal %T: %s", m, err)
	}
	return string(b)
}

// Request is an RPC call request.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`

	// FIXME: Set Reply on the Message instead of the Request/Response?
	replier Replier
}

// Reply sends a Response message with the corresponding
// Request's ID and message type (whether batched or not) to the codec that
// origintaed the request.
func (req *Request) Reply(resp *Response) error {
	if req.replier == nil {
		return ErrReplyNotAvailable
	}
	return req.replier.Reply(resp)
}

// Response is an RPC call response.
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

// ErrResponse is returned as part of a Response message when there is an error.
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

// IsErrorCode returns true iff the error has an ErrorCode. If allowedCodes
// is provided, then it also checks that it matches one of the allowedCodes.
func IsErrorCode(err error, allowedCodes ...int) bool {
	errResp, ok := err.(interface{ ErrorCode() int })
	if !ok {
		return false
	}
	if len(allowedCodes) == 0 {
		// No whitelist = allow all
		return true
	}
	gotCode := errResp.ErrorCode()
	for _, wantCode := range allowedCodes {
		if gotCode == wantCode {
			return true
		}
	}
	return false
}
