package aggregate

import (
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
