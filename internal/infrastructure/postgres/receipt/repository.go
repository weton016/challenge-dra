package receipt

import (
	"context"
	"errors"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
	"dracma/pix/internal/infrastructure/postgres"
	"dracma/pix/internal/infrastructure/postgres/codec"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ DB *postgres.Store }

func New(db *postgres.Store) Repository { return Repository{DB: db} }

func (r Repository) FindReceipt(ctx context.Context, id valueobject.TransactionID) (port.Receipt, error) {
	var payload []byte
	var decision string
	err := r.DB.Pool.QueryRow(ctx, `SELECT payload,decision FROM external_receipts WHERE id=$1`, id.String()).Scan(&payload, &decision)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.Receipt{}, errs.ErrNotFound
	}
	if err != nil {
		return port.Receipt{}, err
	}
	transfer, err := codec.DecodeTransfer(payload, decision, "", string(valueobject.SettlementSettled))
	if err != nil {
		return port.Receipt{}, err
	}
	return port.Receipt{Transfer: transfer, Result: dto.FromTransfer(transfer)}, nil
}

func (r Repository) SaveReceipt(ctx context.Context, transfer aggregate.Transfer, result dto.TransferResult) error {
	saved, err := r.DB.Receive(ctx, codec.RequestOf(transfer))
	if err != nil {
		return err
	}
	if saved.ID != result.ID || saved.Decision != result.Decision || saved.Settlement != result.Settlement {
		return errs.ErrInvariant
	}
	return nil
}
