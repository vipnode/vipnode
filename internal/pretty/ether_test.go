package pretty

import (
	"math/big"
	"testing"
)

func TestEther(t *testing.T) {
	cases := []struct {
		Amount *big.Int
		Want   string
	}{
		{
			Amount: big.NewInt(0),
			Want:   "0 wei",
		},
		{
			Amount: big.NewInt(5000000000),
			Want:   "5 gwei",
		},
		{
			Amount: big.NewInt(500000),
			Want:   "0.0005 gwei",
		},
		{
			Amount: new(big.Int).Mul(ethInWei, big.NewInt(15)),
			Want:   "15 ether",
		},
	}

	for i, tc := range cases {
		got := Ether(*tc.Amount).String()
		if got != tc.Want {
			t.Errorf("case #%d: got: %q; want %q", i, got, tc.Want)
		}
	}
}
