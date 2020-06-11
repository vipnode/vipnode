package memory

import (
	"testing"

	"github.com/vipnode/vipnode/v2/pool/store"
)

func TestMemoryStore(t *testing.T) {
	t.Run("MemoryStore", func(t *testing.T) {
		store.TestSuite(t, func() store.Store {
			return New()
		})
	})
}
