package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"billing-payment-api/internal/dto"
	"billing-payment-api/internal/handler"
	"billing-payment-api/internal/model"
	"billing-payment-api/internal/repository"
	"billing-payment-api/internal/router"
	"billing-payment-api/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// A separate committed schema allows real concurrent transactions on multiple
// connections. Only this uniquely named test schema is removed during cleanup.
func paymentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminSQL.Close() })
	schema := fmt.Sprintf("payment_test_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*config)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Unit{}, &model.Invoice{}, &model.InvoiceItem{}, &model.Payment{}, &model.PaymentAllocation{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func paymentTestRouter(db *gorm.DB) http.Handler {
	return router.SetupRouter(
		handler.NewHealthHandler(db),
		handler.NewInvoiceHandler(service.NewInvoiceService(repository.NewInvoiceRepository(db))),
		handler.NewPaymentHandler(service.NewPaymentService(repository.NewPaymentRepository(db))),
	)
}

func paymentRequest(r http.Handler, body string) *httptest.ResponseRecorder {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func seedInvoice(t *testing.T, db *gorm.DB, unit, number, due string, total, paid int64) model.Invoice {
	t.Helper()
	date, err := time.Parse("2006-01-02", due)
	if err != nil {
		t.Fatal(err)
	}
	invoice := model.Invoice{InvoiceNumber: number, DueDate: date, TotalAmountCents: total, PaidAmountCents: paid,
		InvoiceItems: []model.InvoiceItem{{Description: "Fee", AmountCents: total}},
	}
	if err := repository.NewInvoiceRepository(db).Create(&invoice, unit); err != nil {
		t.Fatal(err)
	}
	return invoice
}

func readInvoice(t *testing.T, r http.Handler, id uint) dto.InvoiceResponse {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/invoices/%d", id), nil))
	var invoice dto.InvoiceResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &invoice) != nil {
		t.Fatalf("GET invoice = %d: %s", w.Code, w.Body.String())
	}
	return invoice
}

func TestPaymentAllocation(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	// Insert in a different order from allocation priority. Equal due dates
	// must sort by invoice number, not insertion ID.
	third := seedInvoice(t, db, "A101", "INV003", "2026-08-15", 50000, 0)
	second := seedInvoice(t, db, "A101", "INV002", "2026-08-01", 50000, 0)
	first := seedInvoice(t, db, "A101", "INV001", "2026-08-01", 100000, 0)
	other := seedInvoice(t, db, "B202", "INV000-OTHER", "2026-07-01", 10000, 0)
	paid := seedInvoice(t, db, "A101", "INV000-PAID", "2026-07-01", 10000, 10000)
	zero := seedInvoice(t, db, "A101", "INV000-ZERO", "2026-07-01", 0, 0)

	w := paymentRequest(r, `{"unit":" A101 ","amount_thb":1200}`)
	var response dto.PaymentResponse
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatalf("payment = %d: %s", w.Code, w.Body.String())
	}
	if response.ID == 0 || response.Unit != "A101" || response.AmountTHB != 120000 || len(response.Allocations) != 2 {
		t.Fatalf("unexpected payment: %+v", response)
	}
	a, b := response.Allocations[0], response.Allocations[1]
	if a.InvoiceID != first.ID || a.AmountTHB != 100000 || a.PaidAmountTHB != 100000 || a.OutstandingAmountTHB != 0 || a.Status != "PAID" {
		t.Fatalf("first allocation: %+v", a)
	}
	if b.InvoiceID != second.ID || b.AmountTHB != 20000 || b.PaidAmountTHB != 20000 || b.OutstandingAmountTHB != 30000 || b.Status != "PARTIAL" {
		t.Fatalf("second allocation: %+v", b)
	}
	for _, tc := range []struct {
		id                uint
		paid, outstanding int64
		status            string
	}{
		{first.ID, 100000, 0, "PAID"}, {second.ID, 20000, 30000, "PARTIAL"},
		{third.ID, 0, 50000, "UNPAID"}, {other.ID, 0, 10000, "UNPAID"},
		{paid.ID, 10000, 0, "PAID"}, {zero.ID, 0, 0, "PAID"},
	} {
		got := readInvoice(t, r, tc.id)
		if int64(got.PaidAmountTHB) != tc.paid || int64(got.OutstandingAmountTHB) != tc.outstanding || got.Status != tc.status || len(got.Items) != 1 {
			t.Fatalf("invoice balance: %+v", got)
		}
	}
	// Subsequent partial payment resumes from the already-partially-paid invoice.
	w = paymentRequest(r, `{"unit":"A101","amount_thb":800.01}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("overpayment after partial payment: %d %s", w.Code, w.Body.String())
	}
	w = paymentRequest(r, `{"unit":"A101","amount_thb":0.29}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("partial payment: %d %s", w.Code, w.Body.String())
	}
	if got := readInvoice(t, r, second.ID); got.PaidAmountTHB != 20029 || got.OutstandingAmountTHB != 29971 {
		t.Fatalf("decimal payment: %+v", got)
	}
	w = paymentRequest(r, `{"unit":"A101","amount_thb":799.71}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("settlement: %d %s", w.Code, w.Body.String())
	}
	for _, id := range []uint{first.ID, second.ID, third.ID} {
		if got := readInvoice(t, r, id); got.Status != "PAID" || got.OutstandingAmountTHB != 0 {
			t.Fatalf("not settled: %+v", got)
		}
	}
	w = paymentRequest(r, `{"unit":"A101","amount_thb":1}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "no outstanding") {
		t.Fatalf("already paid: %d %s", w.Code, w.Body.String())
	}
	var payments []model.Payment
	if err := db.Preload("Allocations").Find(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if len(payments) != 3 {
		t.Fatalf("payment count = %d", len(payments))
	}
	for _, payment := range payments {
		var allocated int64
		for _, allocation := range payment.Allocations {
			allocated += allocation.AmountCents
		}
		if allocated != payment.AmountCents {
			t.Fatalf("unbalanced payment %d: %d != %d", payment.ID, allocated, payment.AmountCents)
		}
	}
	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/invoices", nil))
	var invoices []dto.InvoiceResponse
	if list.Code != http.StatusOK || json.Unmarshal(list.Body.Bytes(), &invoices) != nil {
		t.Fatalf("list: %s", list.Body.String())
	}
	for _, invoice := range invoices {
		if invoice.Unit == "A101" && (invoice.Status != "PAID" || invoice.OutstandingAmountTHB != 0) {
			t.Fatalf("stale list balance: %+v", invoice)
		}
	}
}

func TestPaymentInvalidAndOverpayment(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	invoice := seedInvoice(t, db, "A101", "INV001", "2026-08-01", 100000, 0)
	if err := db.Create(&model.Unit{UnitNumber: "EMPTY"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"missing unit", `{"amount_thb":1}`, 400},
		{"blank unit", `{"unit":"  ","amount_thb":1}`, 400},
		{"long unit", `{"unit":"` + strings.Repeat("A", 51) + `","amount_thb":1}`, 400},
		{"missing amount", `{"unit":"A101"}`, 400},
		{"null amount", `{"unit":"A101","amount_thb":null}`, 400},
		{"zero", `{"unit":"A101","amount_thb":0}`, 400},
		{"negative", `{"unit":"A101","amount_thb":-1}`, 400},
		{"precision", `{"unit":"A101","amount_thb":0.001}`, 400},
		{"string", `{"unit":"A101","amount_thb":"1"}`, 400},
		{"overflow", `{"unit":"A101","amount_thb":92233720368547758.08}`, 400},
		{"malformed", `{"unit":`, 400},
		{"unknown unit", `{"unit":"MISSING","amount_thb":1}`, 404},
		{"no invoices", `{"unit":"EMPTY","amount_thb":1}`, 409},
		{"overpayment", `{"unit":"A101","amount_thb":1000.01}`, 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := paymentRequest(r, tc.body)
			if w.Code != tc.code {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.code, w.Body.String())
			}
		})
	}
	if got := readInvoice(t, r, invoice.ID); got.PaidAmountTHB != 0 {
		t.Fatalf("rejected requests changed invoice: %+v", got)
	}
	for _, table := range []string{"payments", "payment_allocations"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("rejected requests left %d records in %s", count, table)
		}
	}
	// An exact full payment for one invoice is accepted.
	w := paymentRequest(r, `{"unit":"A101","amount_thb":1000}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("full payment: %d %s", w.Code, w.Body.String())
	}
	if got := readInvoice(t, r, invoice.ID); got.Status != "PAID" {
		t.Fatalf("not paid: %+v", got)
	}
}

func TestPaymentRollbackOnAllocationFailure(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	first := seedInvoice(t, db, "A101", "INV001", "2026-08-01", 10000, 0)
	second := seedInvoice(t, db, "A101", "INV002", "2026-08-15", 10000, 0)
	// Force the second ledger insert to fail, after the first balance and ledger
	// entry have already been written, to verify rollback of the whole payment.
	if err := db.Exec(fmt.Sprintf("ALTER TABLE payment_allocations ADD CONSTRAINT test_reject_second CHECK (invoice_id <> %d)", second.ID)).Error; err != nil {
		t.Fatal(err)
	}
	w := paymentRequest(r, `{"unit":"A101","amount_thb":150}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected failure: %d %s", w.Code, w.Body.String())
	}
	for _, id := range []uint{first.ID, second.ID} {
		if got := readInvoice(t, r, id); got.PaidAmountTHB != 0 {
			t.Fatalf("rollback failed: %+v", got)
		}
	}
	for _, table := range []string{"payments", "payment_allocations"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("rollback left %d records in %s", count, table)
		}
	}
}

func TestConcurrentPayments(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	invoice := seedInvoice(t, db, "A101", "INV001", "2026-08-01", 10000, 0)
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- paymentRequest(r, `{"unit":"A101","amount_thb":100}`) }()
	}
	close(start)
	codes := map[int]int{}
	for i := 0; i < 2; i++ {
		select {
		case w := <-results:
			codes[w.Code]++
		case <-time.After(15 * time.Second):
			t.Fatal("concurrent payments timed out")
		}
	}
	if codes[http.StatusCreated] != 1 || codes[http.StatusConflict] != 1 {
		t.Fatalf("concurrent results: %v", codes)
	}
	if got := readInvoice(t, r, invoice.ID); got.PaidAmountTHB != 10000 || got.Status != "PAID" {
		t.Fatalf("concurrent balance: %+v", got)
	}
	for _, table := range []string{"payments", "payment_allocations"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("expected one record in %s, got %d", table, count)
		}
	}
}

func TestExistingInvoiceBalanceMigration(t *testing.T) {
	db := paymentTestDB(t)
	invoice := seedInvoice(t, db, "A101", "INV001", "2026-08-01", 180000, 0)
	// Reproduce the pre-payment schema while preserving an existing invoice.
	if err := db.Migrator().DropColumn(&model.Invoice{}, "PaidAmountCents"); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Unit{}, &model.Invoice{}, &model.InvoiceItem{}, &model.Payment{}, &model.PaymentAllocation{}); err != nil {
		t.Fatal(err)
	}
	got := readInvoice(t, paymentTestRouter(db), invoice.ID)
	if got.TotalAmountTHB != 180000 || got.PaidAmountTHB != 0 || got.OutstandingAmountTHB != 180000 || got.Status != "UNPAID" || len(got.Items) != 1 {
		t.Fatalf("migration changed existing invoice: %+v", got)
	}
}

func TestGetInvoicesByUnitAndTHB(t *testing.T) {
	db := paymentTestDB(t)
	r := paymentTestRouter(db)
	seedInvoice(t, db, "A101", "INV001", "2026-08-01", 180000, 0)
	seedInvoice(t, db, "A101", "INV002", "2026-08-15", 50000, 0)
	seedInvoice(t, db, "B202", "INV003", "2026-08-01", 90000, 0)
	w := paymentRequest(r, `{"unit":"A101","amount_thb":599}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("payment: %d %s", w.Code, w.Body.String())
	}
	var payment map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &payment); err != nil {
		t.Fatal(err)
	}
	if string(payment["amount_thb"]) != "599.00" || payment["amount_cents"] != nil {
		t.Fatalf("wrong payment currency: %s", w.Body.String())
	}
	for _, tc := range []struct {
		query string
		count int
	}{
		{"?unit=A101", 2}, {"?unit=%20A101%20", 2}, {"?unit=B202", 1},
		{"?unit=MISSING", 0}, {"?unit=a101", 0}, {"", 3},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/invoices"+tc.query, nil))
		var invoices []map[string]json.RawMessage
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &invoices) != nil || len(invoices) != tc.count {
			t.Fatalf("%s: %d %s", tc.query, w.Code, w.Body.String())
		}
		if tc.count == 0 && strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatal("expected empty array")
		}
		if tc.query == "?unit=A101" {
			first := invoices[0]
			if string(first["total_amount_thb"]) != "1800.00" || string(first["paid_amount_thb"]) != "599.00" || string(first["outstanding_amount_thb"]) != "1201.00" || string(first["status"]) != `"PARTIAL"` {
				t.Fatalf("incorrect THB balances: %s", w.Body.String())
			}
			var items []map[string]json.RawMessage
			if err := json.Unmarshal(first["items"], &items); err != nil {
				t.Fatal(err)
			}
			if string(items[0]["amount_thb"]) != "1800.00" || strings.Contains(w.Body.String(), "amount_cents") {
				t.Fatalf("incorrect THB fields: %s", w.Body.String())
			}
		}
	}
}
