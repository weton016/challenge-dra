package aggregate

import (
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/policy"
	"dracma/pix/internal/domain/valueobject"
)

type Transfer struct {
	id          valueobject.TransactionID
	source      valueobject.AccountID
	destination valueobject.AccountID
	external    valueobject.ExternalRef
	amount      valueobject.Amount
	decision    valueobject.Decision
	reason      valueobject.Reason
	settlement  valueobject.Settlement
}

func NewInternalTransfer(id valueobject.TransactionID, source, destination valueobject.AccountID, amount valueobject.Amount) (Transfer, error) {
	if source.Int64() <= 0 {
		return Transfer{}, errs.ErrInvalid
	}
	if destination.Int64() <= 0 {
		return Transfer{}, errs.ErrInvalid
	}
	if source == destination {
		return Transfer{}, errs.ErrInvalid
	}
	return Transfer{id: id, source: source, destination: destination, amount: amount}, nil
}

func NewExternalTransfer(id valueobject.TransactionID, source valueobject.AccountID, external valueobject.ExternalRef, amount valueobject.Amount, localDestination int64) (Transfer, error) {
	if source.Int64() <= 0 {
		return Transfer{}, errs.ErrInvalid
	}
	if external.String() == "" {
		return Transfer{}, errs.ErrInvalid
	}
	if localDestination != 0 {
		return Transfer{}, errs.ErrInvalid
	}
	return Transfer{
		id:          id,
		source:      source,
		destination: valueobject.TechnicalID(policy.Target(source.Int64(), 0, true)),
		external:    external,
		amount:      amount,
	}, nil
}

func ParseTransfer(id string, source, destination int64, external string, amount int64) (Transfer, error) {
	tx, err := valueobject.NewTransactionID(id)
	if err != nil {
		return Transfer{}, err
	}
	src, err := valueobject.NewClientID(source)
	if err != nil {
		return Transfer{}, err
	}
	value, err := valueobject.NewAmount(amount)
	if err != nil {
		return Transfer{}, err
	}
	if external != "" {
		return parseExternal(tx, src, external, value, destination)
	}
	dest, err := valueobject.NewClientID(destination)
	if err != nil {
		return Transfer{}, err
	}
	return NewInternalTransfer(tx, src, dest, value)
}

func parseExternal(tx valueobject.TransactionID, source valueobject.AccountID, external string, amount valueobject.Amount, localDestination int64) (Transfer, error) {
	ref, err := valueobject.NewExternalRef(external)
	if err != nil {
		return Transfer{}, err
	}
	return NewExternalTransfer(tx, source, ref, amount, localDestination)
}

func RestoreTransfer(transfer Transfer, decision valueobject.Decision, reason valueobject.Reason, settlement valueobject.Settlement) (Transfer, error) {
	if decision != valueobject.Approved && decision != valueobject.Declined {
		return Transfer{}, errs.ErrInvalid
	}
	switch settlement {
	case valueobject.SettlementInternal, valueobject.SettlementPending, valueobject.SettlementSettled, valueobject.SettlementReversed:
	default:
		return Transfer{}, errs.ErrInvalid
	}
	transfer.decision = decision
	transfer.reason = reason
	transfer.settlement = settlement
	return transfer, nil
}

func (t Transfer) Refund() (Transfer, error) {
	if t.external.String() == "" {
		return Transfer{}, errs.ErrInvalid
	}
	id, err := valueobject.RefundID(t.id)
	if err != nil {
		return Transfer{}, err
	}
	return Transfer{id: id, source: t.destination, destination: t.source, amount: t.amount}, nil
}

func (t Transfer) Usable(source, destination Account) error {
	if !source.currency.Equal(valueobject.BRL()) {
		return errs.ErrInvalid
	}
	if !destination.currency.Equal(valueobject.BRL()) {
		return errs.ErrInvalid
	}
	if source.id != t.source {
		return errs.ErrInvalid
	}
	if destination.id != t.destination {
		return errs.ErrInvalid
	}
	return nil
}

func (t Transfer) WithReason(reason valueobject.Reason) Transfer {
	t.reason = reason
	t.settlement = valueobject.SettlementInternal
	t.decision = valueobject.Approved
	if reason != valueobject.ReasonNone {
		t.decision = valueobject.Declined
		return t
	}
	if t.external.String() != "" {
		t.settlement = valueobject.SettlementPending
	}
	return t
}

func (t Transfer) AcceptReplay(stored Transfer) error {
	if t.id != stored.id {
		return errs.ErrNotFound
	}
	if !t.samePayload(stored) {
		return errs.ErrConflict
	}
	return nil
}

func (t Transfer) samePayload(other Transfer) bool {
	if t.source != other.source {
		return false
	}
	if t.destination != other.destination {
		return false
	}
	if t.external != other.external {
		return false
	}
	if t.amount != other.amount {
		return false
	}
	return true
}

func (t Transfer) ID() valueobject.TransactionID      { return t.id }
func (t Transfer) Source() valueobject.AccountID      { return t.source }
func (t Transfer) Destination() valueobject.AccountID { return t.destination }
func (t Transfer) External() valueobject.ExternalRef  { return t.external }
func (t Transfer) Amount() valueobject.Amount         { return t.amount }
func (t Transfer) Decision() valueobject.Decision     { return t.decision }
func (t Transfer) Reason() valueobject.Reason         { return t.reason }
func (t Transfer) Settlement() valueobject.Settlement { return t.settlement }
