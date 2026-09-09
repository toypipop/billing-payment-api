package model

import (
	"fmt"
	"sort"
)

// PlanPayment calculates allocations without changing the supplied invoices.
// Call it with the unit's locked invoices before saving the plan in one transaction.
func PlanPayment(invoices []Invoice, amountCents int64) ([]PaymentAllocation, error) {
	if amountCents <= 0 {
		return nil, fmt.Errorf("payment amount must be greater than zero")
	}
	outstanding := make([]Invoice, 0, len(invoices))
	for _, invoice := range invoices {
		if invoice.Outstanding() > 0 {
			outstanding = append(outstanding, invoice)
		}
	}
	if len(outstanding) == 0 {
		return nil, ErrNoOutstandingInvoices
	}
	sort.Slice(outstanding, func(i, j int) bool {
		if outstanding[i].DueDate.Equal(outstanding[j].DueDate) {
			return outstanding[i].InvoiceNumber < outstanding[j].InvoiceNumber
		}
		return outstanding[i].DueDate.Before(outstanding[j].DueDate)
	})
	remaining := amountCents
	allocations := make([]PaymentAllocation, 0)
	for _, invoice := range outstanding {
		if remaining == 0 {
			break
		}
		allocated := min(remaining, invoice.Outstanding())
		invoice.PaidAmountCents += allocated
		allocations = append(allocations, PaymentAllocation{
			InvoiceID: invoice.ID, AmountCents: allocated, Invoice: invoice,
		})
		remaining -= allocated
	}
	// Subtracting avoids overflowing a sum of balances across many invoices.
	if remaining > 0 {
		return nil, ErrOverpayment
	}
	return allocations, nil
}
