package service

import (
	"context"
	"errors"
	"testing"

	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/model"
)

func TestInvoiceBalances(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		total, paid, outstanding int64
		status                   string
	}{
		{"unpaid", 180000, 0, 180000, "UNPAID"},
		{"partial", 180000, 120000, 60000, "PARTIAL"},
		{"paid", 180000, 180000, 0, "PAID"},
		{"zero", 0, 0, 0, "PAID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := toInvoiceResponse(&model.Invoice{TotalAmountCents: tc.total, PaidAmountCents: tc.paid, Unit: model.Unit{UnitNumber: "A101"}})
			if got.Status != tc.status || int64(got.PaidAmountTHB) != tc.paid || int64(got.OutstandingAmountTHB) != tc.outstanding || got.Unit != "A101" {
				t.Fatalf("unexpected response: %+v", got)
			}
		})
	}
}

func TestCreateInvoiceValidation(t *testing.T) {
	amount := dto.Amount(100)
	for _, req := range []dto.CreateInvoiceRequest{
		{Unit: "A101", DueDate: "2026-08-01"},
		{Unit: "A101", DueDate: "2026-08-01", Items: []dto.CreateInvoiceItemRequest{{Description: "  ", AmountTHB: &amount}}},
	} {
		if _, err := NewInvoiceService(nil).Create(context.Background(), req); !errors.Is(err, ErrInvalidInvoice) {
			t.Fatalf("expected invalid invoice, got %v", err)
		}
	}
}
