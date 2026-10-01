package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/infrastructure/tigerbeetle"
	"dracma/pix/internal/presentation/httpapi/request"
	"dracma/pix/internal/presentation/httpapi/response"
)

const okID = "0190f3a2-7c1e-4d2b-9a6f-1234567890ab"

func TestInputBoundaries(t *testing.T) {
	a := API{Key: "test-key-12345678"}
	h := a.Handler(false)
	cases := []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/healthz", "", 200},
		{"POST", "/transfers", "{", 400},
		{"POST", "/transfers", `{"transaction_id":"x","source_id":1,"destination_id":2,"amount_minor":1}`, 400},
		{"POST", "/transfers", `{"transaction_id":"` + okID + `","source_id":1,"destination_id":2,"amount_minor":1,"extra":true}`, 400},
		{"POST", "/transfers", `{"transaction_id":"` + okID + `","source_id":1,"destination_id":2,"amount_minor":1} {}`, 400},
		{"POST", "/transfers", `{"transaction_id":"` + okID + `","source_id":1,"destination_id":2,"amount_minor":9223372036854775808}`, 400},
		{"POST", "/transfers", `{"transaction_id":"` + okID + `","source_id":1,"destination_id":1,"amount_minor":1}`, 400},
		{"POST", "/transfers", strings.Repeat(" ", 5000) + "{}", 400},
		{"GET", "/transfers/not-a-uuid", "", 400},
		{"GET", "/accounts/-1/balance", "", 400},
		{"GET", "/accounts/no/balance", "", 400},
		{"GET", "/accounts/no/statement", "", 400},
		{"GET", "/accounts/1/statement?after=-1", "", 400},
		{"GET", "/accounts/1/statement?until=bad", "", 400},
	}
	for _, c := range cases {
		t.Run(c.path+c.body[:min(5, len(c.body))], func(t *testing.T) {
			if w := call(h, c.method, c.path, c.body); w.Code != c.status {
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
	}{{errs.ErrNotFound, 404}, {errs.ErrConflict, 409}, {errors.New("ledger error"), 503}} {
		w = httptest.NewRecorder()
		response.Fail(w, c.err)
		if w.Code != c.code {
			t.Fatal(w.Code)
		}
	}
}

func open(t *testing.T) *tigerbeetle.Ledger {
	t.Helper()
	address := os.Getenv("TEST_TB_ADDRESS")
	if address == "" {
		t.Skip("TEST_TB_ADDRESS required")
	}
	l, err := tigerbeetle.Open(address)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return l
}

// setup gives each test fresh accounts base..base+2 with 100 cents each.
func setup(t *testing.T) (API, int64) {
	t.Helper()
	l := open(t)
	t.Cleanup(l.Close)
	var b [8]byte
	_, _ = rand.Read(b[:])
	base := int64(binary.LittleEndian.Uint64(b[:])>>4) | 1<<40
	if err := l.Seed(context.Background(), base, 3, 100); err != nil {
		t.Fatal(err)
	}
	return API{Ledger: l, Key: "test-key-12345678", Timeout: time.Second}, base
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := fmt.Sprintf("%x", b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test-key-12345678")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func body(id string, source, destination int64, external string, amount int64) string {
	data, _ := json.Marshal(dto.TransferCommand{ID: id, Source: source, Destination: destination, External: external, Amount: amount})
	return string(data)
}

func TestHTTPAndUnavailableLedger(t *testing.T) {
	a, base := setup(t)
	h := a.Handler(false)
	id := newID()
	if w := call(h, "POST", "/transfers", body(id, base, base+1, "", 10)); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, path := range []string{"/readyz", "/transfers/" + id, fmt.Sprintf("/accounts/%d/balance", base), fmt.Sprintf("/accounts/%d/statement?after=0", base)} {
		if w := call(h, "GET", path, ""); w.Code != 200 {
			t.Fatal(path, w.Body.String())
		}
	}
	for _, path := range []string{"/transfers/" + newID(), fmt.Sprintf("/accounts/%d/balance", base+100), fmt.Sprintf("/accounts/%d/statement", base+100)} {
		if w := call(h, "GET", path, ""); w.Code != 404 {
			t.Fatal(path, w.Body.String())
		}
	}
	if w := call(h, "POST", "/transfers", body(id, base, base+1, "", 11)); w.Code != 409 {
		t.Fatal(w.Body.String())
	}
	sim := a.Handler(true)
	if w := call(sim, "POST", "/transfers", "{}"); w.Code != 400 {
		t.Fatal(w.Body.String())
	}
	remote := newID()
	if w := call(sim, "POST", "/transfers", body(remote, base, 0, "recipient", 10)); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call(sim, "GET", "/transfers/"+remote, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call(sim, "GET", "/transfers/"+newID(), ""); w.Code != 404 {
		t.Fatal(w.Body.String())
	}
	if w := call(sim, "POST", "/transfers", body(remote, base, 0, "recipient", 11)); w.Code != 409 {
		t.Fatal(w.Body.String())
	}
	closed := open(t)
	closed.Close()
	h = API{Ledger: closed, Key: a.Key, Timeout: time.Second}.Handler(false)
	for _, path := range []string{"/readyz", "/transfers/" + id, fmt.Sprintf("/accounts/%d/balance", base), fmt.Sprintf("/accounts/%d/statement", base)} {
		if w := call(h, "GET", path, ""); w.Code != 503 {
			t.Fatal(path, w.Body.String())
		}
	}
	if w := call(h, "POST", "/transfers", body(newID(), base, base+1, "", 1)); w.Code != 503 {
		t.Fatal(w.Body.String())
	}
}

func TestDecisionsOverHTTP(t *testing.T) {
	a, base := setup(t)
	h := a.Handler(false)
	post := func(payload string) dto.TransferResult {
		t.Helper()
		w := call(h, "POST", "/transfers", payload)
		var got dto.TransferResult
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 {
			t.Fatal(w.Code, w.Body.String(), err)
		}
		return got
	}
	balance := func(id int64) dto.Balance {
		t.Helper()
		var got dto.Balance
		w := call(h, "GET", fmt.Sprintf("/accounts/%d/balance", id), "")
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 {
			t.Fatal(w.Code, w.Body.String(), err)
		}
		return got
	}
	id := newID()
	got := post(body(strings.ToUpper(id), base, base+1, "", 100))
	if got != (dto.TransferResult{ID: id, Decision: "APPROVED", Settlement: "INTERNAL"}) {
		t.Fatal(got)
	}
	account := balance(base)
	if account.ID != base || account.Balance != 0 || account.Sequence == 0 {
		t.Fatal(account)
	}
	var page dto.Statement
	w := call(h, "GET", fmt.Sprintf("/accounts/%d/statement", base), "")
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Entries) != 1 {
		t.Fatal(w.Body.String(), err)
	}
	entry := page.Entries[0]
	if entry.TransactionID != id || entry.Delta != -100 || entry.Balance != 0 || entry.Sequence != account.Sequence || page.Next != account.Sequence {
		t.Fatal(page)
	}
	got = post(body(newID(), base, base+1, "", 1))
	if got.Decision != "DECLINED" || got.Reason != "INSUFFICIENT_FUNDS" || got.Settlement != "INTERNAL" {
		t.Fatal(got)
	}
	got = post(body(newID(), base+1, base+100, "", 1))
	if got.Decision != "DECLINED" || got.Reason != "ACCOUNT_NOT_FOUND" {
		t.Fatal(got)
	}
	got = post(body(newID(), base+1, 0, "shop", 10))
	if got.Decision != "APPROVED" || got.Reason != "" || got.Settlement != "PENDING" {
		t.Fatal(got)
	}
	if b := balance(base + 1); b.Balance != 190 {
		t.Fatal(b)
	}
}

func TestWorkerRetriesAndCompletes(t *testing.T) {
	a, base := setup(t)
	ctx := context.Background()
	remote := newID()
	if w := call(a.Handler(false), "POST", "/transfers", body(remote, base, 0, "recipient", 10)); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	failing := []string{"http://bad\nurl", "http://127.0.0.1:1"}
	for _, c := range []struct {
		status int
		body   string
	}{{500, "error"}, {200, "invalid json"}, {200, `{"transaction_id":"other","decision":"APPROVED"}`}, {200, `{"transaction_id":"` + remote + `","decision":"MAYBE"}`}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		defer server.Close()
		failing = append(failing, server.URL)
	}
	for _, url := range failing {
		if err := deliverUntilFound(t, a, url); err == nil {
			t.Fatal("expected delivery error", url)
		}
	}
	sim := httptest.NewServer(a.Handler(true))
	defer sim.Close()
	worker, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { a.RunWorker(worker, sim.URL); close(done) }()
	defer func() { cancel(); <-done }()
	h := a.Handler(false)
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var got dto.TransferResult
		w := call(h, "GET", "/transfers/"+remote, "")
		if json.Unmarshal(w.Body.Bytes(), &got) == nil && got.Settlement == "SETTLED" {
			closed := open(t)
			closed.Close()
			if _, err := (API{Ledger: closed}).Deliver(ctx, sim.URL); err == nil {
				t.Fatal("expected closed ledger error")
			}
			return
		}
	}
	t.Fatal("worker did not settle")
}

// deliverUntilFound sweeps until a page with open reservations is delivered.
func deliverUntilFound(t *testing.T, a API, url string) error {
	t.Helper()
	for range 1000 {
		n, err := a.Deliver(context.Background(), url)
		if n > 0 {
			return err
		}
	}
	t.Fatal("no pending reservation found")
	return nil
}

func TestDecodeValid(t *testing.T) {
	input := dto.TransferCommand{ID: okID, Source: 1, Destination: 2, Amount: 1}
	b, _ := json.Marshal(input)
	got, err := request.DecodeTransfer(httptest.NewRecorder(), httptest.NewRequest("POST", "/transfers", strings.NewReader(string(b))))
	if err != nil || got.Command() != input {
		t.Fatal(got, err)
	}
}
