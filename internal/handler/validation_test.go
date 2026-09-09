package handler_test

import (
	"billing-payment-api/internal/handler"
	"billing-payment-api/internal/router"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStrictRequestValidation(t *testing.T) {
	r := router.SetupRouter(handler.NewHealthHandler(nil), handler.NewInvoiceHandler(nil), handler.NewPaymentHandler(nil))
	valid := `{"unit":"A101","amount_thb":599}`
	for _, tc := range []struct {
		name, path, contentType, body string
		status                        int
	}{
		{"missing media type", "/payments", "", valid, 415},
		{"text", "/payments", "text/plain", valid, 415},
		{"empty", "/payments", "application/json", "", 400},
		{"null", "/payments", "application/json", "null", 400},
		{"array", "/payments", "application/json", "[]", 400},
		{"string amount", "/payments", "application/json", `{"unit":"A101","amount_thb":"599"}`, 400},
		{"unknown field", "/payments", "application/json", `{"unit":"A101","amount_thb":599,"amout":1}`, 400},
		{"two objects", "/payments", "application/json", valid + ` {}`, 400},
		{"trailing text", "/payments", "application/json", valid + ` broken`, 400},
		{"comma", "/payments", "application/json; charset=utf-8", `{"unit":"A101","amount_thb":599,}`, 400},
		{"large body", "/payments", "application/json", `{"unit":"` + strings.Repeat("A", 1<<20) + `","amount_thb":599}`, 413},
		{"large suffix", "/payments", "application/json", valid + strings.Repeat(" ", 1<<20), 413},
		{"long description", "/invoices", "application/json", `{"unit":"A101","due_date":"2026-08-01","items":[{"description":"` + strings.Repeat("a", 501) + `","amount_thb":1}]}`, 400},
		{"many items", "/invoices", "application/json", `{"unit":"A101","due_date":"2026-08-01","items":[` + strings.TrimSuffix(strings.Repeat(`{"description":"Fee","amount_thb":1},`, 101), ",") + `]}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
	for _, keys := range [][]string{{""}, {"bad key"}, {strings.Repeat("a", 129)}, {"ก"}, {"one", "two"}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/payments", strings.NewReader(valid))
		req.Header.Set("Content-Type", "application/json")
		req.Header["Idempotency-Key"] = keys
		r.ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_IDEMPOTENCY_KEY") {
			t.Fatalf("key: %d %s", w.Code, w.Body.String())
		}
	}
}
