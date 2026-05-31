package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetHandlerServesPlaytestScript(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/playtest.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/javascript") {
		t.Fatalf("expected JavaScript content type, got %q", got)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "window.ManatombPlaytestConfig") {
		t.Fatal("expected playtest script to read the playtest config")
	}
	if strings.Contains(body, "{{") {
		t.Fatal("playtest script still contains template delimiters")
	}
}

func TestAssetHandlerRejectsUnsupportedMethods(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/assets/playtest.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rr.Code)
	}
}
