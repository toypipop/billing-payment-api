package handler

import (
	"errors"
	"net/http"
	"strconv"

	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/response"
	"billing-payment-api/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InvoiceHandler struct {
	invoiceService *service.InvoiceService
}

func NewInvoiceHandler(invoiceService *service.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{invoiceService: invoiceService}
}

func (h *InvoiceHandler) Create(c *gin.Context) {
	var req dto.CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid invoice request: "+err.Error(), nil)
		return
	}

	invoice, err := h.invoiceService.Create(req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInvoice) {
			response.Error(c, http.StatusBadRequest, "INVALID_INVOICE", err.Error(), nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not create invoice", err)
		return
	}

	c.Header("Location", "/invoices/"+strconv.FormatUint(uint64(invoice.ID), 10))
	c.JSON(http.StatusCreated, invoice)
}

func (h *InvoiceHandler) GetAll(c *gin.Context) {
	var unit *string
	if values, present := c.Request.URL.Query()["unit"]; present {
		if len(values) != 1 {
			response.Error(c, http.StatusBadRequest, "INVALID_UNIT", "provide exactly one unit", nil)
			return
		}
		unit = &values[0]
	}
	invoices, err := h.invoiceService.GetAll(unit)
	if err != nil {
		if errors.Is(err, service.ErrInvalidUnit) {
			response.Error(c, http.StatusBadRequest, "INVALID_UNIT", err.Error(), nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not get invoices", err)
		return
	}
	c.JSON(http.StatusOK, invoices)
}

func (h *InvoiceHandler) GetByID(c *gin.Context) {
	// PostgreSQL stores IDs in signed bigint columns.
	id, err := strconv.ParseUint(c.Param("id"), 10, 63)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "INVALID_INVOICE_ID", "invalid invoice id", nil)
		return
	}

	invoice, err := h.invoiceService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "INVOICE_NOT_FOUND", "invoice not found", nil)
			return
		}

		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not get invoice", err)
		return
	}

	c.JSON(http.StatusOK, invoice)
}
