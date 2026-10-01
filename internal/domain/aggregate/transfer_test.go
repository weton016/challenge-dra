package aggregate

import (
	"errors"
	"strings"
	"testing"

	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

const (
	idA = "00000000-0000-4000-8000-00000000000a"
	idB = "00000000-0000-4000-8000-00000000000b"
)

func TestParseTransfer(t *testing.T) {
	internal, err := ParseTransfer(idA, 1, 2, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if internal.ID().String() != idA || internal.Source().Int64() != 1 || internal.Destination().Int64() != 2 {
		t.Fatal("internal")
	}
	if internal.Amount().Minor() != 1 || internal.External().String() != "" || internal.Decision() != "" || internal.Reason() != "" || internal.Settlement() != "" {
		t.Fatal("payload")
	}
	external, err := ParseTransfer(idB, 1, 0, "recipient", 10)
	if err != nil || external.Destination().Int64() != -2 || external.External().String() != "recipient" {
		t.Fatal(err, external.Destination().Int64())
	}
	boundary, err := ParseTransfer(idB, 32, 0, "shop", 1)
	if err != nil || boundary.Destination().Int64() != -1 {
		t.Fatal(err, boundary.Destination().Int64())
	}
	invalid := []struct {
		id                  string
		source, destination int64
		external            string
		amount              int64
	}{
		{"", 1, 2, "", 1},
		{"bad id", 1, 2, "", 1},
		{idA, 0, 2, "", 1},
		{idA, 1, 0, "", 1},
		{idA, 1, 1, "", 1},
		{idA, 1, 2, "", 0},
		{idA, 1, 2, "", -1},
		{idA, 1, 2, "x", 1},
		{idA, 1, 0, strings.Repeat("a", 141), 1},
	}
	for _, raw := range invalid {
		if _, err = ParseTransfer(raw.id, raw.source, raw.destination, raw.external, raw.amount); !errors.Is(err, errs.ErrInvalid) {
			t.Fatalf("accepted %#v", raw)
		}
	}
}

func TestTransferConstructors(t *testing.T) {
	id := mustID(t, idA)
	amount := mustAmount(t, 1)
	client := mustClient(t, 1)
	other := mustClient(t, 2)
	technical, err := valueobject.NewTechnicalID(-1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewInternalTransfer(id, technical, other, amount); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("technical source")
	}
	if _, err = NewInternalTransfer(id, client, valueobject.AccountID{}, amount); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("zero destination")
	}
	if _, err = NewInternalTransfer(id, client, client, amount); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("same account")
	}
	ref, err := valueobject.NewExternalRef("shop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewExternalTransfer(id, technical, ref, amount, 0); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("external source")
	}
	if _, err = NewExternalTransfer(id, client, valueobject.ExternalRef{}, amount, 0); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("empty ref")
	}
	if _, err = NewExternalTransfer(id, client, ref, amount, 2); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("local destination")
	}
}

func TestRestoreTransfer(t *testing.T) {
	internal, err := ParseTransfer(idA, 1, 2, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreTransfer(internal, valueobject.Declined, valueobject.Reason("ACCOUNT_NOT_FOUND"), valueobject.SettlementInternal)
	if err != nil || restored.Decision().String() != "DECLINED" || restored.Reason().String() != "ACCOUNT_NOT_FOUND" || restored.Settlement().String() != "INTERNAL" {
		t.Fatal(err, restored.Decision(), restored.Reason(), restored.Settlement())
	}
	external, err := ParseTransfer(idB, 1, 0, "shop", 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, settlement := range []valueobject.Settlement{valueobject.SettlementPending, valueobject.SettlementSettled, valueobject.SettlementReversed} {
		restored, err = RestoreTransfer(external, valueobject.Approved, "", settlement)
		if err != nil || restored.Decision().String() != "APPROVED" || restored.Settlement() != settlement || restored.External().String() != "shop" {
			t.Fatal(err, settlement, restored.Settlement())
		}
	}
	if _, err = RestoreTransfer(internal, "NOPE", "", valueobject.SettlementInternal); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = RestoreTransfer(internal, valueobject.Approved, "", "OPEN"); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestFingerprint(t *testing.T) {
	external, err := ParseTransfer(idA, 1, 0, "shop", 5)
	if err != nil {
		t.Fatal(err)
	}
	other, err := ParseTransfer(idB, 1, 0, "shop", 5)
	if err != nil {
		t.Fatal(err)
	}
	if external.Fingerprint() != other.Fingerprint() {
		t.Fatal("the id is not part of the payload")
	}
	if external.Fingerprint() == ([16]byte{}) {
		t.Fatal("empty fingerprint")
	}
	changed := external
	changed.source = mustClient(t, 4)
	if external.Fingerprint() == changed.Fingerprint() {
		t.Fatal("source")
	}
	changed = external
	changed.destination = mustClient(t, 3)
	if external.Fingerprint() == changed.Fingerprint() {
		t.Fatal("destination")
	}
	changed = external
	changed.external, err = valueobject.NewExternalRef("other")
	if err != nil {
		t.Fatal(err)
	}
	if external.Fingerprint() == changed.Fingerprint() {
		t.Fatal("external")
	}
	changed = external
	changed.amount = mustAmount(t, 6)
	if external.Fingerprint() == changed.Fingerprint() {
		t.Fatal("amount")
	}
}

func TestWithReason(t *testing.T) {
	internal, err := ParseTransfer(idA, 1, 2, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	approved := internal.WithReason(valueobject.ReasonNone)
	if approved.Decision() != valueobject.Approved || approved.Reason() != valueobject.ReasonNone || approved.Settlement() != valueobject.SettlementInternal {
		t.Fatal(approved.Decision(), approved.Reason(), approved.Settlement())
	}
	if internal.Decision() != "" || internal.Settlement() != "" {
		t.Fatal("caller mutated")
	}
	for _, reason := range []valueobject.Reason{valueobject.ReasonInvalidAmount, valueobject.ReasonInsufficientFunds, valueobject.ReasonBalanceOverflow} {
		declined := internal.WithReason(reason)
		if declined.Decision() != valueobject.Declined || declined.Reason() != reason || declined.Settlement() != valueobject.SettlementInternal {
			t.Fatal(reason, declined.Decision(), declined.Settlement())
		}
	}
	external, err := ParseTransfer(idB, 1, 0, "shop", 5)
	if err != nil {
		t.Fatal(err)
	}
	pending := external.WithReason(valueobject.ReasonNone)
	if pending.Decision() != valueobject.Approved || pending.Reason() != valueobject.ReasonNone || pending.Settlement() != valueobject.SettlementPending {
		t.Fatal(pending.Decision(), pending.Settlement())
	}
	held := external.WithReason(valueobject.ReasonInsufficientFunds)
	if held.Decision() != valueobject.Declined || held.Settlement() != valueobject.SettlementInternal {
		t.Fatal(held.Decision(), held.Settlement())
	}
}
