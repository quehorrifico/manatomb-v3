package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	app := &App{SessionCookieSecure: true}
	handler := app.WithSecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "https://manatomb.app/", nil))

	for name, want := range map[string]string{
		"Content-Security-Policy":   "base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'",
		"Permissions-Policy":        "camera=(), geolocation=(), microphone=()",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Strict-Transport-Security": "max-age=31536000",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
	} {
		if got := recorder.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestSecurityHeadersOmitHSTSForLocalHTTP(t *testing.T) {
	app := &App{}
	handler := app.WithSecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/", nil))
	if got := recorder.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("local HTTP response unexpectedly enabled HSTS: %q", got)
	}
}

func TestSecurityHeadersRejectCrossOriginWrites(t *testing.T) {
	app := &App{PublicBaseURL: "https://manatomb.app"}
	called := false
	handler := app.WithSecurityHeadersMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	request := httptest.NewRequest(http.MethodPost, "https://manatomb.app/settings", nil)
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || called {
		t.Fatalf("cross-origin write status=%d called=%t", recorder.Code, called)
	}
}

func TestSecurityHeadersAllowSameOriginWrites(t *testing.T) {
	app := &App{PublicBaseURL: "https://manatomb.app"}
	called := false
	handler := app.WithSecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "https://manatomb.app/settings", nil)
	request.Header.Set("Origin", "https://manatomb.app")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("same-origin write status=%d called=%t", recorder.Code, called)
	}
}

func TestSecurityHeadersPreventCachingSignedInPages(t *testing.T) {
	app := &App{}
	handler := app.WithSecurityHeadersMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	request := httptest.NewRequest(http.MethodGet, "https://manatomb.app/settings", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("signed-in cache control = %q", got)
	}
	if got := recorder.Header().Values("Vary"); len(got) == 0 || got[0] != "Cookie" {
		t.Fatalf("signed-in Vary headers = %#v", got)
	}
}
