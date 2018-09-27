package main

import (
	"errors"
	"fmt"
	"net/http"

	ws "github.com/vipnode/vipnode/jsonrpc2/ws/gorilla"
	"github.com/vipnode/vipnode/pool"
)

func runPool(options Options) error {
	if options.Pool.Store != "memory" {
		return errors.New("storage driver not implemented")
	}
	p := pool.New()
	p.Version = fmt.Sprintf("vipnode/pool/%s", Version)
	handler := &server{
		ws: &ws.Upgrader{},
	}
	if err := handler.Register("vipnode_", p); err != nil {
		return err
	}
	logger.Infof("Starting pool (version %s), listening on: %s", Version, options.Pool.Bind)
	// TODO: Add TLS support using autocert
	return http.ListenAndServe(options.Pool.Bind, handler)
}
