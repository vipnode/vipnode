package payment

import (
	"errors"

	"github.com/vipnode/vipnode/pool/store"
)

// NullPayment is a no-op payment service, used for testing and pro-bono services.
type NullPayment struct{}

func (p *NullPayment) Verify(sig string, account store.Account, nodeID store.NodeID) error {
	return errors.New("NullPayment does not implement Verify")
}

func (p *NullPayment) Withdraw(account store.Account, amount store.Amount) (tx string, err error) {
	return "", errors.New("NullPayment does not implement Withdraw")
}
