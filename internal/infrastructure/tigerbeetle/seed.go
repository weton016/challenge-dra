package tigerbeetle

import (
	"context"
	"errors"
	"fmt"
	"math"

	"dracma/pix/internal/domain/errs"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

// Seed creates client accounts first..first+count-1, each funded from the
// treasury once. A second seed keeps the balances already given.
func (l *Ledger) Seed(ctx context.Context, first, count, balance int64) error {
	if first < 1 || count < 1 || balance < 0 || first > math.MaxInt64-count {
		return errs.ErrInvalid
	}
	size := int64(8189)
	for next := first; next < first+count; {
		n := min(size, first+count-next)
		err := l.seedChunk(ctx, next, n, balance)
		if errors.Is(err, tb.ErrTooMuchData) && size > 1 {
			size /= 2
			continue
		}
		if err != nil {
			return err
		}
		next += n
	}
	return nil
}

func (l *Ledger) seedChunk(ctx context.Context, first, count, balance int64) error {
	flags := tb.AccountFlags{DebitsMustNotExceedCredits: true, History: true}.ToUint16()
	accounts := make([]tb.Account, 0, count)
	funding := make([]tb.Transfer, 0, count)
	for id := first; id < first+count; id++ {
		acct := accountID(id)
		accounts = append(accounts, tb.Account{ID: acct, Ledger: ledgerBRL, Code: codeClient, Flags: flags})
		funding = append(funding, tb.Transfer{ID: derive("seed", acct), DebitAccountID: treasury, CreditAccountID: acct, Amount: amountOf(balance), Ledger: ledgerBRL, Code: codeFunding})
	}
	if err := createAccounts(ctx, l, accounts); err != nil || balance == 0 {
		return err
	}
	results, err := l.create(ctx, funding)
	if err != nil {
		return err
	}
	for i, r := range results {
		switch r.Status {
		case tb.TransferCreated, tb.TransferExists, tb.TransferExistsWithDifferentAmount:
		case tb.TransferExceedsCredits:
			return fmt.Errorf("seed %d: supply of MaxInt64 exhausted: %w", first+int64(i), errs.ErrInvalid)
		default:
			return fmt.Errorf("seed %d: %s: %w", first+int64(i), r.Status, errs.ErrInvariant)
		}
	}
	return nil
}
