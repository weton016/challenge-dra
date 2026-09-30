package usecase

import (
	"context"
	"errors"
	"strings"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

type ReceiveExternal struct {
	Receipts port.ReceiptStore
}

func (u ReceiveExternal) Execute(ctx context.Context, cmd dto.TransferCommand) (dto.TransferResult, error) {
	transfer, err := aggregate.ParseTransfer(cmd.ID, cmd.Source, cmd.Destination, cmd.External, cmd.Amount)
	if err != nil {
		return dto.TransferResult{}, err
	}
	if transfer.External().String() == "" {
		return dto.TransferResult{}, errs.ErrInvalid
	}
	stored, err := u.Receipts.FindReceipt(ctx, transfer.ID())
	if err == nil {
		if err = transfer.AcceptReplay(stored.Transfer); err != nil {
			return dto.TransferResult{}, err
		}
		return stored.Result, nil
	}
	if !errors.Is(err, errs.ErrNotFound) {
		return dto.TransferResult{}, err
	}
	result := dto.TransferResult{
		ID:         transfer.ID().String(),
		Decision:   valueobject.Approved.String(),
		Settlement: valueobject.SettlementSettled.String(),
	}
	if strings.HasPrefix(transfer.External().String(), "reject:") {
		result.Decision = valueobject.Declined.String()
	}
	if err = u.Receipts.SaveReceipt(ctx, transfer, result); err != nil {
		return dto.TransferResult{}, err
	}
	return result, nil
}

type GetReceipt struct {
	Receipts port.ReceiptStore
}

func (u GetReceipt) Execute(ctx context.Context, id string) (dto.TransferResult, error) {
	parsed, err := valueobject.NewTransactionID(id)
	if err != nil {
		return dto.TransferResult{}, err
	}
	stored, err := u.Receipts.FindReceipt(ctx, parsed)
	if err != nil {
		return dto.TransferResult{}, err
	}
	return stored.Result, nil
}
