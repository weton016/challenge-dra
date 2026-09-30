package usecase

import (
	"context"
	"errors"
	"testing"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

func TestQueries(t *testing.T) {
	ctx := context.Background()
	store := newMemory()
	put(store, mustAccount(t, 1, 80, 4), mustAccount(t, 2, 0, 0))
	approved, err := (AuthorizeTransfer{Transfers: store, Accounts: store}).Execute(ctx, dto.TransferCommand{ID: "saved", Source: 1, Destination: 2, Amount: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, err := (GetTransfer{Transfers: store}).Execute(ctx, "saved")
	if err != nil || got != approved {
		t.Fatal(err, got)
	}
	if _, err = (GetTransfer{Transfers: store}).Execute(ctx, "bad id"); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = (GetTransfer{Transfers: store}).Execute(ctx, "missing"); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}

	balance, err := (GetBalance{Accounts: store}).Execute(ctx, 1)
	if err != nil || balance.ID != 1 || balance.Balance != 79 || balance.Sequence != 5 {
		t.Fatal(err, balance)
	}
	if _, err = (GetBalance{Accounts: store}).Execute(ctx, 0); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = (GetBalance{Accounts: store}).Execute(ctx, 9); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	store.accountErr = errors.New("balance failed")
	if _, err = (GetBalance{Accounts: store}).Execute(ctx, 1); err == nil {
		t.Fatal("balance error")
	}
	store.accountErr = nil

	statement := GetStatement{Accounts: store, Statements: store}
	page, err := statement.Execute(ctx, 1, 0, 0)
	if err != nil || store.seenUntil != 5 || page.Until != 5 || len(page.Entries) != 0 {
		t.Fatal(err, store.seenUntil, page)
	}
	if _, err = statement.Execute(ctx, 1, 0, 9); err != nil || store.seenUntil != 5 {
		t.Fatal(err, store.seenUntil)
	}
	if _, err = statement.Execute(ctx, 1, 1, 2); err != nil || store.seenUntil != 2 {
		t.Fatal(err, store.seenUntil)
	}
	if _, err = statement.Execute(ctx, 1, -1, 0); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = statement.Execute(ctx, 1, 0, -1); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = statement.Execute(ctx, 0, 0, 0); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = statement.Execute(ctx, 9, 0, 0); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	tx, err := valueobject.NewTransactionID("saved")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := entity.NewEntry(tx, store.accounts[1].ID(), -1, 5, store.accounts[1].Balance())
	if err != nil {
		t.Fatal(err)
	}
	store.rows = []entity.Entry{entry}
	page, err = statement.Execute(ctx, 1, 0, 5)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].TransactionID != "saved" || page.Entries[0].Delta != -1 || page.Entries[0].Sequence != 5 || page.Entries[0].Balance != 79 || page.Next != 5 {
		t.Fatal(err, page)
	}
	store.accountErr = errors.New("statement account")
	if _, err = statement.Execute(ctx, 1, 0, 0); err == nil {
		t.Fatal("statement account")
	}
	store.accountErr = nil
	store.statementErr = errors.New("statement failed")
	if _, err = statement.Execute(ctx, 1, 0, 0); err == nil {
		t.Fatal("statement")
	}
}
