package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"dracma/pix/internal/domain/errs"
)

func Write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func Fail(w http.ResponseWriter, err error) {
	code, message := 503, "OUTCOME_UNKNOWN_RETRY_SAME_ID"
	switch {
	case errors.Is(err, errs.ErrInvalid):
		code, message = 400, "INVALID_REQUEST"
	case errors.Is(err, errs.ErrConflict):
		code, message = 409, "IDEMPOTENCY_CONFLICT"
	case errors.Is(err, errs.ErrNotFound):
		code, message = 404, "NOT_FOUND"
	}
	Write(w, code, map[string]string{"error": message})
}
