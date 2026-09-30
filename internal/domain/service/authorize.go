package service

import (
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/entity"
	"dracma/pix/internal/domain/policy"
	"dracma/pix/internal/domain/valueobject"
)

type Authorization struct {
	Transfer    aggregate.Transfer
	Source      aggregate.Account
	Destination aggregate.Account
	Entries     []entity.Entry
}

func Authorize(transfer aggregate.Transfer, source, destination aggregate.Account) (Authorization, error) {
	if err := transfer.Usable(source, destination); err != nil {
		return Authorization{}, err
	}
	reason := policy.Decide(source.Balance().Minor(), destination.Balance().Minor(), transfer.Amount().Minor())
	decided := transfer.WithReason(reason)
	if reason != valueobject.ReasonNone {
		return Authorization{Transfer: decided, Source: source, Destination: destination}, nil
	}
	debited, debit, err := source.Debit(transfer.Amount(), transfer.ID())
	if err != nil {
		return Authorization{}, err
	}
	credited, credit, err := destination.Credit(transfer.Amount(), transfer.ID())
	if err != nil {
		return Authorization{}, err
	}
	return Authorization{Transfer: decided, Source: debited, Destination: credited, Entries: []entity.Entry{debit, credit}}, nil
}
