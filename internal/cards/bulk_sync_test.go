package cards

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParseRetryAfterSeconds(t *testing.T) {
	t.Parallel()

	got, ok := parseRetryAfter("120", time.Date(2026, time.April, 8, 12, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("parseRetryAfter() ok = false, want true")
	}
	if got != 120*time.Second {
		t.Fatalf("parseRetryAfter() = %v, want %v", got, 120*time.Second)
	}
}

func TestScryfallStatusErrorMaintenanceHTML(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header: http.Header{
			"Content-Type": []string{"text/html; charset=utf-8"},
		},
		Body: io.NopCloser(strings.NewReader(`<!DOCTYPE html><title>Offline for Maintenance</title>`)),
	}

	err := scryfallStatusError("bulk descriptor list request", resp)
	if err == nil {
		t.Fatal("scryfallStatusError() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "temporarily offline for maintenance") {
		t.Fatalf("scryfallStatusError() = %q, want maintenance message", err.Error())
	}
	if strings.Contains(strings.ToLower(err.Error()), "<html") {
		t.Fatalf("scryfallStatusError() leaked HTML body: %q", err.Error())
	}
}
