package router

import (
	"log/slog"
	"net/http"
	"time"

	"billing-payment-api/internal/handler"
	"billing-payment-api/internal/middleware"
	"billing-payment-api/internal/response"

	"github.com/gin-gonic/gin"
)

func SetupRouter(healthHandler *handler.HealthHandler, invoiceHandler *handler.InvoiceHandler, paymentHandler *handler.PaymentHandler, timeouts ...time.Duration) *gin.Engine {
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.HandleMethodNotAllowed = true
	r.Use(middleware.RequestLog(slog.Default()))
	timeout := 10 * time.Second
	if len(timeouts) > 0 && timeouts[0] > 0 {
		timeout = timeouts[0]
	}
	r.Use(middleware.Deadline(timeout))
	r.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found", nil)
	})
	r.NoMethod(func(c *gin.Context) {
		response.Error(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	})

	r.GET("/health", healthHandler.Check)
	r.POST("/invoices", invoiceHandler.Create)
	r.GET("/invoices", invoiceHandler.GetAll)
	r.GET("/invoices/:id", invoiceHandler.GetByID)
	r.POST("/payments", paymentHandler.Create)

	return r
}
