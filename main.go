package main

import (
	"log/slog"
	"os"

	"billing-payment-api/internal/config"
	"billing-payment-api/internal/database"
	"billing-payment-api/internal/handler"
	"billing-payment-api/internal/repository"
	"billing-payment-api/internal/router"
	"billing-payment-api/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	settings, err := config.Load()
	if err != nil {
		slog.Error("startup_failed", "error", err)
		os.Exit(1)
	}
	db, err := database.Connect()
	if err != nil {
		slog.Error("startup_failed", "error", err)
		os.Exit(1)
	}

	invoiceRepository := repository.NewInvoiceRepository(db)
	invoiceService := service.NewInvoiceService(invoiceRepository)
	invoiceHandler := handler.NewInvoiceHandler(invoiceService)
	healthHandler := handler.NewHealthHandler(db)
	paymentRepository := repository.NewPaymentRepository(db)
	paymentService := service.NewPaymentService(paymentRepository)
	paymentHandler := handler.NewPaymentHandler(paymentService)

	r := router.SetupRouter(healthHandler, invoiceHandler, paymentHandler, settings.RequestTimeout)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("server_starting", "address", ":"+port)
	if err := r.Run(":" + port); err != nil {
		slog.Error("server_failed", "error", err)
		os.Exit(1)
	}
}
