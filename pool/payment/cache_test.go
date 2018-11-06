package payment

import (
	"math/big"
	"testing"
	"time"

	"github.com/vipnode/vipnode/pool/store"
)

func TestBalanceCache(t *testing.T) {
	val := big.NewInt(42)
	getter := func(account store.Account) (*big.Int, error) {
		return val, nil
	}
	now := time.Now()
	cache := balanceCache{
		Getter: getter,
		nowFn:  func() time.Time { return now },
	}

	assertKey := func(key store.Account, want int64) {
		t.Helper()
		got, err := cache.Get(key)
		if err != nil {
			t.Error(err)
		}
		if wantBig := big.NewInt(want); wantBig.Cmp(got) != 0 {
			t.Errorf("got: %q; want: %q", got, wantBig)
		}
	}

	assertKey("foo", 42)
	val = big.NewInt(69)

	assertKey("bar", 69)
	assertKey("foo", 42)

	// Reset and expire after 5s moving forward
	cache.Reset(time.Second * 5)
	assertKey("foo", 69)
	val = big.NewInt(100)
	assertKey("foo", 69)
	now = now.Add(time.Second * 10)
	assertKey("foo", 100)
}
