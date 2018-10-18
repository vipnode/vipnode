package request

import (
	"crypto/ecdsa"
	"encoding/base64"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discv5"
)

// NodeRequest represents the components of an RPC request signed by an Ethereum address.
type NodeRequest struct {
	Method    string
	NodeID    string // Hex-encoded (e.g. public wallet or contract address)
	Nonce     int64
	ExtraArgs []interface{}
}

func (r NodeRequest) hash() ([]byte, error) {
	req, err := assemble(r.Method, r.NodeID, r.Nonce, r.ExtraArgs...)
	if err != nil {
		return nil, err
	}

	// Keccak256 does not require a MAC: https://crypto.stackexchange.com/questions/17735/is-hmac-needed-for-a-sha-3-based-mac
	hashed := crypto.Keccak256(req)
	return hashed, nil
}

// SignedArgs returns a slice of arguments with the first element containing
// the signature for the remaining arguments. This is a convenient form factor
// for doing signed RPC calls.
func (r NodeRequest) SignedArgs(privkey *ecdsa.PrivateKey) ([]interface{}, error) {
	sig, err := r.Sign(privkey)
	if err != nil {
		return nil, err
	}

	args := make([]interface{}, 0, 3+len(r.ExtraArgs))
	args = append(args, sig, r.NodeID, r.Nonce)
	args = append(args, r.ExtraArgs...)
	return args, nil
}

// Sign produces a base64-encoded signature of the request.
func (r NodeRequest) Sign(privkey *ecdsa.PrivateKey) (string, error) {
	hashed, err := r.hash()
	if err != nil {
		return "", err
	}

	sigbytes, err := crypto.Sign(hashed, privkey)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(sigbytes), nil
}

// Verify validates this request against a base64-encoded signature (presumably
// produced by NodeRequest.Sign).
func (r NodeRequest) Verify(sig string) error {
	// Convert pubkey to public key
	node, err := discv5.HexID(r.NodeID)
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
	// crypto.Sign produces a signature in the form [R || S || V] (65 bytes)
	// where V is 0 or 1 and crypto.VerifySignature wants [R || S] (64 bytes).
	// ¯\_(ツ)_/¯
	sigbytes = sigbytes[:64]

	hashed, err := r.hash()
	if err != nil {
		return err
	}

	if crypto.VerifySignature(crypto.FromECDSAPub(pubkey), hashed, sigbytes) {
		// Success!
		return nil
	}

	return ErrBadSignature
}
