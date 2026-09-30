package valueobject

import "dracma/pix/internal/domain/errs"

type Amount struct{ minor int64 }

func NewAmount(minor int64) (Amount, error) {
	if minor <= 0 {
		return Amount{}, errs.ErrInvalid
	}
	return Amount{minor: minor}, nil
}

func (a Amount) Minor() int64 { return a.minor }

type Balance struct{ minor int64 }

func TryBalance(minor int64) (Balance, bool) {
	if minor < 0 {
		return Balance{}, false
	}
	return Balance{minor: minor}, true
}

func NewBalance(minor int64) (Balance, error) {
	balance, ok := TryBalance(minor)
	if !ok {
		return Balance{}, errs.ErrInvalid
	}
	return balance, nil
}

func (b Balance) Minor() int64 { return b.minor }

func Add(left, delta int64) (int64, bool) {
	sum := left + delta
	if (delta > 0 && sum < left) || (delta < 0 && sum > left) {
		return 0, false
	}
	return sum, true
}

type Currency struct{ code string }

func NewCurrency(code string) (Currency, error) {
	if code != "BRL" {
		return Currency{}, errs.ErrInvalid
	}
	return Currency{code: code}, nil
}

func BRL() Currency { return Currency{code: "BRL"} }

func (c Currency) String() string { return c.code }

func (c Currency) Equal(other Currency) bool { return c.code == other.code }
