package jsonrpc2

import (
	"encoding/json"
	"io"

	"github.com/vipnode/vipnode/v2/internal/pretty"
)

type rwc struct {
	io.Reader
	io.Writer
	io.Closer
}

// Codec is an straction for receiving and sending JSONRPC messages.
type Codec interface {
	ReadMessage() (*Message, error)
	WriteMessage(*Message) error
	Close() error
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
}

func (codec *jsonCodec) RemoteAddr() string {
	return codec.remoteAddr
}

func (codec *jsonCodec) ReadMessage() (*Message, error) {
	var msg Message
	err := json.NewDecoder(codec.rwc).Decode(&msg)
	return &msg, err
}

func (codec *jsonCodec) WriteMessage(msg *Message) error {
	return json.NewEncoder(codec.rwc).Encode(msg)
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
