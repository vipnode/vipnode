package ws

import (
	"net/http"

	"github.com/vipnode/vipnode/v2/jsonrpc2"
)

// Upgrader takes an HTTP request, upgrades it to a websocket server and
// returns a codec interface. This allows switching between different websocket
// implementations.
type Upgrader interface {
	Upgrade(*http.Request, http.ResponseWriter, http.Header) (jsonrpc2.Codec, error)
}
