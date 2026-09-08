package repository

import (
	"billing-payment-api/internal/model"

	"gorm.io/gorm"
)

type InvoiceRepository struct {
	db *gorm.DB
}

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Create(invoice *model.Invoice) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(invoice).Error
	})
}

func (r *InvoiceRepository) GetByID(id uint) (*model.Invoice, error) {
	var invoice model.Invoice
	err := r.db.Preload("InvoiceItems").First(&invoice, id).Error
	if err != nil {
		return nil, err
	}

	return &invoice, nil
}
