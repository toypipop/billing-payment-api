package model

import "time"

type Unit struct {
	ID         uint      `gorm:"primaryKey" json:"-"`
	UnitNumber string    `gorm:"type:varchar(50);not null;uniqueIndex" json:"-"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`

	Invoices []Invoice `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
}
