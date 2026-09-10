package database

import "gorm.io/gorm"

// MigratePerformanceIndexes creates indexes for queries that remain selective
// when a unit has a large history of fully paid invoices.
func MigratePerformanceIndexes(db *gorm.DB) error {
	return db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_invoices_outstanding_by_unit_due_date
		ON invoices (unit_id, due_date, invoice_number COLLATE "C")
		WHERE paid_amount_cents < total_amount_cents
	`).Error
}
