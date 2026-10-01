package usecase

import (
	"context"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/aggregate"
)

type AuthorizeTransfer struct {
	Transfers port.TransferStore
}

func (u AuthorizeTransfer) Execute(ctx context.Context, cmd dto.TransferCommand) (dto.TransferResult, error) {
	transfer, err := aggregate.ParseTransfer(cmd.ID, cmd.Source, cmd.Destination, cmd.External, cmd.Amount)
	if err != nil {
		return dto.TransferResult{}, err
	}
	return u.Transfers.Authorize(ctx, transfer)
}
