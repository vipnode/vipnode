package pool

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/vipnode/vipnode/internal/pretty"
)

// normalizeNodeURI takes an enode:// URI string and some defaults to replace any missing components.
// FIXME: Replace with ethnode.ParseNodeURI
func normalizeNodeURI(nodeURI, nodeID, defaultHost, defaultPort string) (string, error) {
	host, port := defaultHost, defaultPort

	if nodeURI != "" {
		// Confirm that nodeURI matches nodeID
		uri, err := url.Parse(nodeURI)
		if err != nil {
			return "", err
		}

		if h := uri.Hostname(); h != "::" && h != "" {
			host = h
		}

		if p := uri.Port(); p != "" {
			port = p
		}

		if username := uri.User.Username(); username != "" && username != nodeID {
			return "", fmt.Errorf("nodeID %q does not match nodeURI: %s", pretty.Abbrev(nodeID), nodeURI)
		}
	}

	if host == "" || host == "[::]" {
		return "", errors.New("NodeURI is missing host")
	}

	u := &url.URL{
		Scheme: "enode",
		User:   url.User(nodeID),
		Host:   host + ":" + port,
	}
	return u.String(), nil
}
