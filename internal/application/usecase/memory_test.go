package usecase

import (
	"context"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/service"
	"dracma/pix/internal/domain/valueobject"
)

type memory struct {
	accounts         map[int64]aggregate.Account
	transfers        map[string]aggregate.Transfer
	receipts         map[string]port.Receipt
	rows             []entity.Entry
	seenUntil        int64
	accountErr       error
	failAccountAfter int
	accountCalls     int
	transferErr      error
	commitErr        error
	statementErr     error
	receiptErr       error
	receiptSaveErr   error
	commits          int
}

func newMemory() *memory {
	return &memory{
		accounts:  map[int64]aggregate.Account{},
		transfers: map[string]aggregate.Transfer{},
		receipts:  map[string]port.Receipt{},
	}
}

func (m *memory) FindTransfer(ctx context.Context, id valueobject.TransactionID) (aggregate.Transfer, error) {
	if m.transferErr != nil {
		return aggregate.Transfer{}, m.transferErr
	}
	transfer, ok := m.transfers[id.String()]
	if !ok {
		return aggregate.Transfer{}, errs.ErrNotFound
	}
	return transfer, nil
}

func (m *memory) Commit(ctx context.Context, authorization service.Authorization) error {
	if m.commitErr != nil {
		return m.commitErr
	}
	m.commits++
	m.transfers[authorization.Transfer.ID().String()] = authorization.Transfer
	if len(authorization.Entries) == 0 {
		return nil
	}
	m.accounts[authorization.Source.ID().Int64()] = authorization.Source
	m.accounts[authorization.Destination.ID().Int64()] = authorization.Destination
	return nil
}

func (m *memory) FindAccount(ctx context.Context, id valueobject.AccountID) (aggregate.Account, error) {
	m.accountCalls++
	if m.accountErr != nil && m.accountCalls > m.failAccountAfter {
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

func (m *memory) FindReceipt(ctx context.Context, id valueobject.TransactionID) (port.Receipt, error) {
	if m.receiptErr != nil {
		return port.Receipt{}, m.receiptErr
	}
	receipt, ok := m.receipts[id.String()]
	if !ok {
		return port.Receipt{}, errs.ErrNotFound
	}
	return receipt, nil
}

func (m *memory) SaveReceipt(ctx context.Context, transfer aggregate.Transfer, result dto.TransferResult) error {
	if m.receiptSaveErr != nil {
		return m.receiptSaveErr
	}
	m.receipts[transfer.ID().String()] = port.Receipt{Transfer: transfer, Result: result}
	return nil
}
