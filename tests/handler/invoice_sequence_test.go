package handler_test

import (
	"billing-payment-api/internal/database"
	"billing-payment-api/internal/model"
	"billing-payment-api/internal/repository"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestInvoiceSequenceConcurrentAndRollback(t *testing.T) {
	db := paymentTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type result struct {
		number string
		err    error
	}
	results := make(chan result, 8)
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(i int) {
			<-start
			invoice := model.Invoice{DueDate: time.Now(), TotalAmountCents: 10000}
			err := repository.NewInvoiceRepository(db).Create(ctx, &invoice, fmt.Sprintf("U%d", i))
			results <- result{invoice.InvoiceNumber, err}
		}(i)
	}
	close(start)
	seen := map[string]bool{}
	for i := 0; i < 8; i++ {
		select {
		case got := <-results:
			if got.err != nil || seen[got.number] {
				t.Fatalf("duplicate or failed creation: %+v", got)
			}
			seen[got.number] = true
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for i := 1; i <= 8; i++ {
		if !seen[fmt.Sprintf("INV-%010d", i)] {
			t.Fatalf("missing sequence number %d: %v", i, seen)
		}
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	rolledBack := model.Invoice{DueDate: time.Now(), TotalAmountCents: 10000}
	if err := repository.NewInvoiceRepository(tx).Create(ctx, &rolledBack, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateInvoiceSequence(db); err != nil {
		t.Fatal(err)
	}
	next := model.Invoice{DueDate: time.Now(), TotalAmountCents: 10000}
	if err := repository.NewInvoiceRepository(db).Create(ctx, &next, "NEXT"); err != nil {
		t.Fatal(err)
	}
	if rolledBack.InvoiceNumber != "INV-0000000009" || next.InvoiceNumber != "INV-0000000010" {
		t.Fatalf("sequence reset or reused: %s %s", rolledBack.InvoiceNumber, next.InvoiceNumber)
	}
}

func TestInvoiceSequenceMigrationPreservesExisting(t *testing.T) {
	db := paymentTestDB(t)
	old := seedInvoice(t, db, "OLD", "INV-0000000042", "2026-08-01", 10000, 0)
	legacy := seedInvoice(t, db, "OLD", "INV-LEGACY-RANDOM", "2026-08-01", 10000, 0)
	if err := db.Exec("DROP SEQUENCE invoice_number_seq").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateInvoiceSequence(db); err != nil {
		t.Fatal(err)
	}
	next := model.Invoice{DueDate: time.Now(), TotalAmountCents: 10000}
	if err := repository.NewInvoiceRepository(db).Create(context.Background(), &next, "NEW"); err != nil {
		t.Fatal(err)
	}
	if next.InvoiceNumber != "INV-0000000043" {
		t.Fatalf("next number: %s", next.InvoiceNumber)
	}
	for _, want := range []model.Invoice{old, legacy} {
		var got model.Invoice
		if err := db.First(&got, want.ID).Error; err != nil || got.InvoiceNumber != want.InvoiceNumber {
			t.Fatalf("existing number changed: %+v %v", got, err)
		}
	}
}
