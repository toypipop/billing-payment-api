package main

import (
	"os"

	"billing-payment-api/internal/database"
	"billing-payment-api/internal/handler"
	"billing-payment-api/internal/repository"
	"billing-payment-api/internal/router"
	"billing-payment-api/internal/service"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		panic(err)
	}

	invoiceRepository := repository.NewInvoiceRepository(db)
	invoiceService := service.NewInvoiceService(invoiceRepository)
	invoiceHandler := handler.NewInvoiceHandler(invoiceService)
	healthHandler := handler.NewHealthHandler(db)

	r := router.SetupRouter(healthHandler, invoiceHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}
