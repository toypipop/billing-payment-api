package router_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"billing-payment-api/internal/handler"
	"billing-payment-api/internal/repository"
	"billing-payment-api/internal/router"
	"billing-payment-api/internal/service"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestHTTPErrorContract(t *testing.T) {
	// A closed pool gives deterministic database errors without a running server.
	db, err := gorm.Open(postgres.Open("host=localhost user=test dbname=test sslmode=disable"), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	r := router.SetupRouter(handler.NewHealthHandler(db),
		handler.NewInvoiceHandler(service.NewInvoiceService(repository.NewInvoiceRepository(db))),
		handler.NewPaymentHandler(service.NewPaymentService(repository.NewPaymentRepository(db))))
	for _, tc := range []struct {
		method, path, body, code string
		status                   int
	}{
		{"GET", "/missing", "", "ROUTE_NOT_FOUND", 404},
		{"GET", "/invoices/", "", "ROUTE_NOT_FOUND", 404},
		{"DELETE", "/invoices", "", "METHOD_NOT_ALLOWED", 405},
		{"GET", "/payments", "", "METHOD_NOT_ALLOWED", 405},
		{"GET", "/invoices/0", "", "INVALID_INVOICE_ID", 400},
		{"GET", "/invoices?unit=", "", "INVALID_UNIT", 400},
		{"GET", "/invoices?unit=%20%20", "", "INVALID_UNIT", 400},
		{"GET", "/invoices?unit=A&unit=B", "", "INVALID_UNIT", 400},
		{"GET", "/invoices?unit=" + strings.Repeat("A", 51), "", "INVALID_UNIT", 400},
		{"POST", "/payments", `{"unit":"A101","amount_thb":"599"}`, "INVALID_REQUEST", 400},
		{"POST", "/invoices", `{"unit":"A101","due_date":"2026-08-01","items":[{"description":"Fee","amount_thb":"599"}]}`, "INVALID_REQUEST", 400},
		{"POST", "/payments", `{"unit":"A101","amount":599}`, "INVALID_REQUEST", 400},
		{"GET", "/invoices/9223372036854775808", "", "INVALID_INVOICE_ID", 400},
		{"POST", "/invoices", `{}`, "INVALID_REQUEST", 400},
		{"POST", "/payments", `{"unit":"A101","amount_thb":0}`, "INVALID_PAYMENT", 400},
		{"POST", "/payments", `{"unit":`, "INVALID_REQUEST", 400},
		{"GET", "/health", "", "DATABASE_UNAVAILABLE", 503},
		{"GET", "/invoices", "", "INTERNAL_ERROR", 500},
		{"GET", "/invoices/1", "", "INTERNAL_ERROR", 500},
		{"POST", "/invoices", `{"unit":"A101","due_date":"2026-08-01","items":[{"description":"Fee","amount_thb":100}]}`, "INTERNAL_ERROR", 500},
		{"POST", "/payments", `{"unit":"A101","amount_thb":1}`, "INTERNAL_ERROR", 500},
	} {
		t.Run(tc.method+tc.path+tc.code, func(t *testing.T) {
			output.Reset()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			var body map[string]string
			if w.Code != tc.status || json.Unmarshal(w.Body.Bytes(), &body) != nil || body["code"] != tc.code || body["error"] == "" || body["request_id"] == "" || body["request_id"] != w.Header().Get("X-Request-ID") {
				t.Fatalf("response = %d: %s", w.Code, w.Body.String())
			}
			if tc.status == 405 && w.Header().Get("Allow") == "" {
				t.Fatal("405 is missing Allow header")
			}
			if strings.Contains(w.Body.String(), "database is closed") {
				t.Fatal("internal database error exposed")
			}
			var log map[string]any
			if err := json.Unmarshal(output.Bytes(), &log); err != nil {
				t.Fatalf("invalid access log: %v: %s", err, output.String())
			}
			if log["status"] != float64(w.Code) || log["request_id"] != body["request_id"] || log["error_code"] != tc.code {
				t.Fatalf("log does not match response: %s", output.String())
			}
			if tc.status >= 500 && !strings.Contains(output.String(), "database is closed") {
				t.Fatalf("missing internal cause: %s", output.String())
			}
		})
	}
}
