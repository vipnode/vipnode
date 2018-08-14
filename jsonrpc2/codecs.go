package jsonrpc2

import (
	"encoding/json"
	"io"
	"net"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
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

// WebSocketcoding returns a Codec that wraps JSON encoding and decoding over a
// websocket connection.
func WebSocketCodec(conn net.Conn) wsCodec {
	r := wsutil.NewReader(conn, ws.StateServerSide)
	w := wsutil.NewWriter(conn, ws.StateServerSide, ws.OpText)
	return wsCodec{
		jsonCodec: IOCodec(r, w),
		conn:      conn,
		r:         r,
		w:         w,
	}
}

var _ Codec = wsCodec{}

type wsCodec struct {
	jsonCodec
	conn net.Conn
	r    *wsutil.Reader
	w    *wsutil.Writer
}

func (codec wsCodec) ReadMessage() (*Message, error) {
	header, err := codec.r.NextFrame()
	if err != nil {
		return nil, err
	}

	// FIXME: This is broken because of server/client websocket asymmetry

	return codec.jsonCodec.ReadMessage()
}

func (codec wsCodec) WriteMessage(msg *Message) error {
	err := codec.jsonCodec.WriteMessage(msg)
	if err != nil {
		return err
	}
	if err = codec.w.Flush(); err != nil {
		return err
	}
	return nil
}
