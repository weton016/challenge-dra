package aggregate

import (
	"errors"
	"math"
	"testing"

	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

func mustID(t *testing.T, raw string) valueobject.TransactionID {
	t.Helper()
	id, err := valueobject.NewTransactionID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustClient(t *testing.T, id int64) valueobject.AccountID {
	t.Helper()
	out, err := valueobject.NewClientID(id)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustAmount(t *testing.T, minor int64) valueobject.Amount {
	t.Helper()
	out, err := valueobject.NewAmount(minor)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustAccount(t *testing.T, id, balance, sequence int64) Account {
	t.Helper()
	var aid valueobject.AccountID
	var err error
	if id < 0 {
		aid, err = valueobject.NewTechnicalID(id)
	} else {
		aid, err = valueobject.NewClientID(id)
	}
	if err != nil {
		t.Fatal(err)
	}
	bal, err := valueobject.NewBalance(balance)
	if err != nil {
		t.Fatal(err)
	}
	account, err := NewAccount(aid, bal, sequence, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func TestNewAccount(t *testing.T) {
	account := mustAccount(t, 1, 40, 2)
	if account.ID().Int64() != 1 || account.Balance().Minor() != 40 || account.Sequence() != 2 || account.Currency().String() != "BRL" {
		t.Fatal("accessors")
	}
	technical := mustAccount(t, -2, 0, 0)
	if !technical.ID().IsTechnical() {
		t.Fatal("technical account")
	}
	balance, err := valueobject.NewBalance(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewAccount(valueobject.AccountID{}, balance, 0, valueobject.BRL()); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("zero id")
	}
	if _, err = NewAccount(mustClient(t, 1), balance, -1, valueobject.BRL()); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("sequence")
	}
	if _, err = NewAccount(mustClient(t, 1), balance, 0, valueobject.Currency{}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("currency")
	}
}

func TestDebitAndCredit(t *testing.T) {
	tx := mustID(t, "move")
	amount := mustAmount(t, 40)
	account := mustAccount(t, 1, 40, 3)
	debited, entry, err := account.Debit(amount, tx)
	if err != nil {
		t.Fatal(err)
	}
	if debited.Balance().Minor() != 0 || debited.Sequence() != 4 || entry.Delta() != -40 || entry.Balance().Minor() != 0 || entry.Sequence() != 4 {
		t.Fatal(debited.Balance().Minor(), entry.Delta())
	}
	if entry.Transaction().String() != "move" || entry.Account().Int64() != 1 {
		t.Fatal("entry identity")
	}
	if account.Sequence() != 3 {
		t.Fatal("caller mutated")
	}
	credited, credit, err := debited.Credit(mustAmount(t, 5), tx)
	if err != nil || credited.Balance().Minor() != 5 || credit.Delta() != 5 {
		t.Fatal(err)
	}
	if _, _, err = account.Debit(mustAmount(t, 41), tx); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal("insufficient")
	}
	rich := mustAccount(t, 1, math.MaxInt64, 0)
	if _, _, err = rich.Credit(mustAmount(t, 1), tx); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal("overflow")
	}
	for _, call := range []struct {
		name   string
		amount valueobject.Amount
		tx     valueobject.TransactionID
		debit  bool
	}{
		{"zero debit", valueobject.Amount{}, tx, true},
		{"empty debit", amount, valueobject.TransactionID{}, true},
		{"zero credit", valueobject.Amount{}, tx, false},
		{"empty credit", amount, valueobject.TransactionID{}, false},
	} {
		t.Run(call.name, func(t *testing.T) {
			var callErr error
			if call.debit {
				_, _, callErr = account.Debit(call.amount, call.tx)
			} else {
				_, _, callErr = account.Credit(call.amount, call.tx)
			}
			if !errors.Is(callErr, errs.ErrInvalid) {
				t.Fatal(callErr)
			}
		})
	}
	if _, _, err = (Account{}).apply(-1, tx); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal("zero account")
	}
	if _, _, err = account.apply(-1, valueobject.TransactionID{}); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal("empty transaction")
	}
	if _, _, err = account.apply(0, tx); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal("zero delta")
	}
	full := mustAccount(t, 1, 10, math.MaxInt64)
	if _, _, err = full.Debit(mustAmount(t, 1), tx); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal("sequence")
	}
	if full.Sequence() != math.MaxInt64 {
		t.Fatal("sequence changed")
	}
}
