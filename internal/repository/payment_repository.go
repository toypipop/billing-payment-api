package repository

import (
	"context"
	"errors"
	"time"

	"billing-payment-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentRepository struct{ db *gorm.DB }

func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Allocate(ctx context.Context, unitNumber string, amountCents int64, key string) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if key != "" {
			// Lock the key before the unit, including when competing requests name
			// different units. Transaction-scoped locks release on commit/rollback.
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "billing-payment:"+key).Error; err != nil {
				return err
			}
			err := tx.Preload("Unit").Preload("Allocations", func(db *gorm.DB) *gorm.DB {
				return db.Order("id ASC")
			}).Preload("Allocations.Invoice").Where("idempotency_key = ?", key).First(&payment).Error
			if err == nil {
				if payment.Unit.UnitNumber != unitNumber || payment.AmountCents != amountCents {
					return model.ErrIdempotencyConflict
				}
				// Return the original allocation balances, even after later payments.
				for i := range payment.Allocations {
					payment.Allocations[i].Invoice.PaidAmountCents = payment.Allocations[i].PaidAmountAfterCents
				}
				payment.Replayed = true
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		// Serialize payments for this unit. Invoice creation locks the same unit
		// through its upsert, so new invoices cannot appear midway through allocation.
		var unit model.Unit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("unit_number = ?", unitNumber).First(&unit).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrUnitNotFound
			}
			return err
		}
		var invoices []model.Invoice
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("unit_id = ? AND paid_amount_cents < total_amount_cents", unit.ID).
			Order("due_date ASC").Order(`invoice_number COLLATE "C" ASC`).Find(&invoices).Error; err != nil {
			return err
		}
		allocations, err := model.PlanPayment(invoices, amountCents)
		if err != nil {
			return err
		}

		payment = model.Payment{UnitID: unit.ID, AmountCents: amountCents, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
		if key != "" {
			payment.IdempotencyKey = &key
		}
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}
		for i := range allocations {
			allocation := &allocations[i]
			allocation.PaymentID = payment.ID
			allocation.PaidAmountAfterCents = allocation.Invoice.PaidAmountCents
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
