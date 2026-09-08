package model

import "time"

type InvoiceItem struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	InvoiceID   uint      `gorm:"not null;index" json:"-"`
	Description string    `gorm:"type:text;not null" json:"-"`
	AmountCents int64     `gorm:"column:amount_cents;not null;check:amount_cents >= 0" json:"-"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`

	Invoice Invoice `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}
