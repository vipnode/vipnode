package request

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// AddressRequest represents the components of an RPC request signed by an Ethereum address.
type AddressRequest struct {
	Method    string
	Address   string // Hex-encoded (e.g. public wallet or contract address)
	Nonce     int64
	ExtraArgs []interface{}
}

func (r AddressRequest) hash() ([]byte, error) {
	req, err := assemble(r.Method, r.Address, r.Nonce, r.ExtraArgs...)
	if err != nil {
		return nil, err
	}

	// Address-signed messages have the form:
	//   keccak256("\x19Ethereum Signed Message:\n"${message length}${message})
	req = append([]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(req))), req...)

	// Keccak256 does not require a MAC: https://crypto.stackexchange.com/questions/17735/is-hmac-needed-for-a-sha-3-based-mac
	hashed := crypto.Keccak256(req)
	return hashed, nil
}

// SignedArgs returns a slice of arguments with the first element containing
// the signature for the remaining arguments. This is a convenient form factor
// for doing signed RPC calls.
func (r AddressRequest) SignedArgs(privkey *ecdsa.PrivateKey) ([]interface{}, error) {
	sig, err := r.Sign(privkey)
	if err != nil {
		return nil, err
	}

	args := make([]interface{}, 0, 3+len(r.ExtraArgs))
	args = append(args, sig, r.Address, r.Nonce)
	args = append(args, r.ExtraArgs...)
	return args, nil
}

// Sign produces a hex-encoded signature of the request.
func (r AddressRequest) Sign(privkey *ecdsa.PrivateKey) (string, error) {
	hashed, err := r.hash()
	if err != nil {
		return "", err
	}

	sigbytes, err := crypto.Sign(hashed, privkey)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sigbytes), nil
}

// Verify validates this request against a base64-encoded signature (presumably
// produced by Request.Sign).
func (r AddressRequest) Verify(sig string) error {
	if strings.HasPrefix(sig, "0x") {
		sig = sig[2:]
	}
	sigbytes, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("failed to decode sig: %s", err)
	}

	if len(sigbytes) != 65 {
		return fmt.Errorf("signature wrong length: %d", len(sigbytes))
	}
	if sigbytes[64] == 27 || sigbytes[64] == 28 {
		// The sig is of the form [R || S || V] (65 bytes) but V is either 27
		// or 28 "for legacy reasons", and we want it to be 0 or 1. ¯\_(ツ)_/¯
		sigbytes[64] -= 27
	}

	hashed, err := r.hash()
	if err != nil {
		return fmt.Errorf("failed to hash request: %s", err)
	}

	pubkey, err := crypto.SigToPub(hashed, sigbytes)
	if err != nil {
		return err
	}

	address := crypto.PubkeyToAddress(*pubkey).String()
	if strings.ToLower(address) != strings.ToLower(r.Address) {
		return ErrBadSignature
	}
	return nil
}
