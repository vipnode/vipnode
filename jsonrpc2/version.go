package jsonrpc2

import (
	"bytes"
	"fmt"
)

// FIXME: This is unused, not sure if it's a great idea to enforce versions
// this way. Leaning towards no...

// fixedVersion is an uninstantiated type which always marshals to our constant
// Version, and only succeeds to unmarshal if the version has the prefix "2.".
//
// Inspired by https://go-review.googlesource.com/c/tools/+/136675/1/internal/jsonrpc2/jsonrpc2.go#221
type fixedVersion struct{}

func (v fixedVersion) MarshalJSON() ([]byte, error) {
	return []byte(Version), nil
}

func (v fixedVersion) UnmarshalJSON(version []byte) error {
	if bytes.HasPrefix(version, []byte(`"2.`)) {
		return nil
	}
	return fmt.Errorf("unsupported version: %s", version)
}
