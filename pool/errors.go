package pool

import (
	"fmt"
	"strings"
)

// NoHostNodesError is returned when the pool does not have any hosts available.
type NoHostNodesError struct {
	NumTried int
}

func (err NoHostNodesError) Error() string {
	if err.NumTried == 0 {
		return "no host nodes available"
	}
	return fmt.Sprintf("no available host nodes found after trying %d nodes", err.NumTried)
}

// VerifyFailedError is returned when a signature fails to verify. It embeds
// the underlying Cause.
type VerifyFailedError struct {
	Cause  error
	Method string
}

func (err VerifyFailedError) Error() string {
	return fmt.Sprintf("method %q failed to verify signature: %s", err.Method, err.Cause)
}

// RemoteHostErrors is used when a subset of RPC calls to hosts fail.
type RemoteHostErrors struct {
	Method string
	Errors []error
}

func (err RemoteHostErrors) Error() string {
	if len(err.Errors) == 0 {
		return "no remote host errors"
	}

	var s strings.Builder
	fmt.Fprintf(&s, "failed to call %q on %d hosts: ", err.Method, len(err.Errors))
	for i, e := range err.Errors {
		s.WriteString(e.Error())
		if i != len(err.Errors)-1 {
			s.WriteString("; ")
		}
	}
	return s.String()
}
