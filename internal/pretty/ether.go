package pretty

import (
	"fmt"
	"math/big"
	"strings"
	"unicode"
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

// ParseEther takes a string like "2 eth" and converts it to the wei equivalent.
func ParseEther(s string) (*big.Int, error) {
	var splitPos int
	for pos, ch := range s {
		if !unicode.IsNumber(ch) && ch != '-' && ch != '.' {
			splitPos = pos
			break
		}
	}

	if splitPos == 0 {
		// Must be wei
		if r, ok := new(big.Int).SetString(s, 0); ok {
			return r, nil
		}
		return nil, fmt.Errorf("failed to parse ether value: %q", s)
	}

	number, unit := s[:splitPos], s[splitPos:]

	n, ok := new(big.Rat).SetString(number)
	if !ok {
		return nil, fmt.Errorf("failed to parse ether value: %q", s)
	}

	mul := new(big.Rat)
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "wei":
	case "kwei", "babbage":
		mul.SetInt64(1e3)
	case "mwei", "lovelace":
		mul.SetInt64(1e6)
	case "gwei", "shannon":
		mul.SetInt64(1e9)
	case "microether", "szabo":
		mul.SetInt64(1e12)
	case "milliether", "finney":
		mul.SetInt64(1e15)
	case "ether", "eth":
		mul.SetInt64(1e18)
	default:
		return nil, fmt.Errorf("failed to parse ether unit: %q", s)
	}

	n.Mul(n, mul)
	return new(big.Int).Div(n.Num(), n.Denom()), nil
}
