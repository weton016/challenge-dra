package tigerbeetle

import (
	"context"

	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/policy"
	"dracma/pix/internal/domain/valueobject"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

var history = tb.AccountFilterFlags{Debits: true, Credits: true}

// FindAccount reads the newest history snapshot: its timestamp is the
// account sequence that pins statement pages.
func (l *Ledger) FindAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error) {
	acct := accountID(id.Int64())
	newest := history
	newest.Reversed = true
	filter := tb.AccountFilter{AccountID: acct, Limit: 1, Flags: newest.ToUint32()}
	rows, err := call(ctx, func() ([]tb.AccountBalance, error) { return l.client.GetAccountBalances(filter) })
	if err != nil {
		return aggregate.Account{}, err
	}
	if len(rows) == 0 {
		return l.emptyAccount(ctx, id)
	}
	balance, err := available(rows[0])
	if err != nil {
		return aggregate.Account{}, err
	}
	return aggregate.NewAccount(id, balance, int64(rows[0].Timestamp), valueobject.BRL())
}

func (l *Ledger) emptyAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error) {
	ids := []tb.Uint128{accountID(id.Int64())}
	found, err := call(ctx, func() ([]tb.Account, error) { return l.client.LookupAccounts(ids) })
	if err != nil {
		return aggregate.Account{}, err
	}
	if len(found) == 0 {
		return aggregate.Account{}, errs.ErrNotFound
	}
	return aggregate.NewAccount(id, valueobject.Balance{}, 0, valueobject.BRL())
}

func available(b tb.AccountBalance) (valueobject.Balance, error) {
	minor, ok := policy.Available(b.CreditsPosted.BigInt(), b.DebitsPosted.BigInt(), b.DebitsPending.BigInt())
	if !ok {
		return valueobject.Balance{}, errs.ErrInvariant
	}
	return valueobject.NewBalance(minor)
}

// FindStatement pairs each transfer with the balance snapshot TigerBeetle
// took in the same commit. Funding and capture rows move no client money
// and are skipped, but still advance the cursor.
func (l *Ledger) FindStatement(ctx context.Context, id valueobject.AccountID, after, until int64) (entity.Statement, error) {
	if until <= after {
		return entity.NewStatement(nil, until, after)
	}
	filter := tb.AccountFilter{AccountID: accountID(id.Int64()), TimestampMin: uint64(after + 1), TimestampMax: uint64(until), Limit: statementPage, Flags: history.ToUint32()}
	transfers, err := call(ctx, func() ([]tb.Transfer, error) { return l.client.GetAccountTransfers(filter) })
	if err != nil {
		return entity.Statement{}, err
	}
	balances, err := call(ctx, func() ([]tb.AccountBalance, error) { return l.client.GetAccountBalances(filter) })
	if err != nil {
		return entity.Statement{}, err
	}
	if len(balances) != len(transfers) {
		return entity.Statement{}, errs.ErrInvariant
	}
	entries, next := []entity.Entry{}, after
	for i, t := range transfers {
		if balances[i].Timestamp != t.Timestamp {
			return entity.Statement{}, errs.ErrInvariant
		}
		next = int64(t.Timestamp)
		entry, ok, err := statementEntry(id, t, balances[i])
		if err != nil {
			return entity.Statement{}, err
		}
		if ok {
			entries = append(entries, entry)
		}
	}
	return entity.NewStatement(entries, until, next)
}

func statementEntry(id valueobject.AccountID, t tb.Transfer, b tb.AccountBalance) (entity.Entry, bool, error) {
	amount, err := minorOf(t.Amount)
	if err != nil {
		return entity.Entry{}, false, err
	}
	movement, txID := movementOf(t)
	delta := policy.Delta(movement, t.DebitAccountID == accountID(id.Int64()), amount)
	if delta == 0 || t.Code == codeFunding {
		return entity.Entry{}, false, nil
	}
	tx, err := valueobject.NewTransactionID(txID)
	if err == nil && movement == policy.Released {
		tx, err = valueobject.RefundID(tx)
	}
	if err != nil {
		return entity.Entry{}, false, errs.ErrInvariant
	}
	balance, err := available(b)
	if err != nil {
		return entity.Entry{}, false, err
	}
	entry, err := entity.NewEntry(tx, id, delta, int64(t.Timestamp), balance)
	return entry, err == nil, err
}

func movementOf(t tb.Transfer) (policy.Movement, string) {
	flags := t.TransferFlags()
	switch {
	case flags.Pending:
		return policy.Reserved, uuidOf(t.ID)
	case flags.PostPendingTransfer:
		return policy.Captured, uuidOf(t.PendingID)
	case flags.VoidPendingTransfer:
		return policy.Released, uuidOf(t.PendingID)
	}
	return policy.Posted, uuidOf(t.ID)
}
