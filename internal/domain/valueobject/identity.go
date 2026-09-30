package valueobject

import (
	"regexp"
	"strings"

	"dracma/pix/internal/domain/errs"
)

var transactionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type TransactionID struct{ value string }

func NewTransactionID(raw string) (TransactionID, error) {
	if !transactionIDPattern.MatchString(raw) {
		return TransactionID{}, errs.ErrInvalid
	}
	return TransactionID{value: raw}, nil
}

func RefundID(original TransactionID) (TransactionID, error) {
	if original.value == "" || strings.Contains(original.value, ":") {
		return TransactionID{}, errs.ErrInvalid
	}
	return TransactionID{value: "refund:" + original.value}, nil
}

// ParseTransactionID accepts a client id or the internal refund: form.
func ParseTransactionID(raw string) (TransactionID, error) {
	if id, err := NewTransactionID(raw); err == nil {
		return id, nil
	}
	const prefix = "refund:"
	if !strings.HasPrefix(raw, prefix) {
		return TransactionID{}, errs.ErrInvalid
	}
	original, err := NewTransactionID(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return TransactionID{}, err
	}
	return RefundID(original)
}

func (id TransactionID) String() string { return id.value }

type AccountID struct{ value int64 }

func NewClientID(id int64) (AccountID, error) {
	if id <= 0 {
		return AccountID{}, errs.ErrInvalid
	}
	return AccountID{value: id}, nil
}

func NewTechnicalID(id int64) (AccountID, error) {
	if id > -1 || id < -32 {
		return AccountID{}, errs.ErrInvalid
	}
	return AccountID{value: id}, nil
}

func TechnicalID(raw int64) AccountID {
	id, err := NewTechnicalID(raw)
	if err != nil {
		return AccountID{}
	}
	return id
}

func (id AccountID) Int64() int64 { return id.value }

func (id AccountID) IsClient() bool { return id.value > 0 }

func (id AccountID) IsTechnical() bool { return id.value < 0 }

type ExternalRef struct{ value string }

func NewExternalRef(raw string) (ExternalRef, error) {
	if raw == "" || len(raw) > 140 {
		return ExternalRef{}, errs.ErrInvalid
	}
	return ExternalRef{value: raw}, nil
}

func (e ExternalRef) String() string { return e.value }
