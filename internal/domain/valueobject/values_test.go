package valueobject

import (
	"errors"
	"strings"
	"testing"

	"dracma/pix/internal/domain/errs"
)

func TestTransactionID(t *testing.T) {
	id, err := NewTransactionID("0190F3A2-7C1E-4D2B-9A6F-1234567890AB")
	if err != nil || id.String() != "0190f3a2-7c1e-4d2b-9a6f-1234567890ab" {
		t.Fatal(err, id)
	}
	for _, raw := range []string{
		"", "bad id", "abc_123-A",
		"0190f3a27c1e4d2b9a6f1234567890ab",
		"0190f3a2-7c1e-4d2b-9a6f-1234567890ab0",
		"g190f3a2-7c1e-4d2b-9a6f-1234567890ab",
		"00000000-0000-0000-0000-000000000000",
		"FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
		"0190f3a2-7c1e-0d2b-9a6f-1234567890ab",
		"0190f3a2-7c1e-9d2b-9a6f-1234567890ab",
		"0190f3a2-7c1e-4d2b-7a6f-1234567890ab",
		"0190f3a2-7c1e-4d2b-ca6f-1234567890ab",
		"refund:0190f3a2-7c1e-4d2b-9a6f-1234567890ab",
	} {
		if _, err = NewTransactionID(raw); !errors.Is(err, errs.ErrInvalid) {
			t.Fatalf("accepted %q", raw)
		}
	}
	refund, err := RefundID(id)
	if err != nil || refund.String() != "refund:"+id.String() {
		t.Fatal(err)
	}
	if _, err = RefundID(TransactionID{}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("empty refund")
	}
	if _, err = RefundID(refund); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("nested refund")
	}
}

func TestAccountID(t *testing.T) {
	client, err := NewClientID(1)
	if err != nil || client.Int64() != 1 || !client.IsClient() || client.IsTechnical() {
		t.Fatal(err)
	}
	technical, err := NewTechnicalID(-1)
	if err != nil || technical.Int64() != -1 || !technical.IsTechnical() || technical.IsClient() {
		t.Fatal(err)
	}
	if _, err = NewTechnicalID(-32); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{0, -33, 1} {
		if _, err = NewTechnicalID(id); !errors.Is(err, errs.ErrInvalid) {
			t.Fatalf("accepted technical %d", id)
		}
	}
	if _, err = NewClientID(0); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("zero client")
	}
	if _, err = NewClientID(-1); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("negative client")
	}
	zero := AccountID{}
	if zero.IsClient() || zero.IsTechnical() || zero.Int64() != 0 {
		t.Fatal("zero id")
	}
	if got := TechnicalID(-2); got.Int64() != -2 || !got.IsTechnical() {
		t.Fatal(got.Int64())
	}
	if TechnicalID(4).Int64() != 0 {
		t.Fatal("invalid technical id")
	}
}

func TestExternalRef(t *testing.T) {
	ref, err := NewExternalRef("shop")
	if err != nil || ref.String() != "shop" {
		t.Fatal(err)
	}
	if _, err = NewExternalRef(strings.Repeat("a", 140)); err != nil {
		t.Fatal(err)
	}
	if _, err = NewExternalRef(""); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("empty")
	}
	if _, err = NewExternalRef(strings.Repeat("a", 141)); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("long")
	}
}

func TestMoneyAndCurrency(t *testing.T) {
	amount, err := NewAmount(1)
	if err != nil || amount.Minor() != 1 {
		t.Fatal(err)
	}
	for _, minor := range []int64{0, -1} {
		if _, err = NewAmount(minor); !errors.Is(err, errs.ErrInvalid) {
			t.Fatalf("amount %d", minor)
		}
	}
	balance, err := NewBalance(0)
	if err != nil || balance.Minor() != 0 {
		t.Fatal(err)
	}
	if _, err = NewBalance(-1); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("negative balance")
	}
	if _, ok := TryBalance(-1); ok {
		t.Fatal("try balance")
	}
	currency, err := NewCurrency("BRL")
	if err != nil || currency.String() != "BRL" || !currency.Equal(BRL()) || currency.Equal(Currency{}) {
		t.Fatal(err)
	}
	if _, err = NewCurrency("USD"); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("usd")
	}
}

func TestOutcomeParsing(t *testing.T) {
	for _, raw := range []string{string(Approved), string(Declined)} {
		decision, err := ParseDecision(raw)
		if err != nil || decision.String() != raw {
			t.Fatal(raw, err)
		}
	}
	if _, err := ParseDecision("MAYBE"); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("decision")
	}
	for _, raw := range []string{string(ReasonNone), string(ReasonInvalidAmount), string(ReasonInsufficientFunds), string(ReasonBalanceOverflow), string(ReasonAccountNotFound)} {
		reason, err := ParseReason(raw)
		if err != nil || reason.String() != raw {
			t.Fatal(raw, err)
		}
	}
	if _, err := ParseReason("OTHER"); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("reason")
	}
	for _, raw := range []string{string(SettlementInternal), string(SettlementPending), string(SettlementSettled), string(SettlementReversed)} {
		settlement, err := ParseSettlement(raw)
		if err != nil || settlement.String() != raw {
			t.Fatal(raw, err)
		}
	}
	if _, err := ParseSettlement("OPEN"); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("settlement")
	}
}
