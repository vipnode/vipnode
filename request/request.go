package request

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

// ErrBadSignature is returned when the signature does not verify the payload.
var ErrBadSignature = errors.New("bad signature")

// Assemble encodes an RPC request for signing or verifying.
func Assemble(method string, nodeID string, nonce int, args ...interface{}) ([]byte, error) {
	// The signed payload is the method concatenated with the JSON-encoded arg array.
	// Example: foo["1234abcd",2]
	var payload []interface{}
	payload = append(payload, nodeID, nonce)
	payload = append(payload, args...)

	out, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Prepend method
	return append([]byte(method), out...), nil
}

// Sign produces a base64-encoded signature from an RPC request.
func Sign(prv *ecdsa.PrivateKey, method string, nodeID string, nonce int, args ...interface{}) (string, error) {
	req, err := Assemble(method, nodeID, nonce, args...)
	if err != nil {
		return "", err
	}

	// Keccak256 does not require a MAC: https://crypto.stackexchange.com/questions/17735/is-hmac-needed-for-a-sha-3-based-mac
	hashed := crypto.Keccak256(req)
	sigbytes, err := crypto.Sign(hashed, prv)
	if err != nil {
		return "", err
	}

	out := base64.StdEncoding.EncodeToString(sigbytes)
	return out, nil
}

// Verify checks a base64-encoded signature of an RPC request.
func Verify(sig string, method string, nodeID string, nonce int, args ...interface{}) error {
	pubkey, err := base64.StdEncoding.DecodeString(nodeID)
	if err != nil {
		return err
	}

	req, err := Assemble(method, nodeID, nonce, args...)
	if err != nil {
		return err
	}

	sigbytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return err
	}

	hashed := crypto.Keccak256(req)
	if !crypto.VerifySignature(pubkey, hashed, sigbytes) {
		return ErrBadSignature
	}
	// Success!
	return nil
}
