package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/errs"
)

func TestOpenRejectsBadURLAndUnreachableDatabase(t *testing.T) {
	if _, err := Open(context.Background(), "http://localhost/pix"); err == nil {
		t.Fatal("parsed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := Open(ctx, "postgres://pix:pix-local-only@127.0.0.1:1/pix_test?sslmode=disable"); err == nil {
		t.Fatal("ping")
	}
}

func TestSeedRejectsNegativeBalance(t *testing.T) {
	s := testStore(t)
	if err := s.Seed(context.Background(), 1, -1); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestReplayRejectsCorruptPayload(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `INSERT INTO operations(id,payload,decision) VALUES ('corrupt','"bad"','APPROVED')`)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Balance(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authorize(ctx, dto.TransferCommand{ID: "corrupt", Source: 1, Destination: 2, Amount: 10}); err == nil {
		t.Fatal("corrupt replay approved")
	}
	after, err := s.Balance(ctx, 1)
	if err != nil || after != before {
		t.Fatal(after, before, err)
	}
}

func TestReceiptLookup(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	command := dto.TransferCommand{ID: "ext", Source: 1, External: "shop", Amount: 5}
	saved, err := s.Receive(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Receipt(ctx, command.ID)
	if err != nil || got.ID != command.ID || got.Decision != saved.Decision || got.Settlement != "SETTLED" {
		t.Fatal(got, err)
	}
	if _, err = s.Receipt(ctx, "absent"); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestClaimAndReceiveRejectCorruptPayload(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `INSERT INTO operations(id,payload,decision,settlement) VALUES ('bad-claim','"bad"','APPROVED','PENDING')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, `INSERT INTO outbox(operation_id) VALUES ('bad-claim')`); err != nil {
		t.Fatal(err)
	}
	claimed, _, err := s.Claim(ctx)
	if err == nil || claimed.ID != "" {
		t.Fatal(claimed, err)
	}
	if _, err = s.Pool.Exec(ctx, `INSERT INTO external_receipts(id,payload,decision,amount_minor) VALUES ('bad-recv','"bad"','APPROVED',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Receive(ctx, dto.TransferCommand{ID: "bad-recv", Source: 1, External: "shop", Amount: 1}); err == nil {
		t.Fatal("corrupt receipt")
	}
}

func TestCorruptRefundRollsBack(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	command := dto.TransferCommand{ID: "external", Source: 1, External: "receiver", Amount: 20}
	if _, err := s.Authorize(ctx, command); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE operations SET payload='"bad"' WHERE id=$1`, command.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Settle(ctx, command.ID, false); err == nil {
		t.Fatal("refund decoded a corrupt payload")
	}
	got, err := s.Result(ctx, command.ID)
	if err != nil || got.Settlement != "PENDING" {
		t.Fatal(got, err)
	}
	var refunds int
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM operations WHERE id LIKE 'refund:%'`).Scan(&refunds); err != nil || refunds != 0 {
		t.Fatal(refunds, err)
	}
	account, err := s.Balance(ctx, 1)
	if err != nil || account.Balance != 80 {
		t.Fatal(account, err)
	}
}

func TestRefundWithoutFundsStaysPending(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	command := dto.TransferCommand{ID: "external", Source: 1, External: "receiver", Amount: 20}
	if _, err := s.Authorize(ctx, command); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO operations(id,payload,decision) VALUES ('drain','{}','APPROVED')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO entries(operation_id,account_id,delta_minor,account_sequence,balance_after_minor) VALUES ('drain',-2,-20,2,0)`); err != nil {
		t.Fatal(err)
	}
	if err := s.Settle(ctx, command.ID, false); err == nil || err.Error() != "refund invariant failed" {
		t.Fatal(err)
	}
	got, err := s.Result(ctx, command.ID)
	if err != nil || got.Settlement != "PENDING" {
		t.Fatal(got, err)
	}
	account, err := s.Balance(ctx, 1)
	if err != nil || account.Balance != 80 {
		t.Fatal(account, err)
	}
	var refunds int
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM operations WHERE id LIKE 'refund:%'`).Scan(&refunds); err != nil || refunds != 0 {
		t.Fatal(refunds, err)
	}
}

func TestClosedPoolDoesNotDecide(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.Pool.Close()
	command := dto.TransferCommand{ID: "closed", Source: 1, Destination: 2, Amount: 1}
	if _, err := s.Authorize(ctx, command); err == nil {
		t.Fatal("authorize")
	}
	if _, err := s.Result(ctx, "closed"); err == nil {
		t.Fatal("result")
	}
	if _, err := s.Balance(ctx, 1); err == nil {
		t.Fatal("balance")
	}
	if _, err := s.Statement(ctx, 1, 0, 0); err == nil {
		t.Fatal("statement")
	}
	if err := s.Seed(ctx, 1, 0); err == nil {
		t.Fatal("seed")
	}
	if err := s.Migrate(ctx); err == nil {
		t.Fatal("migrate")
	}
	if _, _, err := s.Claim(ctx); err == nil {
		t.Fatal("claim")
	}
	if err := s.Settle(ctx, "closed", true); err == nil {
		t.Fatal("settle")
	}
	if _, err := s.Receipt(ctx, "closed"); err == nil {
		t.Fatal("receipt")
	}
	if _, err := s.Receive(ctx, dto.TransferCommand{ID: "closed", Source: 1, External: "shop", Amount: 1}); err == nil {
		t.Fatal("receive")
	}
	if _, err := s.applyBatch(ctx, []pending{{request: command}}); err == nil {
		t.Fatal("batch")
	}
}

func TestMissingIDs(t *testing.T) {
	added := []savedOperation{
		{TransferResult: dto.TransferResult{ID: "a"}},
		{TransferResult: dto.TransferResult{ID: "b"}},
		{TransferResult: dto.TransferResult{ID: "c"}},
	}
	got := missingIDs(added, []string{"c", "a"})
	if len(got) != 1 || got[0] != "b" {
		t.Fatal(got)
	}
	if missingIDs(added, []string{"a", "b", "c"}) != nil {
		t.Fatal("complete insert")
	}
	all := missingIDs(added, nil)
	if len(all) != 3 || all[0] != "a" || all[2] != "c" {
		t.Fatal(all)
	}
}

func TestConflictingBatchInsertDoesNotPostEntries(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	held := dto.TransferCommand{ID: "held", Source: 1, Destination: 2, Amount: 1}
	if _, err := s.Authorize(ctx, held); err != nil {
		t.Fatal(err)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	fresh := dto.TransferCommand{ID: "fresh", Source: 1, Destination: 2, Amount: 1}
	state := &batchState{added: []savedOperation{
		{Request: held, TransferResult: dto.TransferResult{ID: held.ID, Decision: "APPROVED", Settlement: "INTERNAL"}},
		{Request: fresh, TransferResult: dto.TransferResult{ID: fresh.ID, Decision: "DECLINED", Reason: "INSUFFICIENT_FUNDS", Settlement: "INTERNAL"}},
	}}
	skipped, err := state.write(ctx, tx)
	if err != nil || len(skipped) != 1 || skipped[0] != "held" {
		t.Fatal(skipped, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var freshOps, freshEntries int
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM operations WHERE id='fresh'`).Scan(&freshOps); err != nil || freshOps != 0 {
		t.Fatal(freshOps, err)
	}
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM entries WHERE operation_id='fresh'`).Scan(&freshEntries); err != nil || freshEntries != 0 {
		t.Fatal(freshEntries, err)
	}
}

func TestBatcherStopsAndLateReplyWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	small := NewBatcher(ctx, nil, 0, 0)
	if small.size != 1 || small.window != 8*time.Millisecond {
		t.Fatal(small.size, small.window)
	}
	large := NewBatcher(ctx, nil, 1000, time.Millisecond)
	if large.size != 64 || large.window != time.Millisecond {
		t.Fatal(large.size, large.window)
	}
	small.logStats()
	cancel()
	time.Sleep(20 * time.Millisecond)

	done, stop := context.WithCancel(context.Background())
	stop()
	for i := 0; i < 40; i++ {
		item := pending{reply: make(chan answer, 1), queued: time.Now()}
		item.reply <- answer{result: dto.TransferResult{ID: "done", Decision: "APPROVED"}, err: errs.ErrInvariant}
		got, err := small.wait(done, item)
		if got.ID != "done" || got.Decision != "APPROVED" || !errors.Is(err, errs.ErrInvariant) {
			t.Fatal(got, err)
		}
	}
	waiting := &Batcher{size: 8, window: time.Hour, queue: make(chan pending)}
	batch := waiting.collect(done, pending{request: dto.TransferCommand{ID: "only"}})
	if len(batch) != 1 || batch[0].request.ID != "only" {
		t.Fatal(len(batch))
	}
}
