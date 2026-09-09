package service

import (
	"context"
	"errors"
	"fmt"

	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/model"
)

var ErrInvalidPayment = errors.New("invalid payment")

type PaymentService struct{ paymentRepository PaymentStore }

type PaymentStore interface {
	Allocate(context.Context, string, int64, string) (*model.Payment, error)
}

func NewPaymentService(paymentRepository PaymentStore) *PaymentService {
	return &PaymentService{paymentRepository: paymentRepository}
}

func (s *PaymentService) Create(ctx context.Context, req dto.CreatePaymentRequest) (*dto.PaymentResponse, error) {
	unitNumber, valid := normalizeUnit(req.Unit)
	if !valid {
		return nil, fmt.Errorf("%w: unit must contain 1 to 50 characters", ErrInvalidPayment)
	}
	if req.AmountTHB == nil || *req.AmountTHB <= 0 {
		return nil, fmt.Errorf("%w: amount_thb must be greater than zero", ErrInvalidPayment)
	}
	if req.IdempotencyKey != "" && !ValidIdempotencyKey(req.IdempotencyKey) {
		return nil, fmt.Errorf("%w: invalid idempotency key", ErrInvalidPayment)
	}
	payment, err := s.paymentRepository.Allocate(ctx, unitNumber, int64(*req.AmountTHB), req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	response := &dto.PaymentResponse{
		Replayed: payment.Replayed,
		ID:       payment.ID, UnitID: payment.UnitID, Unit: payment.Unit.UnitNumber,
		AmountTHB: dto.Amount(payment.AmountCents), CreatedAt: payment.CreatedAt.UTC(),
		Allocations: make([]dto.PaymentAllocationResponse, 0, len(payment.Allocations)),
	}
	for _, allocation := range payment.Allocations {
		invoice := allocation.Invoice
		response.Allocations = append(response.Allocations, dto.PaymentAllocationResponse{
			InvoiceID: invoice.ID, InvoiceNumber: invoice.InvoiceNumber,
			AmountTHB: dto.Amount(allocation.AmountCents), PaidAmountTHB: dto.Amount(invoice.PaidAmountCents),
			OutstandingAmountTHB: dto.Amount(invoice.Outstanding()),
			Status:               invoice.Status(),
		})
	}
	return response, nil
}

func ValidIdempotencyKey(key string) bool {
	if len(key) == 0 || len(key) > 128 {
		return false
	}
	for _, ch := range key {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}
