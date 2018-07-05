package request

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/vipnode/ethboot/forked/discv5"
)

// ErrBadSignature is returned when the signature does not verify the payload.
var ErrBadSignature = errors.New("bad signature")

// assemble encodes an RPC request for signing or verifying.
func assemble(method string, nodeID string, nonce int, args ...interface{}) ([]byte, error) {
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

// hash will assemble and hash an RPC request for signing and verifying.
func hash(method string, nodeID string, nonce int, args ...interface{}) ([]byte, error) {
	req, err := assemble(method, nodeID, nonce, args...)
	if err != nil {
		return nil, err
	}

	// Keccak256 does not require a MAC: https://crypto.stackexchange.com/questions/17735/is-hmac-needed-for-a-sha-3-based-mac
	hashed := crypto.Keccak256(req)
	return hashed, nil
}

// Sign produces a base64-encoded signature from an RPC request.
func Sign(prv *ecdsa.PrivateKey, method string, nodeID string, nonce int, args ...interface{}) (string, error) {
	hashed, err := hash(method, nodeID, nonce, args...)
	if err != nil {
		return "", err
	}

	sigbytes, err := crypto.Sign(hashed, prv)
	if err != nil {
		return "", err
	}

	out := base64.StdEncoding.EncodeToString(sigbytes)
	return out, nil
}

// Verify checks a base64-encoded signature of an RPC request.
func Verify(sig string, method string, nodeID string, nonce int, args ...interface{}) error {
	// Convert nodeID to public key
	node, err := discv5.HexID(nodeID)
	if err != nil {
		return err
	}
	pubkey, err := node.Pubkey()
	if err != nil {
		return err
	}

	sigbytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return err
	}

	hashed, err := hash(method, nodeID, nonce, args...)
	if err != nil {
		return err
	}

	// XXX: This is hacky, ideally crypto.VerifySignature(...) should work but
	// the types we're using here aren't compatible. Need to rework this a bit.
	signkey, err := crypto.SigToPub(hashed, sigbytes)
	if err != nil {
		return err
	}
	if crypto.PubkeyToAddress(*signkey) != crypto.PubkeyToAddress(*pubkey) {
		return ErrBadSignature
	}

	return nil // Success
}
