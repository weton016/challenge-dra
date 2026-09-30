package entity

import (
	"errors"
	"testing"

	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

func TestEntry(t *testing.T) {
	tx, err := valueobject.NewTransactionID("entry")
	if err != nil {
		t.Fatal(err)
	}
	other, err := valueobject.NewTransactionID("other")
	if err != nil {
		t.Fatal(err)
	}
	account, err := valueobject.NewClientID(1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := valueobject.NewClientID(2)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := valueobject.NewBalance(10)
	if err != nil {
		t.Fatal(err)
	}
	amount, err := valueobject.NewAmount(10)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := NewEntry(tx, account, -10, 1, balance)
	if err != nil {
		t.Fatal(err)
	}
	credit, err := NewEntry(tx, second, 10, 4, balance)
	if err != nil {
		t.Fatal(err)
	}
	if err = Opposite(entry, credit, amount); err != nil {
		t.Fatal(err)
	}
	recorded := Record(tx, account, -10, 1, balance)
	if recorded.Delta() != entry.Delta() || recorded.Transaction().String() != "entry" || recorded.Account().Int64() != 1 || recorded.Sequence() != 1 || recorded.Balance().Minor() != 10 {
		t.Fatal("record")
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
	bad := entry
	bad.delta = -9
	if !errors.Is(Opposite(bad, credit, amount), errs.ErrInvariant) {
		t.Fatal("debit delta")
	}
	badCredit := credit
	badCredit.delta = 9
	if !errors.Is(Opposite(entry, badCredit, amount), errs.ErrInvariant) {
		t.Fatal("credit delta")
	}
	bad = entry
	bad.transaction = other
	if !errors.Is(Opposite(bad, credit, amount), errs.ErrInvariant) {
		t.Fatal("transaction")
	}
	same := credit
	same.account = entry.account
	if !errors.Is(Opposite(entry, same, amount), errs.ErrInvariant) {
		t.Fatal("same account")
	}
}

func TestStatement(t *testing.T) {
	tx, err := valueobject.NewTransactionID("s")
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
