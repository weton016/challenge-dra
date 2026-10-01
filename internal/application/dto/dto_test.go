package dto

import (
	"errors"
	"math"
	"strings"
	"testing"

	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

const (
	idA = "00000000-0000-4000-8000-0000000000a1"
	idB = "00000000-0000-4000-8000-0000000000b2"
	idC = "00000000-0000-4000-8000-0000000000c3"
)

func TestValidateAndTarget(t *testing.T) {
	internal := TransferCommand{ID: idA, Source: 1, Destination: 2, Amount: 1}
	if err := internal.Validate(); err != nil {
		t.Fatal(err)
	}
	if internal.Target() != 2 {
		t.Fatal(internal.Target())
	}
	wide := TransferCommand{ID: idB, Source: 1, Destination: 2, Amount: math.MaxInt64}
	if err := wide.Validate(); err != nil {
		t.Fatal(err)
	}
	external := TransferCommand{ID: idB, Source: 1, External: "shop", Amount: 10}
	if err := external.Validate(); err != nil || external.Target() != -2 {
		t.Fatal(err, external.Target())
	}
	boundary := TransferCommand{ID: idC, Source: 32, External: strings.Repeat("a", 140), Amount: 1}
	if err := boundary.Validate(); err != nil || boundary.Target() != -1 {
		t.Fatal(err, boundary.Target())
	}
	invalid := []TransferCommand{
		{},
		{ID: "bad id", Source: 1, Destination: 2, Amount: 1},
		{ID: "abc_123-A", Source: 1, Destination: 2, Amount: 1},
		{ID: idA, Source: 0, Destination: 2, Amount: 1},
		{ID: idA, Source: -1, Destination: 2, Amount: 1},
		{ID: idA, Source: 1, Destination: 0, Amount: 1},
		{ID: idA, Source: 1, Destination: 1, Amount: 1},
		{ID: idA, Source: 1, Destination: 2, Amount: 0},
		{ID: idA, Source: 1, Destination: 2, Amount: -1},
		{ID: idA, Source: 1, Destination: 2, External: "shop", Amount: 1},
		{ID: idA, Source: 1, External: strings.Repeat("a", 141), Amount: 1},
	}
	for _, cmd := range invalid {
		if err := cmd.Validate(); !errors.Is(err, errs.ErrInvalid) {
			t.Fatalf("accepted %#v: %v", cmd, err)
		}
	}
}

func TestFromTransfer(t *testing.T) {
	raw, err := aggregate.ParseTransfer(idA, 1, 2, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	declined, err := aggregate.RestoreTransfer(raw, valueobject.Declined, valueobject.Reason("ACCOUNT_NOT_FOUND"), valueobject.SettlementInternal)
	if err != nil {
		t.Fatal(err)
	}
	got := FromTransfer(declined)
	want := TransferResult{ID: idA, Decision: "DECLINED", Reason: "ACCOUNT_NOT_FOUND", Settlement: "INTERNAL"}
	if got != want {
		t.Fatal(got)
	}
	external, err := aggregate.ParseTransfer(idB, 1, 0, "shop", 5)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := aggregate.RestoreTransfer(external, valueobject.Approved, valueobject.ReasonNone, valueobject.SettlementPending)
	if err != nil {
		t.Fatal(err)
	}
	got = FromTransfer(pending)
	want = TransferResult{ID: idB, Decision: "APPROVED", Settlement: "PENDING"}
	if got != want {
		t.Fatal(got)
	}
}

func TestFromAccount(t *testing.T) {
	id, err := valueobject.NewClientID(1)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := valueobject.NewBalance(0)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := aggregate.NewAccount(id, balance, 0, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	if got := FromAccount(empty); got != (Balance{ID: 1}) {
		t.Fatal(got)
	}
	balance, err = valueobject.NewBalance(80)
	if err != nil {
		t.Fatal(err)
	}
	account, err := aggregate.NewAccount(id, balance, 4, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	got := FromAccount(account)
	if got != (Balance{ID: 1, Balance: 80, Sequence: 4}) {
		t.Fatal(got)
	}
}

func TestFromStatement(t *testing.T) {
	empty, err := entity.NewStatement(nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	page := FromStatement(empty)
	if page.Until != 0 || page.Next != 0 || page.Entries == nil || len(page.Entries) != 0 {
		t.Fatal(page)
	}
	tx, err := valueobject.NewTransactionID(idC)
	if err != nil {
		t.Fatal(err)
	}
	account, err := valueobject.NewClientID(1)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := valueobject.NewBalance(79)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := entity.NewEntry(tx, account, -1, 5, balance)
	if err != nil {
		t.Fatal(err)
	}
	statement, err := entity.NewStatement([]entity.Entry{entry}, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	page = FromStatement(statement)
	if page.Until != 5 || page.Next != 5 || len(page.Entries) != 1 {
		t.Fatal(page)
	}
	got := page.Entries[0]
	if got.TransactionID != idC || got.Delta != -1 || got.Sequence != 5 || got.Balance != 79 {
		t.Fatal(got)
	}
}
