package policy

import "math/big"

// Movement is how a ledger transfer touches one account.
type Movement int

const (
	Posted   Movement = iota // single-phase transfer
	Reserved                 // pending transfer
	Captured                 // pending transfer posted
	Released                 // pending transfer voided
)

// Available is what an account may still spend: posted credits minus posted
// and reserved debits. Pending credits are not spendable yet.
func Available(creditsPosted, debitsPosted, debitsPending *big.Int) (int64, bool) {
	left := new(big.Int).Sub(creditsPosted, debitsPosted)
	left.Sub(left, debitsPending)
	if left.Sign() < 0 || !left.IsInt64() {
		return 0, false
	}
	return left.Int64(), true
}

// Delta is the change a movement causes in Available on the debit or credit side.
func Delta(movement Movement, debit bool, amount int64) int64 {
	switch movement {
	case Posted:
		if debit {
			return -amount
		}
		return amount
	case Reserved:
		if debit {
			return -amount
		}
	case Captured:
		if !debit {
			return amount
		}
	case Released:
		if debit {
			return amount
		}
	}
	return 0
}

func Target(source, destination int64, external bool) int64 {
	if external {
		return -(source%32 + 1)
	}
	return destination
}
