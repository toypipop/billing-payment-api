package service

import (
	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/model"
	"billing-payment-api/internal/repository"
)

type InvoiceService struct {
	invoiceRepository *repository.InvoiceRepository
}

func NewInvoiceService(invoiceRepository *repository.InvoiceRepository) *InvoiceService {
	return &InvoiceService{invoiceRepository: invoiceRepository}
}

func (s *InvoiceService) Create(req dto.CreateInvoiceRequest) (*dto.InvoiceResponse, error) {
	var totalAmountCents int64
	items := make([]model.InvoiceItem, 0, len(req.Items))

	for _, item := range req.Items {
		totalAmountCents += item.AmountCents
		items = append(items, model.InvoiceItem{
			Description: item.Description,
			AmountCents: item.AmountCents,
		})
	}

	invoice := model.Invoice{
		InvoiceNumber:    req.InvoiceNumber,
		UnitID:           req.UnitID,
		DueDate:          req.DueDate,
		TotalAmountCents: totalAmountCents,
		InvoiceItems:     items,
	}

	if err := s.invoiceRepository.Create(&invoice); err != nil {
		return nil, err
	}

	return toInvoiceResponse(&invoice), nil
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
