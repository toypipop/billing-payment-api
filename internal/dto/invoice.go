package dto

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

type CreateInvoiceRequest struct {
	Unit    string                     `json:"unit" binding:"required,max=50"`
	DueDate string                     `json:"due_date" binding:"required"`
	Items   []CreateInvoiceItemRequest `json:"items" binding:"required,min=1,dive"`
}

type CreateInvoiceItemRequest struct {
	Description string  `json:"description" binding:"required"`
	AmountTHB   *Amount `json:"amount_thb" binding:"required"`
}

// Amount represents THB in JSON and exact integer satang in Go. This avoids
// floating-point rounding on both input and output.
type Amount int64

func (a Amount) MarshalJSON() ([]byte, error) {
	if a < 0 {
		return nil, fmt.Errorf("amount_thb must be non-negative")
	}
	return []byte(fmt.Sprintf("%d.%02d", a/100, a%100)), nil
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	var number json.Number
	if len(data) == 0 || data[0] == '"' || string(data) == "null" {
		return fmt.Errorf("amount_thb must be a non-negative JSON number with at most two decimal places")
	}
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	value, ok := new(big.Rat).SetString(number.String())
	if !ok || value.Sign() < 0 {
		return fmt.Errorf("amount_thb must be a non-negative number")
	}
	value.Mul(value, big.NewRat(100, 1))
	if !value.IsInt() || !value.Num().IsInt64() {
		return fmt.Errorf("amount_thb must fit in int64 satang and have at most two decimal places")
	}
	*a = Amount(value.Num().Int64())
	return nil
}

type InvoiceResponse struct {
	ID                   uint                  `json:"id"`
	InvoiceNumber        string                `json:"invoice_number"`
	UnitID               uint                  `json:"unit_id"`
	Unit                 string                `json:"unit"`
	PaidAmountTHB        Amount                `json:"paid_amount_thb"`
	OutstandingAmountTHB Amount                `json:"outstanding_amount_thb"`
	Status               string                `json:"status"`
	DueDate              time.Time             `json:"due_date"`
	TotalAmountTHB       Amount                `json:"total_amount_thb"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
	Items                []InvoiceItemResponse `json:"items"`
}

type InvoiceItemResponse struct {
	ID          uint      `json:"id"`
	Description string    `json:"description"`
	AmountTHB   Amount    `json:"amount_thb"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
