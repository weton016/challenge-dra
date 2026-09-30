// Command audit independently reconstructs the financial history in one snapshot.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"dracma/pix/internal/infrastructure/postgres"
	"github.com/jackc/pgx/v5"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer s.Pool.Close()
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SET LOCAL statement_timeout='4min'"); err != nil {
		return err
	}
	checks := map[string]string{
		"balance_mismatches":   `SELECT count(*) FROM accounts a LEFT JOIN (SELECT account_id,sum(delta_minor::numeric) delta,count(*) n FROM entries GROUP BY account_id) s ON s.account_id=a.id LEFT JOIN LATERAL (SELECT account_sequence seq, balance_after_minor bal FROM entries e WHERE e.account_id=a.id ORDER BY account_sequence DESC LIMIT 1) t ON true WHERE coalesce(t.seq,0)<>coalesce(s.n,0) OR coalesce(t.bal, a.initial_balance)<>a.initial_balance+coalesce(s.delta,0)`,
		"invalid_prefixes":     `SELECT count(*) FROM (SELECT e.*,a.initial_balance+sum(delta_minor::numeric) OVER(PARTITION BY account_id ORDER BY account_sequence) expected,row_number() OVER(PARTITION BY account_id ORDER BY account_sequence) seq FROM entries e JOIN accounts a ON a.id=e.account_id) h WHERE expected<0 OR expected<>balance_after_minor OR seq<>account_sequence`,
		"invalid_operations":   `SELECT count(*) FROM operations o LEFT JOIN (SELECT operation_id,count(*) n,sum(delta_minor::numeric) total FROM entries GROUP BY operation_id) e ON o.id=e.operation_id WHERE (o.decision='APPROVED' AND (coalesce(e.n,0)<>2 OR e.total<>0)) OR (o.decision='DECLINED' AND coalesce(e.n,0)<>0) OR o.decision='PROCESSING'`,
		"invalid_entry_values": `SELECT count(*) FROM entries e JOIN operations o ON o.id=e.operation_id WHERE abs(e.delta_minor::numeric)<>(o.payload->>'amount_minor')::numeric OR (e.delta_minor<0 AND e.account_id<>(o.payload->>'source_id')::bigint) OR (e.delta_minor>0 AND e.account_id<>CASE WHEN coalesce(o.payload->>'destination_external','')<>'' THEN -((o.payload->>'source_id')::bigint%32+1) ELSE (o.payload->>'destination_id')::bigint END)`,
		"negative_accounts":    `SELECT count(*) FROM accounts a LEFT JOIN LATERAL (SELECT balance_after_minor bal FROM entries e WHERE e.account_id=a.id ORDER BY account_sequence DESC LIMIT 1) t ON true WHERE coalesce(t.bal, a.balance_minor)<0`,
		"conservation_error":   `SELECT CASE WHEN (SELECT sum(coalesce(t.bal, a.balance_minor)::numeric) FROM accounts a LEFT JOIN LATERAL (SELECT balance_after_minor bal FROM entries e WHERE e.account_id=a.id ORDER BY account_sequence DESC LIMIT 1) t ON true)=(SELECT sum(initial_balance::numeric) FROM accounts) THEN 0 ELSE 1 END`,
	}
	report := map[string]any{"at": time.Now().UTC(), "checks": map[string]int64{}}
	failed := false
	for name, query := range checks {
		var n int64
		if err = tx.QueryRow(ctx, query).Scan(&n); err != nil {
			return err
		}
		report["checks"].(map[string]int64)[name] = n
		if n != 0 {
			failed = true
		}
	}
	var accounts, operations, pending int64
	if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM accounts),(SELECT count(*) FROM operations),(SELECT count(*) FROM outbox WHERE NOT done)`).Scan(&accounts, &operations, &pending); err != nil {
		return err
	}
	report["accounts"] = accounts
	report["operations"] = operations
	report["pending_external"] = pending
	report["passed"] = !failed
	if err = json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("financial audit failed")
	}
	return tx.Commit(ctx)
}
