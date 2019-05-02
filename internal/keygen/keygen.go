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
	`+0HMUDyMBxFbNat59Vl6Sg+3EcVgiXt1y+JNtnjKb18=`,
	`exk5qBaBxCex5A5Rx9/0qokyz4tu2aAwCJNzsEtIXnk=`,
	`eqS1AIeHCa6xA4WWE2Q8hooTFVyUtQasJeDH3TSxkGU=`,
	`DvqsYyCf9KmbmcC34hTwIjzVWSJnXKTxSxJAlKABSBs=`,
	`gciM+wfV80vZxh5RAevJzcRBJyGkoxvmWRxyaNkOlpc=`,
	`q8Ye3Cq3+/JKASV1KrSKy6wLLwjUdkffM4sBtKcOyPQ=`,
	`dAoxmCIoohJePxhAlwL1res/Ict7U+ZhpTHlSXD6zt4=`,
	`bx7mVtX5RV8BgsMmWqGs9aPnSNaSRfjB0sPBBKMfex4=`,
	`wRQRSoYNlu/7SO75LJYieZv25TlEgH/NE8fyq/OWUQQ=`,
	`O5oR0U9/xwfmOQZenln+Vr5rUZe3ooCs2ZFkKYx9VDQ=`,
	`MhQtBdZogSKlwB0S7UTffnVrT6G/LXncFiaCaCQBTrQ=`,
	`1raBg6tKGFSJ3JVV+cmwezOdNKYRAG1/CwMfw8SdHxQ=`,
	`hyHctcYKirNqWFPyU46aTsU1DVOW6SbBH8IR+jLOIa0=`,
	`vQzyvauJh+RZojBXy1jzziBr8PBWP+rV3Qefx0ANzsU=`,
	`qHC5oCzkoXfYEzNmkS/CN4aJTKUHI1/NoWW9u8AbbQM=`,
	`WalBCrYbrAA4Npnaez6nDaEIyX4NMyC6GGpY7Y0iMoM=`,
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

func EncodeKey(key *ecdsa.PrivateKey) string {
	data := crypto.FromECDSA(key)
	return base64.StdEncoding.EncodeToString(data)
}

func NewKey(t *testing.T) *ecdsa.PrivateKey {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}
