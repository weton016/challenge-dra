package integration

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/application/usecase"
	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/repository"
	"dracma/pix/internal/domain/service"
	"dracma/pix/internal/domain/valueobject"
	"dracma/pix/internal/infrastructure/postgres"
	"dracma/pix/internal/infrastructure/postgres/account"
	"dracma/pix/internal/infrastructure/postgres/receipt"
	"dracma/pix/internal/infrastructure/postgres/statement"
	"dracma/pix/internal/infrastructure/postgres/transfer"
)

func testStore(t *testing.T) *postgres.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for PostgreSQL integration")
	}
	if !strings.Contains(url, "/pix_test?") {
		t.Fatal("tests require isolated pix_test database")
	}
	store, err := postgres.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Pool.Close)
	if err = store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Pool.Exec(context.Background(), `TRUNCATE outbox,entries,operations,external_receipts,accounts CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err = store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = store.Seed(context.Background(), 3, 100); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestPortsAgainstPostgres(t *testing.T) {
	ctx := context.Background()
	db := testStore(t)
	accounts := account.New(db)
	transfers := transfer.New(db)
	statements := statement.New(db)
	receipts := receipt.New(db)
	var (
		_ port.TransferStore  = transfers
		_ port.AccountStore   = accounts
		_ port.StatementStore = statements
		_ port.ReceiptStore   = receipts
	)
	authorize := usecase.AuthorizeTransfer{Transfers: transfers, Accounts: accounts}
	approved, err := authorize.Execute(ctx, dto.TransferCommand{ID: "pay", Source: 1, Destination: 2, Amount: 40})
	if err != nil || approved.Decision != "APPROVED" || approved.Settlement != "INTERNAL" {
		t.Fatal(err, approved)
	}
	again, err := authorize.Execute(ctx, dto.TransferCommand{ID: "pay", Source: 1, Destination: 2, Amount: 40})
	if err != nil || again != approved {
		t.Fatal(err, again)
	}
	if _, err = authorize.Execute(ctx, dto.TransferCommand{ID: "pay", Source: 1, Destination: 2, Amount: 1}); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	external, err := authorize.Execute(ctx, dto.TransferCommand{ID: "out", Source: 3, External: "shop", Amount: 5})
	if err != nil || external.Settlement != "PENDING" {
		t.Fatal(err, external)
	}
	exact, err := authorize.Execute(ctx, dto.TransferCommand{ID: "exact", Source: 1, Destination: 2, Amount: 60})
	if err != nil || exact.Decision != "APPROVED" {
		t.Fatal(err, exact)
	}
	denied, err := authorize.Execute(ctx, dto.TransferCommand{ID: "poor", Source: 1, Destination: 2, Amount: 1})
	if err != nil || denied.Reason != "INSUFFICIENT_FUNDS" {
		t.Fatal(err, denied)
	}
	missing, err := authorize.Execute(ctx, dto.TransferCommand{ID: "gone", Source: 1, Destination: 99, Amount: 1})
	if err != nil || missing.Reason != "ACCOUNT_NOT_FOUND" {
		t.Fatal(err, missing)
	}
	if _, err = transfers.FindTransfer(ctx, mustID(t, "absent")); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = accounts.FindAccount(ctx, mustClient(t, 99)); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	technical, err := accounts.FindAccount(ctx, mustTechnical(t, -1))
	if err != nil || !technical.ID().IsTechnical() {
		t.Fatal(err)
	}
	balance, err := (usecase.GetBalance{Accounts: accounts}).Execute(ctx, 1)
	if err != nil || balance.ID != 1 || balance.Balance != 0 {
		t.Fatal(err, balance)
	}
	page, err := (usecase.GetStatement{Accounts: accounts, Statements: statements}).Execute(ctx, 1, 0, 0)
	if err != nil || page.Until < 1 || len(page.Entries) < 1 || page.Entries[0].Delta == 0 {
		t.Fatal(err, page)
	}
	if _, err = db.Pool.Exec(ctx, `UPDATE operations SET payload='"bad"' WHERE id='pay'`); err != nil {
		t.Fatal(err)
	}
	if _, err = transfers.FindTransfer(ctx, mustID(t, "pay")); err == nil {
		t.Fatal("corrupt payload")
	}

	receive := usecase.ReceiveExternal{Receipts: receipts}
	saved, err := receive.Execute(ctx, dto.TransferCommand{ID: "ext", Source: 1, External: "reject:shop", Amount: 3})
	if err != nil || saved.Decision != "DECLINED" || saved.Settlement != "SETTLED" {
		t.Fatal(err, saved)
	}
	got, err := (usecase.GetReceipt{Receipts: receipts}).Execute(ctx, "ext")
	if err != nil || got != saved {
		t.Fatal(err, got)
	}
	if _, err = receipts.FindReceipt(ctx, mustID(t, "absent")); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, `UPDATE external_receipts SET payload='"bad"' WHERE id='ext'`); err != nil {
		t.Fatal(err)
	}
	if _, err = receipts.FindReceipt(ctx, mustID(t, "ext")); err == nil {
		t.Fatal("corrupt receipt")
	}
	mismatch, err := aggregate.ParseTransfer("mismatch", 1, 0, "reject:shop", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = receipts.SaveReceipt(ctx, mismatch, dto.TransferResult{ID: "mismatch", Decision: "APPROVED", Settlement: "SETTLED"}); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	if _, err = statements.FindStatement(ctx, mustClient(t, 99), 0, 0); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, `INSERT INTO operations(id,payload,decision) VALUES ('bad id','{}','APPROVED')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, `INSERT INTO entries(operation_id,account_id,delta_minor,account_sequence,balance_after_minor) VALUES ('bad id',1,-1,100,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err = statements.FindStatement(ctx, mustClient(t, 1), 99, 100); err == nil {
		t.Fatal("bad entry id")
	}
	parsed, err := aggregate.ParseTransfer("forged", 1, 2, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	forged, err := aggregate.RestoreTransfer(parsed, valueobject.Approved, "", valueobject.SettlementInternal)
	if err != nil {
		t.Fatal(err)
	}
	if err = transfers.Commit(ctx, service.Authorization{Transfer: forged}); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
}

func TestDomainRepositories(t *testing.T) {
	ctx := context.Background()
	db := testStore(t)
	accounts := account.New(db)
	transfers := transfer.New(db)
	statements := statement.New(db)
	receipts := receipt.New(db)
	var (
		_ repository.AccountRepository   = accounts
		_ repository.TransferRepository  = transfers
		_ repository.StatementRepository = statements
	)
	current, err := accounts.FindAccount(ctx, mustClient(t, 3))
	if err != nil || current.Balance().Minor() != 100 {
		t.Fatal(err, current.Balance().Minor())
	}
	if err = accounts.SaveAccount(ctx, current); err != nil {
		t.Fatal(err)
	}
	balance, err := valueobject.NewBalance(1)
	if err != nil {
		t.Fatal(err)
	}
	forged, err := aggregate.NewAccount(current.ID(), balance, 0, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	if err = accounts.SaveAccount(ctx, forged); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}
	again, err := accounts.FindAccount(ctx, current.ID())
	if err != nil || again.Balance().Minor() != 100 {
		t.Fatal(err, again.Balance().Minor())
	}
	missing, err := aggregate.NewAccount(mustClient(t, 99), balance, 0, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	if err = accounts.SaveAccount(ctx, missing); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}

	declined, err := aggregate.ParseTransfer("denied-save", 3, 1, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = transfers.SaveTransfer(ctx, declined); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	declined, err = aggregate.RestoreTransfer(declined, valueobject.Declined, valueobject.ReasonInsufficientFunds, valueobject.SettlementInternal)
	if err != nil {
		t.Fatal(err)
	}
	if err = transfers.SaveTransfer(ctx, declined); err != nil {
		t.Fatal(err)
	}
	if err = transfers.SaveTransfer(ctx, declined); err != nil {
		t.Fatal(err)
	}
	found, err := transfers.FindTransfer(ctx, declined.ID())
	if err != nil || found.Decision().String() != "DECLINED" || found.Reason().String() != "INSUFFICIENT_FUNDS" {
		t.Fatal(err, found.Decision(), found.Reason())
	}
	other, err := aggregate.ParseTransfer("denied-save", 3, 2, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	other, err = aggregate.RestoreTransfer(other, valueobject.Declined, valueobject.ReasonInsufficientFunds, valueobject.SettlementInternal)
	if err != nil {
		t.Fatal(err)
	}
	if err = transfers.SaveTransfer(ctx, other); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	approved, err := aggregate.ParseTransfer("naked", 3, 1, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	approved, err = aggregate.RestoreTransfer(approved, valueobject.Approved, "", valueobject.SettlementInternal)
	if err != nil {
		t.Fatal(err)
	}
	if err = transfers.SaveTransfer(ctx, approved); !errors.Is(err, errs.ErrInvariant) {
		t.Fatal(err)
	}

	authorize := usecase.AuthorizeTransfer{Transfers: transfers, Accounts: accounts}
	if _, err = authorize.Execute(ctx, dto.TransferCommand{ID: "posted", Source: 3, Destination: 1, Amount: 10}); err != nil {
		t.Fatal(err)
	}
	posted, err := transfers.FindTransfer(ctx, mustID(t, "posted"))
	if err != nil {
		t.Fatal(err)
	}
	if err = transfers.SaveTransfer(ctx, posted); err != nil {
		t.Fatal(err)
	}
	page, err := statements.FindStatement(ctx, mustClient(t, 3), 0, 0)
	if err != nil || len(page.Entries()) != 1 {
		t.Fatal(err, len(page.Entries()))
	}
	if _, err = db.Pool.Exec(ctx, `UPDATE operations SET payload='"bad"' WHERE id='denied-save'`); err != nil {
		t.Fatal(err)
	}
	if err = transfers.SaveTransfer(ctx, declined); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	local, err := aggregate.ParseTransfer("local", 1, 2, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = receipts.SaveReceipt(ctx, local, dto.TransferResult{ID: "local", Decision: "APPROVED", Settlement: "SETTLED"}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestRepositoriesClosedPool(t *testing.T) {
	ctx := context.Background()
	db := testStore(t)
	accounts := account.New(db)
	transfers := transfer.New(db)
	statements := statement.New(db)
	receipts := receipt.New(db)
	db.Pool.Close()
	if _, err := accounts.FindAccount(ctx, mustClient(t, 1)); err == nil {
		t.Fatal("find account")
	}
	current, err := aggregate.ParseTransfer("closed", 1, 2, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := valueobject.NewBalance(100)
	if err != nil {
		t.Fatal(err)
	}
	accountRow, err := aggregate.NewAccount(mustClient(t, 1), balance, 0, valueobject.BRL())
	if err != nil {
		t.Fatal(err)
	}
	if err = accounts.SaveAccount(ctx, accountRow); err == nil {
		t.Fatal("save account")
	}
	if _, err = transfers.FindTransfer(ctx, mustID(t, "closed")); err == nil {
		t.Fatal("find transfer")
	}
	declined, err := aggregate.RestoreTransfer(current, valueobject.Declined, valueobject.ReasonInsufficientFunds, valueobject.SettlementInternal)
	if err != nil {
		t.Fatal(err)
	}
	if err = transfers.SaveTransfer(ctx, declined); err == nil {
		t.Fatal("save transfer")
	}
	if err = transfers.Commit(ctx, service.Authorization{Transfer: declined}); err == nil {
		t.Fatal("commit")
	}
	if _, err = statements.FindStatement(ctx, mustClient(t, 1), 0, 0); err == nil {
		t.Fatal("statement")
	}
	if _, err = receipts.FindReceipt(ctx, mustID(t, "closed")); err == nil {
		t.Fatal("find receipt")
	}
}

func mustID(t *testing.T, raw string) valueobject.TransactionID {
	t.Helper()
	id, err := valueobject.NewTransactionID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustClient(t *testing.T, id int64) valueobject.AccountID {
	t.Helper()
	parsed, err := valueobject.NewClientID(id)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func mustTechnical(t *testing.T, id int64) valueobject.AccountID {
	t.Helper()
	parsed, err := valueobject.NewTechnicalID(id)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
