package payment

import (
	"errors"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/vipnode/vipnode/pool/store"
)

func ContractPayment(backend bind.ContractBackend, contractAddress common.Address) (*contractPayment, error) {
	return nil, errors.New("not implemented yet")
}

// ContractPayment uses the github.com/vipnode/vipnode-contract smart contract for payment.
type contractPayment struct {
	backend bind.ContractBackend
}

func (p *contractPayment) Verify(sig []byte, account store.Account, nodeID store.NodeID) error {
	return errors.New("ContractPayment does not implement Verify")
}

func (p *contractPayment) Withdraw(account store.Account, amount store.Amount) (tx string, err error) {
	return "", errors.New("ContractPayment does not implement Withdraw")
}
