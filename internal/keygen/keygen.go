package keygen

import (
	"crypto/ecdsa"
	"encoding/base64"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

var hardcodedKeys = []string{
	`Qz7wmX+MXfvY85IsJMnMFd8fOI4msOT24bp6Iw/NuPo=`,
	`OX9lnxz+fWNmEEBXCKfEmEsh5oGhCXXdHJBqilgZPNc=`,
	`pebDrvrc9iHxN8k7YJva6bvr6Mzimb9ZbFrGpVy/Wb0=`,
	`cl3X1He2rNZiXbsii3M9zxBTi9B7gB1Tqgk6u5rMytE=`,
}

func HardcodedKey(t *testing.T) *ecdsa.PrivateKey {
	return HardcodedKeyIdx(t, 0)
}

func HardcodedKeyIdx(t *testing.T, idx int) *ecdsa.PrivateKey {
	if idx >= len(hardcodedKeys)-1 {
		t.Fatalf("keygen.HardcodedKey: Ran out of hardcoded keys")
	}
	data, err := base64.StdEncoding.DecodeString(hardcodedKeys[idx])
	if err != nil {
		t.Fatalf("keygen.HardcodedKey: %s", err)
	}
	privkey, err := crypto.ToECDSA(data)
	if err != nil {
		t.Fatalf("keygen.HardcodedKey: %s", err)
	}
	return privkey
}

func NewKey(t *testing.T) *ecdsa.PrivateKey {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}
