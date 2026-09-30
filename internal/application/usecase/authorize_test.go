package usecase

import (
	"context"
	"errors"
	"math"
	"testing"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/errs"
)

func TestAuthorizeTransfer(t *testing.T) {
	ctx := context.Background()
	store := newMemory()
	put(store, mustAccount(t, 1, 100, 1), mustAccount(t, 2, 10, 4), mustAccount(t, -2, 0, 0))
	uc := AuthorizeTransfer{Transfers: store, Accounts: store}
	var _ port.TransferStore = store
	var _ port.AccountStore = store

	if _, err := uc.Execute(ctx, dto.TransferCommand{}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	approved, err := uc.Execute(ctx, dto.TransferCommand{ID: "ok", Source: 1, Destination: 2, Amount: 40})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Decision != "APPROVED" || approved.Reason != "" || approved.Settlement != "INTERNAL" || approved.ID != "ok" {
		t.Fatal(approved)
	}
	if store.accounts[1].Balance().Minor() != 60 || store.accounts[2].Balance().Minor() != 50 {
		t.Fatal(store.accounts[1].Balance().Minor(), store.accounts[2].Balance().Minor())
	}
	again, err := uc.Execute(ctx, dto.TransferCommand{ID: "ok", Source: 1, Destination: 2, Amount: 40})
	if err != nil || again != approved || store.commits != 1 {
		t.Fatal(err, again, store.commits)
	}
	if _, err = uc.Execute(ctx, dto.TransferCommand{ID: "ok", Source: 1, Destination: 2, Amount: 1}); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}

	exact, err := uc.Execute(ctx, dto.TransferCommand{ID: "exact", Source: 1, Destination: 2, Amount: 60})
	if err != nil || exact.Decision != "APPROVED" || store.accounts[1].Balance().Minor() != 0 {
		t.Fatal(err, exact, store.accounts[1].Balance().Minor())
	}
	denied, err := uc.Execute(ctx, dto.TransferCommand{ID: "poor", Source: 1, Destination: 2, Amount: 1})
	if err != nil || denied.Decision != "DECLINED" || denied.Reason != "INSUFFICIENT_FUNDS" || store.accounts[1].Balance().Minor() != 0 {
		t.Fatal(err, denied, store.accounts[1].Balance().Minor())
	}
	full := mustAccount(t, 3, math.MaxInt64, 0)
	put(store, full, mustAccount(t, 1, 10, 0))
	blocked, err := uc.Execute(ctx, dto.TransferCommand{ID: "over", Source: 1, Destination: 3, Amount: 1})
	if err != nil || blocked.Reason != "BALANCE_OVERFLOW" || store.accounts[3].Balance().Minor() != math.MaxInt64 {
		t.Fatal(err, blocked)
	}
	external, err := uc.Execute(ctx, dto.TransferCommand{ID: "out", Source: 1, External: "shop", Amount: 5})
	if err != nil || external.Settlement != "PENDING" || store.accounts[-2].Balance().Minor() != 5 {
		t.Fatal(err, external, store.accounts[-2].Balance().Minor())
	}

	missingSource := newMemory()
	put(missingSource, mustAccount(t, 2, 0, 0))
	gone, err := (AuthorizeTransfer{Transfers: missingSource, Accounts: missingSource}).Execute(ctx, dto.TransferCommand{ID: "miss-src", Source: 1, Destination: 2, Amount: 1})
	if err != nil || gone.Reason != "ACCOUNT_NOT_FOUND" || gone.Decision != "DECLINED" || missingSource.accounts[2].Balance().Minor() != 0 {
		t.Fatal(err, gone)
	}
	missingDest := newMemory()
	put(missingDest, mustAccount(t, 1, 40, 0))
	gone, err = (AuthorizeTransfer{Transfers: missingDest, Accounts: missingDest}).Execute(ctx, dto.TransferCommand{ID: "miss-dst", Source: 1, Destination: 2, Amount: 1})
	if err != nil || gone.Reason != "ACCOUNT_NOT_FOUND" || missingDest.accounts[1].Balance().Minor() != 40 {
		t.Fatal(err, gone)
	}

	broken := newMemory()
	put(broken, mustAccount(t, 1, 10, 0), mustAccount(t, 2, 0, 0))
	broken.transferErr = errors.New("read failed")
	if _, err = (AuthorizeTransfer{Transfers: broken, Accounts: broken}).Execute(ctx, dto.TransferCommand{ID: "x", Source: 1, Destination: 2, Amount: 1}); err == nil || errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	broken.transferErr = nil
	broken.accountErr = errors.New("account failed")
	if _, err = (AuthorizeTransfer{Transfers: broken, Accounts: broken}).Execute(ctx, dto.TransferCommand{ID: "x", Source: 1, Destination: 2, Amount: 1}); err == nil {
		t.Fatal("account error")
	}
	broken.accountErr = nil
	broken.accountCalls = 0
	second := newMemory()
	put(second, mustAccount(t, 1, 10, 0), mustAccount(t, 2, 0, 0))
	second.accountErr = errors.New("destination failed")
	second.failAccountAfter = 1
	if _, err = (AuthorizeTransfer{Transfers: second, Accounts: second}).Execute(ctx, dto.TransferCommand{ID: "x", Source: 1, Destination: 2, Amount: 1}); err == nil {
		t.Fatal("destination account error")
	}
	broken.commitErr = errors.New("commit failed")
	if _, err = (AuthorizeTransfer{Transfers: broken, Accounts: broken}).Execute(ctx, dto.TransferCommand{ID: "x", Source: 1, Destination: 2, Amount: 1}); err == nil || broken.commits != 0 {
		t.Fatal(err, broken.commits)
	}
	exhausted := newMemory()
	put(exhausted, mustAccount(t, 1, 10, math.MaxInt64), mustAccount(t, 2, 0, 0))
	if _, err = (AuthorizeTransfer{Transfers: exhausted, Accounts: exhausted}).Execute(ctx, dto.TransferCommand{ID: "seq", Source: 1, Destination: 2, Amount: 1}); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	uncommitted := newMemory()
	uncommitted.commitErr = errors.New("commit failed")
	if _, err = (AuthorizeTransfer{Transfers: uncommitted, Accounts: uncommitted}).Execute(ctx, dto.TransferCommand{ID: "miss", Source: 1, Destination: 2, Amount: 1}); err == nil {
		t.Fatal("missing commit")
	}
}
