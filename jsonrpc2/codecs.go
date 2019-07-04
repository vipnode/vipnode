package jsonrpc2

import (
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/vipnode/vipnode/internal/pretty"
)

// TODO: Do SetWriteDeadline(duration) on writes?
type rwc struct {
	io.Reader
	io.Writer
	io.Closer
}

type MessageReader interface {
	// ReadMessage returns the next message received in the codec stream. If
	// the codec is reply-capable, then request messages will have a Replier
	// associated to them. Batched messages should be converted into
	// single-message streams.
	ReadMessage() (*Message, error)
}

type MessageWriter interface {
	// WriteMessage sends a message to the codec stream. If the codec is
	// reply-capable, then Message.Request.Reply(...) should be used instead.
	WriteMessage(*Message) error
}

// BatchWriter is an optional interface for supporting batched writes.
type BatchWriter interface {
	// WriteBatch writes a list of messages as a batch in the codec stream.
	WriteBatch([]Message) error
}

// Codec is an straction for receiving and sending JSONRPC messages.
type Codec interface {
	MessageReader
	MessageWriter

	// Close the codec stream.
	Close() error
	// RemoteAddr is identifier address of the stream's remote endpoint.
	RemoteAddr() string
}

var _ Codec = &jsonCodec{}

// IOCodec returns a Codec that wraps JSON encoding and decoding over IO.
func IOCodec(rwc io.ReadWriteCloser) *jsonCodec {
	return &jsonCodec{
		rwc: rwc,
	}
}

type jsonCodec struct {
	rwc        io.ReadWriteCloser
	remoteAddr string

	mu          sync.Mutex
	batchBuffer []Message
}

func (codec *jsonCodec) RemoteAddr() string {
	return codec.remoteAddr
}

func (codec *jsonCodec) WriteMessage(msg *Message) error {
	return json.NewEncoder(codec.rwc).Encode(msg)
}

func (codec *jsonCodec) WriteBatch(batch []Message) error {
	return json.NewEncoder(codec.rwc).Encode(batch)
}

// ReadMessage supports consuming batched messages. When ReadMessage is
// called, batched messages are automatically converted into single message
// streams, but the associated Replier takes care of writing it back into the
// batch.
func (codec *jsonCodec) ReadMessage() (*Message, error) {
	// Do we already have a message in the buffer?
	codec.mu.Lock()
	if len(codec.batchBuffer) > 0 {
		// Pop
		var msg Message
		msg, codec.batchBuffer = codec.batchBuffer[len(codec.batchBuffer)-1], codec.batchBuffer[:len(codec.batchBuffer)-1]
		codec.mu.Unlock()
		return &msg, nil
	}
	codec.mu.Unlock()

	// Get new messages. We load the next sequence of messages into a
	// json.RawMessage so we can differentiate between a batched and unbatched
	// message without borking the json.Decoder internal state.

	// FIXME: Consider using a different streaming parser so we could do it all
	// in one pass without pre-loading everything into memory and doing two
	// passes.
	var raw json.RawMessage
	if err := json.NewDecoder(codec.rwc).Decode(&raw); err != nil {
		return nil, err
	}

	if !isArray(raw) {
		// Single message, skip the batch parsing
		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}

		if msg.Request != nil && msg.ID != nil {
			// Associate a replier on requests that are not notifications.
			msg.Request.replier = &reply{
				codec: codec,
				id:    msg.ID,
			}
		}
		return &msg, nil
	}

	// Batch message parsing time!
	var batch []Message
	if err := json.Unmarshal(raw, &batch); err != nil {
		return nil, err
	}

	if len(batch) == 0 {
		// FIXME: Should we be forgiving of this? Just return an empty message,
		// or keep blocking until we get one?
		return nil, errors.New("empty message batch")
	}

	batchWriter := &batchedWriter{
		writer: codec,
	}
	for _, msg := range batch {
		if msg.Request != nil && msg.ID != nil {
			// Associate a replier on requests that are not notifications.
			msg.Request.replier = &batchedReply{
				codec: batchWriter,
				id:    msg.ID,
			}
			// We're only expecting replies on non-notifications
			batchWriter.expected += 1
		}
	}

	// Pop the last one and append the rest
	msg := batch[len(batch)-1]
	codec.mu.Lock()
	codec.batchBuffer = append(codec.batchBuffer, batch[:len(batch)-1]...)
	codec.mu.Unlock()

	return &msg, nil
}

func (codec *jsonCodec) Close() error {
	return codec.rwc.Close()
}

// DebugCodec logs each incoming and outgoing message with a given label prefix
// (use something like the IP address or user ID).
func DebugCodec(labelPrefix string, codec Codec) *debugCodec {
	return &debugCodec{
		Codec: codec,
		Label: labelPrefix,
	}
}

type debugCodec struct {
	Codec
	Label string
}

func (codec *debugCodec) ReadMessage() (*Message, error) {
	msg, err := codec.Codec.ReadMessage()
	dump, _ := json.Marshal(msg)
	out := pretty.Abbrev(string(dump), 100)
	if err != nil {
		logger.Printf("%s <- Error(%q) - %s\n", codec.Label, err.Error(), out)
	} else {
		logger.Printf("%s <- %s\n", codec.Label, out)
	}
	return msg, err
}

func (codec *debugCodec) WriteMessage(msg *Message) error {
	err := codec.Codec.WriteMessage(msg)
	dump, _ := json.Marshal(msg)
	out := pretty.Abbrev(string(dump), 100)
	if err != nil {
		logger.Printf("%s  -> Error(%q) - %s\n", codec.Label, err.Error(), out)
	} else {
		logger.Printf("%s  -> %s\n", codec.Label, out)
	}
	return err
}

// ErrBatchClosed is returned when a WriteMessage is called on a batch that is
// already completed (number of writes should correspond to the number of
// reads).
var ErrBatchClosed = errors.New("failed writing message to a closed batch")

type batchedWriter struct {
	writer   BatchWriter
	expected int

	mu       sync.Mutex
	messages []Message
}

// WriteMessage will write to the BatchWriter when the expected number of
// messages are written. Once flushed successfully, the batch will be
// considered closed, its internal state will be cleared, and future writes
// will be rejected with an error.
func (codec *batchedWriter) WriteMessage(msg *Message) error {
	codec.mu.Lock()
	defer codec.mu.Unlock()

	if codec.expected == 0 || len(codec.messages) > codec.expected {
		return ErrBatchClosed
	}
	if codec.messages == nil {
		codec.messages = make([]Message, 0, codec.expected)
	}
	codec.messages = append(codec.messages, *msg)

	if len(codec.messages) < codec.expected {
		// Keep accumulaing the buffer
		return nil
	}

	// Flush the buffer
	if err := codec.writer.WriteBatch(codec.messages); err != nil {
		return err
	}

	// Close the batch
	codec.expected = 0
	codec.messages = nil
	return nil
}
