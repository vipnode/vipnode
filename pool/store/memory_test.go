package store

import (
	"testing"
)

func TestMemoryStore(t *testing.T) {
	t.Run("MemoryStore", func(t *testing.T) {
		TestSuite(t, func() Store {
			return MemoryStore()
		})
	})
}
