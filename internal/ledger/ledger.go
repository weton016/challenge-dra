// Package ledger contains deterministic monetary rules. Amounts are BRL cents.
package ledger

import (
	"errors"
	"math"
	"regexp"
)

var (
	ErrInvalid  = errors.New("invalid request")
	ErrConflict = errors.New("transaction id reused with different payload")
	ErrNotFound = errors.New("not found")
	validID     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

type Request struct {
	ID          string `json:"transaction_id"`
	Source      int64  `json:"source_id"`
	Destination int64  `json:"destination_id,omitempty"`
	External    string `json:"destination_external,omitempty"`
	Amount      int64  `json:"amount_minor"`
}

type Result struct {
	ID         string `json:"transaction_id"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason,omitempty"`
	Settlement string `json:"settlement_status"`
}

type Account struct {
	ID       int64 `json:"id"`
	Balance  int64 `json:"balance_minor"`
	Sequence int64 `json:"sequence"`
}

type Entry struct {
	TransactionID string `json:"transaction_id"`
	Delta         int64  `json:"delta_minor"`
	Sequence      int64  `json:"sequence"`
	Balance       int64  `json:"balance_after_minor"`
}

type Statement struct {
	Entries []Entry `json:"entries"`
	Until   int64   `json:"until"`
	Next    int64   `json:"next_after"`
}

func (r Request) Validate() error {
	if !validID.MatchString(r.ID) || r.Source <= 0 || r.Amount <= 0 {
		return ErrInvalid
	}
	if r.External != "" {
		if len(r.External) > 140 || r.Destination != 0 {
			return ErrInvalid
		}
		return nil
	}
	if r.Destination <= 0 || r.Source == r.Destination {
		return ErrInvalid
	}
	return nil
}

func (r Request) Target() int64 {
	if r.External != "" {
		return -(r.Source%32 + 1)
	}
	return r.Destination
}

// Decide validates every debit against the current balance, including equality.
func Decide(source, destination, amount int64) string {
	if amount <= 0 || source < 0 || destination < 0 {
		return "INVALID_AMOUNT"
	}
	if source < amount {
		return "INSUFFICIENT_FUNDS"
	}
	if destination > math.MaxInt64-amount {
		return "BALANCE_OVERFLOW"
	}
	return ""
}
