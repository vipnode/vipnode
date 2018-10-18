package request

import (
	"testing"
)

func TestRequestAddressVerify(t *testing.T) {
	req := AddressRequest{
		Method:    "pool_addNode",
		Address:   "0x961Aa96FebeE5465149a0787B03bFa14D8e9033F",
		Nonce:     1539807817010,
		ExtraArgs: []interface{}{"19b5013d24243a659bda7f1df13933bb05820ab6c3ebf6b5e0854848b97e1f7e308f703466e72486c5bc7fe8ed402eb62f6303418e05d330a5df80738ac974f6"},
	}
	sig := "0xf1637065051d63ec83576167b10d676540cb2c3f6c2996f50a6b8e465a9de6d756708d3d2351f857b9d965b2295c74d03a98f164d1e4b71ab5f169390cafa5391c"
	if err := req.Verify(sig); err != nil {
		t.Error(err)
	}
	req.Method = "wrong_method"
	if err := req.Verify(sig); err != ErrBadSignature {
		t.Errorf("expected BadSignature, got: %s", err)
	}
}
