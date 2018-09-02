package jsonrpc2

import (
	"encoding/json"
	"io"
)

// Codec is an straction for receiving and sending JSONRPC messages.
type Codec interface {
	ReadMessage() (*Message, error)
	WriteMessage(*Message) error
	Close() error
}

var _ Codec = &jsonCodec{}

// IOCodec returns a Codec that wraps JSON encoding and decoding over IO.
func IOCodec(rwc io.ReadWriteCloser) *jsonCodec {
	return &jsonCodec{
		dec:    json.NewDecoder(rwc),
		enc:    json.NewEncoder(rwc),
		closer: rwc,
	}
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
	out, _ := json.Marshal(msg)
	if err != nil {
		logger.Printf("%s <- Error(%q) - %s\n", codec.Label, err.Error(), out[:100])
	} else {
		logger.Printf("%s <- %s\n", codec.Label, out[:100])
	}
	return msg, err
}

func (codec *debugCodec) WriteMessage(msg *Message) error {
	err := codec.Codec.WriteMessage(msg)
	out, _ := json.Marshal(msg)
	if err != nil {
		logger.Printf("%s  -> Error(%q) - %s\n", codec.Label, err.Error(), out[:100])
	} else {
		logger.Printf("%s  -> %s\n", codec.Label, out[:100])
	}
	return err
}

type jsonCodec struct {
	dec    *json.Decoder
	enc    *json.Encoder
	closer io.Closer
}

func (codec *jsonCodec) ReadMessage() (*Message, error) {
	var msg Message
	err := codec.dec.Decode(&msg)
	return &msg, err
}

func (codec *jsonCodec) WriteMessage(msg *Message) error {
	return codec.enc.Encode(msg)
}

func (codec *jsonCodec) Close() error {
	return codec.closer.Close()
}
