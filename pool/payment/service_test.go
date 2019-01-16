package payment

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/pool/store/memory"
	"github.com/vipnode/vipnode/request"
)

type fakeContract struct {
	Balance map[store.Account]big.Int
	Paid    map[store.Account]big.Int
}

func (c *fakeContract) GetBalance(account store.Account) (*big.Int, error) {
	r := c.Balance[account]
	return &r, nil
}

func (c *fakeContract) OpSettle(account store.Account, paymentAmount *big.Int, newBalance *big.Int) (tx string, err error) {
	c.Balance[account] = *newBalance
	paid := c.Paid[account]
	paid = *(&paid).Add(&paid, paymentAmount)
	c.Paid[account] = paid
	txID := fmt.Sprintf("tx[balance=%d paid=%d]", &newBalance, &paid)
	return txID, nil
}

func TestPaymentWithdraw(t *testing.T) {
	feeFn := func(amount *big.Int) *big.Int {
		// Always remove 1000 as fee
		return amount.Sub(amount, big.NewInt(1000))
	}
	withdrawMin := big.NewInt(500)
	contract := &fakeContract{
		Balance: map[store.Account]big.Int{},
		Paid:    map[store.Account]big.Int{},
	}

	memStore := memory.New()
	p := PaymentService{
		NonceStore:   memStore,
		AccountStore: memStore,
		BalanceStore: memStore,

		Settle:      contract.OpSettle,
		WithdrawFee: feeFn,
		WithdrawMin: withdrawMin,
	}

	privkey := keygen.HardcodedKey(t)
	wallet := crypto.PubkeyToAddress(privkey.PublicKey).Hex()
	getSig := func(nonce int64) string {
		t.Helper()
		req := request.AddressRequest{
			Method:  "pool_withdraw",
			Address: wallet,
			Nonce:   nonce,
		}
		sig, err := req.Sign(privkey)
		if err != nil {
			t.Fatal(err)
		}
		return sig
	}
	nonce := time.Now().UnixNano()
	err := p.Withdraw(context.Background(), getSig(nonce), wallet, nonce)
	if _, ok := err.(WithdrawBalanceMinimumError); !ok {
		t.Errorf("expected WithdrawBalanceMinimumError error, got: %s", err)
	}

	if err := memStore.AddAccountBalance(store.Account(wallet), big.NewInt(5000)); err != nil {
		t.Fatal(err)
	}

	nonce++
	if err := p.Withdraw(context.Background(), getSig(nonce), wallet, nonce); err != nil {
		t.Error(err)
	}

	if got, want := contract.Paid[store.Account(wallet)], big.NewInt(4000); got.Cmp(want) != 0 {
		t.Errorf("wrong paid amount: got: %d; want %d", &got, want)
	}

	if got, want := contract.Balance[store.Account(wallet)], big.NewInt(0); got.Cmp(want) != 0 {
		t.Errorf("wrong balance amount: got: %d; want %d", &got, want)
	}

}
