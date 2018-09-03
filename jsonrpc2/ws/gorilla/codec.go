// Websocket implementation using Gorilla's Websocket library
package gorilla

import (
	"context"
	"io"
	"log"
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

type wsCodec struct {
	muWrite sync.Mutex
	muRead  sync.Mutex
	conn    *websocket.Conn
}

func (codec *wsCodec) ReadMessage() (*jsonrpc2.Message, error) {
	codec.muRead.Lock()
	defer codec.muRead.Unlock()
	var msg jsonrpc2.Message
	if err := codec.conn.ReadJSON(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (codec *wsCodec) WriteMessage(msg *jsonrpc2.Message) error {
	codec.muWrite.Lock()
	defer codec.muWrite.Unlock()
	return codec.conn.WriteJSON(msg)
}

func (codec *wsCodec) Close() error {
	return codec.conn.Close()
}

func WebsocketHandler(srv *jsonrpc2.Server) http.HandlerFunc {
	upgrader := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade error from %s: %s", r.RemoteAddr, err)
			return
		}
		defer conn.Close()
		codec := &wsCodec{conn: conn}
		// DEBUG:
		//codec = jsonrpc2.DebugCodec(r.RemoteAddr, codec)
		remote := &jsonrpc2.Remote{
			Codec:  codec,
			Server: srv,
			Client: &jsonrpc2.Client{},

			// TODO: Unhardcode these?
			PendingLimit:   50,
			PendingDiscard: 10,
		}
		if err := remote.Serve(); err != nil && err != io.EOF {
			log.Printf("jsonrpc2.Remote.Serve() error: %s", err)
		}
	}
}
