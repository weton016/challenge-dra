package entity

import "dracma/pix/internal/domain/errs"

type Statement struct {
	entries []Entry
	until   int64
	next    int64
}

func NewStatement(entries []Entry, until, next int64) (Statement, error) {
	if until < 0 {
		return Statement{}, errs.ErrInvalid
	}
	if next < 0 {
		return Statement{}, errs.ErrInvalid
	}
	copied := make([]Entry, len(entries))
	copy(copied, entries)
	return Statement{entries: copied, until: until, next: next}, nil
}

func (s Statement) Entries() []Entry {
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

func (s Statement) Until() int64 { return s.until }

func (s Statement) Next() int64 { return s.next }
