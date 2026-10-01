package controller

import (
	"net/http"

	"dracma/pix/internal/application/usecase"
	"dracma/pix/internal/infrastructure/tigerbeetle"
	"dracma/pix/internal/presentation/httpapi/request"
	"dracma/pix/internal/presentation/httpapi/response"
)

type Transfers struct{ Ledger *tigerbeetle.Ledger }

func (c Transfers) Post(w http.ResponseWriter, r *http.Request) {
	input, err := request.DecodeTransfer(w, r)
	if err != nil {
		response.Fail(w, err)
		return
	}
	result, err := (usecase.AuthorizeTransfer{Transfers: c.Ledger}).Execute(r.Context(), input.Command())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Write(w, http.StatusOK, result)
}

func (c Transfers) Get(w http.ResponseWriter, r *http.Request) {
	result, err := (usecase.GetTransfer{Transfers: c.Ledger}).Execute(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Write(w, http.StatusOK, result)
}
