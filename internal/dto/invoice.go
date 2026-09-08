package dto

import "time"

type CreateInvoiceRequest struct {
	InvoiceNumber string                     `json:"invoice_number" binding:"required"`
	UnitID        uint                       `json:"unit_id" binding:"required"`
	DueDate       time.Time                  `json:"due_date" binding:"required"`
	Items         []CreateInvoiceItemRequest `json:"items" binding:"required,min=1,dive"`
}

type CreateInvoiceItemRequest struct {
	Description string `json:"description" binding:"required"`
	AmountCents int64  `json:"amount_cents" binding:"required,gte=0"`
}

type InvoiceResponse struct {
	ID               uint                  `json:"id"`
	InvoiceNumber    string                `json:"invoice_number"`
	UnitID           uint                  `json:"unit_id"`
	DueDate          time.Time             `json:"due_date"`
	TotalAmountCents int64                 `json:"total_amount_cents"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
	Items            []InvoiceItemResponse `json:"items"`
}

type InvoiceItemResponse struct {
	ID          uint      `json:"id"`
	Description string    `json:"description"`
	AmountCents int64     `json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
