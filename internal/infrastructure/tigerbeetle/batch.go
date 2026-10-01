package tigerbeetle

import (
	"context"
	"sync"
	"time"

	"dracma/pix/internal/domain/errs"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

// batchMax is the most events one request of a production cluster holds.
const batchMax = 8189

// batch gathers creates for one window. Events in a TigerBeetle request
// succeed or fail one by one (only linked chains are atomic), so sharing a
// request changes no decision; it only saves the per-request cost.
type batch struct {
	transfers []tb.Transfer
	results   []tb.CreateTransferResult
	err       error
	done      chan struct{}
}

type batcher struct {
	window time.Duration
	mu     sync.Mutex
	open   *batch
}

func (l *Ledger) create(ctx context.Context, transfers []tb.Transfer) ([]tb.CreateTransferResult, error) {
	if l.batch.window == 0 || len(transfers) > batchMax {
		results, err := call(ctx, func() ([]tb.CreateTransferResult, error) { return l.client.CreateTransfers(transfers) })
		return checked(results, len(transfers), err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, offset := l.join(transfers)
	select {
	case <-b.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if b.err != nil {
		return nil, b.err
	}
	return b.results[offset : offset+len(transfers)], nil
}

func (l *Ledger) join(transfers []tb.Transfer) (*batch, int) {
	l.batch.mu.Lock()
	defer l.batch.mu.Unlock()
	b := l.batch.open
	if b == nil || len(b.transfers)+len(transfers) > batchMax {
		b = &batch{done: make(chan struct{})}
		l.batch.open = b
		time.AfterFunc(l.batch.window, func() { l.flush(b) })
	}
	offset := len(b.transfers)
	b.transfers = append(b.transfers, transfers...)
	return b, offset
}

func (l *Ledger) flush(b *batch) {
	l.batch.mu.Lock()
	if l.batch.open == b {
		l.batch.open = nil
	}
	l.batch.mu.Unlock()
	results, err := l.client.CreateTransfers(b.transfers)
	b.results, b.err = checked(results, len(b.transfers), err)
	close(b.done)
}

func checked(results []tb.CreateTransferResult, want int, err error) ([]tb.CreateTransferResult, error) {
	if err == nil && len(results) != want {
		err = errs.ErrInvariant
	}
	return results, err
}
