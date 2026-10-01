package valueobject

import "dracma/pix/internal/domain/errs"

type Decision string

const (
	Approved Decision = "APPROVED"
	Declined Decision = "DECLINED"
)

func ParseDecision(raw string) (Decision, error) {
	switch Decision(raw) {
	case Approved, Declined:
		return Decision(raw), nil
	default:
		return "", errs.ErrInvalid
	}
}

func (d Decision) String() string { return string(d) }

type Reason string

const (
	ReasonNone              Reason = ""
	ReasonInvalidAmount     Reason = "INVALID_AMOUNT"
	ReasonInsufficientFunds Reason = "INSUFFICIENT_FUNDS"
	ReasonBalanceOverflow   Reason = "BALANCE_OVERFLOW"
	ReasonAccountNotFound   Reason = "ACCOUNT_NOT_FOUND"
)

func ParseReason(raw string) (Reason, error) {
	switch Reason(raw) {
	case ReasonNone, ReasonInvalidAmount, ReasonInsufficientFunds, ReasonBalanceOverflow, ReasonAccountNotFound:
		return Reason(raw), nil
	default:
		return "", errs.ErrInvalid
	}
}

func (r Reason) String() string { return string(r) }

type Settlement string

const (
	SettlementInternal Settlement = "INTERNAL"
	SettlementPending  Settlement = "PENDING"
	SettlementSettled  Settlement = "SETTLED"
	SettlementReversed Settlement = "REVERSED"
)

func ParseSettlement(raw string) (Settlement, error) {
	switch Settlement(raw) {
	case SettlementInternal, SettlementPending, SettlementSettled, SettlementReversed:
		return Settlement(raw), nil
	default:
		return "", errs.ErrInvalid
	}
}

func (s Settlement) String() string { return string(s) }
