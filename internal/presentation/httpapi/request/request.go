package request

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
)

type Transfer struct {
	ID          string `json:"transaction_id"`
	Source      int64  `json:"source_id"`
	Destination int64  `json:"destination_id,omitempty"`
	External    string `json:"destination_external,omitempty"`
	Amount      int64  `json:"amount_minor"`
}

func DecodeTransfer(w http.ResponseWriter, r *http.Request) (Transfer, error) {
	var input Transfer
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, errs.ErrInvalid
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return input, errs.ErrInvalid
	}
	if err := input.Validate(); err != nil {
		return input, err
	}
	return input, nil
}

func (t Transfer) Validate() error {
	_, err := aggregate.ParseTransfer(t.ID, t.Source, t.Destination, t.External, t.Amount)
	return err
}

func (t Transfer) Command() dto.TransferCommand {
	return dto.TransferCommand{ID: t.ID, Source: t.Source, Destination: t.Destination, External: t.External, Amount: t.Amount}
}

func ParseAccountID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errs.ErrInvalid
	}
	return id, nil
}

func ParseCursor(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0, errs.ErrInvalid
	}
	return n, nil
}
