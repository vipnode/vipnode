package request

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
)

// ErrBadSignature is returned when the signature does not verify the payload.
var ErrBadSignature = errors.New("bad signature")

// assemble encodes an RPC request for signing or verifying.
func assemble(method string, pubkey string, nonce int64, args ...interface{}) ([]byte, error) {
	// The signed payload is the method concatenated with the JSON-encoded arg array.
	// Example: foo["1234abcd",2]
	var payload []interface{}
	payload = append(payload, pubkey, nonce)
	payload = append(payload, args...)

	out, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Prepend method
	return append([]byte(method), out...), nil
}

// Sign produces a base64-encoded signature from an RPC request.
// If the pubkey is a wallet address, then the "\x19Ethereum Signed Message:\n"
// prefix will be applied.
func Sign(privkey *ecdsa.PrivateKey, method string, pubkey string, nonce int64, args ...interface{}) (string, error) {
	if len(pubkey) <= 42 {
		return AddressRequest{
			Method:    method,
			Address:   pubkey,
			Nonce:     nonce,
			ExtraArgs: args,
		}.Sign(privkey)
	}
	return NodeRequest{
		Method:    method,
		NodeID:    pubkey,
		Nonce:     nonce,
		ExtraArgs: args,
	}.Sign(privkey)
}

// Verify checks a base64-encoded signature of an RPC request.
// If the pubkey is a wallet address, then the "\x19Ethereum Signed Message:\n"
// prefix will be applied.
func Verify(sig string, method string, pubkey string, nonce int64, args ...interface{}) error {
	if len(pubkey) <= 42 {
		return AddressRequest{
			Method:    method,
			Address:   pubkey,
			Nonce:     nonce,
			ExtraArgs: args,
		}.Verify(sig)
	}
	return NodeRequest{
		Method:    method,
		NodeID:    pubkey,
		Nonce:     nonce,
		ExtraArgs: args,
	}.Verify(sig)
}
