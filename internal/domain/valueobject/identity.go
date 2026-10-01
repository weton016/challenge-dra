package valueobject

import (
	"regexp"
	"strings"

	"dracma/pix/internal/domain/errs"
)

// RFC 4122 variant only: the ledger derives its own ids with variant bits 11,
// so a client id can never take the id of a settlement or decline record.
var transactionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type TransactionID struct{ value string }

// NewTransactionID accepts a UUID in any case and keeps it lowercase.
func NewTransactionID(raw string) (TransactionID, error) {
	value := strings.ToLower(raw)
	if !transactionIDPattern.MatchString(value) {
		return TransactionID{}, errs.ErrInvalid
	}
	return TransactionID{value: value}, nil
}

func RefundID(original TransactionID) (TransactionID, error) {
	if original.value == "" || strings.Contains(original.value, ":") {
		return TransactionID{}, errs.ErrInvalid
	}
	return TransactionID{value: "refund:" + original.value}, nil
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
