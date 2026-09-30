package codec

import (
	"encoding/json"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/valueobject"
)

func RequestOf(transfer aggregate.Transfer) dto.TransferCommand {
	request := dto.TransferCommand{ID: transfer.ID().String(), Source: transfer.Source().Int64(), External: transfer.External().String(), Amount: transfer.Amount().Minor()}
	if request.External == "" {
		request.Destination = transfer.Destination().Int64()
	}
	return request
}

func DecodeTransfer(payload []byte, decision, reason, settlement string) (aggregate.Transfer, error) {
	var request dto.TransferCommand
	if err := json.Unmarshal(payload, &request); err != nil {
		return aggregate.Transfer{}, err
	}
	parsed, err := aggregate.ParseTransfer(request.ID, request.Source, request.Destination, request.External, request.Amount)
	if err != nil {
		return aggregate.Transfer{}, err
	}
	return aggregate.RestoreTransfer(parsed, valueobject.Decision(decision), valueobject.Reason(reason), valueobject.Settlement(settlement))
}
