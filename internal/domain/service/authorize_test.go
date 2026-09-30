package service

import (
	"errors"
	"math"
	"testing"

	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

func mustAccount(t *testing.T, id, balance, sequence int64) aggregate.Account {
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
	account, err := aggregate.NewAccount(aid, bal, sequence, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func TestAuthorize(t *testing.T) {
	transfer, err := aggregate.ParseTransfer("ok", 1, 2, "", 40)
	if err != nil {
		t.Fatal(err)
	}
	source := mustAccount(t, 1, 100, 1)
	destination := mustAccount(t, 2, 10, 4)
	approved, err := Authorize(transfer, source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Transfer.Decision().String() != "APPROVED" || approved.Transfer.Reason().String() != "" || approved.Transfer.Settlement().String() != "INTERNAL" {
		t.Fatal(approved.Transfer.Decision(), approved.Transfer.Reason(), approved.Transfer.Settlement())
	}
	if approved.Source.Balance().Minor() != 60 || approved.Source.Sequence() != 2 {
		t.Fatal("source")
	}
	if approved.Destination.Balance().Minor() != 50 || approved.Destination.Sequence() != 5 {
		t.Fatal("destination")
	}
	if len(approved.Entries) != 2 {
		t.Fatal("entries")
	}
	if err = entity.Opposite(approved.Entries[0], approved.Entries[1], transfer.Amount()); err != nil {
		t.Fatal(err)
	}
	if source.Balance().Minor() != 100 {
		t.Fatal("mutated input")
	}

	exact, err := aggregate.ParseTransfer("exact", 1, 2, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	drained, err := Authorize(exact, source, destination)
	if err != nil || drained.Source.Balance().Minor() != 0 || len(drained.Entries) != 2 {
		t.Fatal(err)
	}

	poor, err := aggregate.ParseTransfer("poor", 1, 2, "", 101)
	if err != nil {
		t.Fatal(err)
	}
	declined, err := Authorize(poor, source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if declined.Transfer.Decision().String() != "DECLINED" || declined.Transfer.Reason().String() != "INSUFFICIENT_FUNDS" || declined.Transfer.Settlement().String() != "INTERNAL" || declined.Entries != nil {
		t.Fatal(declined.Transfer.Decision(), declined.Transfer.Reason(), len(declined.Entries))
	}
	if declined.Source.Sequence() != source.Sequence() {
		t.Fatal("declined moved")
	}

	external, err := aggregate.ParseTransfer("out", 1, 0, "shop", 10)
	if err != nil {
		t.Fatal(err)
	}
	technical := mustAccount(t, external.Destination().Int64(), 0, 0)
	pending, err := Authorize(external, source, technical)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Transfer.Settlement().String() != "PENDING" || pending.Destination.Balance().Minor() != 10 {
		t.Fatal(pending.Transfer.Settlement(), pending.Destination.Balance().Minor())
	}
	externalDecline, err := aggregate.ParseTransfer("out-no", 1, 0, "shop", 1000)
	if err != nil {
		t.Fatal(err)
	}
	held, err := Authorize(externalDecline, source, technical)
	if err != nil || held.Transfer.Decision().String() != "DECLINED" || held.Transfer.Settlement().String() != "INTERNAL" {
		t.Fatal(err, held.Transfer.Settlement())
	}

	overflow, err := aggregate.ParseTransfer("over", 1, 2, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	full := mustAccount(t, 2, math.MaxInt64, 0)
	blocked, err := Authorize(overflow, source, full)
	if err != nil || blocked.Transfer.Reason().String() != "BALANCE_OVERFLOW" || blocked.Entries != nil {
		t.Fatal(err, blocked.Transfer.Reason())
	}

	id, err := valueobject.NewTransactionID("zero")
	if err != nil {
		t.Fatal(err)
	}
	zero, err := aggregate.NewInternalTransfer(id, source.ID(), destination.ID(), valueobject.Amount{})
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := Authorize(zero, source, destination)
	if err != nil || invalid.Transfer.Reason().String() != "INVALID_AMOUNT" || invalid.Entries != nil {
		t.Fatal(err, invalid.Transfer.Reason())
	}
}

func TestAuthorizeRejectsUnusableAccounts(t *testing.T) {
	transfer, err := aggregate.ParseTransfer("ok", 1, 2, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	source := mustAccount(t, 1, 10, 0)
	destination := mustAccount(t, 2, 0, 0)
	if _, err = Authorize(transfer, aggregate.Account{}, destination); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("source currency")
	}
	if _, err = Authorize(transfer, source, aggregate.Account{}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("destination currency")
	}
	if _, err = Authorize(transfer, mustAccount(t, 3, 10, 0), destination); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("source id")
	}
	if _, err = Authorize(transfer, source, mustAccount(t, 4, 0, 0)); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("destination id")
	}
	exhausted := mustAccount(t, 1, 10, math.MaxInt64)
	if _, err = Authorize(transfer, exhausted, destination); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal("debit sequence")
	}
	fullSequence := mustAccount(t, 2, 0, math.MaxInt64)
	if _, err = Authorize(transfer, source, fullSequence); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal("credit sequence")
	}
}
