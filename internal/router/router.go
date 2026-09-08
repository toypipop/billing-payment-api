package router

import (
	"billing-payment-api/internal/handler"

	"github.com/gin-gonic/gin"
)

func SetupRouter(healthHandler *handler.HealthHandler, invoiceHandler *handler.InvoiceHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/health", healthHandler.Check)
	r.POST("/invoices", invoiceHandler.Create)
	r.GET("/invoices/:id", invoiceHandler.GetByID)

	return r
}
