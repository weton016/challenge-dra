package usecase

import (
	"context"
	"errors"
	"testing"

	"dracma/pix/internal/application/dto"
	"dracma/pix/internal/application/port"
	"dracma/pix/internal/domain/errs"
)

func TestReceiveAndReceipt(t *testing.T) {
	ctx := context.Background()
	store := newMemory()
	var _ port.ReceiptStore = store
	receive := ReceiveExternal{Receipts: store}
	if _, err := receive.Execute(ctx, dto.TransferCommand{ID: "in", Source: 1, Destination: 2, Amount: 1}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := receive.Execute(ctx, dto.TransferCommand{}); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	approved, err := receive.Execute(ctx, dto.TransferCommand{ID: "ext", Source: 1, External: "shop", Amount: 5})
	if err != nil || approved.Decision != "APPROVED" || approved.Settlement != "SETTLED" || approved.Reason != "" {
		t.Fatal(err, approved)
	}
	again, err := receive.Execute(ctx, dto.TransferCommand{ID: "ext", Source: 1, External: "shop", Amount: 5})
	if err != nil || again != approved {
		t.Fatal(err, again)
	}
	if _, err = receive.Execute(ctx, dto.TransferCommand{ID: "ext", Source: 1, External: "shop", Amount: 6}); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	rejected, err := receive.Execute(ctx, dto.TransferCommand{ID: "no", Source: 1, External: "reject:shop", Amount: 5})
	if err != nil || rejected.Decision != "DECLINED" || rejected.Settlement != "SETTLED" {
		t.Fatal(err, rejected)
	}
	store.receiptErr = errors.New("receipt read")
	if _, err = receive.Execute(ctx, dto.TransferCommand{ID: "new", Source: 1, External: "shop", Amount: 1}); err == nil {
		t.Fatal("read")
	}
	store.receiptErr = nil
	store.receiptSaveErr = errors.New("receipt write")
	if _, err = receive.Execute(ctx, dto.TransferCommand{ID: "new", Source: 1, External: "shop", Amount: 1}); err == nil {
		t.Fatal("write")
	}

	receipt := GetReceipt{Receipts: store}
	got, err := receipt.Execute(ctx, "ext")
	if err != nil || got != approved {
		t.Fatal(err, got)
	}
	if _, err = receipt.Execute(ctx, "bad id"); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = receipt.Execute(ctx, "absent"); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	store.receiptErr = errors.New("receipt read")
	if _, err = receipt.Execute(ctx, "ext"); err == nil {
		t.Fatal("get receipt")
	}
}
