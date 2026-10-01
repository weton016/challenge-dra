package aggregate

import (
	"errors"
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
