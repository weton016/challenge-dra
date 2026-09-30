// Package httpapi is the HTTP presentation: routing, auth and controllers.
package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"

	"dracma/pix/internal/infrastructure/postgres"
	"dracma/pix/internal/presentation/httpapi/controller"
	"dracma/pix/internal/presentation/httpapi/response"
)

type API struct {
	Store   *postgres.Store
	Batch   *postgres.Batcher
	Key     string
	Timeout time.Duration
}

func (a API) Handler(simulator bool) http.Handler {
	mux := http.NewServeMux()
	health := controller.Health{Store: a.Store}
	mux.HandleFunc("GET /healthz", health.Live)
	mux.HandleFunc("GET /readyz", health.Ready)
	if simulator {
		sim := controller.Simulator{Store: a.Store}
		mux.HandleFunc("POST /transfers", sim.Receive)
		mux.HandleFunc("GET /transfers/{id}", sim.Receipt)
	} else {
		transfers := controller.Transfers{Batch: a.Batch, Store: a.Store}
		accounts := controller.Accounts{Store: a.Store}
		operations := controller.Operations{Store: a.Store}
		mux.HandleFunc("POST /transfers", transfers.Post)
		mux.HandleFunc("GET /transfers/{id}", operations.Get)
		mux.HandleFunc("GET /accounts/{id}/balance", accounts.Balance)
		mux.HandleFunc("GET /accounts/{id}/statement", accounts.Statement)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+a.Key)) != 1 {
			response.Write(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED"})
			return
		}
		timeout := a.Timeout
		if timeout == 0 {
			timeout = 1500 * time.Millisecond
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
