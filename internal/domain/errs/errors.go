// Package errs holds the sentinel failures of the Pix domain.
package errs

import "errors"

var (
	ErrInvalid   = errors.New("invalid request")
	ErrConflict  = errors.New("transaction id reused with different payload")
	ErrNotFound  = errors.New("not found")
	ErrInvariant = errors.New("financial invariant failed")
)
