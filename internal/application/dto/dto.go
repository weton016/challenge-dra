package dto

import (
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/policy"
)

type TransferCommand struct {
	ID          string `json:"transaction_id"`
	Source      int64  `json:"source_id"`
	Destination int64  `json:"destination_id,omitempty"`
	External    string `json:"destination_external,omitempty"`
	Amount      int64  `json:"amount_minor"`
}

func (c TransferCommand) Validate() error {
	_, err := aggregate.ParseTransfer(c.ID, c.Source, c.Destination, c.External, c.Amount)
	return err
}

func (c TransferCommand) Target() int64 {
	return policy.Target(c.Source, c.Destination, c.External != "")
}

type TransferResult struct {
	ID         string `json:"transaction_id"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason,omitempty"`
	Settlement string `json:"settlement_status"`
}

func FromTransfer(transfer aggregate.Transfer) TransferResult {
	return TransferResult{
		ID:         transfer.ID().String(),
		Decision:   transfer.Decision().String(),
		Reason:     transfer.Reason().String(),
		Settlement: transfer.Settlement().String(),
	}
}

type Balance struct {
	ID       int64 `json:"id"`
	Balance  int64 `json:"balance_minor"`
	Sequence int64 `json:"sequence"`
}

func FromAccount(account aggregate.Account) Balance {
	return Balance{ID: account.ID().Int64(), Balance: account.Balance().Minor(), Sequence: account.Sequence()}
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

func FromStatement(page entity.Statement) Statement {
	entries := page.Entries()
	out := Statement{Until: page.Until(), Next: page.Next(), Entries: make([]Entry, 0, len(entries))}
	for _, entry := range entries {
		out.Entries = append(out.Entries, Entry{
			TransactionID: entry.Transaction().String(),
			Delta:         entry.Delta(),
			Sequence:      entry.Sequence(),
			Balance:       entry.Balance().Minor(),
		})
	}
	return out
}
