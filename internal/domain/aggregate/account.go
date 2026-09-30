package aggregate

import (
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

type Account struct {
	id       valueobject.AccountID
	balance  valueobject.Balance
	sequence int64
	currency valueobject.Currency
}

func NewAccount(id valueobject.AccountID, balance valueobject.Balance, sequence int64, currency valueobject.Currency) (Account, error) {
	if id.Int64() == 0 {
		return Account{}, errs.ErrInvalid
	}
	if sequence < 0 {
		return Account{}, errs.ErrInvalid
	}
	if !currency.Equal(valueobject.BRL()) {
		return Account{}, errs.ErrInvalid
	}
	return Account{id: id, balance: balance, sequence: sequence, currency: currency}, nil
}

func (a Account) ID() valueobject.AccountID      { return a.id }
func (a Account) Balance() valueobject.Balance   { return a.balance }
func (a Account) Sequence() int64                { return a.sequence }
func (a Account) Currency() valueobject.Currency { return a.currency }

func (a Account) Debit(amount valueobject.Amount, tx valueobject.TransactionID) (Account, entity.Entry, error) {
	if amount.Minor() <= 0 {
		return Account{}, entity.Entry{}, errs.ErrInvalid
	}
	if tx.String() == "" {
		return Account{}, entity.Entry{}, errs.ErrInvalid
	}
	return a.apply(-amount.Minor(), tx)
}

func (a Account) Credit(amount valueobject.Amount, tx valueobject.TransactionID) (Account, entity.Entry, error) {
	if amount.Minor() <= 0 {
		return Account{}, entity.Entry{}, errs.ErrInvalid
	}
	if tx.String() == "" {
		return Account{}, entity.Entry{}, errs.ErrInvalid
	}
	return a.apply(amount.Minor(), tx)
}

func (a Account) apply(delta int64, tx valueobject.TransactionID) (Account, entity.Entry, error) {
	if a.id.Int64() == 0 || tx.String() == "" || delta == 0 {
		return Account{}, entity.Entry{}, errs.ErrInvariant
	}
	next, ok := valueobject.Add(a.balance.Minor(), delta)
	if !ok {
		return Account{}, entity.Entry{}, errs.ErrInvariant
	}
	balance, ok := valueobject.TryBalance(next)
	if !ok {
		return Account{}, entity.Entry{}, errs.ErrInvariant
	}
	seq := a.sequence + 1
	if seq <= 0 {
		return Account{}, entity.Entry{}, errs.ErrInvariant
	}
	a.balance = balance
	a.sequence = seq
	return a, entity.Record(tx, a.id, delta, seq, balance), nil
}
