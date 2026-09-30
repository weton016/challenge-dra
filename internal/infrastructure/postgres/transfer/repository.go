package transfer

import (
	"context"
	"encoding/json"
	"errors"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/service"
	"dracma/pix/internal/domain/valueobject"
	"dracma/pix/internal/infrastructure/postgres"
	"dracma/pix/internal/infrastructure/postgres/codec"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ DB *postgres.Store }

func New(db *postgres.Store) Repository { return Repository{DB: db} }

func (r Repository) FindTransfer(ctx context.Context, id valueobject.TransactionID) (aggregate.Transfer, error) {
	var payload []byte
	var decision, reason, settlement string
	err := r.DB.Pool.QueryRow(ctx, `SELECT payload,decision,reason,settlement FROM operations WHERE id=$1`, id.String()).Scan(&payload, &decision, &reason, &settlement)
	if errors.Is(err, pgx.ErrNoRows) {
		return aggregate.Transfer{}, errs.ErrNotFound
	}
	if err != nil {
		return aggregate.Transfer{}, err
	}
	return codec.DecodeTransfer(payload, decision, reason, settlement)
}

func (r Repository) Commit(ctx context.Context, authorization service.Authorization) error {
	result, err := r.DB.Authorize(ctx, codec.RequestOf(authorization.Transfer))
	if err != nil {
		return err
	}
	if result.Decision != authorization.Transfer.Decision().String() || result.Reason != authorization.Transfer.Reason().String() || result.Settlement != authorization.Transfer.Settlement().String() {
		return errs.ErrInvariant
	}
	return nil
}

// SaveTransfer stores the operation. An approval is refused until its two entries exist.
func (r Repository) SaveTransfer(ctx context.Context, transfer aggregate.Transfer) error {
	decision := transfer.Decision().String()
	if decision != string(valueobject.Approved) && decision != string(valueobject.Declined) {
		return errs.ErrInvalid
	}
	if decision == string(valueobject.Approved) {
		if err := requirePair(ctx, r, transfer.ID().String()); err != nil {
			return err
		}
	}
	request := codec.RequestOf(transfer)
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	tx, err := r.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = upsertOperation(ctx, tx, request, payload, decision, transfer.Reason().String(), transfer.Settlement().String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func requirePair(ctx context.Context, r Repository, id string) error {
	var n int
	err := r.DB.Pool.QueryRow(ctx, `SELECT count(*) FROM entries WHERE operation_id=$1`, id).Scan(&n)
	if err != nil {
		return err
	}
	if n != 2 {
		return errs.ErrInvariant
	}
	return nil
}

func upsertOperation(ctx context.Context, tx pgx.Tx, request dto.TransferCommand, payload []byte, decision, reason, settlement string) error {
	var existing []byte
	err := tx.QueryRow(ctx, `SELECT payload FROM operations WHERE id=$1`, request.ID).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO operations(id,payload,decision,reason,settlement) VALUES ($1,$2,$3,$4,$5)`, request.ID, payload, decision, reason, settlement)
		return err
	}
	if err != nil {
		return err
	}
	if !sameRequest(existing, request) {
		return errs.ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE operations SET decision=$2, reason=$3, settlement=$4 WHERE id=$1`, request.ID, decision, reason, settlement)
	return err
}

func sameRequest(payload []byte, request dto.TransferCommand) bool {
	var saved dto.TransferCommand
	if err := json.Unmarshal(payload, &saved); err != nil {
		return false
	}
	return saved == request
}
