package dto

import "time"

type CreatePaymentRequest struct {
	Unit      string  `json:"unit" binding:"required,max=50"`
	AmountTHB *Amount `json:"amount_thb" binding:"required"`
}

type PaymentResponse struct {
	ID          uint                        `json:"id"`
	UnitID      uint                        `json:"unit_id"`
	Unit        string                      `json:"unit"`
	AmountTHB   Amount                      `json:"amount_thb"`
	CreatedAt   time.Time                   `json:"created_at"`
	Allocations []PaymentAllocationResponse `json:"allocations"`
}

type PaymentAllocationResponse struct {
	InvoiceID            uint   `json:"invoice_id"`
	InvoiceNumber        string `json:"invoice_number"`
	AmountTHB            Amount `json:"amount_thb"`
	PaidAmountTHB        Amount `json:"paid_amount_thb"`
	OutstandingAmountTHB Amount `json:"outstanding_amount_thb"`
	Status               string `json:"status"`
}
