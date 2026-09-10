package database_test

import (
	"billing-payment-api/internal/database"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestPoolAndDatabaseTimeouts(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	schema := fmt.Sprintf("pool_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	uri, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := uri.Query()
	query.Set("search_path", schema)
	uri.RawQuery = query.Encode()
	t.Setenv("DATABASE_URL", uri.String())
	t.Setenv("DB_MAX_OPEN_CONNS", "1")
	t.Setenv("DB_MAX_IDLE_CONNS", "0")
	t.Setenv("DB_STATEMENT_TIMEOUT", "500ms")
	t.Setenv("DB_LOCK_TIMEOUT", "100ms")
	db, err := database.Connect()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	held, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	_, err = sqlDB.ExecContext(waitCtx, "SELECT 1")
	stop()
	held.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pool wait: %v", err)
	}
	// Force new physical connections: each must inherit timeouts.
	for i := 0; i < 2; i++ {
		_, err = sqlDB.ExecContext(ctx, "SELECT pg_sleep(2)")
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "57014" {
			t.Fatalf("statement timeout: %v", err)
		}
	}
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "LOCK TABLE "+schema+".units IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	_, err = sqlDB.ExecContext(ctx, "SELECT * FROM units")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("lock timeout: %v", err)
	}
}
