package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"dracma/pix/internal/application/dto"
)

// Deliver sends every unsettled external reservation to the receiving
// institution and settles the ones it answers. Failures stay pending.
func (a API) Deliver(ctx context.Context, url string) (int, error) {
	pending, err := a.Ledger.Pending(ctx, 200)
	if err != nil {
		return 0, err
	}
	var failed error
	for _, input := range pending {
		failed = errors.Join(failed, a.deliverOne(ctx, url, input))
	}
	return len(pending), failed
}

func (a API) deliverOne(ctx context.Context, url string, input dto.TransferCommand) error {
	data, _ := json.Marshal(input)
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/transfers", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.Key)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 500 * time.Millisecond}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("external HTTP %d", res.StatusCode)
	}
	var result dto.TransferResult
	if err = json.NewDecoder(res.Body).Decode(&result); err != nil {
		return err
	}
	if result.ID != input.ID || (result.Decision != "APPROVED" && result.Decision != "DECLINED") {
		return fmt.Errorf("invalid external decision")
	}
	return a.Ledger.Settle(ctx, input.ID, result.Decision == "APPROVED")
}

func (a API) RunWorker(ctx context.Context, url string) {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_, _ = a.Deliver(ctx, url)
		}
	}
}
