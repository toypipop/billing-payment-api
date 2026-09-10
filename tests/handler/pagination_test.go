package handler_test

import (
	"billing-payment-api/internal/dto"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestInvoicePagination(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	for i := 0; i < 55; i++ {
		seedInvoice(t, db, "A101", fmt.Sprintf("INV%03d", i), "2026-08-01", 10000, 0)
	}
	seedInvoice(t, db, "B202", "OTHER", "2026-08-01", 10000, 0)
	for _, tc := range []struct {
		query string
		count int
		first uint
	}{
		{"", 50, 1}, {"?limit=2", 2, 1}, {"?limit=2&after_id=2", 2, 3},
		{"?unit=A101&limit=100&after_id=53", 2, 54}, {"?after_id=56", 0, 0},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/invoices"+tc.query, nil))
		var got []dto.InvoiceResponse
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil {
			t.Fatalf("%s: %d %s", tc.query, w.Code, w.Body.String())
		}
		if len(got) != tc.count || (len(got) > 0 && got[0].ID != tc.first) {
			t.Errorf("%s: count=%d rows=%v", tc.query, len(got), got)
		}
	}
}

func TestInvalidPagination(t *testing.T) {
	r := paymentTestRouter(nil)
	for _, query := range []string{"limit=0", "limit=-1", "limit=101", "limit=abc", "limit=", "limit=1&limit=2", "after_id=-1", "after_id=abc", "after_id=", "after_id=9223372036854775808", "after_id=1&after_id=2"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/invoices?"+query, nil))
		if w.Code != 400 {
			t.Errorf("%s: got %d", query, w.Code)
		}
	}
}
