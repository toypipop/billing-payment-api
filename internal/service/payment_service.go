package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/repository"
)

var ErrInvalidPayment = errors.New("invalid payment")

type PaymentService struct{ paymentRepository *repository.PaymentRepository }

func NewPaymentService(paymentRepository *repository.PaymentRepository) *PaymentService {
	return &PaymentService{paymentRepository: paymentRepository}
}

func (s *PaymentService) Create(ctx context.Context, req dto.CreatePaymentRequest) (*dto.PaymentResponse, error) {
	unitNumber := strings.TrimSpace(req.Unit)
	if unitNumber == "" || utf8.RuneCountInString(unitNumber) > 50 {
		return nil, fmt.Errorf("%w: unit must contain 1 to 50 characters", ErrInvalidPayment)
	}
	if req.AmountTHB == nil || *req.AmountTHB <= 0 {
		return nil, fmt.Errorf("%w: amount_thb must be greater than zero", ErrInvalidPayment)
	}
	payment, err := s.paymentRepository.Allocate(ctx, unitNumber, int64(*req.AmountTHB))
	if err != nil {
		return nil, err
	}
	response := &dto.PaymentResponse{
		ID: payment.ID, UnitID: payment.UnitID, Unit: payment.Unit.UnitNumber,
		AmountTHB: dto.Amount(payment.AmountCents), CreatedAt: payment.CreatedAt,
		Allocations: make([]dto.PaymentAllocationResponse, 0, len(payment.Allocations)),
	}
	for _, allocation := range payment.Allocations {
		invoice := allocation.Invoice
		response.Allocations = append(response.Allocations, dto.PaymentAllocationResponse{
			InvoiceID: invoice.ID, InvoiceNumber: invoice.InvoiceNumber,
			AmountTHB: dto.Amount(allocation.AmountCents), PaidAmountTHB: dto.Amount(invoice.PaidAmountCents),
			OutstandingAmountTHB: dto.Amount(invoice.TotalAmountCents - invoice.PaidAmountCents),
			Status:               invoiceStatus(invoice.TotalAmountCents, invoice.PaidAmountCents),
		})
	}
	return response, nil
}
