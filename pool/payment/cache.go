package payment

import (
	"math/big"
	"sync"
	"time"

	"github.com/vipnode/vipnode/pool/store"
)

type balanceItem struct {
	value  *big.Int
	expire time.Time
}

// FIXME: Sort of a DOS vector I suppose, since people can fill the memory with
// a bunch of accounts. Need a cleanup goroutine?
type balanceCache struct {
	Getter func(account store.Account) (*big.Int, error)

	mu          sync.Mutex
	expireAfter time.Duration
	cache       map[store.Account]balanceItem
	nowFn       func() time.Time // For testing override
}

func (b *balanceCache) now() time.Time {
	if b.nowFn != nil {
		return b.nowFn()
	}
	return time.Now()
}

func (b *balanceCache) Reset(expireAfter time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireAfter = expireAfter
	b.cache = nil
}

func (b *balanceCache) Set(account store.Account, amount *big.Int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cache == nil {
		b.cache = map[store.Account]balanceItem{}
	}
	expire := time.Time{}
	if b.expireAfter != 0 {
		expire = b.now().Add(b.expireAfter)
	}
	b.cache[account] = balanceItem{
		amount,
		expire,
	}
}

func (b *balanceCache) Get(account store.Account) (*big.Int, error) {
	b.mu.Lock()
	if b.cache == nil {
	} else if r, ok := b.cache[account]; ok {
		// Hit
		if r.expire.IsZero() || b.now().Before(r.expire) {
			// Not expired
			b.mu.Unlock()
			return r.value, nil
		}
		// Clear expired
		delete(b.cache, account)
	}
	getter := b.Getter
	b.mu.Unlock()

	// Miss (outside of cache lock)
	val, err := getter(account)
	if err != nil {
		return nil, err
	}
	b.Set(account, val)
	return val, nil
}
