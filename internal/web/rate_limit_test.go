package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFixedWindowRateLimiter(t *testing.T) {
	t.Parallel()

	limiter := newFixedWindowRateLimiter(2, time.Minute)
	now := time.Date(2026, time.June, 6, 12, 0, 0, 0, time.UTC)
	if allowed, _ := limiter.allow("client", now); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, _ := limiter.allow("client", now.Add(time.Second)); !allowed {
		t.Fatal("second request should be allowed")
	}
	if allowed, retryAfter := limiter.allow("client", now.Add(2*time.Second)); allowed || retryAfter <= 0 {
		t.Fatalf("third request = allowed %t retry %s", allowed, retryAfter)
	}
	if allowed, _ := limiter.allow("client", now.Add(time.Minute)); !allowed {
		t.Fatal("request after the window should be allowed")
	}
}

func TestRateLimitMiddlewareProtectsForgotPassword(t *testing.T) {
	t.Parallel()

	app := &App{}
	handler := app.WithRateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for attempt := 1; attempt <= 6; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "http://example.test/forgot-password", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		want := http.StatusNoContent
		if attempt == 6 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", attempt, rec.Code, want)
		}
	}
}

func TestRequestClientIPOnlyUsesConfiguredTrustedProxyHops(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req.RemoteAddr = "10.0.0.8:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.20, 203.0.113.40")

	if got := requestClientIP(req, 0); got != "10.0.0.8" {
		t.Fatalf("untrusted forwarded address = %q", got)
	}
	if got := requestClientIP(req, 1); got != "203.0.113.40" {
		t.Fatalf("one trusted proxy address = %q", got)
	}
	if got := requestClientIP(req, 2); got != "198.51.100.20" {
		t.Fatalf("two trusted proxies address = %q", got)
	}
	if got := requestClientIP(req, 3); got != "10.0.0.8" {
		t.Fatalf("missing trusted proxy address = %q", got)
	}
}
