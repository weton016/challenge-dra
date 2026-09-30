package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/errs"
	"github.com/jackc/pgx/v5"
)

func (s *Store) Claim(ctx context.Context) (dto.TransferCommand, bool, error) {
	var payload []byte
	err := s.Pool.QueryRow(ctx, `WITH claimed AS (
 UPDATE outbox SET lease_until=now()+interval '2 seconds',attempts=attempts+1
 WHERE operation_id=(SELECT operation_id FROM outbox WHERE NOT done AND lease_until<now()
 ORDER BY lease_until FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING operation_id)
 SELECT payload FROM operations JOIN claimed ON operations.id=claimed.operation_id`).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.TransferCommand{}, false, nil
	}
	if err != nil {
		return dto.TransferCommand{}, false, err
	}
	var r dto.TransferCommand
	err = json.Unmarshal(payload, &r)
	return r, true, err
}

// Settle serializes confirmation and refund on the original operation row.
func (s *Store) Settle(ctx context.Context, id string, accepted bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var payload []byte
	var state string
	err = tx.QueryRow(ctx, `SELECT payload,settlement FROM operations WHERE id=$1 FOR UPDATE`, id).Scan(&payload, &state)
	if err != nil {
		return err
	}
	if state != "PENDING" {
		return nil
	}
	state = "SETTLED"
	if !accepted {
		if err = refund(ctx, tx, id, payload); err != nil {
			return err
		}
		state = "REVERSED"
	}
	_, err = tx.Exec(ctx, `UPDATE operations SET settlement=$2 WHERE id=$1`, id, state)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE outbox SET done=true WHERE operation_id=$1`, id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func refund(ctx context.Context, tx pgx.Tx, id string, payload []byte) error {
	var original dto.TransferCommand
	if err := json.Unmarshal(payload, &original); err != nil {
		return err
	}
	r := dto.TransferCommand{ID: "refund:" + id, Source: original.Target(), Destination: original.Source, Amount: original.Amount}
	result, err := authorize(ctx, tx, r)
	if err != nil {
		return err
	}
	if result.Decision != "APPROVED" {
		return errors.New("refund invariant failed")
	}
	_, err = tx.Exec(ctx, `UPDATE operations SET reverses_id=$2 WHERE id=$1`, r.ID, id)
	return err
}

// Receive is the simulated institution's separate transaction boundary.
func (s *Store) Receipt(ctx context.Context, id string) (dto.TransferResult, error) {
	r := dto.TransferResult{ID: id, Settlement: "SETTLED"}
	err := s.Pool.QueryRow(ctx, `SELECT decision FROM external_receipts WHERE id=$1`, id).Scan(&r.Decision)
	if errors.Is(err, pgx.ErrNoRows) {
		err = errs.ErrNotFound
	}
	return r, err
}

func (s *Store) Receive(ctx context.Context, r dto.TransferCommand) (dto.TransferResult, error) {
	if err := r.Validate(); err != nil {
		return dto.TransferResult{}, err
	}
	if r.External == "" {
		return dto.TransferResult{}, errs.ErrInvalid
	}
	decision := "APPROVED"
	if strings.HasPrefix(r.External, "reject:") {
		decision = "DECLINED"
	}
	payload, _ := json.Marshal(r)
	_, err := s.Pool.Exec(ctx, `INSERT INTO external_receipts(id,payload,decision,amount_minor) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, r.ID, payload, decision, r.Amount)
	if err != nil {
		return dto.TransferResult{}, err
	}
	var saved []byte
	err = s.Pool.QueryRow(ctx, `SELECT payload,decision FROM external_receipts WHERE id=$1`, r.ID).Scan(&saved, &decision)
	if err != nil {
		return dto.TransferResult{}, err
	}
	var previous dto.TransferCommand
	if err = json.Unmarshal(saved, &previous); err != nil {
		return dto.TransferResult{}, err
	}
	if previous != r {
		return dto.TransferResult{}, errs.ErrConflict
	}
	return dto.TransferResult{ID: r.ID, Decision: decision, Settlement: "SETTLED"}, nil
}
