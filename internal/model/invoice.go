package model

import "time"

const (
	InvoiceUnpaid  = "UNPAID"
	InvoicePartial = "PARTIAL"
	InvoicePaid    = "PAID"
)

func (invoice Invoice) Outstanding() int64 {
	return invoice.TotalAmountCents - invoice.PaidAmountCents
}

func (invoice Invoice) Status() string {
	if invoice.Outstanding() == 0 {
		return InvoicePaid
	}
	if invoice.PaidAmountCents > 0 {
		return InvoicePartial
	}
	return InvoiceUnpaid
}

type Invoice struct {
	ID               uint      `gorm:"primaryKey" json:"-"`
	InvoiceNumber    string    `gorm:"type:varchar(50);not null;uniqueIndex" json:"-"`
	UnitID           uint      `gorm:"not null;index" json:"-"`
	DueDate          time.Time `gorm:"type:date;not null" json:"-"`
	TotalAmountCents int64     `gorm:"column:total_amount_cents;not null;check:total_amount_cents >= 0" json:"-"`
	PaidAmountCents  int64     `gorm:"not null;default:0;check:paid_amount_cents >= 0 AND paid_amount_cents <= total_amount_cents" json:"-"`
	CreatedAt        time.Time `json:"-"`
	UpdatedAt        time.Time `json:"-"`

	Unit         Unit          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	InvoiceItems []InvoiceItem `gorm:"foreignKey:InvoiceID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}
