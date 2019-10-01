package ethnode

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// ParseNodeURI takes an "enode://..." string (Ethereum Node URI) and parses it to
// the relevant components.
func ParseNodeURI(enode string) (*NodeURI, error) {
	if !strings.HasPrefix(enode, "enode://") && !strings.Contains(enode, "://") {
		enode = "enode://" + enode
	}

	u, err := url.Parse(enode)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "enode" {
		return nil, errors.New("invalid enode scheme: " + u.Scheme)
	}

	r := NodeURI(*u)
	return &r, nil
}

// NodeURI is a representation of an Ethereum Node URI, represented as an
// "enode://" string
type NodeURI url.URL

// ID returns the EnodeID
func (u *NodeURI) ID() string {
	if u.Scheme == "" {
		// "<ID>"
		return u.Path
	}
	if u.User == nil {
		// "enode://<ID>"
		return u.Host
	}
	// "enode://<ID>@<Host>"
	return u.User.Username()
}

// HostPort returns the remote host:port component required to connect to the
// node, if included in the enode URI. If no remote address is provided, then
// empty string is returned.
func (u *NodeURI) RemoteAddress() string {
	if u.User == nil {
		return ""
	}

	// Future versions of Ethereum might support DNS-resolved hostnames instead
	// of IPs, so we avoid stripping out hosts.
	if hostname := (*url.URL)(u).Hostname(); hostname == "localhost" {
		return ""
	} else if ip := net.ParseIP(hostname); ip.IsUnspecified() || ip.IsLoopback() {
		return ""
	}

	return u.Host
}
