// Websocket implementation using Gorilla's Websocket library
package gorilla

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/vipnode/vipnode/jsonrpc2"
)

// WebSocketDial returns a Codec that wraps a client-side connection with JSON
// encoding and decoding.
func WebSocketDial(ctx context.Context, url string) (jsonrpc2.Codec, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return &wsCodec{conn: conn}, nil
}

var _ jsonrpc2.Codec = &wsCodec{}

func overrideEOF(err error) error {
	if err == nil {
		return nil
	}
	if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
		return err
	}
	return io.EOF
}

type wsCodec struct {
	muWrite sync.Mutex
	muRead  sync.Mutex
	conn    *websocket.Conn
}

func (codec *wsCodec) RemoteAddr() string {
	return codec.conn.RemoteAddr().String()
}

func (codec *wsCodec) ReadMessage() (*jsonrpc2.Message, error) {
	codec.muRead.Lock()
	defer codec.muRead.Unlock()
	var msg jsonrpc2.Message
	if err := codec.conn.ReadJSON(&msg); err != nil {
		return nil, overrideEOF(err)
	}
	return &msg, nil
}

func (codec *wsCodec) WriteMessage(msg *jsonrpc2.Message) error {
	codec.muWrite.Lock()
	defer codec.muWrite.Unlock()
	return overrideEOF(codec.conn.WriteJSON(msg))
}

func (codec *wsCodec) Close() error {
	return codec.conn.Close()
}

// Upgrader upgrades an HTTP request to a WebSocket request and returns the
// appropriate jsonrpc2 codec.
type Upgrader struct {
	websocket.Upgrader
}

func (u *Upgrader) Upgrade(r *http.Request, w http.ResponseWriter, h http.Header) (jsonrpc2.Codec, error) {
	conn, err := u.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return &wsCodec{conn: conn}, nil
}
