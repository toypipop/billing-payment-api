package handler_test

import (
	"billing-payment-api/internal/handler"
	"billing-payment-api/internal/model"
	"billing-payment-api/internal/repository"
	"billing-payment-api/internal/router"
	"billing-payment-api/internal/service"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPaymentDeadlineRollsBack(t *testing.T) {
	db := paymentTestDB(t)
	invoice := seedInvoice(t, db, "A101", "INV001", "2026-08-01", 10000, 0)
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Exec("SELECT id FROM units WHERE id = ? FOR UPDATE", invoice.UnitID).Error; err != nil {
		t.Fatal(err)
	}
	r := router.SetupRouter(handler.NewHealthHandler(db), handler.NewInvoiceHandler(service.NewInvoiceService(repository.NewInvoiceRepository(db))), handler.NewPaymentHandler(service.NewPaymentService(repository.NewPaymentRepository(db))), 100*time.Millisecond)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/payments", strings.NewReader(`{"unit":"A101","amount_thb":100}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "deadline-key")
	started := time.Now()
	r.ServeHTTP(w, req)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "REQUEST_TIMEOUT") || time.Since(started) > 2*time.Second {
		t.Fatalf("timeout: %d %s elapsed=%s", w.Code, w.Body.String(), time.Since(started))
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Payment{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("payment persisted: %d %v", count, err)
	}
	if got := keyedPayment(r, "deadline-key", `{"unit":"A101","amount_thb":100}`); got.Code != 201 {
		t.Fatalf("retry: %d %s", got.Code, got.Body.String())
	}
}

func TestPaymentDeadlineAfterWritesRollsBack(t *testing.T) {
	db := paymentTestDB(t)
	invoice := seedInvoice(t, db, "A101", "INV001", "2026-08-01", 10000, 0)
	// Allocation insertion happens after the payment insert and balance update.
	for _, sql := range []string{
		`CREATE FUNCTION delay_allocation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(2); RETURN NEW; END $$`,
		`CREATE TRIGGER delay_allocation BEFORE INSERT ON payment_allocations FOR EACH ROW EXECUTE FUNCTION delay_allocation()`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := router.SetupRouter(handler.NewHealthHandler(db), handler.NewInvoiceHandler(service.NewInvoiceService(repository.NewInvoiceRepository(db))), handler.NewPaymentHandler(service.NewPaymentService(repository.NewPaymentRepository(db))), 200*time.Millisecond)
	w := keyedPayment(r, "partial-deadline", `{"unit":"A101","amount_thb":100}`)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "REQUEST_TIMEOUT") {
		t.Fatalf("timeout: %d %s", w.Code, w.Body.String())
	}
	for _, table := range []string{"payments", "payment_allocations"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s: count=%d err=%v", table, count, err)
		}
	}
	var got model.Invoice
	if err := db.First(&got, invoice.ID).Error; err != nil || got.PaidAmountCents != 0 {
		t.Fatalf("balance after timeout: %+v %v", got, err)
	}
	if err := db.Exec("DROP TRIGGER delay_allocation ON payment_allocations").Error; err != nil {
		t.Fatal(err)
	}
	if retry := keyedPayment(r, "partial-deadline", `{"unit":"A101","amount_thb":100}`); retry.Code != 201 {
		t.Fatalf("retry: %d %s", retry.Code, retry.Body.String())
	}
}
