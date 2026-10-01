package usecase

import (
	"context"
	"errors"
	"testing"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/errs"
)

const (
	idOK  = "00000000-0000-4000-8000-000000000001"
	idOut = "00000000-0000-4000-8000-000000000002"
)

func TestAuthorizeTransfer(t *testing.T) {
	ctx := context.Background()
	store := newMemory()
	uc := AuthorizeTransfer{Transfers: store}
	var _ port.TransferStore = store

	if _, err := uc.Execute(ctx, dto.TransferCommand{}); !errors.Is(err, errs.ErrInvalid) || store.authorized != 0 {
		t.Fatal(err, store.authorized)
	}
	if _, err := uc.Execute(ctx, dto.TransferCommand{ID: "not-a-uuid", Source: 1, Destination: 2, Amount: 1}); !errors.Is(err, errs.ErrInvalid) || store.authorized != 0 {
		t.Fatal(err, store.authorized)
	}
	approved, err := uc.Execute(ctx, dto.TransferCommand{ID: idOK, Source: 1, Destination: 2, Amount: 40})
	if err != nil || approved != (dto.TransferResult{ID: idOK, Decision: "APPROVED", Settlement: "INTERNAL"}) {
		t.Fatal(err, approved)
	}
	again, err := uc.Execute(ctx, dto.TransferCommand{ID: idOK, Source: 1, Destination: 2, Amount: 40})
	if err != nil || again != approved {
		t.Fatal(err, again)
	}
	if _, err = uc.Execute(ctx, dto.TransferCommand{ID: idOK, Source: 1, Destination: 2, Amount: 1}); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	external, err := uc.Execute(ctx, dto.TransferCommand{ID: idOut, Source: 1, External: "shop", Amount: 5})
	if err != nil || external.Settlement != "PENDING" {
		t.Fatal(err, external)
	}
	store.transferErr = errors.New("ledger unavailable")
	if _, err = uc.Execute(ctx, dto.TransferCommand{ID: idOK, Source: 1, Destination: 2, Amount: 40}); !errors.Is(err, store.transferErr) {
		t.Fatal(err)
	}
}
