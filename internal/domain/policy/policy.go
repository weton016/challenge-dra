package policy

import (
	"math"

	"dracma/pix/internal/domain/valueobject"
)

func Decide(source, destination, amount int64) valueobject.Reason {
	if amount <= 0 || source < 0 || destination < 0 {
		return valueobject.ReasonInvalidAmount
	}
	if source < amount {
		return valueobject.ReasonInsufficientFunds
	}
	if destination > math.MaxInt64-amount {
		return valueobject.ReasonBalanceOverflow
	}
	return valueobject.ReasonNone
}

func Target(source, destination int64, external bool) int64 {
	if external {
		return -(source%32 + 1)
	}
	return destination
}
