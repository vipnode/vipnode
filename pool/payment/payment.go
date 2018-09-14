// XXX: Rough draft, do not use yet.
package payment

import "github.com/vipnode/vipnode/pool/store"

// Payment provider assumes all verification happens outside of it. The
// provider just does the operations that are requested of it, with no
// additional security.
type Payment interface {
	// Verify confirms that sig is a signature produced by the account's owner,
	// used for authorizing nodeID to use its balance.
	Verify(sig string, account store.Account, nodeID store.NodeID) error

	// Withdraw transfers amount to the account. Fees are subtracted from the
	// amount. Balance is not verified.
	Withdraw(account store.Account, amount store.Amount) (tx string, err error)
}
