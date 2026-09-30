package controller

import (
	"net/http"

	"dracma/pix/internal/infrastructure/postgres"
	"dracma/pix/internal/presentation/httpapi/request"
	"dracma/pix/internal/presentation/httpapi/response"
)

type Transfers struct {
	Batch *postgres.Batcher
	Store *postgres.Store
}

func (c Transfers) Post(w http.ResponseWriter, r *http.Request) {
	input, err := request.DecodeTransfer(w, r)
	if err != nil {
		response.Fail(w, err)
		return
	}
	var result any
	if c.Batch != nil {
		result, err = c.Batch.Authorize(r.Context(), input.Command())
	} else {
		result, err = c.Store.Authorize(r.Context(), input.Command())
	}
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Write(w, http.StatusOK, result)
}
