package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dracma/pix/internal/httpapi"
	"dracma/pix/internal/ledger"
	"dracma/pix/internal/postgres"
	"github.com/cucumber/godog"
)

type world struct {
	store            *postgres.Store
	servers          [2]*httptest.Server
	sim              *httptest.Server
	api              httpapi.API
	last             ledger.Result
	status, approved int
}

func TestAcceptance(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	if !strings.Contains(url, "/pix_test?") {
		t.Fatal("requires isolated pix_test")
	}
	s, err := postgres.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Pool.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &world{store: s, api: httpapi.API{Store: s, Batch: postgres.NewBatcher(ctx, s, 64, 2*time.Millisecond), Key: "test-key-12345678", Timeout: 3 * time.Second}}
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
			r, e := s.Result(context.Background(), "ext")
			if e != nil {
				return e
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

func (w *world) seed(amount int64) error {
	ctx := context.Background()
	if err := w.store.Migrate(ctx); err != nil {
		return err
	}
	if _, err := w.store.Pool.Exec(ctx, `TRUNCATE outbox,entries,operations,external_receipts,accounts CASCADE`); err != nil {
		return err
	}
	if err := w.store.Migrate(ctx); err != nil {
		return err
	}
	w.last = ledger.Result{}
	w.status = 0
	w.approved = 0
	return w.store.Seed(ctx, 3, amount)
}

func (w *world) send(r ledger.Request, index int) (ledger.Result, int, error) {
	body, _ := json.Marshal(r)
	req, _ := http.NewRequest("POST", w.servers[index].URL+"/transfers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key-12345678")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ledger.Result{}, 0, err
	}
	defer res.Body.Close()
	var result ledger.Result
	if res.StatusCode == 200 {
		err = json.NewDecoder(res.Body).Decode(&result)
	}
	return result, res.StatusCode, err
}

func (w *world) transfer(amount int64, id string) error {
	var err error
	w.last, w.status, err = w.send(ledger.Request{ID: id, Source: 1, Destination: 2, Amount: amount}, 0)
	return err
}

func (w *world) balance(want int64) error {
	req, _ := http.NewRequest("GET", w.servers[1].URL+"/accounts/1/balance", nil)
	req.Header.Set("Authorization", "Bearer test-key-12345678")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var a ledger.Account
	if err = json.NewDecoder(res.Body).Decode(&a); err != nil {
		return err
	}
	if a.Balance != want {
		return fmt.Errorf("balance %d want %d", a.Balance, want)
	}
	return nil
}

func (w *world) entries(want int, id string) error {
	var n int
	err := w.store.Pool.QueryRow(context.Background(), "SELECT count(*) FROM entries WHERE operation_id=$1", id).Scan(&n)
	if err != nil {
		return err
	}
	if n != want {
		return fmt.Errorf("entries %d", n)
	}
	return nil
}

func (w *world) concurrent(count int, amount int64) error {
	var wg sync.WaitGroup
	results := make(chan ledger.Result, count)
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, status, e := w.send(ledger.Request{ID: fmt.Sprintf("parallel-%d", i), Source: 1, Destination: 2, Amount: amount}, i%2)
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
	w.last, w.status, err = w.send(ledger.Request{ID: "ext", Source: 1, External: "reject:recipient", Amount: amount}, 0)
	return err
}

func (w *world) refund() error {
	found, err := w.api.DeliverOne(context.Background(), w.sim.URL)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("missing outbox")
	}
	return w.store.Settle(context.Background(), "ext", false)
}
