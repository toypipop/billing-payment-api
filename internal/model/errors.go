package model

import "errors"

var (
	ErrIdempotencyConflict   = errors.New("idempotency key was already used for a different unit or amount")
	ErrInvoiceNotFound       = errors.New("invoice not found")
	ErrUnitNotFound          = errors.New("unit not found")
	ErrNoOutstandingInvoices = errors.New("unit has no outstanding invoices")
	ErrOverpayment           = errors.New("payment exceeds unit outstanding balance; no payment was recorded")
)
