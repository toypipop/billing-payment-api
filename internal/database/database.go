package database

import (
	"billing-payment-api/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"os"
	"strconv"

	"billing-payment-api/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const defaultDatabaseURL = "postgres://billing_user:billing_password@localhost:5432/billing_payment?sslmode=disable"

func Connect() (*gorm.DB, error) {
	settings, err := config.Load()
	if err != nil {
		return nil, err
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDatabaseURL
	}

	// Request and startup logs record errors centrally; avoid duplicate SQL logs
	// containing interpolated invoice or payment values.
	connection, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	connection.RuntimeParams["statement_timeout"] = strconv.FormatInt(settings.StatementTimeout.Milliseconds(), 10)
	connection.RuntimeParams["lock_timeout"] = strconv.FormatInt(settings.LockTimeout.Milliseconds(), 10)
	connection.ConnectTimeout = settings.RequestTimeout
	sqlDB := stdlib.OpenDB(*connection)
	sqlDB.SetMaxOpenConns(settings.MaxOpenConns)
	sqlDB.SetMaxIdleConns(settings.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(settings.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(settings.ConnMaxIdleTime)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	if err := db.AutoMigrate(&model.Unit{}, &model.Invoice{}, &model.InvoiceItem{}, &model.Payment{}, &model.PaymentAllocation{}); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	if err := MigrateInvoiceSequence(db); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}
