package jsonrpc2

import "encoding/json"

// Helpers for JSON parsing

// isArray returns true if the message is a JSON array (starts
// with '[', spaces skipped).
func isArray(raw json.RawMessage) bool {
	for _, b := range raw {
		if isSpace(b) {
			continue
		}
		return b == '['
	}
	return false
}

// isSpace returns true if the byte is considered a space in JSON syntax.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
