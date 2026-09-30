package errs

import (
	"errors"
	"testing"
)

func TestSentinels(t *testing.T) {
	all := []error{ErrInvalid, ErrConflict, ErrNotFound, ErrInvariant}
	seen := map[string]struct{}{}
	for i, err := range all {
		if err == nil || err.Error() == "" {
			t.Fatal("empty sentinel")
		}
		if _, ok := seen[err.Error()]; ok {
			t.Fatal("duplicate message")
		}
		seen[err.Error()] = struct{}{}
		for j := i + 1; j < len(all); j++ {
			if errors.Is(err, all[j]) {
				t.Fatalf("%v overlaps %v", err, all[j])
			}
		}
	}
}
