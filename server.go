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
	jsonrpc2.HTTPServer
	ws       ws.Upgrader
	debugLog bool
	header   http.Header
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// Assume RPC over HTTP
		for k, values := range s.header {
			for _, v := range values {
				w.Header().Set(k, v)
			}
		}
		s.HTTPServer.ServeHTTP(w, r)
	case http.MethodGet:
		if r.Header.Get("Connection") != "Upgrade" {
			http.Error(w, "incorrect vipnode api handshake", http.StatusBadRequest)
			return
		}
		// Assume WebSocket upgrade request
		codec, err := s.ws.Upgrade(r, w, nil)
		if err != nil {
			logger.Debugf("websocket upgrade error from %s: %s", r.RemoteAddr, err)
			return
		}
		if s.debugLog {
			codec = jsonrpc2.DebugCodec(r.RemoteAddr, codec)
		}
		remote := &jsonrpc2.Remote{
			Codec:  codec,
			Server: &s.HTTPServer.Server,
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
