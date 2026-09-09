package model

import "time"

type Payment struct {
	ID          uint  `gorm:"primaryKey"`
	UnitID      uint  `gorm:"not null;index"`
	AmountCents int64 `gorm:"not null;check:amount_cents > 0"`
	CreatedAt   time.Time
	Unit        Unit                `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Allocations []PaymentAllocation `gorm:"foreignKey:PaymentID"`
}

type PaymentAllocation struct {
	ID          uint  `gorm:"primaryKey"`
	PaymentID   uint  `gorm:"not null;uniqueIndex:idx_payment_invoice"`
	InvoiceID   uint  `gorm:"not null;index;uniqueIndex:idx_payment_invoice"`
	AmountCents int64 `gorm:"not null;check:amount_cents > 0"`
	CreatedAt   time.Time
	Payment     Payment `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Invoice     Invoice `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}
