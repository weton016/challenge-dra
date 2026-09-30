package repository

import (
	"context"

	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/valueobject"
)

type AccountRepository interface {
	FindAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error)
	SaveAccount(ctx context.Context, account aggregate.Account) error
}

type TransferRepository interface {
	FindTransfer(ctx context.Context, id valueobject.TransactionID) (aggregate.Transfer, error)
	SaveTransfer(ctx context.Context, transfer aggregate.Transfer) error
}

type StatementRepository interface {
	FindStatement(ctx context.Context, id valueobject.AccountID, after, until int64) (entity.Statement, error)
}
