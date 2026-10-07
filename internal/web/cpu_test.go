package web

import (
	"context"
	"io"
	"manatomb/app/internal/account"
	"manatomb/app/internal/forge"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCPUGatewayOwnershipOriginAndUnavailable(t *testing.T) {
	secret := "local-gateway-shared-secret-at-least-32"
	calls := 0
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-ManaTomb-Owner") != "42" || r.Header.Get("Authorization") != "Bearer "+secret {
			t.Error("incorrect trusted engine identity")
		}
		if r.URL.Path != "/v1/sessions/test/actions" {
			t.Error(r.URL.Path)
		}
		io.WriteString(w, `{"accepted":true}`)
	}))
	client, err := forge.NewClient(true, engine.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Forge: client, PublicBaseURL: "http://site.test"}
	call := func(owner bool, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/cpu/sessions/test/actions", strings.NewReader(`{"action":"ok"}`))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-ManaTomb-Owner", "999")
		if owner {
			r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser, &account.User{ID: 42}))
		}
		w := httptest.NewRecorder()
		app.CPUAPI(w, r)
		return w
	}
	if call(false, "http://site.test").Code != 401 {
		t.Fatal("anonymous admitted")
	}
	if call(true, "https://attacker.test").Code != 403 {
		t.Fatal("cross-site mutation admitted")
	}
	if calls != 0 {
		t.Fatal("untrusted request reached engine")
	}
	if call(true, "http://site.test").Code != 200 {
		t.Fatal("authorized proxy failed")
	}
	engine.Close()
	if call(true, "http://site.test").Code != 503 {
		t.Fatal("engine outage not contained")
	}
	app.Forge = nil
	if call(true, "http://site.test").Code != 503 {
		t.Fatal("disabled flag bypassed")
	}
}
func TestCPUDisabledConfigurationDoesNotExposePage(t *testing.T) {
	app := &App{}
	r := httptest.NewRequest("GET", "/cpu", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser, &account.User{ID: 1}))
	w := httptest.NewRecorder()
	app.CPUPage(w, r)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("GET", "/api/cpu/config", nil).WithContext(r.Context())
	w = httptest.NewRecorder()
	app.CPUAPI(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestCPUNotFoundRemainsJSONThroughSiteMiddleware(t *testing.T) {
	app := &App{}
	handler := app.WithNotFoundMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { cpuError(w, 404, "no session") }))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/cpu/sessions", nil))
	if w.Code != 404 || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || !strings.Contains(w.Body.String(), "no session") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestCPUDefaultInspectionThroughRealClient(t *testing.T) {
	secret := "local-gateway-shared-secret-at-least-32"
	calls := 0
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/defaults/feline-ferocity" || r.Method != "GET" {
			t.Error("Unexpected path or method")
		}
		io.WriteString(w, `{"name":"Feline Ferocity","commanders":[],"main":[]}`)
	}))
	defer engine.Close()
	client, err := forge.NewClient(true, engine.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Forge: client, PublicBaseURL: "http://site.test"}
	r := httptest.NewRequest("GET", "/api/cpu/defaults/feline-ferocity", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser, &account.User{ID: 42}))
	w := httptest.NewRecorder()
	app.CPUAPI(w, r)
	if w.Code != 200 || calls != 1 || !strings.Contains(w.Body.String(), "Feline Ferocity") {
		t.Fatal(w.Code, w.Body.String(), calls)
	}
	if _, _, err = client.Request(context.Background(), 42, "GET", "/v1/defaults/../../secret", nil); err == nil {
		t.Fatal("Unsafe default path accepted")
	}
}

func TestCPUPrivateResponsesAndSignInReturn(t *testing.T) {
	app := &App{}
	r := httptest.NewRequest("GET", "/cpu", nil)
	w := httptest.NewRecorder()
	app.CPUPage(w, r)
	if w.Code != 303 || w.Header().Get("Location") != "/login?next=%2Fcpu" {
		t.Fatal(w.Code, w.Header())
	}
	for _, path := range []string{"/api/cpu/config", "/api/cpu/decks", "/api/cpu/decks/1"} {
		w = httptest.NewRecorder()
		app.CPUAPI(w, httptest.NewRequest("GET", path, nil))
		if w.Header().Get("Cache-Control") != "no-store" || w.Code != 401 {
			t.Fatal(path, w.Code, w.Header())
		}
	}
}
