// Package tigerbeetle keeps every financial fact in TigerBeetle. The client
// batches concurrent requests itself, so there is no batcher or queue here.
package tigerbeetle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net"
	"slices"
	"strings"
	"sync"

	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
	tb "github.com/tigerbeetle/tigerbeetle-go"
)

const (
	ledgerBRL      = 1
	ledgerExternal = 2

	codeClient    = 1
	codeTechnical = 2
	codeSystem    = 3

	codeInternal = 1
	codeExternal = 2
	codeFunding  = 3
	codeDecline  = 4
	codeMemo     = 5
	codeAccepted = 6
	codeRejected = 7

	reasonInsufficient = 1
	reasonNotFound     = 2

	technicalShards = 32
	lookupChunk     = 200
	statementPage   = 100
)

var (
	treasury   = u128(2, 1)
	mint       = u128(2, 2)
	memoDebit  = u128(2, 3)
	memoCredit = u128(2, 4)
	simPayer   = u128(3, 1)
	simPayee   = u128(3, 2)
)

type Ledger struct {
	client tb.Client
	scan   struct {
		sync.Mutex
		cursor, oldest uint64
	}
}

// Open resolves host names because the TigerBeetle client only takes IPs.
func Open(address string) (*Ledger, error) {
	addresses := []string{}
	for _, raw := range strings.Split(address, ",") {
		host, port, err := net.SplitHostPort(strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("resolve %s: %w", host, err)
		}
		addresses = append(addresses, net.JoinHostPort(ips[0].String(), port))
	}
	client, err := tb.NewClient(tb.ToUint128(0), addresses)
	if err != nil {
		return nil, err
	}
	return &Ledger{client: client}, nil
}

func (l *Ledger) Close() { l.client.Close() }

func (l *Ledger) Ping(ctx context.Context) error {
	_, err := call(ctx, func() (struct{}, error) { return struct{}{}, l.client.Nop() })
	return err
}

// call bounds a TigerBeetle request by ctx. The client retries forever, so
// on timeout the request may still commit: callers answer "outcome unknown".
func call[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	type reply struct {
		value T
		err   error
	}
	if err := ctx.Err(); err != nil {
		var zero T
		return zero, err
	}
	done := make(chan reply, 1)
	go func() {
		value, err := fn()
		done <- reply{value, err}
	}()
	select {
	case r := <-done:
		return r.value, r.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

func u128(hi, lo uint64) tb.Uint128 {
	var b [16]byte
	for i := range 8 {
		b[i] = byte(lo >> (8 * i))
		b[8+i] = byte(hi >> (8 * i))
	}
	return tb.BytesToUint128(b)
}

func accountID(id int64) tb.Uint128 {
	if id < 0 {
		return u128(1, uint64(-id))
	}
	return u128(0, uint64(id))
}

func clientOf(id tb.Uint128) int64 {
	lo, _ := id.Uint64()
	return int64(lo)
}

func transferID(id valueobject.TransactionID) tb.Uint128 {
	raw, _ := hex.DecodeString(strings.ReplaceAll(id.String(), "-", ""))
	slices.Reverse(raw)
	return tb.BytesToUint128([16]byte(raw))
}

func uuidOf(id tb.Uint128) string {
	b := id.Bytes()
	slices.Reverse(b[:])
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// derive names the ledger's own transfers. Variant bits 11 keep them out of
// the RFC 4122 space that clients may use.
func derive(tag string, id tb.Uint128) tb.Uint128 {
	raw := id.Bytes()
	sum := sha256.Sum256(append([]byte(tag), raw[:]...))
	sum[8] |= 0xc0
	b := [16]byte(sum[:16])
	slices.Reverse(b[:])
	return tb.BytesToUint128(b)
}

func amountOf(minor int64) tb.Uint128 { return tb.ToUint128(uint64(minor)) }

func minorOf(amount tb.Uint128) (int64, error) {
	lo, hi := amount.Uint64()
	if hi != 0 || lo > math.MaxInt64 {
		return 0, errs.ErrInvariant
	}
	return int64(lo), nil
}

func createAccounts(ctx context.Context, l *Ledger, accounts []tb.Account) error {
	results, err := call(ctx, func() ([]tb.CreateAccountResult, error) { return l.client.CreateAccounts(accounts) })
	if err != nil {
		return err
	}
	for i, r := range results {
		if r.Status != tb.AccountCreated && r.Status != tb.AccountExists {
			return fmt.Errorf("account %d: %s: %w", clientOf(accounts[i].ID), r.Status, errs.ErrInvariant)
		}
	}
	return nil
}

// Bootstrap creates the technical shards and system accounts, then mints
// MaxInt64 into the treasury. That fixed supply bounds every balance to int64.
func (l *Ledger) Bootstrap(ctx context.Context) error {
	guarded := tb.AccountFlags{DebitsMustNotExceedCredits: true}.ToUint16()
	accounts := []tb.Account{
		{ID: treasury, Ledger: ledgerBRL, Code: codeSystem, Flags: guarded},
		{ID: mint, Ledger: ledgerBRL, Code: codeSystem},
		{ID: memoDebit, Ledger: ledgerBRL, Code: codeSystem},
		{ID: memoCredit, Ledger: ledgerBRL, Code: codeSystem},
		{ID: simPayer, Ledger: ledgerExternal, Code: codeSystem},
		{ID: simPayee, Ledger: ledgerExternal, Code: codeSystem},
	}
	for shard := int64(1); shard <= technicalShards; shard++ {
		accounts = append(accounts, tb.Account{ID: accountID(-shard), Ledger: ledgerBRL, Code: codeTechnical, Flags: guarded})
	}
	if err := createAccounts(ctx, l, accounts); err != nil {
		return err
	}
	supply := tb.Transfer{ID: derive("mint", treasury), DebitAccountID: mint, CreditAccountID: treasury, Amount: amountOf(math.MaxInt64), Ledger: ledgerBRL, Code: codeFunding}
	results, err := l.create(ctx, []tb.Transfer{supply})
	if err != nil {
		return err
	}
	if s := results[0].Status; s != tb.TransferCreated && s != tb.TransferExists {
		return fmt.Errorf("mint: %s: %w", s, errs.ErrInvariant)
	}
	return nil
}

func (l *Ledger) create(ctx context.Context, transfers []tb.Transfer) ([]tb.CreateTransferResult, error) {
	results, err := call(ctx, func() ([]tb.CreateTransferResult, error) { return l.client.CreateTransfers(transfers) })
	if err == nil && len(results) != len(transfers) {
		err = errs.ErrInvariant
	}
	return results, err
}

// lookup returns the transfers found, keyed by id. Chunks fit a reply in
// the development cluster's smaller messages.
func (l *Ledger) lookup(ctx context.Context, ids ...tb.Uint128) (map[tb.Uint128]tb.Transfer, error) {
	out := make(map[tb.Uint128]tb.Transfer, len(ids))
	for chunk := range slices.Chunk(ids, lookupChunk) {
		found, err := call(ctx, func() ([]tb.Transfer, error) { return l.client.LookupTransfers(chunk) })
		if err != nil {
			return nil, err
		}
		for _, t := range found {
			out[t.ID] = t
		}
	}
	return out, nil
}
