package policy

import (
	"math"
	"testing"

	"dracma/pix/internal/domain/valueobject"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name                        string
		source, destination, amount int64
		want                        valueobject.Reason
	}{
		{"exact balance", 100, 0, 100, valueobject.ReasonNone},
		{"less than balance", 100, 50, 99, valueobject.ReasonNone},
		{"insufficient", 99, 0, 100, valueobject.ReasonInsufficientFunds},
		{"empty", 0, 0, 1, valueobject.ReasonInsufficientFunds},
		{"zero", 1, 0, 0, valueobject.ReasonInvalidAmount},
		{"negative amount", 1, 0, -1, valueobject.ReasonInvalidAmount},
		{"negative source", -1, 0, 1, valueobject.ReasonInvalidAmount},
		{"negative destination", 1, -1, 1, valueobject.ReasonInvalidAmount},
		{"overflow", 2, math.MaxInt64, 1, valueobject.ReasonBalanceOverflow},
		{"exact upper bound", 2, math.MaxInt64 - 1, 1, valueobject.ReasonNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Decide(c.source, c.destination, c.amount); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestTarget(t *testing.T) {
	if Target(32, 99, false) != 99 {
		t.Fatal("internal target")
	}
	if Target(1, 0, true) != -2 {
		t.Fatal("technical account")
	}
	if Target(32, 0, true) != -1 {
		t.Fatal("shard boundary")
	}
}
