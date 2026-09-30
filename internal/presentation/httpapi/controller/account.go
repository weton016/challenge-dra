package controller

import (
	"net/http"

	"dracma/pix/internal/application/usecase"
	"dracma/pix/internal/infrastructure/postgres"
	"dracma/pix/internal/infrastructure/postgres/account"
	"dracma/pix/internal/infrastructure/postgres/statement"
	"dracma/pix/internal/infrastructure/postgres/transfer"
	"dracma/pix/internal/presentation/httpapi/request"
	"dracma/pix/internal/presentation/httpapi/response"
)

type Accounts struct{ Store *postgres.Store }

func (c Accounts) Balance(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseAccountID(r.PathValue("id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	result, err := (usecase.GetBalance{Accounts: account.New(c.Store)}).Execute(r.Context(), id)
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
	result, err := (usecase.GetStatement{Accounts: account.New(c.Store), Statements: statement.New(c.Store)}).Execute(r.Context(), id, after, until)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Write(w, http.StatusOK, result)
}

type Operations struct{ Store *postgres.Store }

func (c Operations) Get(w http.ResponseWriter, r *http.Request) {
	result, err := (usecase.GetTransfer{Transfers: transfer.New(c.Store)}).Execute(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Write(w, http.StatusOK, result)
}
