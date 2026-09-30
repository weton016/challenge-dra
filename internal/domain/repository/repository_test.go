package repository

import (
	"context"
	"errors"
	"testing"

	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

type memory struct {
	accounts  map[int64]aggregate.Account
	transfers map[string]aggregate.Transfer
	entries   map[int64][]entity.Entry
}

func newMemory() *memory {
	return &memory{
		accounts:  map[int64]aggregate.Account{},
		transfers: map[string]aggregate.Transfer{},
		entries:   map[int64][]entity.Entry{},
	}
}

func (m *memory) FindAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error) {
	account, ok := m.accounts[id.Int64()]
	if !ok {
		return aggregate.Account{}, errs.ErrNotFound
	}
	return account, nil
}

func (m *memory) SaveAccount(ctx context.Context, account aggregate.Account) error {
	m.accounts[account.ID().Int64()] = account
	return nil
}

func (m *memory) FindTransfer(ctx context.Context, id valueobject.TransactionID) (aggregate.Transfer, error) {
	transfer, ok := m.transfers[id.String()]
	if !ok {
		return aggregate.Transfer{}, errs.ErrNotFound
	}
	return transfer, nil
}

func (m *memory) SaveTransfer(ctx context.Context, transfer aggregate.Transfer) error {
	if stored, ok := m.transfers[transfer.ID().String()]; ok {
		return transfer.AcceptReplay(stored)
	}
	m.transfers[transfer.ID().String()] = transfer
	return nil
}

func (m *memory) FindStatement(ctx context.Context, id valueobject.AccountID, after, until int64) (entity.Statement, error) {
	account, ok := m.accounts[id.Int64()]
	if !ok {
		return entity.Statement{}, errs.ErrNotFound
	}
	var selected []entity.Entry
	for _, entry := range m.entries[id.Int64()] {
		if entry.Sequence() <= after {
			continue
		}
		if until != 0 && entry.Sequence() > until {
			continue
		}
		selected = append(selected, entry)
	}
	next := after
	if len(selected) > 0 {
		next = selected[len(selected)-1].Sequence()
	}
	end := until
	if end == 0 {
		end = account.Sequence()
	}
	return entity.NewStatement(selected, end, next)
}

func TestRepositoryContract(t *testing.T) {
	var (
		_ AccountRepository   = (*memory)(nil)
		_ TransferRepository  = (*memory)(nil)
		_ StatementRepository = (*memory)(nil)
	)
	ctx := context.Background()
	store := newMemory()
	id, err := valueobject.NewClientID(1)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := valueobject.NewBalance(80)
	if err != nil {
		t.Fatal(err)
	}
	account, err := aggregate.NewAccount(id, balance, 2, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.FindAccount(ctx, account.ID()); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	if err = store.SaveAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	found, err := store.FindAccount(ctx, account.ID())
	if err != nil || found.Balance().Minor() != 80 {
		t.Fatal(err)
	}

	transfer, err := aggregate.ParseTransfer("same", 1, 2, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.FindTransfer(ctx, transfer.ID()); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	if err = store.SaveTransfer(ctx, transfer); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveTransfer(ctx, transfer); err != nil {
		t.Fatal("replay")
	}
	changed, err := aggregate.ParseTransfer("same", 1, 2, "", 6)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(store.SaveTransfer(ctx, changed), errs.ErrConflict) {
		t.Fatal("conflict")
	}
	stored, err := store.FindTransfer(ctx, transfer.ID())
	if err != nil || stored.Amount().Minor() != 5 {
		t.Fatal(err, stored.Amount().Minor())
	}

	missing, err := valueobject.NewClientID(9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.FindStatement(ctx, missing, 0, 0); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	after, err := valueobject.NewBalance(75)
	if err != nil {
		t.Fatal(err)
	}
	laterBalance, err := valueobject.NewBalance(74)
	if err != nil {
		t.Fatal(err)
	}
	later, err := valueobject.NewTransactionID("later")
	if err != nil {
		t.Fatal(err)
	}
	first, err := entity.NewEntry(transfer.ID(), account.ID(), -5, 1, after)
	if err != nil {
		t.Fatal(err)
	}
	second, err := entity.NewEntry(later, account.ID(), -1, 2, laterBalance)
	if err != nil {
		t.Fatal(err)
	}
	store.entries[account.ID().Int64()] = []entity.Entry{first, second}
	page, err := store.FindStatement(ctx, account.ID(), 0, 0)
	if err != nil || page.Until() != 2 || page.Next() != 2 || len(page.Entries()) != 2 {
		t.Fatal(err, page.Until(), page.Next(), len(page.Entries()))
	}
	bounded, err := store.FindStatement(ctx, account.ID(), 1, 1)
	if err != nil || len(bounded.Entries()) != 0 || bounded.Next() != 1 || bounded.Until() != 1 {
		t.Fatal(err, len(bounded.Entries()), bounded.Next())
	}
	middle, err := store.FindStatement(ctx, account.ID(), 0, 1)
	if err != nil || len(middle.Entries()) != 1 || middle.Entries()[0].Sequence() != 1 {
		t.Fatal(err)
	}
}
