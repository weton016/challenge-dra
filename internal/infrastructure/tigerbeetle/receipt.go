package tigerbeetle

import (
	"context"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

// SaveReceipt is the simulated institution's record, kept on its own
// ledger. The first decision for an id wins; a different payload conflicts.
func (l *Ledger) SaveReceipt(ctx context.Context, t aggregate.Transfer, result dto.TransferResult) (dto.TransferResult, error) {
	code := uint16(codeAccepted)
	if result.Decision == valueobject.Declined.String() {
		code = codeRejected
	}
	receipt := tb.Transfer{
		ID:              derive("receipt", transferID(t.ID())),
		DebitAccountID:  simPayer,
		CreditAccountID: simPayee,
		Amount:          amountOf(t.Amount().Minor()),
		UserData128:     tb.BytesToUint128(t.Fingerprint()),
		Ledger:          ledgerExternal,
		Code:            code,
	}
	results, err := l.create(ctx, []tb.Transfer{receipt})
	if err != nil {
		return dto.TransferResult{}, err
	}
	if s := results[0].Status; s != tb.TransferCreated && s != tb.TransferExists {
		return dto.TransferResult{}, statusError(s)
	}
	return result, nil
}

func (l *Ledger) FindReceipt(ctx context.Context, id valueobject.TransactionID) (dto.TransferResult, error) {
	rid := derive("receipt", transferID(id))
	found, err := l.lookup(ctx, rid)
	if err != nil {
		return dto.TransferResult{}, err
	}
	receipt, ok := found[rid]
	if !ok {
		return dto.TransferResult{}, errs.ErrNotFound
	}
	result := dto.TransferResult{ID: id.String(), Decision: valueobject.Approved.String(), Settlement: valueobject.SettlementSettled.String()}
	if receipt.Code == codeRejected {
		result.Decision = valueobject.Declined.String()
	}
	return result, nil
}
