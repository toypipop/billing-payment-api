package handler_test

import (
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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCreateInvoiceUnitLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	schema := fmt.Sprintf("invoice_test_%d", time.Now().UnixNano())
	if err := tx.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SET LOCAL search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.AutoMigrate(&model.Unit{}, &model.Invoice{}, &model.InvoiceItem{}); err != nil {
		t.Fatal(err)
	}
	r := router.SetupRouter(handler.NewHealthHandler(tx), handler.NewInvoiceHandler(service.NewInvoiceService(repository.NewInvoiceRepository(tx))))
	empty := httptest.NewRecorder()
	r.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/invoices", nil))
	if empty.Code != http.StatusOK || strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Fatalf("empty list = %d: %s", empty.Code, empty.Body.String())
	}
	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	payload := `{"unit":"A101","due_date":"2026-08-01","items":[{"description":"Common Fee","amount":1500},{"description":"Water Fee","amount":300}]}`
	first := post(payload)
	if first.Code != http.StatusCreated {
		t.Fatalf("POST = %d: %s", first.Code, first.Body.String())
	}
	var result dto.InvoiceResponse
	if err := json.Unmarshal(first.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.TotalAmountCents != 180000 || len(result.Items) != 2 || result.Items[0].AmountCents != 150000 || result.InvoiceNumber == "" || result.DueDate.Format("2006-01-02") != "2026-08-01" {
		t.Fatalf("unexpected invoice: %+v", result)
	}
	var unit model.Unit
	if err := tx.First(&unit, result.UnitID).Error; err != nil {
		t.Fatal(err)
	}
	if unit.UnitNumber != "A101" {
		t.Fatalf("unit = %+v", unit)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := tx.Model(&unit).Update("updated_at", oldTime).Error; err != nil {
		t.Fatal(err)
	}
	second := post(payload)
	if second.Code != http.StatusCreated {
		t.Fatalf("second POST: %s", second.Body.String())
	}
	var repeated dto.InvoiceResponse
	if err := json.Unmarshal(second.Body.Bytes(), &repeated); err != nil {
		t.Fatal(err)
	}
	if repeated.UnitID != result.UnitID || repeated.ID == result.ID || repeated.InvoiceNumber == result.InvoiceNumber {
		t.Fatalf("unexpected repeated invoice: %+v", repeated)
	}
	if err := tx.First(&unit, result.UnitID).Error; err != nil {
		t.Fatal(err)
	}
	if !unit.UpdatedAt.After(oldTime) {
		t.Fatal("unit UpdatedAt not refreshed")
	}
	third := post(strings.Replace(payload, "A101", "B202", 1))
	if third.Code != http.StatusCreated {
		t.Fatalf("new unit POST: %s", third.Body.String())
	}
	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/invoices", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", list.Code, list.Body.String())
	}
	var invoices []dto.InvoiceResponse
	if err := json.Unmarshal(list.Body.Bytes(), &invoices); err != nil {
		t.Fatal(err)
	}
	if len(invoices) != 3 {
		t.Fatalf("invoice count = %d", len(invoices))
	}
	for i, invoice := range invoices {
		if len(invoice.Items) != 2 || invoice.TotalAmountCents != 180000 || invoice.Items[0].Description != "Common Fee" || invoice.Items[0].AmountCents != 150000 {
			t.Fatalf("incomplete invoice: %+v", invoice)
		}
		if i > 0 && invoices[i-1].ID >= invoice.ID {
			t.Fatal("invoices are not ordered by ID")
		}
	}
	if invoices[0].ID != result.ID || invoices[1].ID != repeated.ID {
		t.Fatal("list is missing created invoices")
	}
	var count int64
	if err := tx.Model(&model.Unit{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("unit count = %d", count)
	}
	for _, amount := range []string{"0", "0.29", "1500.50"} {
		w := post(`{"unit":"A101","due_date":"2026-08-01","items":[{"description":"Fee","amount":` + amount + `}]}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("amount %s: %d %s", amount, w.Code, w.Body.String())
		}
		var got dto.InvoiceResponse
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]int64{"0": 0, "0.29": 29, "1500.50": 150050}[amount]
		if got.TotalAmountCents != want {
			t.Fatalf("amount %s = %d cents", amount, got.TotalAmountCents)
		}
	}
	for _, body := range []string{
		strings.Replace(payload, "2026-08-01", "2026-02-30", 1),
		strings.Replace(payload, "A101", "   ", 1),
		`{"unit":"X","due_date":"2026-08-01","items":[]}`,
		`{"unit":"X","due_date":"2026-08-01","items":[{"description":"Fee"}]}`,
		`{"unit":"X","due_date":"2026-08-01","items":[{"description":"Fee","amount":-1}]}`,
		`{"unit":"X","due_date":"2026-08-01","items":[{"description":"Fee","amount":0.001}]}`,
		`{"unit":"X","due_date":"2026-08-01","items":[{"description":"Fee","amount":92233720368547758.08}]}`,
		`{"unit":"X","due_date":"2026-08-01","items":[{"description":"Fee","amount":92233720368547758.07},{"description":"Fee","amount":1}]}`,
	} {
		w := post(body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("invalid request = %d: %s", w.Code, w.Body.String())
		}
	}
	if err := tx.Model(&model.Unit{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("invalid requests created units: %d", count)
	}

	// A duplicate invoice number must roll back both new rooms and updates to existing rooms.
	repo := repository.NewInvoiceRepository(tx)
	for _, room := range []string{"ROLLBACK", "A101"} {
		if err := tx.Model(&model.Unit{}).Where("unit_number = ?", "A101").Update("updated_at", oldTime).Error; err != nil {
			t.Fatal(err)
		}
		duplicate := model.Invoice{InvoiceNumber: result.InvoiceNumber, DueDate: result.DueDate}
		if err := repo.Create(&duplicate, room); err == nil {
			t.Fatal("expected duplicate invoice number error")
		}
		if err := tx.Model(&model.Unit{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("failed invoice left a room behind: %d", count)
		}
		if err := tx.First(&unit, result.UnitID).Error; err != nil {
			t.Fatal(err)
		}
		if !unit.UpdatedAt.Equal(oldTime) {
			t.Fatal("failed invoice changed room UpdatedAt")
		}
	}
}
