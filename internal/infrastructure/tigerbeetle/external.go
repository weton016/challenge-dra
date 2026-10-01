package tigerbeetle

import (
	"context"
	"encoding/binary"
	"fmt"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

// memoBytes is what one zero-amount transfer carries in its user data
// (16 + 8 + 4). Five memos hold the 140-byte external reference.
const memoBytes = 28

func memos(id tb.Uint128, ref string) []tb.Transfer {
	out := []tb.Transfer{}
	linked := tb.TransferFlags{Linked: true}.ToUint16()
	for i := 0; i*memoBytes < len(ref); i++ {
		var chunk [memoBytes]byte
		copy(chunk[:], ref[i*memoBytes:])
		out = append(out, tb.Transfer{
			ID:              memoID(id, i),
			DebitAccountID:  memoDebit,
			CreditAccountID: memoCredit,
			UserData128:     tb.BytesToUint128([16]byte(chunk[:16])),
			UserData64:      binary.LittleEndian.Uint64(chunk[16:24]),
			UserData32:      binary.LittleEndian.Uint32(chunk[24:]),
			Ledger:          ledgerBRL,
			Code:            codeMemo,
			Flags:           linked,
		})
	}
	out[len(out)-1].Flags = 0
	return out
}

func memoID(id tb.Uint128, i int) tb.Uint128 { return derive(fmt.Sprintf("memo%d", i), id) }

func memoCount(head tb.Transfer) int { return (int(head.UserData32) + memoBytes - 1) / memoBytes }

func readRef(head tb.Transfer, found map[tb.Uint128]tb.Transfer) (string, error) {
	ref := make([]byte, 0, memoCount(head)*memoBytes)
	for i := range memoCount(head) {
		memo, ok := found[memoID(head.ID, i)]
		if !ok {
			return "", errs.ErrInvariant
		}
		data := memo.UserData128.Bytes()
		ref = append(ref, data[:]...)
		ref = binary.LittleEndian.AppendUint64(ref, memo.UserData64)
		ref = binary.LittleEndian.AppendUint32(ref, memo.UserData32)
	}
	return string(ref[:head.UserData32]), nil
}

// Pending returns the next page of external reservations with no
// settlement yet. Each sweep reads in commit order to the end, then the
// next sweep restarts at the oldest one still open, so a delivery that
// keeps failing is retried without hiding the ones after it. A restart
// sweeps from the beginning.
func (l *Ledger) Pending(ctx context.Context, limit int) ([]dto.TransferCommand, error) {
	l.scan.Lock()
	defer l.scan.Unlock()
	filter := tb.QueryFilter{Ledger: ledgerBRL, Code: codeExternal, TimestampMin: l.scan.cursor, Limit: uint32(limit)}
	page, err := call(ctx, func() ([]tb.Transfer, error) { return l.client.QueryTransfers(filter) })
	if err != nil {
		return nil, err
	}
	if len(page) == 0 {
		if l.scan.oldest != 0 {
			l.scan.cursor, l.scan.oldest = l.scan.oldest, 0
		}
		return nil, nil
	}
	open, err := l.unsettled(ctx, page)
	if err != nil {
		return nil, err
	}
	if len(open) > 0 && l.scan.oldest == 0 {
		l.scan.oldest = open[0].Timestamp
	}
	l.scan.cursor = page[len(page)-1].Timestamp + 1
	return l.commands(ctx, open)
}

func (l *Ledger) unsettled(ctx context.Context, page []tb.Transfer) ([]tb.Transfer, error) {
	heads, ids := []tb.Transfer{}, []tb.Uint128{}
	for _, t := range page {
		if t.TransferFlags().Pending {
			heads = append(heads, t)
			ids = append(ids, derive("settle", t.ID))
		}
	}
	settled, err := l.lookup(ctx, ids...)
	if err != nil {
		return nil, err
	}
	open := []tb.Transfer{}
	for i, head := range heads {
		if _, ok := settled[ids[i]]; !ok {
			open = append(open, head)
		}
	}
	return open, nil
}

func (l *Ledger) commands(ctx context.Context, heads []tb.Transfer) ([]dto.TransferCommand, error) {
	ids := []tb.Uint128{}
	for _, head := range heads {
		for i := range memoCount(head) {
			ids = append(ids, memoID(head.ID, i))
		}
	}
	found, err := l.lookup(ctx, ids...)
	if err != nil {
		return nil, err
	}
	out := make([]dto.TransferCommand, 0, len(heads))
	for _, head := range heads {
		ref, err := readRef(head, found)
		if err != nil {
			return nil, err
		}
		amount, err := minorOf(head.Amount)
		if err != nil {
			return nil, err
		}
		out = append(out, dto.TransferCommand{ID: uuidOf(head.ID), Source: clientOf(head.DebitAccountID), External: ref, Amount: amount})
	}
	return out, nil
}

// Settle posts or voids the reservation under an id derived from it.
// TigerBeetle resolves a pending transfer at most once, so a repeated or
// concurrent settlement cannot refund twice.
func (l *Ledger) Settle(ctx context.Context, id string, accepted bool) error {
	tx, err := valueobject.NewTransactionID(id)
	if err != nil {
		return err
	}
	pending := transferID(tx)
	settle := tb.Transfer{ID: derive("settle", pending), PendingID: pending, Ledger: ledgerBRL, Code: codeExternal}
	settle.Flags = tb.TransferFlags{VoidPendingTransfer: true}.ToUint16()
	if accepted {
		settle.Amount = tb.AmountMax
		settle.Flags = tb.TransferFlags{PostPendingTransfer: true}.ToUint16()
	}
	results, err := l.create(ctx, []tb.Transfer{settle})
	if err != nil {
		return err
	}
	if s := results[0].Status; s != tb.TransferCreated && s != tb.TransferExists {
		return fmt.Errorf("settle %s: %s: %w", id, s, errs.ErrInvariant)
	}
	return nil
}
