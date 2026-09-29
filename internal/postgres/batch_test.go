package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dracma/pix/internal/ledger"
)

func TestBatchIntermediateBalancesAndReplay(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.Pool.Exec(ctx, "UPDATE accounts SET balance_minor=$1 WHERE id=2", int64(math.MaxInt64)); err != nil {
		t.Fatal(err)
	}
	results, err := s.applyBatch(ctx, []pending{
		{request: ledger.Request{ID: "exact", Source: 1, Destination: 3, Amount: 100}},
		{request: ledger.Request{ID: "exact", Source: 1, Destination: 3, Amount: 100}},
		{request: ledger.Request{ID: "over", Source: 1, Destination: 3, Amount: 1}},
		{request: ledger.Request{ID: "credit", Source: 3, Destination: 1, Amount: 40}},
		{request: ledger.Request{ID: "overflow", Source: 3, Destination: 2, Amount: 1}},
		{request: ledger.Request{ID: "missing", Source: 1, Destination: 99, Amount: 1}},
		{request: ledger.Request{ID: "ext", Source: 1, External: "shop", Amount: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"exact": "", "over": "INSUFFICIENT_FUNDS", "credit": "", "overflow": "BALANCE_OVERFLOW", "missing": "ACCOUNT_NOT_FOUND", "ext": ""}
	seen := map[string]int{}
	for _, result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		seen[result.result.ID]++
		if reason := want[result.result.ID]; result.result.Reason != reason {
			t.Fatalf("%+v", result.result)
		}
	}
	if seen["exact"] != 2 || results[0].result.Decision != "APPROVED" || results[1].result != results[0].result {
		t.Fatalf("%+v", results)
	}
	if results[6].result.Settlement != "PENDING" {
		t.Fatal(results[6].result)
	}
	assertBalance(t, s, 1, 30)
	assertBalance(t, s, 3, 160)
	var entries, outbox int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM entries WHERE operation_id IN ('over','overflow','missing')").Scan(&entries); err != nil || entries != 0 {
		t.Fatal(entries, err)
	}
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE operation_id='ext' AND NOT done").Scan(&outbox); err != nil || outbox != 1 {
		t.Fatal(outbox, err)
	}
	again, err := s.applyBatch(ctx, []pending{{request: ledger.Request{ID: "exact", Source: 1, Destination: 3, Amount: 100}}})
	if err != nil || again[0].result != results[0].result {
		t.Fatal(again, err)
	}
	assertBalance(t, s, 1, 30)
	conflict, err := s.applyBatch(ctx, []pending{
		{request: ledger.Request{ID: "exact", Source: 1, Destination: 3, Amount: 99}},
		{request: ledger.Request{ID: "after", Source: 3, Destination: 1, Amount: 5}},
	})
	if err != nil || !errors.Is(conflict[0].err, ledger.ErrConflict) || conflict[1].result.Decision != "APPROVED" {
		t.Fatal(conflict, err)
	}
	assertBalance(t, s, 1, 35)
}

func TestTwoBatchersConserveHotAccount(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	left := NewBatcher(ctx, s, 16, 2*time.Millisecond)
	right := NewBatcher(ctx, s, 16, 2*time.Millisecond)
	var wg sync.WaitGroup
	var approved atomic.Int32
	for i := 0; i < 40; i++ {
		wg.Add(1)
		batcher := left
		if i%2 == 1 {
			batcher = right
		}
		go func(i int, batcher *Batcher) {
			defer wg.Done()
			result, err := batcher.Authorize(ctx, ledger.Request{ID: fmt.Sprintf("hot-%d", i), Source: 1, Destination: 2, Amount: 10})
			if err != nil {
				t.Error(err)
				return
			}
			if result.Decision == "APPROVED" {
				approved.Add(1)
			}
		}(i, batcher)
	}
	wg.Wait()
	if approved.Load() != 10 {
		t.Fatalf("approved %d", approved.Load())
	}
	assertBalance(t, s, 1, 0)
	var negative int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM entries WHERE account_id=1 AND balance_after_minor<0").Scan(&negative); err != nil || negative != 0 {
		t.Fatal(negative, err)
	}
}

func TestParallelBatchesIsolateDuplicateID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `INSERT INTO accounts(id,balance_minor,initial_balance) VALUES (4,100,100),(5,100,100),(6,100,100)`)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 8; n++ {
		shared, left, right := fmt.Sprintf("shared-%d", n), fmt.Sprintf("left-%d", n), fmt.Sprintf("right-%d", n)
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, errs[0] = s.applyBatch(ctx, []pending{
				{request: ledger.Request{ID: shared, Source: 1, Destination: 2, Amount: 1}},
				{request: ledger.Request{ID: left, Source: 1, Destination: 2, Amount: 1}},
			})
		}()
		go func() {
			defer wg.Done()
			<-start
			_, errs[1] = s.applyBatch(ctx, []pending{
				{request: ledger.Request{ID: shared, Source: 3, Destination: 4, Amount: 1}},
				{request: ledger.Request{ID: right, Source: 5, Destination: 6, Amount: 1}},
			})
		}()
		close(start)
		wg.Wait()
		for _, got := range errs {
			if got != nil {
				t.Fatal(got)
			}
		}
		assertEntries(t, s, shared, 2)
		assertEntries(t, s, left, 2)
		assertEntries(t, s, right, 2)
	}
}

func TestBatchTimeoutIsNotADecline(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT id FROM accounts WHERE id=1 FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(ctx)
	t.Cleanup(stop)
	batcher := NewBatcher(runCtx, s, 4, time.Millisecond)
	done := make(chan error, 1)
	go func() {
		_, err := batcher.Authorize(ctx, ledger.Request{ID: "slow", Source: 1, Destination: 2, Amount: 10})
		done <- err
	}()
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("expected timeout")
		}
	case <-time.After(batchDeadline + 2*time.Second):
		t.Fatal("batch did not time out")
	}
	var count int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM operations WHERE id='slow'").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := batcher.Authorize(ctx, ledger.Request{ID: "slow", Source: 1, Destination: 2, Amount: 10})
	if err != nil || got.Decision != "APPROVED" {
		t.Fatal(got, err)
	}
}

func TestCallerCancelDoesNotAbortBatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT id FROM accounts WHERE id=1 FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(ctx)
	t.Cleanup(stop)
	batcher := NewBatcher(runCtx, s, 8, 15*time.Millisecond)
	caller, cancelCaller := context.WithCancel(ctx)
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() {
		_, err := batcher.Authorize(caller, ledger.Request{ID: "cancelled", Source: 1, Destination: 2, Amount: 1})
		first <- err
	}()
	time.Sleep(20 * time.Millisecond)
	go func() {
		_, err := batcher.Authorize(ctx, ledger.Request{ID: "kept", Source: 1, Destination: 2, Amount: 1})
		second <- err
	}()
	time.Sleep(40 * time.Millisecond)
	cancelCaller()
	select {
	case err = <-first:
		if err == nil {
			t.Fatal("cancelled caller got a decision")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not return")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("remaining request was cancelled with the caller")
	}
	assertEntries(t, s, "cancelled", 2)
	assertEntries(t, s, "kept", 2)
}

func TestQueueFullRejectsWithoutWaiting(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT id FROM accounts WHERE id=1 FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(ctx)
	t.Cleanup(stop)
	batcher := NewBatcher(runCtx, s, 8, time.Millisecond)
	var full atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < queueSlots+400; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_, err := batcher.Authorize(callCtx, ledger.Request{ID: fmt.Sprintf("queued-%d", i), Source: 1, Destination: 2, Amount: 1})
			if err != nil && err.Error() == "authorization queue full; retry same id" {
				full.Add(1)
			}
		}(i)
	}
	deadline := time.Now().Add(5 * time.Second)
	for full.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if full.Load() == 0 {
		t.Fatal("expected an immediate queue rejection")
	}
}

func TestBatcherRejectsInvalidRequest(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher := NewBatcher(ctx, s, 4, time.Millisecond)
	if _, err := batcher.Authorize(ctx, ledger.Request{}); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatal(err)
	}
	done, cancelDone := context.WithCancel(ctx)
	cancelDone()
	if _, err := batcher.Authorize(done, ledger.Request{ID: "late", Source: 1, Destination: 2, Amount: 1}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestBalanceFollowsLogWhenRowIsStale(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.applyBatch(ctx, []pending{{request: ledger.Request{ID: "log-tail", Source: 1, Destination: 2, Amount: 40}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, "UPDATE accounts SET balance_minor=0, last_sequence=0 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, s, 1, 60)
}

func assertBalance(t *testing.T, s *Store, id, want int64) {
	t.Helper()
	account, err := s.Balance(context.Background(), id)
	if err != nil || account.Balance != want {
		t.Fatalf("account %d balance %d err %v", id, account.Balance, err)
	}
}

func assertEntries(t *testing.T, s *Store, id string, want int) {
	t.Helper()
	var got int
	err := s.Pool.QueryRow(context.Background(), "SELECT count(*) FROM entries WHERE operation_id=$1", id).Scan(&got)
	if err != nil || got != want {
		t.Fatalf("%s entries %d err %v", id, got, err)
	}
}
