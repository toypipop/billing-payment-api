package repository

import (
	"context"
	"errors"

	"billing-payment-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNoOutstandingInvoices = errors.New("unit has no outstanding invoices")
	ErrOverpayment           = errors.New("payment exceeds unit outstanding balance; no payment was recorded")
)

type PaymentRepository struct{ db *gorm.DB }

func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Allocate(ctx context.Context, unitNumber string, amountCents int64) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize payments for this unit. Invoice creation locks the same unit
		// through its upsert, so new invoices cannot appear midway through allocation.
		var unit model.Unit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("unit_number = ?", unitNumber).First(&unit).Error; err != nil {
			return err
		}
		var invoices []model.Invoice
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("unit_id = ? AND paid_amount_cents < total_amount_cents", unit.ID).
			Order("due_date ASC").Order(`invoice_number COLLATE "C" ASC`).Find(&invoices).Error; err != nil {
			return err
		}
		if len(invoices) == 0 {
			return ErrNoOutstandingInvoices
		}

		// Plan before writing. Subtract from remaining instead of summing all
		// balances, which could overflow int64 across many invoices.
		remaining := amountCents
		allocations := make([]model.PaymentAllocation, 0)
		for _, invoice := range invoices {
			if remaining == 0 {
				break
			}
			allocated := min(remaining, invoice.TotalAmountCents-invoice.PaidAmountCents)
			invoice.PaidAmountCents += allocated
			allocations = append(allocations, model.PaymentAllocation{
				InvoiceID: invoice.ID, AmountCents: allocated, Invoice: invoice,
			})
			remaining -= allocated
		}
		if remaining > 0 {
			return ErrOverpayment
		}

		payment = model.Payment{UnitID: unit.ID, AmountCents: amountCents}
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}
		for i := range allocations {
			allocation := &allocations[i]
			allocation.PaymentID = payment.ID
			if err := tx.Model(&model.Invoice{}).Where("id = ?", allocation.InvoiceID).
				Update("paid_amount_cents", allocation.Invoice.PaidAmountCents).Error; err != nil {
				return err
			}
			if err := tx.Omit(clause.Associations).Create(allocation).Error; err != nil {
				return err
			}
		}
		payment.Unit = unit
		payment.Allocations = allocations
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &payment, nil
}
