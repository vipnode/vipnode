package payment

import (
	"testing"

	"github.com/vipnode/vipnode/pool/store"
)

func TestPaymentWithdraw(t *testing.T) {
	memStore := store.MemoryStore()
	p := PaymentService{
		BalanceStore: memStore,
	}
}
