package request

import (
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/vipnode/ethboot/forked/discv5"
)

func getTestingKey(t *testing.T) *ecdsa.PrivateKey {
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

func TestKeyConversion(t *testing.T) {
	// This is mostly self-serving documentation for how to do the conversion
	privkey := getTestingKey(t)

	nodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	node, err := discv5.HexID(nodeID)
	if err != nil {
		t.Fatal(err)
	}
	pubkey, err := node.Pubkey()
	if err != nil {
		t.Fatal(err)
	}
	nodeID2 := discv5.PubkeyID(pubkey).String()
	if nodeID != nodeID2 {
		t.Errorf("re-encoded pubkey does not match:\n  %s !=\n  %s", nodeID, nodeID2)
	}

	hashed := crypto.Keccak256([]byte("foo"))
	sig, err := crypto.Sign(hashed, privkey)
	if err != nil {
		t.Fatalf("failed to sign: %s", err)
	}

	pubkeyBytes := crypto.FromECDSAPub(pubkey)
	if ok := crypto.VerifySignature(pubkeyBytes, hashed, sig[:64]); !ok {
		t.Errorf("failed to verify: %q", sig)
	}
}

func TestRequestSigning(t *testing.T) {
	privkey := getTestingKey(t)
	nodeID := discv5.PubkeyID(&privkey.PublicKey).String()

	request := struct {
		method string
		nodeID string
		nonce  int
		args   []interface{}
	}{
		"somemethod",
		nodeID,
		42,
		[]interface{}{
			"foo",
			1234,
		},
	}

	want := fmt.Sprintf(`somemethod["%s",42,"foo",1234]`, nodeID)
	got, err := assemble(request.method, request.nodeID, request.nonce, request.args...)
	if err != nil {
		t.Fatalf("failed to assemble: %s", err)
	}
	if string(got) != want {
		t.Errorf("got:\n\t%q\nwant:\n\t%q", got, want)
	}

	sig, err := Sign(privkey, request.method, request.nodeID, request.nonce, request.args...)
	if err != nil {
		t.Fatalf("failed to sign: %s", err)
	}

	err = Verify(sig, request.method, request.nodeID, request.nonce, request.args...)
	if err == ErrBadSignature {
		t.Errorf("bad signature: %s", sig)
	} else if err != nil {
		t.Errorf("failed to verify: %s", err)
	}

	// Change the request a bit and see that it fails
	err = Verify(sig, "newmethod", request.nodeID, request.nonce, request.args...)
	if err != ErrBadSignature {
		t.Errorf("expected bad signature, got: %s", err)
	}
}
