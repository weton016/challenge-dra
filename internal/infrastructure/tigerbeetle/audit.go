package tigerbeetle

import (
	"context"
	"math"
	"math/big"

	"dracma/pix/internal/domain/policy"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

type Report struct {
	Accounts        int64 `json:"accounts"`
	InvalidBalances int64 `json:"invalid_balances"`
	Conservation    int64 `json:"conservation_error"`
	HistoryRows     int64 `json:"history_rows"`
	InvalidPrefixes int64 `json:"invalid_prefixes"`
	PendingExternal int64 `json:"pending_external"`
	Passed          bool  `json:"passed"`
}

// Audit recomputes the books from TigerBeetle's own records. TigerBeetle
// has no read snapshot, so run it with traffic stopped.
func (l *Ledger) Audit(ctx context.Context, accounts []int64) (Report, error) {
	var r Report
	if err := l.auditBalances(ctx, &r); err != nil {
		return r, err
	}
	for _, id := range accounts {
		if err := l.auditHistory(ctx, &r, id); err != nil {
			return r, err
		}
	}
	if err := l.auditPending(ctx, &r); err != nil {
		return r, err
	}
	r.Passed = r.InvalidBalances == 0 && r.Conservation == 0 && r.InvalidPrefixes == 0
	return r, nil
}

// auditBalances checks every account outside the mint for a non-negative
// int64 balance, and that the money outside the mint is exactly the supply.
func (l *Ledger) auditBalances(ctx context.Context, r *Report) error {
	net, debitsPending, creditsPending := new(big.Int), new(big.Int), new(big.Int)
	for ts := uint64(0); ; {
		filter := tb.QueryFilter{Ledger: ledgerBRL, TimestampMin: ts, Limit: 8189}
		page, err := call(ctx, func() ([]tb.Account, error) { return l.client.QueryAccounts(filter) })
		if err != nil {
			return err
		}
		if len(page) == 0 {
			break
		}
		for _, a := range page {
			debitsPending.Add(debitsPending, a.DebitsPending.BigInt())
			creditsPending.Add(creditsPending, a.CreditsPending.BigInt())
			if a.ID == mint {
				continue
			}
			if a.Code == codeClient {
				r.Accounts++
			}
			if _, ok := policy.Available(a.CreditsPosted.BigInt(), a.DebitsPosted.BigInt(), a.DebitsPending.BigInt()); !ok {
				r.InvalidBalances++
			}
			net.Add(net, a.CreditsPosted.BigInt())
			net.Sub(net, a.DebitsPosted.BigInt())
		}
		ts = page[len(page)-1].Timestamp + 1
	}
	if net.Cmp(big.NewInt(math.MaxInt64)) != 0 || debitsPending.Cmp(creditsPending) != 0 {
		r.Conservation = 1
	}
	return nil
}

// auditHistory walks one account's snapshots: every prefix must be a valid
// balance and the newest must equal the account's current totals.
func (l *Ledger) auditHistory(ctx context.Context, r *Report, id int64) error {
	acct := accountID(id)
	var last tb.AccountBalance
	for ts := uint64(0); ; {
		filter := tb.AccountFilter{AccountID: acct, TimestampMin: ts, Limit: 8189, Flags: history.ToUint32()}
		page, err := call(ctx, func() ([]tb.AccountBalance, error) { return l.client.GetAccountBalances(filter) })
		if err != nil {
			return err
		}
		if len(page) == 0 {
			break
		}
		r.HistoryRows += int64(len(page))
		for _, b := range page {
			if _, err := available(b); err != nil {
				r.InvalidPrefixes++
			}
		}
		last = page[len(page)-1]
		ts = last.Timestamp + 1
	}
	found, err := call(ctx, func() ([]tb.Account, error) { return l.client.LookupAccounts([]tb.Uint128{acct}) })
	if err != nil {
		return err
	}
	if len(found) != 1 || found[0].CreditsPosted != last.CreditsPosted || found[0].DebitsPosted != last.DebitsPosted || found[0].DebitsPending != last.DebitsPending {
		r.InvalidPrefixes++
	}
	return nil
}

func (l *Ledger) auditPending(ctx context.Context, r *Report) error {
	for ts := uint64(0); ; {
		filter := tb.QueryFilter{Ledger: ledgerBRL, Code: codeExternal, TimestampMin: ts, Limit: lookupChunk}
		page, err := call(ctx, func() ([]tb.Transfer, error) { return l.client.QueryTransfers(filter) })
		if err != nil || len(page) == 0 {
			return err
		}
		open, err := l.unsettled(ctx, page)
		if err != nil {
			return err
		}
		r.PendingExternal += int64(len(open))
		ts = page[len(page)-1].Timestamp + 1
	}
}
