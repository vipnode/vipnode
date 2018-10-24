package payment

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/vipnode/vipnode-contract/go/vipnodepool"
	"github.com/vipnode/vipnode/pool/store"
)

var zeroInt = &big.Int{}

// ErrDepositTimelocked is returned when a balance is checked but the deposit
// is timelocked.
var ErrDepositTimelocked = errors.New("deposit is timelocked")

// ContractPayment returns an abstraction around a vipnode pool payment contract.
func ContractPayment(address common.Address, backend bind.ContractBackend) (*contractPayment, error) {
	contract, err := vipnodepool.NewVipnodePool(address, backend)
	if err != nil {
		return nil, err
	}
	return &contractPayment{
		contract: contract,
		backend:  backend,
	}, nil
}

// ContractPayment uses the github.com/vipnode/vipnode-contract smart contract for payment.
type contractPayment struct {
	contract *vipnodepool.VipnodePool
	backend  bind.ContractBackend
}

func (p *contractPayment) DepositBalance(account store.Account) (store.Amount, error) {
	r, err := p.contract.Clients(&bind.CallOpts{Pending: true}, common.HexToAddress(string(account)))
	if r.TimeLocked.Cmp(zeroInt) != 0 {
		return 0, ErrDepositTimelocked
	}
	return store.Amount(r.Balance.Int64()), err
}

func (p *contractPayment) Withdraw(account store.Account, amount store.Amount) (tx string, err error) {
	return "", errors.New("ContractPayment has not implemented Withdraw")
}
