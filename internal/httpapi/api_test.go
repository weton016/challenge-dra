package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"dracma/pix/internal/ledger"
	"dracma/pix/internal/postgres"
)

func TestInputBoundaries(t *testing.T) {
	a := API{Key: "test-key-12345678"}
	h := a.Handler(false)
	cases := []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/healthz", "", 200},
		{"POST", "/transfers", "{", 400},
		{"POST", "/transfers", `{"transaction_id":"x","source_id":1,"destination_id":2,"amount_minor":1,"extra":true}`, 400},
		{"POST", "/transfers", `{"transaction_id":"x","source_id":1,"destination_id":2,"amount_minor":1} {}`, 400},
		{"POST", "/transfers", `{"transaction_id":"x","source_id":1,"destination_id":2,"amount_minor":9223372036854775808}`, 400},
		{"POST", "/transfers", strings.Repeat(" ", 5000) + "{}", 400},
		{"GET", "/accounts/-1/balance", "", 400},
		{"GET", "/accounts/no/balance", "", 400},
		{"GET", "/accounts/no/statement", "", 400},
		{"GET", "/accounts/1/statement?after=-1", "", 400},
		{"GET", "/accounts/1/statement?until=bad", "", 400},
	}
	for _, c := range cases {
		t.Run(c.path+c.body[:min(5, len(c.body))], func(t *testing.T) {
			r := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
			r.Header.Set("Authorization", "Bearer test-key-12345678")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != c.status {
				t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
			}
		})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/accounts/1/balance", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	for _, c := range []struct {
		err  error
		code int
	}{{ledger.ErrNotFound, 404}, {ledger.ErrConflict, 409}, {errors.New("database error"), 503}} {
		w = httptest.NewRecorder()
		failure(w, c.err)
		if w.Code != c.code {
			t.Fatal(w.Code)
		}
	}
}

func setup(t *testing.T) API {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	if !strings.Contains(url, "/pix_test?") {
		t.Fatal("requires pix_test")
	}
	s, err := postgres.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Pool.Close)
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(context.Background(), `TRUNCATE outbox,entries,operations,external_receipts,accounts CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = s.Seed(context.Background(), 3, 100); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return API{Store: s, Batch: postgres.NewBatcher(ctx, s, 64, 2*time.Millisecond), Key: "test-key-12345678", Timeout: time.Second}
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test-key-12345678")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHTTPAndUnavailableDatabase(t *testing.T) {
	a := setup(t)
	h := a.Handler(false)
	body := `{"transaction_id":"api","source_id":1,"destination_id":2,"amount_minor":10}`
	if w := call(h, "POST", "/transfers", body); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, path := range []string{"/readyz", "/transfers/api", "/accounts/1/balance", "/accounts/1/statement?after=0&until=1"} {
		if w := call(h, "GET", path, ""); w.Code != 200 {
			t.Fatal(path, w.Body.String())
		}
	}
	for _, path := range []string{"/transfers/missing", "/accounts/999/balance", "/accounts/999/statement"} {
		if w := call(h, "GET", path, ""); w.Code != 404 {
			t.Fatal(path, w.Body.String())
		}
	}
	if w := call(h, "POST", "/transfers", strings.Replace(body, "10}", "11}", 1)); w.Code != 409 {
		t.Fatal(w.Body.String())
	}
	sim := a.Handler(true)
	if w := call(sim, "POST", "/transfers", "{}"); w.Code != 400 {
		t.Fatal(w.Body.String())
	}
	ext := `{"transaction_id":"remote","source_id":1,"destination_external":"recipient","amount_minor":10}`
	if w := call(sim, "POST", "/transfers", ext); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call(sim, "GET", "/transfers/remote", ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call(sim, "GET", "/transfers/missing", ""); w.Code != 404 {
		t.Fatal(w.Body.String())
	}
	if w := call(sim, "POST", "/transfers", strings.Replace(ext, "10}", "11}", 1)); w.Code != 409 {
		t.Fatal(w.Body.String())
	}
	a.Store.Pool.Close()
	for _, path := range []string{"/readyz", "/transfers/api", "/accounts/1/balance", "/accounts/1/statement"} {
		if w := call(h, "GET", path, ""); w.Code != 503 {
			t.Fatal(path, w.Body.String())
		}
	}
	if w := call(h, "POST", "/transfers", body); w.Code != 503 {
		t.Fatal(w.Body.String())
	}
}

func TestWorkerRetriesAndCompletes(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	found, err := a.DeliverOne(ctx, "bad")
	if err != nil || found {
		t.Fatal(found, err)
	}
	_, err = a.Store.Authorize(ctx, ledger.Request{ID: "remote", Source: 1, External: "recipient", Amount: 10})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		status int
		body   string
	}{{500, "error"}, {200, "invalid json"}, {200, `{"transaction_id":"other","decision":"APPROVED"}`}, {200, `{"transaction_id":"remote","decision":"MAYBE"}`}}
	for _, c := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(c.status); _, _ = w.Write([]byte(c.body)) }))
		_, err = a.DeliverOne(ctx, server.URL)
		server.Close()
		if err == nil {
			t.Fatal("expected delivery error")
		}
		_, err = a.Store.Pool.Exec(ctx, "UPDATE outbox SET lease_until='-infinity'")
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, url := range []string{"http://bad\nurl", "http://127.0.0.1:1"} {
		if _, err = a.DeliverOne(ctx, url); err == nil {
			t.Fatal("expected network error")
		}
		_, _ = a.Store.Pool.Exec(ctx, "UPDATE outbox SET lease_until='-infinity'")
	}
	sim := httptest.NewServer(a.Handler(true))
	defer sim.Close()
	worker, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { a.RunWorker(worker, sim.URL); close(done) }()
	defer func() { cancel(); <-done }()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("worker did not settle")
		case <-tick.C:
			r, e := a.Store.Result(ctx, "remote")
			if e == nil && r.Settlement == "SETTLED" {
				a.Store.Pool.Close()
				if _, e = a.DeliverOne(ctx, sim.URL); e == nil {
					t.Fatal("expected closed pool error")
				}
				return
			}
		}
	}
}

func TestDecodeValid(t *testing.T) {
	input := ledger.Request{ID: "ok", Source: 1, Destination: 2, Amount: 1}
	b, _ := json.Marshal(input)
	r, err := decode(httptest.NewRecorder(), httptest.NewRequest("POST", "/transfers", strings.NewReader(string(b))))
	if err != nil || r != input {
		t.Fatal(r, err)
	}
}
