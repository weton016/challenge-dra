package entity

import (
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

type Entry struct {
	transaction valueobject.TransactionID
	account     valueobject.AccountID
	delta       int64
	sequence    int64
	balance     valueobject.Balance
}

func NewEntry(tx valueobject.TransactionID, account valueobject.AccountID, delta, sequence int64, balance valueobject.Balance) (Entry, error) {
	if tx.String() == "" {
		return Entry{}, errs.ErrInvalid
	}
	if account.Int64() == 0 {
		return Entry{}, errs.ErrInvalid
	}
	if delta == 0 {
		return Entry{}, errs.ErrInvalid
	}
	if sequence <= 0 {
		return Entry{}, errs.ErrInvalid
	}
	return Entry{transaction: tx, account: account, delta: delta, sequence: sequence, balance: balance}, nil
}

func Record(tx valueobject.TransactionID, account valueobject.AccountID, delta, sequence int64, balance valueobject.Balance) Entry {
	return Entry{transaction: tx, account: account, delta: delta, sequence: sequence, balance: balance}
}

func (e Entry) Transaction() valueobject.TransactionID { return e.transaction }
func (e Entry) Account() valueobject.AccountID         { return e.account }
func (e Entry) Delta() int64                           { return e.delta }
func (e Entry) Sequence() int64                        { return e.sequence }
func (e Entry) Balance() valueobject.Balance           { return e.balance }

func Opposite(debit, credit Entry, amount valueobject.Amount) error {
	if debit.delta != -amount.Minor() {
		return errs.ErrInvariant
	}
	if credit.delta != amount.Minor() {
		return errs.ErrInvariant
	}
	if debit.transaction != credit.transaction {
		return errs.ErrInvariant
	}
	if debit.account == credit.account {
		return errs.ErrInvariant
	}
	return nil
}
