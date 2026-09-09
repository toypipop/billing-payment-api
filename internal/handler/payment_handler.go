package handler

import (
	"errors"
	"net/http"

	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/repository"
	"billing-payment-api/internal/response"
	"billing-payment-api/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PaymentHandler struct{ paymentService *service.PaymentService }

func NewPaymentHandler(paymentService *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

func (h *PaymentHandler) Create(c *gin.Context) {
	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid payment request: "+err.Error(), nil)
		return
	}
	payment, err := h.paymentService.Create(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPayment):
			response.Error(c, http.StatusBadRequest, "INVALID_PAYMENT", err.Error(), nil)
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Error(c, http.StatusNotFound, "UNIT_NOT_FOUND", "unit not found", nil)
		case errors.Is(err, repository.ErrNoOutstandingInvoices):
			response.Error(c, http.StatusConflict, "NO_OUTSTANDING_INVOICES", err.Error(), nil)
		case errors.Is(err, repository.ErrOverpayment):
			response.Error(c, http.StatusConflict, "OVERPAYMENT", err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not record payment", err)
		}
		return
	}
	c.JSON(http.StatusCreated, payment)
}
