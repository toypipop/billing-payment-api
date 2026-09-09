package database

import (
	"os"

	"billing-payment-api/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultDatabaseURL = "postgres://billing_user:billing_password@localhost:5432/billing_payment?sslmode=disable"

func Connect() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDatabaseURL
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&model.Unit{}, &model.Invoice{}, &model.InvoiceItem{}); err != nil {
		return nil, err
	}

	return db, nil
}
