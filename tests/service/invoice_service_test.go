package service_test

import (
	"context"
	"errors"
	"testing"

	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/model"
	"billing-payment-api/internal/service"
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
			store := invoiceStoreStub{invoice: model.Invoice{TotalAmountCents: tc.total, PaidAmountCents: tc.paid, Unit: model.Unit{UnitNumber: "A101"}}}
			got, err := service.NewInvoiceService(store).GetByID(context.Background(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.status || int64(got.PaidAmountTHB) != tc.paid || int64(got.OutstandingAmountTHB) != tc.outstanding || got.Unit != "A101" {
				t.Fatalf("unexpected response: %+v", got)
			}
		})
	}
}

// Exercise the public service API without a database or exposing private helpers.
type invoiceStoreStub struct {
	invoice model.Invoice
}

func (s invoiceStoreStub) GetByID(context.Context, uint) (*model.Invoice, error) {
	return &s.invoice, nil
}

func (s invoiceStoreStub) GetAll(context.Context, *string) ([]model.Invoice, error) {
	return []model.Invoice{s.invoice}, nil
}

func (s invoiceStoreStub) Create(context.Context, *model.Invoice, string) error {
	return errors.New("unexpected Create call")
}

func TestCreateInvoiceValidation(t *testing.T) {
	amount := dto.Amount(100)
	for _, req := range []dto.CreateInvoiceRequest{
		{Unit: "A101", DueDate: "2026-08-01"},
		{Unit: "A101", DueDate: "2026-08-01", Items: []dto.CreateInvoiceItemRequest{{Description: "  ", AmountTHB: &amount}}},
	} {
		if _, err := service.NewInvoiceService(nil).Create(context.Background(), req); !errors.Is(err, service.ErrInvalidInvoice) {
			t.Fatalf("expected invalid invoice, got %v", err)
		}
	}
}
