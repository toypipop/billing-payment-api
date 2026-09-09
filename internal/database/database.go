package database

import (
	"os"

	"billing-payment-api/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const defaultDatabaseURL = "postgres://billing_user:billing_password@localhost:5432/billing_payment?sslmode=disable"

func Connect() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDatabaseURL
	}

	// Request and startup logs record errors centrally; avoid duplicate SQL logs
	// containing interpolated invoice or payment values.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&model.Unit{}, &model.Invoice{}, &model.InvoiceItem{}, &model.Payment{}, &model.PaymentAllocation{}); err != nil {
		return nil, err
	}

	return db, nil
}
