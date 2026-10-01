package usecase

import (
	"context"
	"errors"
	"testing"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/errs"
)

const idReject = "00000000-0000-4000-8000-000000000003"

func TestReceiveAndReceipt(t *testing.T) {
	ctx := context.Background()
	store := newMemory()
	var _ port.ReceiptStore = store
	receive := ReceiveExternal{Receipts: store}
	if _, err := receive.Execute(ctx, dto.TransferCommand{ID: idOK, Source: 1, Destination: 2, Amount: 1}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := receive.Execute(ctx, dto.TransferCommand{}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	approved, err := receive.Execute(ctx, dto.TransferCommand{ID: idOut, Source: 1, External: "shop", Amount: 5})
	if err != nil || approved != (dto.TransferResult{ID: idOut, Decision: "APPROVED", Settlement: "SETTLED"}) {
		t.Fatal(err, approved)
	}
	again, err := receive.Execute(ctx, dto.TransferCommand{ID: idOut, Source: 1, External: "shop", Amount: 5})
	if err != nil || again != approved {
		t.Fatal(err, again)
	}
	if _, err = receive.Execute(ctx, dto.TransferCommand{ID: idOut, Source: 1, External: "shop", Amount: 6}); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	rejected, err := receive.Execute(ctx, dto.TransferCommand{ID: idReject, Source: 1, External: "reject:shop", Amount: 5})
	if err != nil || rejected.Decision != "DECLINED" || rejected.Settlement != "SETTLED" {
		t.Fatal(err, rejected)
	}

	receipt := GetReceipt{Receipts: store}
	got, err := receipt.Execute(ctx, idOut)
	if err != nil || got != approved {
		t.Fatal(err, got)
	}
	if _, err = receipt.Execute(ctx, "bad id"); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = receipt.Execute(ctx, idOK); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	store.receiptErr = errors.New("receipt store")
	if _, err = receipt.Execute(ctx, idOut); err == nil {
		t.Fatal("get receipt")
	}
	if _, err = receive.Execute(ctx, dto.TransferCommand{ID: idOut, Source: 1, External: "shop", Amount: 5}); err == nil {
		t.Fatal("save receipt")
	}
}
