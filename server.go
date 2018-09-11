package main

import (
	"io"
	"net/http"

	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/jsonrpc2/ws"
)

type wsHandler interface {
	Upgrade(*http.Request, http.ResponseWriter, http.Header) (jsonrpc2.Codec, error)
}

type server struct {
	service  jsonrpc2.Server
	http     jsonrpc2.HTTPServer
	ws       ws.Upgrader
	debugLog bool
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// Assume RPC over HTTP
		s.http.ServeHTTP(w, r)
	case http.MethodGet:
		// Assume WebSocket upgrade request
		codec, err := s.ws.Upgrade(r, w, nil)
		if err != nil {
			logger.Debugf("websocket upgrade error from %s: %s", r.RemoteAddr, err)
		}
		if s.debugLog {
			codec = jsonrpc2.DebugCodec(r.RemoteAddr, codec)
		}
		remote := &jsonrpc2.Remote{
			Codec:  codec,
			Server: &s.service,
			Client: &jsonrpc2.Client{},

			PendingLimit:   50,
			PendingDiscard: 10,
		}
		if err := remote.Serve(); err != nil && err != io.EOF {
			logger.Warningf("jsonrpc2.Remote.Serve() error: %s", err)
		}
	default:
		http.Error(w, "unsupported method", http.StatusUnsupportedMediaType)
	}
}
