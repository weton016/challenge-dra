package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"dracma/pix/internal/ledger"
	"github.com/jackc/pgx/v5"
)

const writerLock = 937513

var errBatchRetry = errors.New("conflicting transaction id survived batch retry")

type savedOperation struct {
	Request ledger.Request `json:"payload"`
	ledger.Result
}
type batchEntry struct {
	ID       string `json:"id"`
	Account  int64  `json:"account"`
	Delta    int64  `json:"delta"`
	Sequence int64  `json:"sequence"`
	Balance  int64  `json:"balance"`
}
type batchState struct {
	accounts map[int64]ledger.Account
	known    map[string]savedOperation
	added    []savedOperation
	entries  []batchEntry
}

// applyBatch locks only the accounts in the batch, in id order. Disjoint
// batches run in parallel. A duplicate id is replayed or rejected alone;
// it does not roll back the other decisions.
func (s *Store) applyBatch(ctx context.Context, batch []pending) ([]answer, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	// One batch commits at a time. Two executors waiting on the hot row
	// were updating that line twice as often and pushing the tail past 300 ms.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(writerLock)); err != nil {
		return nil, err
	}
	var results []answer
	for pass := 0; pass < 3; pass++ {
		state, err := readBatch(ctx, tx, batch)
		if err != nil {
			return nil, err
		}
		results = state.decideAll(batch)
		if _, err = tx.Exec(ctx, "SAVEPOINT batch_write"); err != nil {
			return nil, err
		}
		skipped, err := state.write(ctx, tx)
		if err != nil {
			return nil, err
		}
		if len(skipped) == 0 {
			if err = tx.Commit(ctx); err != nil {
				return nil, err
			}
			return results, nil
		}
		if _, err = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT batch_write"); err != nil {
			return nil, err
		}
	}
	return nil, errBatchRetry
}

func readBatch(ctx context.Context, tx pgx.Tx, batch []pending) (*batchState, error) {
	ids := make([]string, 0, len(batch))
	accountIDs := make([]int64, 0, len(batch)*2)
	for _, p := range batch {
		ids = append(ids, p.request.ID)
		accountIDs = append(accountIDs, p.request.Source, p.request.Target())
	}
	state := &batchState{accounts: map[int64]ledger.Account{}, known: map[string]savedOperation{}}
	// The log tail is the balance. The account row is not locked or rewritten here.
	rows, err := tx.Query(ctx, `SELECT a.id,
COALESCE(t.balance_after_minor, a.balance_minor),
COALESCE(t.account_sequence, a.last_sequence)
FROM accounts a
LEFT JOIN LATERAL (
 SELECT balance_after_minor, account_sequence FROM entries e
 WHERE e.account_id=a.id ORDER BY account_sequence DESC LIMIT 1
) t ON true
WHERE a.id = ANY($1) ORDER BY a.id`, accountIDs)
	if err != nil {
		return nil, err
	}
	accounts, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ledger.Account])
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		state.accounts[account.ID] = account
	}
	rows, err = tx.Query(ctx, `SELECT jsonb_build_object('payload',payload,'transaction_id',id,'decision',decision,'reason',reason,'settlement_status',settlement) FROM operations WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	known, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (savedOperation, error) {
		var data []byte
		var saved savedOperation
		if err := row.Scan(&data); err != nil {
			return saved, err
		}
		err := json.Unmarshal(data, &saved)
		return saved, err
	})
	if err != nil {
		return nil, err
	}
	for _, saved := range known {
		state.known[saved.ID] = saved
	}
	return state, nil
}

func (b *batchState) decideAll(batch []pending) []answer {
	results := make([]answer, len(batch))
	for i, item := range batch {
		results[i] = b.decide(item.request)
	}
	return results
}

func (b *batchState) decide(r ledger.Request) answer {
	if old, ok := b.known[r.ID]; ok {
		if old.Request != r {
			return answer{err: ledger.ErrConflict}
		}
		return answer{result: old.Result}
	}
	source, okSource := b.accounts[r.Source]
	target, okTarget := b.accounts[r.Target()]
	reason := "ACCOUNT_NOT_FOUND"
	if okSource && okTarget {
		reason = ledger.Decide(source.Balance, target.Balance, r.Amount)
	}
	result := ledger.Result{ID: r.ID, Decision: "DECLINED", Reason: reason, Settlement: "INTERNAL"}
	if reason == "" {
		result.Decision = "APPROVED"
		result.Reason = ""
		if r.External != "" {
			result.Settlement = "PENDING"
		}
		b.post(r.ID, source, -r.Amount)
		b.post(r.ID, target, r.Amount)
	}
	saved := savedOperation{Request: r, Result: result}
	b.known[r.ID] = saved
	b.added = append(b.added, saved)
	return answer{result: result}
}

func (b *batchState) post(id string, account ledger.Account, delta int64) {
	account.Balance += delta
	account.Sequence++
	b.accounts[account.ID] = account
	b.entries = append(b.entries, batchEntry{ID: id, Account: account.ID, Delta: delta, Sequence: account.Sequence, Balance: account.Balance})
}

func (b *batchState) write(ctx context.Context, tx pgx.Tx) ([]string, error) {
	if len(b.added) == 0 {
		return nil, nil
	}
	operations, err := json.Marshal(b.added)
	if err != nil {
		return nil, err
	}
	skipped, err := insertOperations(ctx, tx, operations, b.added)
	if err != nil || len(skipped) > 0 {
		return skipped, err
	}
	if err = insertMovements(ctx, tx, b.entries); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox(operation_id)
SELECT transaction_id FROM jsonb_to_recordset($1) AS x(transaction_id text,settlement_status text)
WHERE settlement_status='PENDING'`, operations)
	return nil, err
}

func insertOperations(ctx context.Context, tx pgx.Tx, operations []byte, added []savedOperation) ([]string, error) {
	rows, err := tx.Query(ctx, `INSERT INTO operations(id,payload,decision,reason,settlement)
SELECT transaction_id,payload,decision,coalesce(reason,''),settlement_status
FROM jsonb_to_recordset($1) AS x(transaction_id text,payload jsonb,decision text,reason text,settlement_status text)
ON CONFLICT (id) DO NOTHING RETURNING id`, operations)
	if err != nil {
		return nil, err
	}
	inserted, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})
	if err != nil {
		return nil, err
	}
	if len(inserted) == len(added) {
		return nil, nil
	}
	return missingIDs(added, inserted), nil
}

func insertMovements(ctx context.Context, tx pgx.Tx, entries []batchEntry) error {
	if len(entries) == 0 {
		return nil
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO entries(operation_id,account_id,delta_minor,account_sequence,balance_after_minor)
SELECT id,account,delta,sequence,balance FROM jsonb_to_recordset($1) AS x(id text,account bigint,delta bigint,sequence bigint,balance bigint)`, encoded)
	return err
}

func missingIDs(added []savedOperation, inserted []string) []string {
	have := map[string]bool{}
	for _, id := range inserted {
		have[id] = true
	}
	var missing []string
	for _, saved := range added {
		if !have[saved.ID] {
			missing = append(missing, saved.ID)
		}
	}
	return missing
}
