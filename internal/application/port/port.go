package port

import (
	"context"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/service"
	"dracma/pix/internal/domain/valueobject"
)

type TransferStore interface {
	FindTransfer(ctx context.Context, id valueobject.TransactionID) (aggregate.Transfer, error)
	Commit(ctx context.Context, authorization service.Authorization) error
}

type AccountStore interface {
	FindAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error)
}

type StatementStore interface {
	FindStatement(ctx context.Context, id valueobject.AccountID, after, until int64) (entity.Statement, error)
}

type Receipt struct {
	Transfer aggregate.Transfer
	Result   dto.TransferResult
}

type ReceiptStore interface {
	FindReceipt(ctx context.Context, id valueobject.TransactionID) (Receipt, error)
	SaveReceipt(ctx context.Context, transfer aggregate.Transfer, result dto.TransferResult) error
}
