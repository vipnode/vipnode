package keygen

import (
	"crypto/ecdsa"
	"encoding/base64"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func HardcodedKey(t *testing.T) *ecdsa.PrivateKey {
	const hardcodedKey = `Qz7wmX+MXfvY85IsJMnMFd8fOI4msOT24bp6Iw/NuPo=`
	data, err := base64.StdEncoding.DecodeString(hardcodedKey)
	if err != nil {
		t.Fatal(err)
	}
	privkey, err := crypto.ToECDSA(data)
	if err != nil {
		t.Fatal(err)
	}
	return privkey
}
