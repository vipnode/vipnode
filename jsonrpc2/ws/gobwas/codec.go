package gobwas

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/vipnode/vipnode/jsonrpc2"
)

type rwc struct {
	io.Reader
	io.Writer
	io.Closer
}

// WebSocketDial returns a Codec that wraps a client-side connection with JSON
// encoding and decoding.
func WebSocketDial(ctx context.Context, url string) (jsonrpc2.Codec, error) {
	conn, _, _, err := ws.Dial(ctx, url)
	if err != nil {
		return nil, err
	}

	return clientWebSocketCodec(conn), nil
}

func clientWebSocketCodec(conn net.Conn) jsonrpc2.Codec {
	r := wsutil.NewReader(conn, ws.StateClientSide)
	w := wsutil.NewWriter(conn, ws.StateClientSide, ws.OpBinary)
	return &wsCodec{
		inner: jsonrpc2.IOCodec(rwc{r, w, conn}),
		r:     r,
		w:     w,
	}
}

// serverWebSocketCodec returns a server-side Codec that wraps JSON encoding and
// decoding over a websocket connection.
func serverWebSocketCodec(conn net.Conn) jsonrpc2.Codec {
	r := wsutil.NewReader(conn, ws.StateServerSide)
	w := wsutil.NewWriter(conn, ws.StateServerSide, ws.OpBinary)
	return &wsCodec{
		inner: jsonrpc2.IOCodec(rwc{r, w, conn}),
		r:     r,
		w:     w,
	}
}

var _ jsonrpc2.Codec = &wsCodec{}

type wsCodec struct {
	inner jsonrpc2.Codec
	r     *wsutil.Reader
	w     *wsutil.Writer
}

func (codec *wsCodec) ReadMessage() (*jsonrpc2.Message, error) {
	_, err := codec.r.NextFrame()
	if err != nil {
		return nil, err
	}
	return codec.inner.ReadMessage()
}

func (codec *wsCodec) WriteMessage(msg *jsonrpc2.Message) error {
	err := codec.inner.WriteMessage(msg)
	if err != nil {
		return err
	}
	if err = codec.w.Flush(); err != nil {
		return err
	}
	return nil
}

func (codec *wsCodec) Close() error {
	return codec.inner.Close()
}

// Upgrader upgrades an HTTP request to a WebSocket request and returns the
// appropriate jsonrpc2 codec.
type Upgrader struct {
	Upgrader ws.HTTPUpgrader
}

func (u *Upgrader) Upgrade(r *http.Request, w http.ResponseWriter, h http.Header) (jsonrpc2.Codec, error) {
	conn, _, _, err := u.Upgrader.Upgrade(r, w)
	if err != nil {
		return nil, err
	}
	return serverWebSocketCodec(conn), nil
}
