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
			Amount: big.NewInt(-10000000),
			Want:   "-0.01 gwei",
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

func TestParseEther(t *testing.T) {
	cases := []struct {
		Input   string
		Want    *big.Int
		IsError bool
	}{
		{
			Want:  big.NewInt(0),
			Input: "0 wei",
		},
		{
			Want:  big.NewInt(5000000000),
			Input: "5 gwei",
		},
		{
			Want:  big.NewInt(500000),
			Input: "0.0005 gwei",
		},
		{
			Want:  big.NewInt(-10000000),
			Input: "-0.01 gwei",
		},
		{
			Want:  new(big.Int).Mul(ethInWei, big.NewInt(15)),
			Input: "15 ether",
		},
	}

	for i, tc := range cases {
		got, err := ParseEther(tc.Input)
		if (err != nil) != tc.IsError {
			t.Errorf("case #%d: got error: %v; wanted IsError=%t", i, err, tc.IsError)
			continue
		}
		if got.Cmp(tc.Want) != 0 {
			t.Errorf("case #%d: got: %q; want %q (input: %q)", i, got, tc.Want, tc.Input)
		}
	}
}
