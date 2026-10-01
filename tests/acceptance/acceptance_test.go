package acceptance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/infrastructure/tigerbeetle"
	"dracma/pix/internal/presentation/httpapi"
	"github.com/cucumber/godog"
)

// TigerBeetle cannot be truncated: each scenario gets new accounts
// base and base+1, and each Gherkin key a new UUID.
type world struct {
	ledger           *tigerbeetle.Ledger
	servers          [2]*httptest.Server
	sim              *httptest.Server
	api              httpapi.API
	base             int64
	ids              map[string]string
	last             dto.TransferResult
	status, approved int
}

func TestAcceptance(t *testing.T) {
	address := os.Getenv("TEST_TB_ADDRESS")
	if address == "" {
		t.Skip("TEST_TB_ADDRESS required")
	}
	l, err := tigerbeetle.Open(address)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err = l.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	w := &world{ledger: l, api: httpapi.API{Ledger: l, Key: "test-key-12345678", Timeout: 3 * time.Second}}
	w.servers = [2]*httptest.Server{httptest.NewServer(w.api.Handler(false)), httptest.NewServer(w.api.Handler(false))}
	defer w.servers[0].Close()
	defer w.servers[1].Close()
	w.sim = httptest.NewServer(w.api.Handler(true))
	defer w.sim.Close()
	suite := godog.TestSuite{Name: "pix", ScenarioInitializer: func(sc *godog.ScenarioContext) {
		sc.Step(`^contas com saldo inicial de (\d+) centavos$`, w.seed)
		sc.Step(`^transfiro (\d+) centavos com a chave "([^"]*)"$`, w.transfer)
		sc.Step(`^a decisão é "([^"]*)"$`, func(want string) error {
			if w.last.Decision != want {
				return fmt.Errorf("got %+v", w.last)
			}
			return nil
		})
		sc.Step(`^o saldo da origem é (\d+) centavos$`, w.balance)
		sc.Step(`^existem (\d+) partidas para "([^"]*)"$`, w.entries)
		sc.Step(`^o HTTP é (\d+)$`, func(n int) error {
			if w.status != n {
				return fmt.Errorf("HTTP %d", w.status)
			}
			return nil
		})
		sc.Step(`^envio (\d+) débitos simultâneos de (\d+) centavos$`, w.concurrent)
		sc.Step(`^exatamente (\d+) são aprovados$`, func(n int) error {
			if w.approved != n {
				return fmt.Errorf("approved %d", w.approved)
			}
			return nil
		})
		sc.Step(`^envio (\d+) centavos para destino externo rejeitado$`, w.external)
		sc.Step(`^processo a outbox e repito a rejeição$`, w.refund)
		sc.Step(`^a liquidação é "([^"]*)"$`, func(state string) error {
			var r dto.TransferResult
			if err := w.get(fmt.Sprintf("/transfers/%s", w.id("ext")), &r); err != nil {
				return err
			}
			if r.Settlement != state {
				return fmt.Errorf("%+v", r)
			}
			return nil
		})
	}, Options: &godog.Options{Format: "pretty", Paths: []string{"../../features"}, TestingT: t, Strict: true}}
	if suite.Run() != 0 {
		t.Fatal("acceptance failed")
	}
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := fmt.Sprintf("%x", b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func (w *world) id(key string) string {
	if w.ids[key] == "" {
		w.ids[key] = newID()
	}
	return w.ids[key]
}

func (w *world) seed(amount int64) error {
	var b [8]byte
	_, _ = rand.Read(b[:])
	w.base = int64(binary.LittleEndian.Uint64(b[:])>>4) | 1<<40
	w.ids = map[string]string{}
	w.last = dto.TransferResult{}
	w.status = 0
	w.approved = 0
	return w.ledger.Seed(context.Background(), w.base, 2, amount)
}

func (w *world) send(r dto.TransferCommand, index int) (dto.TransferResult, int, error) {
	body, _ := json.Marshal(r)
	req, _ := http.NewRequest("POST", w.servers[index].URL+"/transfers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key-12345678")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return dto.TransferResult{}, 0, err
	}
	defer res.Body.Close()
	var result dto.TransferResult
	if res.StatusCode == 200 {
		err = json.NewDecoder(res.Body).Decode(&result)
	}
	return result, res.StatusCode, err
}

func (w *world) get(path string, out any) error {
	req, _ := http.NewRequest("GET", w.servers[1].URL+path, nil)
	req.Header.Set("Authorization", "Bearer test-key-12345678")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("GET %s: HTTP %d", path, res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (w *world) transfer(amount int64, key string) error {
	var err error
	w.last, w.status, err = w.send(dto.TransferCommand{ID: w.id(key), Source: w.base, Destination: w.base + 1, Amount: amount}, 0)
	return err
}

func (w *world) balance(want int64) error {
	var a dto.Balance
	if err := w.get(fmt.Sprintf("/accounts/%d/balance", w.base), &a); err != nil {
		return err
	}
	if a.Balance != want {
		return fmt.Errorf("balance %d want %d", a.Balance, want)
	}
	return nil
}

// entries counts the postings of one transaction across both statements.
func (w *world) entries(want int, key string) error {
	n := 0
	for _, account := range []int64{w.base, w.base + 1} {
		var page dto.Statement
		if err := w.get(fmt.Sprintf("/accounts/%d/statement", account), &page); err != nil {
			return err
		}
		for _, e := range page.Entries {
			if e.TransactionID == w.id(key) {
				n++
			}
		}
	}
	if n != want {
		return fmt.Errorf("entries %d", n)
	}
	return nil
}

func (w *world) concurrent(count int, amount int64) error {
	var wg sync.WaitGroup
	results := make(chan dto.TransferResult, count)
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, status, e := w.send(dto.TransferCommand{ID: newID(), Source: w.base, Destination: w.base + 1, Amount: amount}, i%2)
			if e == nil && status != 200 {
				e = fmt.Errorf("HTTP %d", status)
			}
			results <- r
			errs <- e
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			return e
		}
	}
	for r := range results {
		if r.Decision == "APPROVED" {
			w.approved++
		}
	}
	return nil
}

func (w *world) external(amount int64) error {
	var err error
	w.last, w.status, err = w.send(dto.TransferCommand{ID: w.id("ext"), Source: w.base, External: "reject:recipient", Amount: amount}, 0)
	return err
}

// refund runs the worker's sweep until this reservation is settled, then
// repeats the rejection, which must not refund twice.
func (w *world) refund() error {
	ctx := context.Background()
	for range 1000 {
		if _, err := w.api.Deliver(ctx, w.sim.URL); err != nil {
			return err
		}
		var r dto.TransferResult
		if err := w.get("/transfers/"+w.id("ext"), &r); err != nil {
			return err
		}
		if r.Settlement != "PENDING" {
			return w.ledger.Settle(ctx, w.id("ext"), false)
		}
	}
	return fmt.Errorf("reservation never delivered")
}
