package port

import (
	"context"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/valueobject"
)

// TransferStore is the financial authority. Authorize decides and records in
// one step; a replay of the same id and payload returns the stored outcome.
type TransferStore interface {
	Authorize(ctx context.Context, transfer aggregate.Transfer) (dto.TransferResult, error)
	FindTransfer(ctx context.Context, id valueobject.TransactionID) (dto.TransferResult, error)
}

type AccountStore interface {
	FindAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error)
}

type StatementStore interface {
	FindStatement(ctx context.Context, id valueobject.AccountID, after, until int64) (entity.Statement, error)
}

// ReceiptStore keeps the simulated institution's decisions. SaveReceipt
// returns the stored result when the id already exists with the same payload.
type ReceiptStore interface {
	FindReceipt(ctx context.Context, id valueobject.TransactionID) (dto.TransferResult, error)
	SaveReceipt(ctx context.Context, transfer aggregate.Transfer, result dto.TransferResult) (dto.TransferResult, error)
}
