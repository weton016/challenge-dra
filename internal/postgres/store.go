package postgres

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"

	"dracma/pix/internal/ledger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 24
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "2000"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "2000"
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Store{Pool: p}, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(937512)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Authorize(ctx context.Context, r ledger.Request) (ledger.Result, error) {
	if err := r.Validate(); err != nil {
		return ledger.Result{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ledger.Result{}, err
	}
	defer tx.Rollback(context.Background())
	result, err := authorize(ctx, tx, r)
	if err != nil {
		return ledger.Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ledger.Result{}, err
	}
	return result, nil
}

func authorize(ctx context.Context, tx pgx.Tx, r ledger.Request) (ledger.Result, error) {
	payload, _ := json.Marshal(r)
	tag, err := tx.Exec(ctx, `INSERT INTO operations(id,payload,decision) VALUES($1,$2,'PROCESSING') ON CONFLICT DO NOTHING`, r.ID, payload)
	if err != nil {
		return ledger.Result{}, err
	}
	if tag.RowsAffected() == 0 {
		return replay(ctx, tx, r)
	}
	reason, err := move(ctx, tx, r)
	if err != nil {
		return ledger.Result{}, err
	}
	result := ledger.Result{ID: r.ID, Decision: "APPROVED", Reason: reason, Settlement: "INTERNAL"}
	if reason != "" {
		result.Decision = "DECLINED"
	}
	if r.External != "" && reason == "" {
		result.Settlement = "PENDING"
	}
	_, err = tx.Exec(ctx, `UPDATE operations SET decision=$2,reason=$3,settlement=$4 WHERE id=$1`, r.ID, result.Decision, result.Reason, result.Settlement)
	if err != nil {
		return ledger.Result{}, err
	}
	if result.Settlement == "PENDING" {
		_, err = tx.Exec(ctx, `INSERT INTO outbox(operation_id) VALUES($1)`, r.ID)
	}
	return result, err
}

func replay(ctx context.Context, tx pgx.Tx, r ledger.Request) (ledger.Result, error) {
	var saved ledger.Request
	var payload []byte
	var out ledger.Result
	err := tx.QueryRow(ctx, `SELECT payload,id,decision,reason,settlement FROM operations WHERE id=$1`, r.ID).Scan(&payload, &out.ID, &out.Decision, &out.Reason, &out.Settlement)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(payload, &saved); err != nil {
		return out, err
	}
	if saved != r {
		return out, ledger.ErrConflict
	}
	return out, nil
}

func move(ctx context.Context, tx pgx.Tx, r ledger.Request) (string, error) {
	rows, err := tx.Query(ctx, `SELECT id,balance_minor,last_sequence FROM accounts WHERE id=ANY($1) ORDER BY id FOR NO KEY UPDATE`, []int64{r.Source, r.Target()})
	if err != nil {
		return "", err
	}
	accounts, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ledger.Account])
	if err != nil {
		return "", err
	}
	if len(accounts) != 2 {
		return "ACCOUNT_NOT_FOUND", nil
	}
	source, destination := accounts[0], accounts[1]
	if source.ID != r.Source {
		source, destination = destination, source
	}
	reason := ledger.Decide(source.Balance, destination.Balance, r.Amount)
	if reason != "" {
		return reason, nil
	}
	_, err = tx.Exec(ctx, `WITH updated AS (
 UPDATE accounts SET balance_minor=balance_minor+v.delta,last_sequence=last_sequence+1
 FROM (VALUES ($1::bigint,-$3::bigint),($2::bigint,$3::bigint)) v(id,delta)
 WHERE accounts.id=v.id AND accounts.balance_minor+v.delta>=0
 RETURNING accounts.id,accounts.last_sequence,accounts.balance_minor,v.delta)
 INSERT INTO entries(operation_id,account_id,delta_minor,account_sequence,balance_after_minor)
 SELECT $4,id,delta,last_sequence,balance_minor FROM updated`, r.Source, r.Target(), r.Amount, r.ID)
	return "", err
}

func (s *Store) Result(ctx context.Context, id string) (ledger.Result, error) {
	var r ledger.Result
	err := s.Pool.QueryRow(ctx, `SELECT id,decision,reason,settlement FROM operations WHERE id=$1`, id).Scan(&r.ID, &r.Decision, &r.Reason, &r.Settlement)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ledger.ErrNotFound
	}
	return r, err
}

func (s *Store) Balance(ctx context.Context, id int64) (ledger.Account, error) {
	var a ledger.Account
	err := s.Pool.QueryRow(ctx, `SELECT id,balance_minor,last_sequence FROM accounts WHERE id=$1 AND id>0`, id).Scan(&a.ID, &a.Balance, &a.Sequence)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ledger.ErrNotFound
	}
	return a, err
}

func (s *Store) Statement(ctx context.Context, id, after, until int64) (ledger.Statement, error) {
	a, err := s.Balance(ctx, id)
	if err != nil {
		return ledger.Statement{}, err
	}
	if until == 0 || until > a.Sequence {
		until = a.Sequence
	}
	out := ledger.Statement{Until: until, Next: after, Entries: []ledger.Entry{}}
	rows, err := s.Pool.Query(ctx, `SELECT operation_id,delta_minor,account_sequence,balance_after_minor FROM entries WHERE account_id=$1 AND account_sequence>$2 AND account_sequence<=$3 ORDER BY account_sequence LIMIT 100`, id, after, until)
	if err != nil {
		return out, err
	}
	out.Entries, err = pgx.CollectRows(rows, pgx.RowToStructByPos[ledger.Entry])
	if len(out.Entries) > 0 {
		out.Next = out.Entries[len(out.Entries)-1].Sequence
	}
	return out, err
}

// Seed never overwrites existing accounts or balances.
func (s *Store) Seed(ctx context.Context, count, balance int64) error {
	if count < 1 || balance < 0 {
		return ledger.ErrInvalid
	}
	for start := int64(1); start <= count; start += 10000 {
		_, err := s.Pool.Exec(ctx, `INSERT INTO accounts(id,balance_minor,initial_balance) SELECT i,$3,$3 FROM generate_series($1::bigint,$2::bigint) i ON CONFLICT DO NOTHING`, start, min(start+9999, count), balance)
		if err != nil {
			return err
		}
	}
	return nil
}
