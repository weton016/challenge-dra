package tigerbeetle

import (
	"context"
	"fmt"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

// Authorize submits the transfer once. TigerBeetle decides the balance check
// and the idempotency of the id in the same commit; there is no read first.
func (l *Ledger) Authorize(ctx context.Context, t aggregate.Transfer) (dto.TransferResult, error) {
	results, err := l.create(ctx, chain(t))
	if err != nil {
		return dto.TransferResult{}, err
	}
	switch s := results[0].Status; s {
	case tb.TransferCreated:
		return dto.FromTransfer(t.WithReason(valueobject.ReasonNone)), nil
	case tb.TransferExists:
		if t.External().String() == "" {
			return dto.FromTransfer(t.WithReason(valueobject.ReasonNone)), nil
		}
		return l.FindTransfer(ctx, t.ID())
	case tb.TransferExceedsCredits:
		return l.decline(ctx, t, reasonInsufficient)
	case tb.TransferDebitAccountNotFound, tb.TransferCreditAccountNotFound:
		return l.decline(ctx, t, reasonNotFound)
	case tb.TransferIDAlreadyFailed:
		return l.replayDecline(ctx, t)
	default:
		return dto.TransferResult{}, statusError(s)
	}
}

func chain(t aggregate.Transfer) []tb.Transfer {
	head := tb.Transfer{
		ID:              transferID(t.ID()),
		DebitAccountID:  accountID(t.Source().Int64()),
		CreditAccountID: accountID(t.Destination().Int64()),
		Amount:          amountOf(t.Amount().Minor()),
		UserData128:     tb.BytesToUint128(t.Fingerprint()),
		Ledger:          ledgerBRL,
		Code:            codeInternal,
	}
	ref := t.External().String()
	if ref == "" {
		return []tb.Transfer{head}
	}
	head.Code = codeExternal
	head.Flags = tb.TransferFlags{Linked: true, Pending: true}.ToUint16()
	head.UserData32 = uint32(len(ref))
	return append([]tb.Transfer{head}, memos(head.ID, ref)...)
}

// statusError maps a create status that is not a decision. A different
// payload under an existing id is the client's conflict; anything else
// means the request broke a ledger rule this adapter relies on.
func statusError(s tb.CreateTransferStatus) error {
	switch s {
	case tb.TransferExistsWithDifferentFlags, tb.TransferExistsWithDifferentPendingID,
		tb.TransferExistsWithDifferentTimeout, tb.TransferExistsWithDifferentDebitAccountID,
		tb.TransferExistsWithDifferentCreditAccountID, tb.TransferExistsWithDifferentAmount,
		tb.TransferExistsWithDifferentUserData128, tb.TransferExistsWithDifferentUserData64,
		tb.TransferExistsWithDifferentUserData32, tb.TransferExistsWithDifferentLedger,
		tb.TransferExistsWithDifferentCode:
		return errs.ErrConflict
	}
	return fmt.Errorf("transfer status %s: %w", s, errs.ErrInvariant)
}

// decline stores a zero-amount record of the refusal, so a replay of the
// failed id can answer the same reason and detect a different payload.
func (l *Ledger) decline(ctx context.Context, t aggregate.Transfer, reason uint32) (dto.TransferResult, error) {
	id := derive("decline", transferID(t.ID()))
	record := tb.Transfer{
		ID:              id,
		DebitAccountID:  memoDebit,
		CreditAccountID: memoCredit,
		UserData128:     tb.BytesToUint128(t.Fingerprint()),
		UserData32:      reason,
		Ledger:          ledgerBRL,
		Code:            codeDecline,
	}
	results, err := l.create(ctx, []tb.Transfer{record})
	if err != nil {
		return dto.TransferResult{}, err
	}
	if results[0].Status == tb.TransferCreated {
		return declined(t, record)
	}
	found, err := l.lookup(ctx, id)
	if err != nil {
		return dto.TransferResult{}, err
	}
	stored, ok := found[id]
	if !ok {
		return dto.TransferResult{}, statusError(results[0].Status)
	}
	return declined(t, stored)
}

// replayDecline answers an id TigerBeetle already refused. If the process
// died before writing the record, the reason is decided again from the
// accounts, which cannot be deleted.
func (l *Ledger) replayDecline(ctx context.Context, t aggregate.Transfer) (dto.TransferResult, error) {
	id := derive("decline", transferID(t.ID()))
	found, err := l.lookup(ctx, id)
	if err != nil {
		return dto.TransferResult{}, err
	}
	if stored, ok := found[id]; ok {
		return declined(t, stored)
	}
	ids := []tb.Uint128{accountID(t.Source().Int64()), accountID(t.Destination().Int64())}
	accounts, err := call(ctx, func() ([]tb.Account, error) { return l.client.LookupAccounts(ids) })
	if err != nil {
		return dto.TransferResult{}, err
	}
	reason := uint32(reasonInsufficient)
	if len(accounts) < len(ids) {
		reason = reasonNotFound
	}
	return l.decline(ctx, t, reason)
}

func declined(t aggregate.Transfer, record tb.Transfer) (dto.TransferResult, error) {
	if record.UserData128 != tb.BytesToUint128(t.Fingerprint()) {
		return dto.TransferResult{}, errs.ErrConflict
	}
	reason, err := reasonOf(record.UserData32)
	if err != nil {
		return dto.TransferResult{}, err
	}
	return dto.FromTransfer(t.WithReason(reason)), nil
}

func reasonOf(code uint32) (valueobject.Reason, error) {
	switch code {
	case reasonInsufficient:
		return valueobject.ReasonInsufficientFunds, nil
	case reasonNotFound:
		return valueobject.ReasonAccountNotFound, nil
	}
	return "", errs.ErrInvariant
}

func (l *Ledger) FindTransfer(ctx context.Context, id valueobject.TransactionID) (dto.TransferResult, error) {
	tid := transferID(id)
	settle, decline := derive("settle", tid), derive("decline", tid)
	found, err := l.lookup(ctx, tid, settle, decline)
	if err != nil {
		return dto.TransferResult{}, err
	}
	result := dto.TransferResult{ID: id.String(), Decision: valueobject.Approved.String()}
	if head, ok := found[tid]; ok {
		result.Settlement = settlementOf(head, found[settle]).String()
		return result, nil
	}
	record, ok := found[decline]
	if !ok {
		return dto.TransferResult{}, errs.ErrNotFound
	}
	reason, err := reasonOf(record.UserData32)
	if err != nil {
		return dto.TransferResult{}, err
	}
	result.Decision, result.Reason = valueobject.Declined.String(), reason.String()
	result.Settlement = valueobject.SettlementInternal.String()
	return result, nil
}

func settlementOf(head, settle tb.Transfer) valueobject.Settlement {
	switch {
	case head.Code != codeExternal:
		return valueobject.SettlementInternal
	case settle.TransferFlags().PostPendingTransfer:
		return valueobject.SettlementSettled
	case settle.TransferFlags().VoidPendingTransfer:
		return valueobject.SettlementReversed
	}
	return valueobject.SettlementPending
}
