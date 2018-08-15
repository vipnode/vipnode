package jsonrpc2

import (
	"encoding/json"
	"io"
)

// TODO: Does WriteMessage need to be mutexed?

// Codec is an straction for receiving and sending JSONRPC messages.
type Codec interface {
	ReadMessage() (*Message, error)
	WriteMessage(*Message) error
}

var _ Codec = jsonCodec{}

// IOCodec returns a Codec that wraps JSON encoding and decoding over IO.
func IOCodec(r io.Reader, w io.Writer) jsonCodec {
	return jsonCodec{
		dec: json.NewDecoder(r),
		enc: json.NewEncoder(w),
	}
}

type jsonCodec struct {
	dec *json.Decoder
	enc *json.Encoder
}

func (codec jsonCodec) ReadMessage() (*Message, error) {
	var msg Message
	err := codec.dec.Decode(&msg)
	return &msg, err
}

func (codec jsonCodec) WriteMessage(msg *Message) error {
	return codec.enc.Encode(msg)
}
