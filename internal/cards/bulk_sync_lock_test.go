package cards

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
)

func TestCardSyncLockPostgresPoolOwnership(t *testing.T) {
	url := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set MANATOMB_TEST_DATABASE_URL to a disposable local PostgreSQL database")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(3)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	failure := errors.New("simulated download failure")
	err = withCardSyncLock(ctx, db, func() error {
		// A second sync sharing the pool must not reuse/reenter the lock owner's
		// PostgreSQL session. Ordinary database work must remain available.
		var n int
		if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
			t.Fatal(err)
		}
		called := false
		nested := withCardSyncLock(ctx, db, func() error { called = true; return nil })
		if !errors.Is(nested, ErrCardSyncInProgress) || called {
			t.Fatalf("overlapping sync admitted: %v", nested)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	// Both a callback failure and a canceled sync must unlock the same session.
	syncCtx, cancelSync := context.WithCancel(ctx)
	if err := withCardSyncLock(syncCtx, db, func() error { cancelSync(); return syncCtx.Err() }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := withCardSyncLock(ctx, db, func() error { return nil }); err != nil {
		t.Fatalf("lock leaked after failure/cancellation: %v", err)
	}
}

func TestCardSyncRetryDelay(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want time.Duration
	}{
		{nil, 24 * time.Hour}, {ErrCardSyncInProgress, time.Minute}, {errors.New("download failed"), 5 * time.Minute},
	} {
		if got := cardBulkSyncNextDelay(24*time.Hour, tc.err); got != tc.want {
			t.Fatalf("delay=%s want=%s", got, tc.want)
		}
	}
}
