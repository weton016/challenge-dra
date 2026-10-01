package tigerbeetle

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

// TigerBeetle cannot truncate, so each test owns a random account range and
// random transaction ids.
func setup(t *testing.T) (*Ledger, int64) {
	t.Helper()
	address := os.Getenv("TEST_TB_ADDRESS")
	if address == "" {
		t.Skip("TEST_TB_ADDRESS required")
	}
	l, err := Open(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(l.Close)
	if err = l.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	base := int64(binary.LittleEndian.Uint64(b[:])>>4) | 1<<40
	if err = l.Seed(context.Background(), base, 3, 100); err != nil {
		t.Fatal(err)
	}
	return l, base
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := fmt.Sprintf("%x", b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func transfer(t *testing.T, id string, source, destination int64, external string, amount int64) aggregate.Transfer {
	t.Helper()
	parsed, err := aggregate.ParseTransfer(id, source, destination, external, amount)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func balance(t *testing.T, l *Ledger, id int64) (int64, int64) {
	t.Helper()
	parsed, _ := valueobject.NewClientID(id)
	account, err := l.FindAccount(context.Background(), parsed)
	if err != nil {
		t.Fatal(err)
	}
	return account.Balance().Minor(), account.Sequence()
}

func expect(t *testing.T, got dto.TransferResult, err error, decision, reason, settlement string) {
	t.Helper()
	if err != nil || got.Decision != decision || got.Reason != reason || got.Settlement != settlement {
		t.Fatalf("%+v %v, want %s %s %s", got, err, decision, reason, settlement)
	}
}

func TestInternalDecisionsAndReplays(t *testing.T) {
	l, base := setup(t)
	ctx := context.Background()
	id := newID()
	pay := transfer(t, id, base, base+1, "", 40)
	got, err := l.Authorize(ctx, pay)
	expect(t, got, err, "APPROVED", "", "INTERNAL")
	got, err = l.Authorize(ctx, pay)
	expect(t, got, err, "APPROVED", "", "INTERNAL")
	if _, err = l.Authorize(ctx, transfer(t, id, base, base+1, "", 41)); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	if b, _ := balance(t, l, base); b != 60 {
		t.Fatal(b)
	}
	tx, _ := valueobject.NewTransactionID(id)
	got, err = l.FindTransfer(ctx, tx)
	expect(t, got, err, "APPROVED", "", "INTERNAL")

	poor := newID()
	got, err = l.Authorize(ctx, transfer(t, poor, base, base+1, "", 61))
	expect(t, got, err, "DECLINED", "INSUFFICIENT_FUNDS", "INTERNAL")
	got, err = l.Authorize(ctx, transfer(t, poor, base, base+1, "", 61))
	expect(t, got, err, "DECLINED", "INSUFFICIENT_FUNDS", "INTERNAL")
	if _, err = l.Authorize(ctx, transfer(t, poor, base, base+1, "", 62)); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	tx, _ = valueobject.NewTransactionID(poor)
	got, err = l.FindTransfer(ctx, tx)
	expect(t, got, err, "DECLINED", "INSUFFICIENT_FUNDS", "INTERNAL")

	gone := newID()
	got, err = l.Authorize(ctx, transfer(t, gone, base, base+100, "", 1))
	expect(t, got, err, "DECLINED", "ACCOUNT_NOT_FOUND", "INTERNAL")
	got, err = l.Authorize(ctx, transfer(t, newID(), base+100, base, "", 1))
	expect(t, got, err, "DECLINED", "ACCOUNT_NOT_FOUND", "INTERNAL")

	tx, _ = valueobject.NewTransactionID(newID())
	if _, err = l.FindTransfer(ctx, tx); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	if b, _ := balance(t, l, base); b != 60 {
		t.Fatal(b)
	}
}

// A crash between TigerBeetle refusing the id and the decline record leaves
// a failed id with no reason. The replay decides the reason again.
func TestDeclineRecordLostInCrash(t *testing.T) {
	l, base := setup(t)
	ctx := context.Background()
	for _, c := range []struct {
		destination int64
		amount      int64
		reason      string
	}{{base + 1, 101, "INSUFFICIENT_FUNDS"}, {base + 100, 1, "ACCOUNT_NOT_FOUND"}} {
		pay := transfer(t, newID(), base, c.destination, "", c.amount)
		if _, err := l.create(ctx, chain(pay)); err != nil {
			t.Fatal(err)
		}
		got, err := l.Authorize(ctx, pay)
		expect(t, got, err, "DECLINED", c.reason, "INTERNAL")
		got, err = l.FindTransfer(ctx, pay.ID())
		expect(t, got, err, "DECLINED", c.reason, "INTERNAL")
	}
}

func TestDeclineRecordConflicts(t *testing.T) {
	l, base := setup(t)
	ctx := context.Background()
	pay := transfer(t, newID(), base, base+1, "", 101)
	got, err := l.decline(ctx, pay, reasonNotFound)
	expect(t, got, err, "DECLINED", "ACCOUNT_NOT_FOUND", "INTERNAL")
	got, err = l.decline(ctx, pay, reasonInsufficient)
	expect(t, got, err, "DECLINED", "ACCOUNT_NOT_FOUND", "INTERNAL")
	other := transfer(t, pay.ID().String(), base, base+1, "", 102)
	if _, err = l.decline(ctx, other, reasonInsufficient); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	bad := transfer(t, newID(), base, base+1, "", 1)
	record := tb.Transfer{ID: derive("decline", transferID(bad.ID())), DebitAccountID: memoDebit, CreditAccountID: memoCredit, UserData128: tb.BytesToUint128(bad.Fingerprint()), UserData32: 9, Ledger: ledgerBRL, Code: codeDecline}
	if _, err = l.create(ctx, []tb.Transfer{record}); err != nil {
		t.Fatal(err)
	}
	if _, err = l.FindTransfer(ctx, bad.ID()); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	if _, err = l.decline(ctx, bad, reasonNotFound); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	if err = statusError(tb.TransferExceedsDebits); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
}

func TestExternalSettlement(t *testing.T) {
	l, base := setup(t)
	ctx := context.Background()
	ref := "reject:" + strings.Repeat("r", 133)
	reject := transfer(t, newID(), base, 0, ref, 40)
	got, err := l.Authorize(ctx, reject)
	expect(t, got, err, "APPROVED", "", "PENDING")
	got, err = l.Authorize(ctx, reject)
	expect(t, got, err, "APPROVED", "", "PENDING")
	if _, err = l.Authorize(ctx, transfer(t, reject.ID().String(), base, 0, ref[:139], 40)); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	accept := transfer(t, newID(), base+1, 0, "shop", 30)
	got, err = l.Authorize(ctx, accept)
	expect(t, got, err, "APPROVED", "", "PENDING")
	if b, _ := balance(t, l, base); b != 60 {
		t.Fatal(b)
	}
	open := map[string]dto.TransferCommand{}
	for i := 0; i < 1000 && (open[reject.ID().String()].ID == "" || open[accept.ID().String()].ID == ""); i++ {
		page, err := l.Pending(ctx, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range page {
			open[c.ID] = c
		}
	}
	want := dto.TransferCommand{ID: reject.ID().String(), Source: base, External: ref, Amount: 40}
	if open[want.ID] != want || open[accept.ID().String()].External != "shop" {
		t.Fatal(open[want.ID], open[accept.ID().String()])
	}
	if err = l.Settle(ctx, want.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = l.Settle(ctx, want.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = l.Settle(ctx, want.ID, true); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	if err = l.Settle(ctx, accept.ID().String(), true); err != nil {
		t.Fatal(err)
	}
	if err = l.Settle(ctx, "bad", true); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	got, err = l.Authorize(ctx, reject)
	expect(t, got, err, "APPROVED", "", "REVERSED")
	got, err = l.FindTransfer(ctx, accept.ID())
	expect(t, got, err, "APPROVED", "", "SETTLED")
	if b, _ := balance(t, l, base); b != 100 {
		t.Fatal(b)
	}
	if b, _ := balance(t, l, base+1); b != 70 {
		t.Fatal(b)
	}
	page, err := l.Pending(ctx, 200)
	for _, c := range page {
		if c.ID == want.ID || c.ID == accept.ID().String() {
			t.Fatal("settled transfer still pending", err)
		}
	}
}

func TestStatementAndBalance(t *testing.T) {
	l, base := setup(t)
	ctx := context.Background()
	id, _ := valueobject.NewClientID(base)
	pay := transfer(t, newID(), base, base+1, "", 10)
	income := transfer(t, newID(), base+2, base, "", 5)
	out := transfer(t, newID(), base, 0, "away", 20)
	for _, tr := range []aggregate.Transfer{pay, income, out} {
		if _, err := l.Authorize(ctx, tr); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Settle(ctx, out.ID().String(), false); err != nil {
		t.Fatal(err)
	}
	b, seq := balance(t, l, base)
	if b != 95 || seq == 0 {
		t.Fatal(b, seq)
	}
	page, err := l.FindStatement(ctx, id, 0, seq)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id      string
		delta   int64
		balance int64
	}
	want := []row{{pay.ID().String(), -10, 90}, {income.ID().String(), 5, 95}, {out.ID().String(), -20, 75}, {"refund:" + out.ID().String(), 20, 95}}
	entries := page.Entries()
	if len(entries) != len(want) || page.Until() != seq || page.Next() != seq {
		t.Fatal(len(entries), page.Until(), page.Next(), seq)
	}
	for i, e := range entries {
		if (row{e.Transaction().String(), e.Delta(), e.Balance().Minor()}) != want[i] {
			t.Fatal(i, e.Transaction(), e.Delta(), e.Balance())
		}
	}
	rest, err := l.FindStatement(ctx, id, entries[1].Sequence(), seq)
	if err != nil || len(rest.Entries()) != 2 {
		t.Fatal(err)
	}
	empty, err := l.FindStatement(ctx, id, seq, seq)
	if err != nil || len(empty.Entries()) != 0 || empty.Next() != seq {
		t.Fatal(err)
	}
	missing, _ := valueobject.NewClientID(base + 100)
	if _, err = l.FindAccount(ctx, missing); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	if err = l.Seed(ctx, base+50, 1, 0); err != nil {
		t.Fatal(err)
	}
	if b, seq := balance(t, l, base+50); b != 0 || seq != 0 {
		t.Fatal(b, seq)
	}
}

func TestReceipts(t *testing.T) {
	l, base := setup(t)
	ctx := context.Background()
	in := transfer(t, newID(), base, 0, "shop", 10)
	result := dto.TransferResult{ID: in.ID().String(), Decision: "DECLINED", Settlement: "SETTLED"}
	for range 2 {
		got, err := l.SaveReceipt(ctx, in, result)
		if err != nil || got != result {
			t.Fatal(got, err)
		}
	}
	got, err := l.FindReceipt(ctx, in.ID())
	if err != nil || got != result {
		t.Fatal(got, err)
	}
	if _, err = l.SaveReceipt(ctx, transfer(t, in.ID().String(), base, 0, "shop", 11), result); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	ok := transfer(t, newID(), base, 0, "shop", 10)
	approved := dto.TransferResult{ID: ok.ID().String(), Decision: "APPROVED", Settlement: "SETTLED"}
	if _, err = l.SaveReceipt(ctx, ok, approved); err != nil {
		t.Fatal(err)
	}
	if got, err = l.FindReceipt(ctx, ok.ID()); err != nil || got != approved {
		t.Fatal(got, err)
	}
	tx, _ := valueobject.NewTransactionID(newID())
	if _, err = l.FindReceipt(ctx, tx); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestSeedAndAudit(t *testing.T) {
	l, base := setup(t)
	ctx := context.Background()
	if err := l.Seed(ctx, base+10, 600, 7); err != nil {
		t.Fatal(err)
	}
	if err := l.Seed(ctx, base+10, 1, 8); err != nil {
		t.Fatal(err)
	}
	if b, _ := balance(t, l, base+10); b != 7 {
		t.Fatal(b)
	}
	for _, bad := range [][3]int64{{0, 1, 1}, {1, 0, 1}, {1, 1, -1}, {math.MaxInt64, 1, 1}} {
		if err := l.Seed(ctx, bad[0], bad[1], bad[2]); !errors.Is(err, errs.ErrInvalid) {
			t.Fatal(bad, err)
		}
	}
	if err := l.Seed(ctx, base+900, 1, math.MaxInt64); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := l.Authorize(ctx, transfer(t, newID(), base, base+1, "", 10)); err != nil {
		t.Fatal(err)
	}
	report, err := l.Audit(ctx, []int64{base, base + 1, base + 5000})
	if err != nil {
		t.Fatal(err)
	}
	if report.InvalidBalances != 0 || report.Conservation != 0 || report.InvalidPrefixes != 1 || report.Accounts < 600 || report.Passed {
		t.Fatalf("%+v", report)
	}
	if report, err = l.Audit(ctx, []int64{base}); err != nil || !report.Passed {
		t.Fatalf("%+v %v", report, err)
	}
}

func TestUnavailable(t *testing.T) {
	l, base := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := l.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	pay := transfer(t, newID(), base, base+1, "", 1)
	id, _ := valueobject.NewClientID(base)
	checks := []func() error{
		func() error { _, err := l.Authorize(ctx, pay); return err },
		func() error { _, err := l.FindTransfer(ctx, pay.ID()); return err },
		func() error { _, err := l.FindAccount(ctx, id); return err },
		func() error { _, err := l.FindStatement(ctx, id, 0, 1); return err },
		func() error { _, err := l.Pending(ctx, 1); return err },
		func() error { return l.Settle(ctx, pay.ID().String(), true) },
		func() error { _, err := l.SaveReceipt(ctx, pay, dto.TransferResult{}); return err },
		func() error { _, err := l.FindReceipt(ctx, pay.ID()); return err },
		func() error { return l.Seed(ctx, base, 1, 1) },
		func() error { _, err := l.Audit(ctx, nil); return err },
		func() error { return l.Bootstrap(ctx) },
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, context.Canceled) {
			t.Fatal(i, err)
		}
	}
	if _, err := Open("no-port"); err == nil {
		t.Fatal("expected address error")
	}
	if _, err := Open("host.invalid:3000"); err == nil {
		t.Fatal("expected resolve error")
	}
	slow, done := context.WithTimeout(context.Background(), time.Millisecond)
	defer done()
	<-slow.Done()
	if _, err := l.Authorize(slow, pay); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestIDs(t *testing.T) {
	id := "0190f3a2-7c1e-4d2b-9a6f-1234567890ab"
	tx, _ := valueobject.NewTransactionID(id)
	if uuidOf(transferID(tx)) != id || transferID(tx).String() != "190f3a27c1e4d2b9a6f1234567890ab" {
		t.Fatal(uuidOf(transferID(tx)))
	}
	derived := uuidOf(derive("settle", transferID(tx)))
	if _, err := valueobject.NewTransactionID(derived); !errors.Is(err, errs.ErrInvalid) || derived[19] < 'c' {
		t.Fatal(derived)
	}
	if accountID(-3) != u128(1, 3) || clientOf(accountID(7)) != 7 {
		t.Fatal("account ids")
	}
	if _, err := minorOf(u128(1, 0)); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	if _, err := minorOf(tb.ToUint128(math.MaxInt64 + 1)); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	if _, err := reasonOf(0); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	if _, err := readRef(tb.Transfer{UserData32: 1}, nil); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
}
