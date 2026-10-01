package usecase

import (
	"context"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

type stored struct {
	fingerprint [16]byte
	result      dto.TransferResult
}

// memory is a fake authority: it approves every new id and replays by fingerprint.
type memory struct {
	accounts     map[int64]aggregate.Account
	transfers    map[string]stored
	receipts     map[string]stored
	rows         []entity.Entry
	seenUntil    int64
	accountErr   error
	transferErr  error
	statementErr error
	receiptErr   error
	authorized   int
}

func newMemory() *memory {
	return &memory{
		accounts:  map[int64]aggregate.Account{},
		transfers: map[string]stored{},
		receipts:  map[string]stored{},
	}
}

func save(table map[string]stored, transfer aggregate.Transfer, result dto.TransferResult) (dto.TransferResult, error) {
	if old, ok := table[transfer.ID().String()]; ok {
		if old.fingerprint != transfer.Fingerprint() {
			return dto.TransferResult{}, errs.ErrConflict
		}
		return old.result, nil
	}
	table[transfer.ID().String()] = stored{fingerprint: transfer.Fingerprint(), result: result}
	return result, nil
}

func (m *memory) Authorize(ctx context.Context, transfer aggregate.Transfer) (dto.TransferResult, error) {
	if m.transferErr != nil {
		return dto.TransferResult{}, m.transferErr
	}
	m.authorized++
	return save(m.transfers, transfer, dto.FromTransfer(transfer.WithReason(valueobject.ReasonNone)))
}

func (m *memory) FindTransfer(ctx context.Context, id valueobject.TransactionID) (dto.TransferResult, error) {
	if m.transferErr != nil {
		return dto.TransferResult{}, m.transferErr
	}
	got, ok := m.transfers[id.String()]
	if !ok {
		return dto.TransferResult{}, errs.ErrNotFound
	}
	return got.result, nil
}

func (m *memory) FindAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error) {
	if m.accountErr != nil {
		return aggregate.Account{}, m.accountErr
	}
	account, ok := m.accounts[id.Int64()]
	if !ok {
		return aggregate.Account{}, errs.ErrNotFound
	}
	return account, nil
}

func (m *memory) FindStatement(ctx context.Context, id valueobject.AccountID, after, until int64) (entity.Statement, error) {
	m.seenUntil = until
	if m.statementErr != nil {
		return entity.Statement{}, m.statementErr
	}
	next := after
	if len(m.rows) > 0 {
		next = m.rows[len(m.rows)-1].Sequence()
	}
	return entity.NewStatement(m.rows, until, next)
}

func (m *memory) FindReceipt(ctx context.Context, id valueobject.TransactionID) (dto.TransferResult, error) {
	if m.receiptErr != nil {
		return dto.TransferResult{}, m.receiptErr
	}
	got, ok := m.receipts[id.String()]
	if !ok {
		return dto.TransferResult{}, errs.ErrNotFound
	}
	return got.result, nil
}

func (m *memory) SaveReceipt(ctx context.Context, transfer aggregate.Transfer, result dto.TransferResult) (dto.TransferResult, error) {
	if m.receiptErr != nil {
		return dto.TransferResult{}, m.receiptErr
	}
	return save(m.receipts, transfer, result)
}
