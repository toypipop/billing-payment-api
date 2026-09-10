package database

import (
	"fmt"
	"gorm.io/gorm"
)

// MigrateInvoiceSequence preserves existing numbers and never resets a sequence.
// The fixed width and maximum keep lexical and numeric ordering identical.
func MigrateInvoiceSequence(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(734829105)").Error; err != nil {
			return err
		}
		var exists bool
		if err := tx.Raw("SELECT to_regclass('invoice_number_seq') IS NOT NULL").Scan(&exists).Error; err != nil {
			return err
		}
		if exists {
			return nil
		}
		var start int64
		if err := tx.Raw(`SELECT COALESCE(MAX(substring(invoice_number FROM 5)::bigint),0)+1 FROM invoices WHERE invoice_number ~ '^INV-[0-9]{10}$'`).Scan(&start).Error; err != nil {
			return err
		}
		return tx.Exec(fmt.Sprintf("CREATE SEQUENCE invoice_number_seq AS bigint START WITH %d MAXVALUE 9999999999 NO CYCLE", start)).Error
	})
}
