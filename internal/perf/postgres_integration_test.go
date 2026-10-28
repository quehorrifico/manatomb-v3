package perf_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
	"manatomb/app/internal/perf"
)

func TestInstrumentedPostgres(t *testing.T) {
	dsn := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MANATOMB_TEST_DATABASE_URL is not set")
	}
	c, err := perf.PostgresConnector(dsn)
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(c)
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "CREATE TEMP TABLE perf_copy_test (n INTEGER) ON COMMIT DROP"); err != nil {
		t.Fatal(err)
	}
	s, err := tx.PrepareContext(ctx, pq.CopyIn("perf_copy_test", "n"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := s.ExecContext(ctx, i); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ExecContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.PrepareContext(ctx, "SELECT n FROM perf_copy_test WHERE n >= $1 ORDER BY n")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := stmt.QueryContext(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	types, err := rows.ColumnTypes()
	if err != nil || len(types) != 1 || types[0].DatabaseTypeName() != "INT4" {
		t.Fatalf("column metadata: %v, %v", types, err)
	}
	count := 0
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("read %d rows", count)
	}
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// Exercise slow-query timing, cancellation, and subsequent pool reuse.
	if _, err := db.ExecContext(ctx, "SELECT pg_sleep(0.11)"); err != nil {
		t.Fatal(err)
	}
	deadline, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if _, err := db.ExecContext(deadline, "SELECT pg_sleep(1)"); err == nil {
		t.Fatal("expected canceled query")
	}
	var one int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("pool reuse: %d, %v", one, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/perf", func(w http.ResponseWriter, r *http.Request) {
		defer perf.Stage(r.Context(), "test_page")()
		if err := db.QueryRowContext(r.Context(), "SELECT 42").Scan(&one); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	perf.Requests(mux, mux, db).ServeHTTP(rec, httptest.NewRequest("GET", "/perf", nil))
	if rec.Code != http.StatusNoContent || rec.Header().Get("X-Request-ID") == "" {
		t.Fatalf("instrumented request: %v", rec)
	}
}
