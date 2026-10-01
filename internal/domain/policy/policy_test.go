package policy

import (
	"math"
	"math/big"
	"testing"
)

func TestAvailable(t *testing.T) {
	n := func(v int64) *big.Int { return big.NewInt(v) }
	huge := new(big.Int).Lsh(big.NewInt(1), 100)
	cases := []struct {
		name                     string
		credits, debits, pending *big.Int
		want                     int64
		ok                       bool
	}{
		{"exact balance spent", n(100), n(100), n(0), 0, true},
		{"reserved counts", n(100), n(40), n(60), 0, true},
		{"one cent left", n(100), n(40), n(59), 1, true},
		{"posted debit", n(100), n(1), n(0), 99, true},
		{"overdrawn", n(100), n(100), n(1), 0, false},
		{"upper bound", n(math.MaxInt64), n(0), n(0), math.MaxInt64, true},
		{"above int64", new(big.Int).Add(n(math.MaxInt64), n(1)), n(0), n(0), 0, false},
		{"large history small balance", new(big.Int).Add(huge, n(7)), huge, n(0), 7, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Available(c.credits, c.debits, c.pending)
			if got != c.want || ok != c.ok {
				t.Fatalf("got %d %v want %d %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestDelta(t *testing.T) {
	cases := []struct {
		movement Movement
		debit    bool
		want     int64
	}{
		{Posted, true, -5},
		{Posted, false, 5},
		{Reserved, true, -5},
		{Reserved, false, 0},
		{Captured, true, 0},
		{Captured, false, 5},
		{Released, true, 5},
		{Released, false, 0},
		{Movement(9), true, 0},
	}
	for _, c := range cases {
		if got := Delta(c.movement, c.debit, 5); got != c.want {
			t.Fatalf("movement %d debit %v: got %d want %d", c.movement, c.debit, got, c.want)
		}
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
