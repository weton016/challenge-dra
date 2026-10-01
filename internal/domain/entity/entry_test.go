package entity

import (
	"errors"
	"testing"

	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

const txID = "00000000-0000-4000-8000-0000000000e1"

func TestEntry(t *testing.T) {
	tx, err := valueobject.NewTransactionID(txID)
	if err != nil {
		t.Fatal(err)
	}
	account, err := valueobject.NewClientID(1)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := valueobject.NewBalance(10)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := NewEntry(tx, account, -10, 1, balance)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Delta() != -10 || entry.Transaction().String() != txID || entry.Account().Int64() != 1 || entry.Sequence() != 1 || entry.Balance().Minor() != 10 {
		t.Fatal("accessors")
	}
	if _, err = NewEntry(valueobject.TransactionID{}, account, -1, 1, balance); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("tx")
	}
	if _, err = NewEntry(tx, valueobject.AccountID{}, -1, 1, balance); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("account")
	}
	if _, err = NewEntry(tx, account, 0, 1, balance); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("delta")
	}
	if _, err = NewEntry(tx, account, -1, 0, balance); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("sequence")
	}
}

func TestStatement(t *testing.T) {
	tx, err := valueobject.NewTransactionID(txID)
	if err != nil {
		t.Fatal(err)
	}
	account, err := valueobject.NewClientID(1)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := valueobject.NewBalance(9)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := NewEntry(tx, account, -1, 1, balance)
	if err != nil {
		t.Fatal(err)
	}
	statement, err := NewStatement([]Entry{entry}, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if statement.Until() != 4 || statement.Next() != 1 || len(statement.Entries()) != 1 {
		t.Fatal("page")
	}
	copied := statement.Entries()
	copied[0] = Entry{}
	if statement.Entries()[0].Delta() != -1 {
		t.Fatal("aliased")
	}
	empty, err := NewStatement(nil, 0, 0)
	if err != nil || len(empty.Entries()) != 0 {
		t.Fatal(err)
	}
	if _, err = NewStatement(nil, -1, 0); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("until")
	}
	if _, err = NewStatement(nil, 0, -1); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("next")
	}
}
