package service

import (
	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/model"
	"billing-payment-api/internal/repository"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrInvalidInvoice = errors.New("invalid invoice")

type InvoiceService struct {
	invoiceRepository *repository.InvoiceRepository
}

func NewInvoiceService(invoiceRepository *repository.InvoiceRepository) *InvoiceService {
	return &InvoiceService{invoiceRepository: invoiceRepository}
}

func (s *InvoiceService) Create(req dto.CreateInvoiceRequest) (*dto.InvoiceResponse, error) {
	unitNumber := strings.TrimSpace(req.Unit)
	if unitNumber == "" {
		return nil, fmt.Errorf("%w: unit is required", ErrInvalidInvoice)
	}
	dueDate, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		return nil, fmt.Errorf("%w: due_date must be a valid YYYY-MM-DD date", ErrInvalidInvoice)
	}
	var totalAmountCents int64
	items := make([]model.InvoiceItem, 0, len(req.Items))

	for _, item := range req.Items {
		if item.Amount == nil || *item.Amount < 0 {
			return nil, fmt.Errorf("%w: amount is required and must be non-negative", ErrInvalidInvoice)
		}
		amountCents := int64(*item.Amount)
		if amountCents > math.MaxInt64-totalAmountCents {
			return nil, fmt.Errorf("%w: total amount is too large", ErrInvalidInvoice)
		}
		totalAmountCents += amountCents
		items = append(items, model.InvoiceItem{
			Description: item.Description,
			AmountCents: amountCents,
		})
	}

	invoice := model.Invoice{
		InvoiceNumber:    "INV-" + rand.Text(),
		DueDate:          dueDate,
		TotalAmountCents: totalAmountCents,
		InvoiceItems:     items,
	}

	if err := s.invoiceRepository.Create(&invoice, unitNumber); err != nil {
		return nil, err
	}

	return toInvoiceResponse(&invoice), nil
}

func (s *InvoiceService) GetAll() ([]dto.InvoiceResponse, error) {
	invoices, err := s.invoiceRepository.GetAll()
	if err != nil {
		return nil, err
	}
	responses := make([]dto.InvoiceResponse, 0, len(invoices))
	for i := range invoices {
		responses = append(responses, *toInvoiceResponse(&invoices[i]))
	}
	return responses, nil
}

func (s *InvoiceService) GetByID(id uint) (*dto.InvoiceResponse, error) {
	invoice, err := s.invoiceRepository.GetByID(id)
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
			AmountCents: item.AmountCents,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return &dto.InvoiceResponse{
		ID:               invoice.ID,
		InvoiceNumber:    invoice.InvoiceNumber,
		UnitID:           invoice.UnitID,
		DueDate:          invoice.DueDate,
		TotalAmountCents: invoice.TotalAmountCents,
		CreatedAt:        invoice.CreatedAt,
		UpdatedAt:        invoice.UpdatedAt,
		Items:            items,
	}
}
