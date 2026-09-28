package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"dracma/pix/internal/ledger"
)

func TestTransferAndReplay(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	r := ledger.Request{ID: "one", Source: 1, Destination: 2, Amount: 100}
	got, err := s.Authorize(ctx, r)
	if err != nil || got.Decision != "APPROVED" {
		t.Fatalf("%+v %v", got, err)
	}
	again, err := s.Authorize(ctx, r)
	if err != nil || again != got {
		t.Fatalf("%+v %v", again, err)
	}
	r.Amount = 99
	if _, err = s.Authorize(ctx, r); !errors.Is(err, ledger.ErrConflict) {
		t.Fatal(err)
	}
	a, _ := s.Balance(ctx, 1)
	if a.Balance != 0 {
		t.Fatal(a)
	}
	b, _ := s.Balance(ctx, 2)
	if b.Balance != 200 {
		t.Fatal(b)
	}
	st, err := s.Statement(ctx, 1, 0, 0)
	if err != nil || len(st.Entries) != 1 || st.Next != 1 || st.Until != 1 {
		t.Fatalf("%+v %v", st, err)
	}
	st, err = s.Statement(ctx, 1, 1, 1)
	if err != nil || len(st.Entries) != 0 {
		t.Fatal(st, err)
	}
	if _, err = s.Balance(ctx, 999); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.Result(ctx, "missing"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.Statement(ctx, 999, 0, 0); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.Authorize(ctx, ledger.Request{}); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatal(err)
	}
	if err = s.Seed(ctx, 0, 100); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestDeclinesRemainTerminal(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	r := ledger.Request{ID: "denied", Source: 1, Destination: 2, Amount: 101}
	got, err := s.Authorize(ctx, r)
	if err != nil || got.Reason != "INSUFFICIENT_FUNDS" {
		t.Fatal(got, err)
	}
	_, err = s.Authorize(ctx, ledger.Request{ID: "deposit", Source: 2, Destination: 1, Amount: 100})
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Authorize(ctx, r)
	if err != nil || got != again {
		t.Fatal(again, err)
	}
	r.ID = "absent"
	r.Destination = 999
	got, err = s.Authorize(ctx, r)
	if err != nil || got.Reason != "ACCOUNT_NOT_FOUND" {
		t.Fatal(got, err)
	}
	_, err = s.Pool.Exec(ctx, "UPDATE accounts SET balance_minor=$1 WHERE id=3", int64(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}
	r.ID = "overflow"
	r.Destination = 3
	r.Amount = 1
	got, err = s.Authorize(ctx, r)
	if err != nil || got.Reason != "BALANCE_OVERFLOW" {
		t.Fatal(got, err)
	}
}

func TestConcurrentDebitsAndDuplicateIDs(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan ledger.Result, 100)
	errs := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, e := s.Authorize(ctx, ledger.Request{ID: fmt.Sprintf("race-%d", i), Source: 1, Destination: 2, Amount: 10})
			results <- r
			errs <- e
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	approved := 0
	for r := range results {
		if r.Decision == "APPROVED" {
			approved++
		}
	}
	if approved != 10 {
		t.Fatalf("approved %d", approved)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Authorize(ctx, ledger.Request{ID: "same", Source: 3, Destination: 1, Amount: 10})
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	var count int
	_ = s.Pool.QueryRow(ctx, "SELECT count(*) FROM entries WHERE operation_id='same'").Scan(&count)
	if count != 2 {
		t.Fatal(count)
	}
	a, _ := s.Balance(ctx, 1)
	if a.Balance != 10 {
		t.Fatal(a)
	}
}

func TestTimeoutRollsBackAndRetry(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, "SELECT id FROM accounts WHERE id=1 FOR UPDATE")
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	r := ledger.Request{ID: "timeout", Source: 1, Destination: 2, Amount: 10}
	if _, err = s.Authorize(short, r); err == nil {
		t.Fatal("expected timeout")
	}
	_ = tx.Rollback(ctx)
	got, err := s.Authorize(ctx, r)
	if err != nil || got.Decision != "APPROVED" {
		t.Fatal(got, err)
	}
	a, _ := s.Balance(ctx, 1)
	if a.Balance != 90 {
		t.Fatal(a)
	}
}

func TestExternalSettlementAndRefund(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	r := ledger.Request{ID: "external", Source: 1, External: "receiver", Amount: 20}
	got, err := s.Authorize(ctx, r)
	if err != nil || got.Settlement != "PENDING" {
		t.Fatal(got, err)
	}
	claimed, found, err := s.Claim(ctx)
	if err != nil || !found || claimed != r {
		t.Fatal(claimed, found, err)
	}
	_, found, err = s.Claim(ctx)
	if err != nil || found {
		t.Fatal(found, err)
	}
	receipt, err := s.Receive(ctx, r)
	if err != nil || receipt.Decision != "APPROVED" {
		t.Fatal(receipt, err)
	}
	again, err := s.Receive(ctx, r)
	if err != nil || again != receipt {
		t.Fatal(again, err)
	}
	changed := r
	changed.Amount = 21
	if _, err = s.Receive(ctx, changed); !errors.Is(err, ledger.ErrConflict) {
		t.Fatal(err)
	}
	if err = s.Settle(ctx, r.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = s.Settle(ctx, r.ID, false); err != nil {
		t.Fatal(err)
	}
	r.ID = "rejected"
	r.External = "reject:recipient"
	if _, err = s.Authorize(ctx, r); err != nil {
		t.Fatal(err)
	}
	receipt, err = s.Receive(ctx, r)
	if err != nil || receipt.Decision != "DECLINED" {
		t.Fatal(receipt, err)
	}
	if err = s.Settle(ctx, r.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = s.Settle(ctx, r.ID, false); err != nil {
		t.Fatal(err)
	}
	a, _ := s.Balance(ctx, 1)
	if a.Balance != 80 {
		t.Fatal(a)
	}
	got, _ = s.Result(ctx, r.ID)
	if got.Settlement != "REVERSED" {
		t.Fatal(got)
	}
	if _, err = s.Receive(ctx, ledger.Request{}); err == nil {
		t.Fatal("invalid accepted")
	}
	r.External = ""
	r.Destination = 2
	if _, err = s.Receive(ctx, r); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatal(err)
	}
}
