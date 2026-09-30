package valueobject

import (
	"errors"
	"math"
	"strings"
	"testing"

	"dracma/pix/internal/domain/errs"
)

func TestTransactionID(t *testing.T) {
	id, err := NewTransactionID("abc_123-A")
	if err != nil || id.String() != "abc_123-A" {
		t.Fatal(err)
	}
	for _, raw := range []string{"", "bad id", strings.Repeat("a", 65), "refund:abc"} {
		if _, err = NewTransactionID(raw); !errors.Is(err, errs.ErrInvalid) {
			t.Fatalf("accepted %q", raw)
		}
	}
	wide, err := NewTransactionID(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	refund, err := RefundID(wide)
	if err != nil || refund.String() != "refund:"+wide.String() {
		t.Fatal(err)
	}
	if _, err = RefundID(TransactionID{}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("empty refund")
	}
	if _, err = RefundID(refund); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal("nested refund")
	}
	parsed, err := ParseTransactionID("abc_123-A")
	if err != nil || parsed.String() != "abc_123-A" {
		t.Fatal(err)
	}
	parsed, err = ParseTransactionID(refund.String())
	if err != nil || parsed != refund {
		t.Fatal(err, parsed)
	}
	for _, raw := range []string{"bad id", "refund:", "refund:bad id", "refund:refund:abc"} {
		if _, err = ParseTransactionID(raw); !errors.Is(err, errs.ErrInvalid) {
			t.Fatalf("parsed %q", raw)
		}
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
	sum, ok := Add(5, 5)
	if !ok || sum != 10 {
		t.Fatal("add")
	}
	if _, ok = Add(math.MaxInt64, 1); ok {
		t.Fatal("overflow")
	}
	if _, ok = Add(math.MinInt64, -1); ok {
		t.Fatal("underflow")
	}
	if _, ok = Add(5, -1); !ok {
		t.Fatal("debit")
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
	for _, raw := range []string{string(ReasonNone), string(ReasonInvalidAmount), string(ReasonInsufficientFunds), string(ReasonBalanceOverflow)} {
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
