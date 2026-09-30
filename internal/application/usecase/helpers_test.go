package usecase

import (
	"testing"

	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/valueobject"
)

func mustAccount(t *testing.T, id, balance, sequence int64) aggregate.Account {
	t.Helper()
	var aid valueobject.AccountID
	var err error
	if id < 0 {
		aid, err = valueobject.NewTechnicalID(id)
	} else {
		aid, err = valueobject.NewClientID(id)
	}
	if err != nil {
		t.Fatal(err)
	}
	bal, err := valueobject.NewBalance(balance)
	if err != nil {
		t.Fatal(err)
	}
	account, err := aggregate.NewAccount(aid, bal, sequence, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func put(m *memory, accounts ...aggregate.Account) {
	for _, account := range accounts {
		m.accounts[account.ID().Int64()] = account
	}
}
