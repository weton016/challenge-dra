package codec

import (
	"encoding/json"
	"errors"
	"testing"

	"dracma/pix/internal/domain/aggregate"
	"dracma/pix/internal/domain/errs"
	"dracma/pix/internal/domain/valueobject"
)

func TestRequestOf(t *testing.T) {
	internal, err := aggregate.ParseTransfer("abc_123-A", 1, 2, "", 40)
	if err != nil {
		t.Fatal(err)
	}
	got := RequestOf(internal)
	if got.ID != "abc_123-A" || got.Source != 1 || got.Destination != 2 || got.External != "" || got.Amount != 40 {
		t.Fatal(got)
	}
	external, err := aggregate.ParseTransfer("out", 32, 0, "shop", 1)
	if err != nil {
		t.Fatal(err)
	}
	got = RequestOf(external)
	if got.ID != "out" || got.Source != 32 || got.Destination != 0 || got.External != "shop" || got.Amount != 1 {
		t.Fatal(got)
	}
	if err = got.Validate(); err != nil || got.Target() != -1 {
		t.Fatal(err, got.Target())
	}
}

func TestDecodeTransfer(t *testing.T) {
	internal, err := aggregate.ParseTransfer("keep", 1, 2, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(RequestOf(internal))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeTransfer(payload, string(valueobject.Declined), "INSUFFICIENT_FUNDS", string(valueobject.SettlementInternal))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ID().String() != "keep" || decoded.Decision().String() != "DECLINED" || decoded.Reason().String() != "INSUFFICIENT_FUNDS" || decoded.Settlement().String() != "INTERNAL" {
		t.Fatal(decoded.Decision(), decoded.Reason(), decoded.Settlement())
	}
	external, err := aggregate.ParseTransfer("out", 1, 0, "shop", 5)
	if err != nil {
		t.Fatal(err)
	}
	payload, err = json.Marshal(RequestOf(external))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = DecodeTransfer(payload, string(valueobject.Approved), "", string(valueobject.SettlementPending))
	if err != nil || decoded.External().String() != "shop" || decoded.Destination().Int64() != -2 || decoded.Settlement().String() != "PENDING" {
		t.Fatal(err, decoded.Destination().Int64(), decoded.Settlement())
	}
	if _, err = DecodeTransfer([]byte("{"), string(valueobject.Approved), "", string(valueobject.SettlementInternal)); err == nil {
		t.Fatal("broken json")
	}
	if _, err = DecodeTransfer([]byte(`{"transaction_id":"bad id","source_id":1,"destination_id":2,"amount_minor":1}`), string(valueobject.Approved), "", string(valueobject.SettlementInternal)); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = DecodeTransfer(payload, "NOPE", "", string(valueobject.SettlementPending)); !errors.Is(err, errs.ErrInvalid) {
		t.Fatal(err)
	}
}
