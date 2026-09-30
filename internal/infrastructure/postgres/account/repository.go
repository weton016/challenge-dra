package account

import (
	"context"
	"errors"

	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
	"dracma/pix/internal/infrastructure/postgres"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ DB *postgres.Store }

func New(db *postgres.Store) Repository { return Repository{DB: db} }

func (r Repository) FindAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error) {
	var balance, sequence int64
	err := r.DB.Pool.QueryRow(ctx, `SELECT COALESCE(t.balance_after_minor, a.balance_minor), COALESCE(t.account_sequence, a.last_sequence)
FROM accounts a
LEFT JOIN LATERAL (
 SELECT balance_after_minor, account_sequence FROM entries e
 WHERE e.account_id=a.id ORDER BY account_sequence DESC LIMIT 1
) t ON true
WHERE a.id=$1`, id.Int64()).Scan(&balance, &sequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return aggregate.Account{}, errs.ErrNotFound
	}
	if err != nil {
		return aggregate.Account{}, err
	}
	amount, err := valueobject.NewBalance(balance)
	if err != nil {
		return aggregate.Account{}, err
	}
	return aggregate.NewAccount(id, amount, sequence, valueobject.BRL())
}

// SaveAccount writes the account row only when it already matches the log tail.
// A different balance would fork the statement from the materialized balance.
func (r Repository) SaveAccount(ctx context.Context, account aggregate.Account) error {
	tag, err := r.DB.Pool.Exec(ctx, `UPDATE accounts a
SET balance_minor=$2, last_sequence=$3
WHERE a.id=$1
AND $2=COALESCE((SELECT balance_after_minor FROM entries e WHERE e.account_id=a.id ORDER BY account_sequence DESC LIMIT 1), a.balance_minor)
AND $3=COALESCE((SELECT account_sequence FROM entries e WHERE e.account_id=a.id ORDER BY account_sequence DESC LIMIT 1), a.last_sequence)`,
		account.ID().Int64(), account.Balance().Minor(), account.Sequence())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var found int64
	err = r.DB.Pool.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1`, account.ID().Int64()).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNotFound
	}
	if err != nil {
		return err
	}
	return errs.ErrInvariant
}
