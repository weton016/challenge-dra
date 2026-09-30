package postgres

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"dracma/pix/internal/application/dto"
)

const (
	queueSlots    = 4096
	batchDeadline = 1200 * time.Millisecond
)

type answer struct {
	result dto.TransferResult
	err    error
}
type pending struct {
	request dto.TransferCommand
	reply   chan answer
	queued  time.Time
}

type queueStats struct {
	enqueued       atomic.Int64
	queueFull      atomic.Int64
	timeoutBefore  atomic.Int64
	timeoutWaiting atomic.Int64
	batchOK        atomic.Int64
	batchFail      atomic.Int64
	lastSize       atomic.Int64
	lastMs         atomic.Int64
	maxMs          atomic.Int64
	maxWait        atomic.Int64
}

// Batcher has a bounded, non-durable queue. Only a committed batch is approved.
// The batch keeps its own deadline: a disconnected caller does not cancel the others.
type Batcher struct {
	store  *Store
	queue  chan pending
	size   int
	window time.Duration
	stats  queueStats
}

func NewBatcher(ctx context.Context, s *Store, size int, window time.Duration) *Batcher {
	if window <= 0 {
		window = 8 * time.Millisecond
	}
	b := &Batcher{store: s, queue: make(chan pending, queueSlots), size: max(1, min(size, 64)), window: window}
	go b.run(ctx)
	return b
}

func (b *Batcher) Authorize(ctx context.Context, r dto.TransferCommand) (dto.TransferResult, error) {
	if err := r.Validate(); err != nil {
		return dto.TransferResult{}, err
	}
	item := pending{request: r, reply: make(chan answer, 1), queued: time.Now()}
	if err := b.enqueue(ctx, item); err != nil {
		return dto.TransferResult{}, err
	}
	return b.wait(ctx, item)
}

func (b *Batcher) enqueue(ctx context.Context, item pending) error {
	select {
	case <-ctx.Done():
		b.stats.timeoutBefore.Add(1)
		return ctx.Err()
	case b.queue <- item:
		b.stats.enqueued.Add(1)
		return nil
	default:
		b.stats.queueFull.Add(1)
		return errors.New("authorization queue full; retry same id")
	}
}

func (b *Batcher) wait(ctx context.Context, item pending) (dto.TransferResult, error) {
	select {
	case got := <-item.reply:
		return got.result, got.err
	case <-ctx.Done():
		select {
		case got := <-item.reply:
			return got.result, got.err
		default:
			waited := time.Since(item.queued).Milliseconds()
			b.stats.timeoutWaiting.Add(1)
			b.noteMax(&b.stats.maxWait, waited)
			return dto.TransferResult{}, ctx.Err()
		}
	}
}

func (b *Batcher) run(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			b.logStats()
		case first := <-b.queue:
			b.apply(ctx, b.collect(ctx, first))
		}
	}
}

func (b *Batcher) apply(ctx context.Context, batch []pending) {
	started := time.Now()
	batchCtx, cancel := context.WithTimeout(ctx, batchDeadline)
	results, err := b.store.applyBatch(batchCtx, batch)
	cancel()
	elapsed := time.Since(started).Milliseconds()
	b.stats.lastSize.Store(int64(len(batch)))
	b.stats.lastMs.Store(elapsed)
	b.noteMax(&b.stats.maxMs, elapsed)
	if err != nil {
		b.stats.batchFail.Add(1)
		slog.Warn("batch failed", "reason", "batch_error", "size", len(batch), "ms", elapsed, "depth", len(b.queue), "err", err)
	} else {
		b.stats.batchOK.Add(1)
	}
	for i, item := range batch {
		if err != nil {
			item.reply <- answer{err: err}
			continue
		}
		item.reply <- results[i]
	}
}

func (b *Batcher) logStats() {
	slog.Info("queue stats",
		"enqueued", b.stats.enqueued.Swap(0),
		"queue_full", b.stats.queueFull.Swap(0),
		"timeout_before", b.stats.timeoutBefore.Swap(0),
		"timeout_waiting", b.stats.timeoutWaiting.Swap(0),
		"batch_ok", b.stats.batchOK.Swap(0),
		"batch_fail", b.stats.batchFail.Swap(0),
		"depth", len(b.queue),
		"last_batch_size", b.stats.lastSize.Load(),
		"last_batch_ms", b.stats.lastMs.Load(),
		"max_batch_ms", b.stats.maxMs.Swap(0),
		"max_wait_ms", b.stats.maxWait.Swap(0),
	)
}

func (b *Batcher) noteMax(slot *atomic.Int64, value int64) {
	for {
		old := slot.Load()
		if value <= old || slot.CompareAndSwap(old, value) {
			return
		}
	}
}

func (b *Batcher) collect(ctx context.Context, first pending) []pending {
	batch := b.drain(first)
	if len(batch) >= b.size {
		return batch
	}
	timer := time.NewTimer(b.window)
	defer timer.Stop()
	for len(batch) < b.size {
		select {
		case <-ctx.Done():
			return batch
		case <-timer.C:
			return batch
		case item := <-b.queue:
			batch = append(batch, item)
		}
	}
	return batch
}

// drain takes work already waiting so a busy account fills one commit
// instead of closing a short batch while the queue is still full.
func (b *Batcher) drain(first pending) []pending {
	batch := []pending{first}
	for len(batch) < b.size {
		select {
		case item := <-b.queue:
			batch = append(batch, item)
		default:
			return batch
		}
	}
	return batch
}
