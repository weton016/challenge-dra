package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"dracma/pix/internal/ledger"
	"dracma/pix/internal/postgres"
)

type API struct {
	Store   *postgres.Store
	Batch   *postgres.Batcher
	Key     string
	Timeout time.Duration
}

func (a API) Handler(simulator bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", a.ready)
	if simulator {
		mux.HandleFunc("POST /transfers", a.receive)
		mux.HandleFunc("GET /transfers/{id}", a.receipt)
	} else {
		mux.HandleFunc("POST /transfers", a.transfer)
		mux.HandleFunc("GET /transfers/{id}", a.result)
		mux.HandleFunc("GET /accounts/{id}/balance", a.balance)
		mux.HandleFunc("GET /accounts/{id}/statement", a.statement)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+a.Key)) != 1 {
			write(w, 401, map[string]string{"error": "UNAUTHORIZED"})
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

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func failure(w http.ResponseWriter, err error) {
	code, message := 503, "OUTCOME_UNKNOWN_RETRY_SAME_ID"
	switch {
	case errors.Is(err, ledger.ErrInvalid):
		code, message = 400, "INVALID_REQUEST"
	case errors.Is(err, ledger.ErrConflict):
		code, message = 409, "IDEMPOTENCY_CONFLICT"
	case errors.Is(err, ledger.ErrNotFound):
		code, message = 404, "NOT_FOUND"
	}
	write(w, code, map[string]string{"error": message})
}

func decode(w http.ResponseWriter, r *http.Request) (ledger.Request, error) {
	var input ledger.Request
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, ledger.ErrInvalid
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return input, ledger.ErrInvalid
	}
	return input, input.Validate()
}

func (a API) transfer(w http.ResponseWriter, r *http.Request) {
	input, err := decode(w, r)
	if err != nil {
		failure(w, err)
		return
	}
	result, err := a.authorize(r.Context(), input)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, result)
}

func (a API) authorize(ctx context.Context, input ledger.Request) (ledger.Result, error) {
	if a.Batch != nil {
		return a.Batch.Authorize(ctx, input)
	}
	return a.Store.Authorize(ctx, input)
}

func (a API) receive(w http.ResponseWriter, r *http.Request) {
	input, err := decode(w, r)
	if err != nil {
		failure(w, err)
		return
	}
	result, err := a.Store.Receive(r.Context(), input)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, result)
}

func (a API) result(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.Result(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, result)
}

func (a API) receipt(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.Receipt(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, result)
}

func accountID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, ledger.ErrInvalid
	}
	return id, nil
}

func (a API) balance(w http.ResponseWriter, r *http.Request) {
	id, err := accountID(r)
	if err != nil {
		failure(w, err)
		return
	}
	result, err := a.Store.Balance(r.Context(), id)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, result)
}

func cursor(r *http.Request, name string) (int64, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		return 0, ledger.ErrInvalid
	}
	return n, nil
}

func (a API) statement(w http.ResponseWriter, r *http.Request) {
	id, err := accountID(r)
	if err != nil {
		failure(w, err)
		return
	}
	after, err := cursor(r, "after")
	if err != nil {
		failure(w, err)
		return
	}
	until, err := cursor(r, "until")
	if err != nil {
		failure(w, err)
		return
	}
	result, err := a.Store.Statement(r.Context(), id, after, until)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, result)
}

func (a API) ready(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.Pool.Ping(r.Context()); err != nil {
		failure(w, err)
		return
	}
	write(w, 200, map[string]string{"status": "ready"})
}
