package handler

import (
	"errors"
	"net/http"

	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/model"

	"billing-payment-api/internal/response"
	"billing-payment-api/internal/service"
	"github.com/gin-gonic/gin"
)

type PaymentHandler struct{ paymentService *service.PaymentService }

func NewPaymentHandler(paymentService *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

func (h *PaymentHandler) Create(c *gin.Context) {
	var req dto.CreatePaymentRequest
	if !bindJSON(c, &req) {
		return
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) > 1 || (len(keys) == 1 && !service.ValidIdempotencyKey(keys[0])) {
		response.Error(c, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must contain 1 to 128 ASCII letters, digits, hyphens or underscores", nil)
		return
	}
	req.IdempotencyKey = c.GetHeader("Idempotency-Key")
	payment, err := h.paymentService.Create(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPayment):
			response.Error(c, http.StatusBadRequest, "INVALID_PAYMENT", err.Error(), nil)
		case errors.Is(err, model.ErrUnitNotFound):
			response.Error(c, http.StatusNotFound, "UNIT_NOT_FOUND", "unit not found", nil)
		case errors.Is(err, model.ErrNoOutstandingInvoices):
			response.Error(c, http.StatusConflict, "NO_OUTSTANDING_INVOICES", err.Error(), nil)
		case errors.Is(err, model.ErrOverpayment):
			response.Error(c, http.StatusConflict, "OVERPAYMENT", err.Error(), nil)
		case errors.Is(err, model.ErrIdempotencyConflict):
			response.Error(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not record payment", err)
		}
		return
	}
	if payment.Replayed {
		c.Header("Idempotency-Replayed", "true")
		c.JSON(http.StatusOK, payment)
		return
	}
	c.JSON(http.StatusCreated, payment)
}
