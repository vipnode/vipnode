package jsonrpc2

import (
	"encoding/json"
	"fmt"
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

type jsonCodec struct {
	dec    *json.Decoder
	enc    *json.Encoder
	closer io.Closer
}

func dump(prefix string, msg *Message) {
	out, _ := json.Marshal(msg)
	fmt.Printf("%s %s\n", prefix, out)
}

func (codec *jsonCodec) ReadMessage() (*Message, error) {
	var msg Message
	err := codec.dec.Decode(&msg)
	//dump(" <- ", &msg)
	return &msg, err
}

func (codec *jsonCodec) WriteMessage(msg *Message) error {
	//dump("-> ", msg)
	return codec.enc.Encode(msg)
}

func (codec *jsonCodec) Close() error {
	return codec.closer.Close()
}
