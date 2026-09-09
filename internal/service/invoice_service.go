package service

import (
	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/model"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalidInvoice = errors.New("invalid invoice")
var ErrInvalidUnit = errors.New("invalid unit")

type InvoiceService struct {
	invoiceRepository InvoiceStore
}

type InvoiceStore interface {
	Create(context.Context, *model.Invoice, string) error
	GetAll(context.Context, *string) ([]model.Invoice, error)
	GetByID(context.Context, uint) (*model.Invoice, error)
}

func NewInvoiceService(invoiceRepository InvoiceStore) *InvoiceService {
	return &InvoiceService{invoiceRepository: invoiceRepository}
}

func (s *InvoiceService) Create(ctx context.Context, req dto.CreateInvoiceRequest) (*dto.InvoiceResponse, error) {
	unitNumber, valid := normalizeUnit(req.Unit)
	if !valid {
		return nil, fmt.Errorf("%w: unit must contain 1 to 50 characters", ErrInvalidInvoice)
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("%w: at least one item is required", ErrInvalidInvoice)
	}
	if len(req.Items) > 100 {
		return nil, fmt.Errorf("%w: at most 100 items are allowed", ErrInvalidInvoice)
	}
	dueDate, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		return nil, fmt.Errorf("%w: due_date must be a valid YYYY-MM-DD date", ErrInvalidInvoice)
	}
	var totalAmountCents int64
	items := make([]model.InvoiceItem, 0, len(req.Items))

	for _, item := range req.Items {
		description := strings.TrimSpace(item.Description)
		if utf8.RuneCountInString(item.Description) > 500 {
			return nil, fmt.Errorf("%w: item description must not exceed 500 characters", ErrInvalidInvoice)
		}
		if description == "" {
			return nil, fmt.Errorf("%w: item description is required", ErrInvalidInvoice)
		}
		if item.AmountTHB == nil || *item.AmountTHB < 0 {
			return nil, fmt.Errorf("%w: amount_thb is required and must be non-negative", ErrInvalidInvoice)
		}
		amountCents := int64(*item.AmountTHB)
		if amountCents > math.MaxInt64-totalAmountCents {
			return nil, fmt.Errorf("%w: total amount is too large", ErrInvalidInvoice)
		}
		totalAmountCents += amountCents
		items = append(items, model.InvoiceItem{
			Description: description,
			AmountCents: amountCents,
		})
	}

	invoice := model.Invoice{
		InvoiceNumber:    "INV-" + rand.Text(),
		DueDate:          dueDate,
		TotalAmountCents: totalAmountCents,
		InvoiceItems:     items,
	}

	if err := s.invoiceRepository.Create(ctx, &invoice, unitNumber); err != nil {
		return nil, err
	}

	return toInvoiceResponse(&invoice), nil
}

func (s *InvoiceService) GetAll(ctx context.Context, unitNumber *string) ([]dto.InvoiceResponse, error) {
	if unitNumber != nil {
		unit, valid := normalizeUnit(*unitNumber)
		if !valid {
			return nil, fmt.Errorf("%w: unit must contain 1 to 50 characters", ErrInvalidUnit)
		}
		unitNumber = &unit
	}
	invoices, err := s.invoiceRepository.GetAll(ctx, unitNumber)
	if err != nil {
		return nil, err
	}
	responses := make([]dto.InvoiceResponse, 0, len(invoices))
	for i := range invoices {
		responses = append(responses, *toInvoiceResponse(&invoices[i]))
	}
	return responses, nil
}

func (s *InvoiceService) GetByID(ctx context.Context, id uint) (*dto.InvoiceResponse, error) {
	invoice, err := s.invoiceRepository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return toInvoiceResponse(invoice), nil
}

func toInvoiceResponse(invoice *model.Invoice) *dto.InvoiceResponse {
	items := make([]dto.InvoiceItemResponse, 0, len(invoice.InvoiceItems))
	for _, item := range invoice.InvoiceItems {
		items = append(items, dto.InvoiceItemResponse{
			ID:          item.ID,
			Description: item.Description,
			AmountTHB:   dto.Amount(item.AmountCents),
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return &dto.InvoiceResponse{
		ID:                   invoice.ID,
		InvoiceNumber:        invoice.InvoiceNumber,
		UnitID:               invoice.UnitID,
		Unit:                 invoice.Unit.UnitNumber,
		PaidAmountTHB:        dto.Amount(invoice.PaidAmountCents),
		OutstandingAmountTHB: dto.Amount(invoice.Outstanding()),
		Status:               invoice.Status(),
		DueDate:              invoice.DueDate,
		TotalAmountTHB:       dto.Amount(invoice.TotalAmountCents),
		CreatedAt:            invoice.CreatedAt,
		UpdatedAt:            invoice.UpdatedAt,
		Items:                items,
	}
}
