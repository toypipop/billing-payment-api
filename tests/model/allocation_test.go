package model_test

import (
	"billing-payment-api/internal/model"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestPlanPayment(t *testing.T) {
	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	invoices := []model.Invoice{
		{ID: 3, InvoiceNumber: "INV003", DueDate: due.AddDate(0, 0, 14), TotalAmountCents: 50000},
		{ID: 2, InvoiceNumber: "INV002", DueDate: due, TotalAmountCents: 50000, PaidAmountCents: 10000},
		{ID: 1, InvoiceNumber: "INV001", DueDate: due, TotalAmountCents: 100000},
		{ID: 4, InvoiceNumber: "INV000", DueDate: due, TotalAmountCents: 10000, PaidAmountCents: 10000},
	}
	before := append([]model.Invoice(nil), invoices...)
	for _, tc := range []struct {
		name    string
		amount  int64
		ids     []uint
		amounts []int64
		wantErr error
	}{
		{"partial", 50000, []uint{1}, []int64{50000}, nil},
		{"exact invoice", 100000, []uint{1}, []int64{100000}, nil},
		{"multiple", 120000, []uint{1, 2}, []int64{100000, 20000}, nil},
		{"settle all", 190000, []uint{1, 2, 3}, []int64{100000, 40000, 50000}, nil},
		{"overpayment", 190001, nil, nil, model.ErrOverpayment},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := model.PlanPayment(invoices, tc.amount)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(invoices, before) {
				t.Fatal("input invoices changed")
			}
			if len(plan) != len(tc.ids) {
				t.Fatalf("plan = %+v", plan)
			}
			for i, allocation := range plan {
				if allocation.InvoiceID != tc.ids[i] || allocation.AmountCents != tc.amounts[i] {
					t.Fatalf("allocation = %+v", allocation)
				}
				if allocation.InvoiceID == 2 && allocation.Invoice.PaidAmountCents != 10000+allocation.AmountCents {
					t.Fatal("previous payment lost")
				}
			}
		})
	}
	if _, err := model.PlanPayment(invoices[3:], 1); !errors.Is(err, model.ErrNoOutstandingInvoices) {
		t.Fatalf("paid invoices: %v", err)
	}
	if _, err := model.PlanPayment(nil, 1); !errors.Is(err, model.ErrNoOutstandingInvoices) {
		t.Fatalf("empty invoices: %v", err)
	}
	for _, amount := range []int64{0, -1} {
		if _, err := model.PlanPayment(invoices, amount); err == nil {
			t.Fatal("invalid payment accepted")
		}
	}
}

func TestPlanPaymentLargeBalances(t *testing.T) {
	plan, err := model.PlanPayment([]model.Invoice{
		{ID: 1, InvoiceNumber: "A", TotalAmountCents: math.MaxInt64},
		{ID: 2, InvoiceNumber: "B", TotalAmountCents: math.MaxInt64},
	}, math.MaxInt64)
	if err != nil || len(plan) != 1 || plan[0].AmountCents != math.MaxInt64 || plan[0].Invoice.Status() != model.InvoicePaid {
		t.Fatalf("plan = %+v, error = %v", plan, err)
	}
}
