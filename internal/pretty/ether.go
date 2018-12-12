package pretty

import (
	"math/big"
	"strings"
)

var ethInWei = big.NewInt(1e18)
var ethInGwei = big.NewInt(1e9)

var boundary = big.NewInt(1e4)
var gweiBoundary = new(big.Int).Div(ethInGwei, boundary)
var ethBoundary = new(big.Int).Div(ethInWei, boundary)

// Ether implements a String() formatter that dynamically adjusts to a closer
// denomation based on the value.
type Ether big.Int

func (e Ether) String() string {
	i := (big.Int)(e)
	unit := "wei"
	denom := big.NewInt(1)

	if i.CmpAbs(ethBoundary) >= 0 {
		unit = "ether"
		denom = ethInWei
	} else if i.CmpAbs(gweiBoundary) >= 0 {
		unit = "gwei"
		denom = ethInGwei
	}
	s := new(big.Rat).SetFrac(&i, denom).FloatString(4)
	s = strings.TrimRight(s, "0.")
	if s == "" {
		s = "0"
	}
	return s + " " + unit
}
