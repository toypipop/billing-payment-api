package repository

import (
	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/model"
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvoiceRepository struct {
	db *gorm.DB
}

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Create(ctx context.Context, invoice *model.Invoice, unitNumber string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		unit := model.Unit{UnitNumber: unitNumber}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "unit_number"}},
			DoUpdates: clause.AssignmentColumns([]string{"updated_at"}),
		}).Create(&unit).Error; err != nil {
			return err
		}
		invoice.UnitID = unit.ID
		if invoice.InvoiceNumber == "" {
			var number int64
			if err := tx.Raw("SELECT nextval('invoice_number_seq')").Scan(&number).Error; err != nil {
				return err
			}
			invoice.InvoiceNumber = fmt.Sprintf("INV-%010d", number)
		}
		if err := tx.Create(invoice).Error; err != nil {
			return err
		}
		invoice.Unit = unit
		return nil
	})
}

func (r *InvoiceRepository) GetAll(ctx context.Context, unitNumber *string, pages ...dto.InvoicePage) ([]model.Invoice, error) {
	var invoices []model.Invoice
	page := dto.InvoicePage{}.Normalize()
	if len(pages) > 0 {
		page = pages[0].Normalize()
	}
	query := r.db.WithContext(ctx).Where("id > ?", page.AfterID).Limit(page.Limit)
	if unitNumber != nil {
		query = query.Where("unit_id IN (?)", r.db.Model(&model.Unit{}).Select("id").Where("unit_number = ?", *unitNumber))
	}
	err := query.Preload("Unit").Preload("InvoiceItems", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Order("id ASC").Find(&invoices).Error
	return invoices, err
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id uint) (*model.Invoice, error) {
	var invoice model.Invoice
	err := r.db.WithContext(ctx).Preload("Unit").Preload("InvoiceItems", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).First(&invoice, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrInvoiceNotFound
		}
		return nil, err
	}

	return &invoice, nil
}
