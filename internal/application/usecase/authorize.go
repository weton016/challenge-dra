package usecase

import (
	"context"
	"errors"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/service"
	"dracma/pix/internal/domain/valueobject"
)

const reasonAccountNotFound = valueobject.Reason("ACCOUNT_NOT_FOUND")

type AuthorizeTransfer struct {
	Transfers port.TransferStore
	Accounts  port.AccountStore
}

func (u AuthorizeTransfer) Execute(ctx context.Context, cmd dto.TransferCommand) (dto.TransferResult, error) {
	transfer, err := aggregate.ParseTransfer(cmd.ID, cmd.Source, cmd.Destination, cmd.External, cmd.Amount)
	if err != nil {
		return dto.TransferResult{}, err
	}
	result, done, err := u.replay(ctx, transfer)
	if err != nil || done {
		return result, err
	}
	return u.decide(ctx, transfer)
}

func (u AuthorizeTransfer) replay(ctx context.Context, transfer aggregate.Transfer) (dto.TransferResult, bool, error) {
	stored, err := u.Transfers.FindTransfer(ctx, transfer.ID())
	if errors.Is(err, errs.ErrNotFound) {
		return dto.TransferResult{}, false, nil
	}
	if err != nil {
		return dto.TransferResult{}, false, err
	}
	if err = transfer.AcceptReplay(stored); err != nil {
		return dto.TransferResult{}, false, err
	}
	return dto.FromTransfer(stored), true, nil
}

func (u AuthorizeTransfer) decide(ctx context.Context, transfer aggregate.Transfer) (dto.TransferResult, error) {
	source, ok, err := u.account(ctx, transfer.Source())
	if err != nil {
		return dto.TransferResult{}, err
	}
	if !ok {
		return u.missing(ctx, transfer)
	}
	destination, ok, err := u.account(ctx, transfer.Destination())
	if err != nil {
		return dto.TransferResult{}, err
	}
	if !ok {
		return u.missing(ctx, transfer)
	}
	authorization, err := service.Authorize(transfer, source, destination)
	if err != nil {
		return dto.TransferResult{}, err
	}
	if err = u.Transfers.Commit(ctx, authorization); err != nil {
		return dto.TransferResult{}, err
	}
	return dto.FromTransfer(authorization.Transfer), nil
}

func (u AuthorizeTransfer) account(ctx context.Context, id valueobject.AccountID) (aggregate.Account, bool, error) {
	account, err := u.Accounts.FindAccount(ctx, id)
	if errors.Is(err, errs.ErrNotFound) {
		return aggregate.Account{}, false, nil
	}
	if err != nil {
		return aggregate.Account{}, false, err
	}
	return account, true, nil
}

func (u AuthorizeTransfer) missing(ctx context.Context, transfer aggregate.Transfer) (dto.TransferResult, error) {
	decided := transfer.WithReason(reasonAccountNotFound)
	if err := u.Transfers.Commit(ctx, service.Authorization{Transfer: decided}); err != nil {
		return dto.TransferResult{}, err
	}
	return dto.FromTransfer(decided), nil
}
