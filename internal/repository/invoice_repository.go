package repository

import (
	"billing-payment-api/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvoiceRepository struct {
	db *gorm.DB
}

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Create(invoice *model.Invoice, unitNumber string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		unit := model.Unit{UnitNumber: unitNumber}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "unit_number"}},
			DoUpdates: clause.AssignmentColumns([]string{"updated_at"}),
		}).Create(&unit).Error; err != nil {
			return err
		}
		invoice.UnitID = unit.ID
		return tx.Create(invoice).Error
	})
}

func (r *InvoiceRepository) GetAll() ([]model.Invoice, error) {
	var invoices []model.Invoice
	err := r.db.Preload("InvoiceItems", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Order("id ASC").Find(&invoices).Error
	return invoices, err
}

func (r *InvoiceRepository) GetByID(id uint) (*model.Invoice, error) {
	var invoice model.Invoice
	err := r.db.Preload("InvoiceItems").First(&invoice, id).Error
	if err != nil {
		return nil, err
	}

	return &invoice, nil
}
