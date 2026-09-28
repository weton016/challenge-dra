package ledger

import (
	"math"
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name                        string
		source, destination, amount int64
		want                        string
	}{
		{"exact balance", 100, 0, 100, ""},
		{"less than balance", 100, 50, 99, ""},
		{"insufficient", 99, 0, 100, "INSUFFICIENT_FUNDS"},
		{"empty", 0, 0, 1, "INSUFFICIENT_FUNDS"},
		{"zero", 1, 0, 0, "INVALID_AMOUNT"},
		{"negative amount", 1, 0, -1, "INVALID_AMOUNT"},
		{"negative source", -1, 0, 1, "INVALID_AMOUNT"},
		{"negative destination", 1, -1, 1, "INVALID_AMOUNT"},
		{"overflow", 2, math.MaxInt64, 1, "BALANCE_OVERFLOW"},
		{"exact upper bound", 2, math.MaxInt64 - 1, 1, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Decide(c.source, c.destination, c.amount); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestRequestValidation(t *testing.T) {
	valid := Request{ID: "abc_123-A", Source: 1, Destination: 2, Amount: 1}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := []Request{
		{ID: "", Source: 1, Destination: 2, Amount: 1},
		{ID: "bad id", Source: 1, Destination: 2, Amount: 1},
		{ID: strings.Repeat("a", 65), Source: 1, Destination: 2, Amount: 1},
		{ID: "a", Source: 0, Destination: 2, Amount: 1},
		{ID: "a", Source: 1, Destination: 0, Amount: 1},
		{ID: "a", Source: 1, Destination: 1, Amount: 1},
		{ID: "a", Source: 1, Destination: 2, Amount: 0},
		{ID: "a", Source: 1, Destination: 2, Amount: -1},
		{ID: "a", Source: 1, Destination: 2, External: "x", Amount: 1},
		{ID: "a", Source: 1, External: strings.Repeat("a", 141), Amount: 1},
	}
	for _, r := range invalid {
		if r.Validate() == nil {
			t.Fatalf("accepted %#v", r)
		}
	}
	valid.External = "recipient"
	valid.Destination = 0
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	if valid.Target() != -2 {
		t.Fatal("wrong technical account")
	}
	valid.Source = 32
	if valid.Target() != -1 {
		t.Fatal("wrong shard boundary")
	}
	valid.External = ""
	valid.Destination = 2
	if valid.Target() != 2 {
		t.Fatal("wrong internal target")
	}
}
