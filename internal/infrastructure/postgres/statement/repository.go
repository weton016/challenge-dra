package statement

import (
	"context"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/valueobject"
	"dracma/pix/internal/infrastructure/postgres"
)

type Repository struct{ DB *postgres.Store }

func New(db *postgres.Store) Repository { return Repository{DB: db} }

func (r Repository) FindStatement(ctx context.Context, id valueobject.AccountID, after, until int64) (entity.Statement, error) {
	page, err := r.DB.Statement(ctx, id.Int64(), after, until)
	if err != nil {
		return entity.Statement{}, err
	}
	entries := make([]entity.Entry, 0, len(page.Entries))
	for _, row := range page.Entries {
		entry, err := restoreEntry(id, row)
		if err != nil {
			return entity.Statement{}, err
		}
		entries = append(entries, entry)
	}
	return entity.NewStatement(entries, page.Until, page.Next)
}

func restoreEntry(account valueobject.AccountID, row dto.Entry) (entity.Entry, error) {
	tx, err := valueobject.ParseTransactionID(row.TransactionID)
	if err != nil {
		return entity.Entry{}, err
	}
	balance, err := valueobject.NewBalance(row.Balance)
	if err != nil {
		return entity.Entry{}, err
	}
	return entity.NewEntry(tx, account, row.Delta, row.Sequence, balance)
}
