package controller

import (
	"net/http"

	"dracma/pix/internal/infrastructure/tigerbeetle"
	"dracma/pix/internal/presentation/httpapi/response"
)

type Health struct{ Ledger *tigerbeetle.Ledger }

func (c Health) Live(w http.ResponseWriter, r *http.Request) {
	response.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (c Health) Ready(w http.ResponseWriter, r *http.Request) {
	if err := c.Ledger.Ping(r.Context()); err != nil {
		response.Fail(w, err)
		return
	}
	response.Write(w, http.StatusOK, map[string]string{"status": "ready"})
}
