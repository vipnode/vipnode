package jsonrpc2

import (
	"encoding/json"
	"errors"
)

// ErrReplyNotAvailable is returned when a Reply is attempted but the message
// does not have a replier associated with it. For example, notifications
// (messages without an ID).
var ErrReplyNotAvailable = errors.New("reply not available")

// FIXME: Should this be named Responder.Respond(Response)?

// Replier has enough context to respond to a message, this
// usually includes a codec, message ID, and message type
// (such as batched or not).
type Replier interface {
	// Reply sends a Response message with the
	// corresponding Request's ID and message type (whether
	// batched or not).
	Reply(resp *Response) error
}

type reply struct {
	codec MessageWriter
	id    json.RawMessage
}

// Reply writes a Response message to a codec with the corresponding
// Request's ID.
func (rep *reply) Reply(resp *Response) error {
	m := &Message{
		Response: resp,
		ID:       rep.id,
		Version:  Version,
	}
	return rep.codec.WriteMessage(m)
}

type batchedReply struct {
	codec *batchedWriter
	id    json.RawMessage
}

// Reply writes a Response message to a codec with the corresponding
// Request's ID.
func (rep *batchedReply) Reply(resp *Response) error {
	m := &Message{
		Response: resp,
		ID:       rep.id,
		Version:  Version,
	}
	return rep.codec.WriteMessage(m)
}
