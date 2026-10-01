// Command audit recomputes the books from TigerBeetle. Run it with no traffic:
// TigerBeetle has no read snapshot.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"dracma/pix/internal/infrastructure/tigerbeetle"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	accounts := flag.String("history", "1,2,3", "comma-separated accounts whose full history is replayed")
	flag.Parse()
	ids := []int64{}
	for _, raw := range strings.Split(*accounts, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	ledger, err := tigerbeetle.Open(os.Getenv("TB_ADDRESS"), 0)
	if err != nil {
		return err
	}
	defer ledger.Close()
	report, err := ledger.Audit(ctx, ids)
	if err != nil {
		return err
	}
	out := struct {
		At time.Time `json:"at"`
		tigerbeetle.Report
	}{time.Now().UTC(), report}
	if err = json.NewEncoder(os.Stdout).Encode(out); err != nil {
		return err
	}
	if !report.Passed {
		return fmt.Errorf("financial audit failed")
	}
	return nil
}
