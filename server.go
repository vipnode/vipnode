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
	ws           ws.Upgrader
	debugLog     bool
	header       http.Header
	onDisconnect func(remote jsonrpc2.Service) error
	healthCheck  func(w io.Writer) error
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		if s.healthCheck == nil {
			http.Error(w, "detailed health check disabled", http.StatusOK)
		} else if err := s.healthCheck(w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

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
		defer codec.Close()

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

		if s.onDisconnect != nil {
			if err := s.onDisconnect(remote); err != nil {
				logger.Warningf("jsonrpc2.Service disconnect error: %s", err)
			}
		}
	default:
		http.Error(w, "unsupported method", http.StatusUnsupportedMediaType)
	}
}
