package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"dracma/pix/internal/ledger"
)

func (a API) DeliverOne(ctx context.Context, url string) (bool, error) {
	input, found, err := a.Store.Claim(ctx)
	if err != nil || !found {
		return false, err
	}
	data, _ := json.Marshal(input)
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/transfers", bytes.NewReader(data))
	if err != nil {
		return true, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Key)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 500 * time.Millisecond}
	res, err := client.Do(req)
	if err != nil {
		return true, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return true, fmt.Errorf("external HTTP %d", res.StatusCode)
	}
	var result ledger.Result
	if err = json.NewDecoder(res.Body).Decode(&result); err != nil {
		return true, err
	}
	if result.ID != input.ID || (result.Decision != "APPROVED" && result.Decision != "DECLINED") {
		return true, fmt.Errorf("invalid external decision")
	}
	return true, a.Store.Settle(ctx, input.ID, result.Decision == "APPROVED")
}

func (a API) RunWorker(ctx context.Context, url string) {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			for i := 0; i < 64; i++ {
				found, err := a.DeliverOne(ctx, url)
				if err != nil || !found {
					break
				}
			}
		}
	}
}
