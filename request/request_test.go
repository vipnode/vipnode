package request

import (
	"fmt"
	"testing"

	"github.com/vipnode/ethboot/forked/discv5"
	"github.com/vipnode/vipnode/internal/keygen"
)

func TestRequestSigning(t *testing.T) {
	privkey := keygen.HardcodedKey(t)
	nodeID := discv5.PubkeyID(&privkey.PublicKey).String()

	request := struct {
		method string
		nodeID string
		nonce  int64
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
