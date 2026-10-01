package usecase

import (
	"context"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

type GetTransfer struct {
	Transfers port.TransferStore
}

func (u GetTransfer) Execute(ctx context.Context, id string) (dto.TransferResult, error) {
	parsed, err := valueobject.NewTransactionID(id)
	if err != nil {
		return dto.TransferResult{}, err
	}
	return u.Transfers.FindTransfer(ctx, parsed)
}

type GetBalance struct {
	Accounts port.AccountStore
}

func (u GetBalance) Execute(ctx context.Context, id int64) (dto.Balance, error) {
	parsed, err := valueobject.NewClientID(id)
	if err != nil {
		return dto.Balance{}, err
	}
	account, err := u.Accounts.FindAccount(ctx, parsed)
	if err != nil {
		return dto.Balance{}, err
	}
	return dto.FromAccount(account), nil
}

type GetStatement struct {
	Accounts   port.AccountStore
	Statements port.StatementStore
}

func (u GetStatement) Execute(ctx context.Context, id, after, until int64) (dto.Statement, error) {
	if after < 0 || until < 0 {
		return dto.Statement{}, errs.ErrInvalid
	}
	parsed, err := valueobject.NewClientID(id)
	if err != nil {
		return dto.Statement{}, err
	}
	account, err := u.Accounts.FindAccount(ctx, parsed)
	if err != nil {
		return dto.Statement{}, err
	}
	if until == 0 || until > account.Sequence() {
		until = account.Sequence()
	}
	page, err := u.Statements.FindStatement(ctx, parsed, after, until)
	if err != nil {
		return dto.Statement{}, err
	}
	return dto.FromStatement(page), nil
}
