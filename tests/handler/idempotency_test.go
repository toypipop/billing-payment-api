package handler_test

import (
	"billing-payment-api/internal/model"
	"billing-payment-api/internal/repository"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func keyedPayment(r http.Handler, key, body string) *httptest.ResponseRecorder {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/payments", strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	r.ServeHTTP(w, req)
	return w
}

func TestPaymentIdempotency(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	invoice := seedInvoice(t, db, "A101", "INV001", "2026-08-01", 100000, 0)
	seedInvoice(t, db, "B202", "INV002", "2026-08-01", 100000, 0)
	first := keyedPayment(r, "payment-1", `{"unit":"A101","amount_thb":599}`)
	if first.Code != 201 {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	second := keyedPayment(r, "payment-2", `{"unit":"A101","amount_thb":401}`)
	if second.Code != 201 {
		t.Fatalf("second: %d %s", second.Code, second.Body.String())
	}
	replay := keyedPayment(r, "payment-1", `{"amount_thb":599.00,"unit":" A101 "}`)
	if replay.Code != 200 || replay.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body.String())
	}
	var a, b any
	if err := json.Unmarshal(first.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(replay.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("response changed: %s / %s", first.Body.String(), replay.Body.String())
	}
	for _, body := range []string{`{"unit":"A101","amount_thb":600}`, `{"unit":"B202","amount_thb":599}`} {
		w := keyedPayment(r, "payment-1", body)
		if w.Code != 409 || !strings.Contains(w.Body.String(), "IDEMPOTENCY_CONFLICT") {
			t.Fatalf("conflict: %d %s", w.Code, w.Body.String())
		}
	}
	if got := readInvoice(t, r, invoice.ID); got.PaidAmountTHB != 100000 {
		t.Fatalf("balance: %+v", got)
	}
	var count int64
	if err := db.Model(&model.Payment{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("count: %d %v", count, err)
	}
}

func TestConcurrentIdempotency(t *testing.T) {
	for _, otherUnit := range []string{"A101", "B202"} {
		t.Run(otherUnit, func(t *testing.T) {
			db := paymentTestDB(t)
			r := paymentTestRouter(db)
			seedInvoice(t, db, "A101", "INV001", "2026-08-01", 100000, 0)
			seedInvoice(t, db, "B202", "INV002", "2026-08-01", 100000, 0)
			start := make(chan struct{})
			results := make(chan *httptest.ResponseRecorder, 2)
			for _, unit := range []string{"A101", otherUnit} {
				go func() { <-start; results <- keyedPayment(r, "same-key", `{"unit":"`+unit+`","amount_thb":100}`) }()
			}
			close(start)
			codes := map[int]int{}
			for i := 0; i < 2; i++ {
				select {
				case w := <-results:
					codes[w.Code]++
				case <-time.After(10 * time.Second):
					t.Fatal("timeout")
				}
			}
			expected := 200
			if otherUnit != "A101" {
				expected = 409
			}
			if codes[201] != 1 || codes[expected] != 1 {
				t.Fatalf("statuses: %v", codes)
			}
			var count int64
			if err := db.Model(&model.Payment{}).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("count: %d %v", count, err)
			}
		})
	}
}

func TestIdempotencyRetryAfterRollback(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	seedInvoice(t, db, "A101", "INV001", "2026-08-01", 100000, 0)
	if err := db.Exec("ALTER TABLE payment_allocations ADD CONSTRAINT reject_test_payment CHECK (amount_cents < 1)").Error; err != nil {
		t.Fatal(err)
	}
	body := `{"unit":"A101","amount_thb":100}`
	if w := keyedPayment(r, "retry-key", body); w.Code != 500 {
		t.Fatalf("expected failure: %d", w.Code)
	}
	if err := db.Exec("ALTER TABLE payment_allocations DROP CONSTRAINT reject_test_payment").Error; err != nil {
		t.Fatal(err)
	}
	if w := keyedPayment(r, "retry-key", body); w.Code != 201 {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	if w := keyedPayment(r, "retry-key", body); w.Code != 200 {
		t.Fatalf("replay: %d %s", w.Code, w.Body.String())
	}
}

func TestInvoiceCreationAndPaymentLocks(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	old := seedInvoice(t, db, "A101", "INV002", "2026-08-15", 10000, 0)
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	newer := model.Invoice{InvoiceNumber: "INV001", DueDate: old.DueDate.AddDate(0, 0, -14), TotalAmountCents: 10000}
	if err := repository.NewInvoiceRepository(tx).Create(context.Background(), &newer, "A101"); err != nil {
		t.Fatal(err)
	}
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- keyedPayment(r, "create-first", `{"unit":"A101","amount_thb":100}`) }()
	select {
	case <-result:
		t.Fatal("payment did not wait")
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-result:
		if w.Code != 201 {
			t.Fatalf("payment: %d %s", w.Code, w.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
	if got := readInvoice(t, r, newer.ID); got.Status != "PAID" {
		t.Fatalf("oldest skipped: %+v", got)
	}
	if got := readInvoice(t, r, old.ID); got.PaidAmountTHB != 0 {
		t.Fatalf("wrong invoice paid: %+v", got)
	}
	tx = db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if _, err := repository.NewPaymentRepository(tx).Allocate(context.Background(), "A101", 10000, "pay-first"); err != nil {
		t.Fatal(err)
	}
	last := model.Invoice{InvoiceNumber: "INV003", DueDate: old.DueDate, TotalAmountCents: 10000}
	created := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { created <- repository.NewInvoiceRepository(db).Create(ctx, &last, "A101") }()
	select {
	case err := <-created:
		t.Fatalf("creation did not wait: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-created:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
	if got := readInvoice(t, r, last.ID); got.Status != "UNPAID" {
		t.Fatalf("later invoice: %+v", got)
	}
}
