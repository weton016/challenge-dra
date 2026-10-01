package controller

import (
	"net/http"

	"dracma/pix/internal/application/usecase"
	"dracma/pix/internal/infrastructure/tigerbeetle"
	"dracma/pix/internal/presentation/httpapi/request"
	"dracma/pix/internal/presentation/httpapi/response"
)

type Accounts struct{ Ledger *tigerbeetle.Ledger }

func (c Accounts) Balance(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseAccountID(r.PathValue("id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	result, err := (usecase.GetBalance{Accounts: c.Ledger}).Execute(r.Context(), id)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Write(w, http.StatusOK, result)
}

func (c Accounts) Statement(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseAccountID(r.PathValue("id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	after, err := request.ParseCursor(r.URL.Query().Get("after"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	until, err := request.ParseCursor(r.URL.Query().Get("until"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	result, err := (usecase.GetStatement{Accounts: c.Ledger, Statements: c.Ledger}).Execute(r.Context(), id, after, until)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Write(w, http.StatusOK, result)
}
