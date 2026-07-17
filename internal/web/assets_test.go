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

func TestAssetHandlerServesDeckBrowserAssets(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/assets/deck_browser.js", contentType: "text/javascript", contains: "global.ManatombDeckBrowser"},
		{path: "/assets/public_deck.js", contentType: "text/javascript", contains: "ManatombPublicDeckConfig"},
		{path: "/assets/public_deck.css", contentType: "text/css", contains: ".mt-public-single"},
		{path: "/assets/profile.js", contentType: "text/javascript", contains: "profile-deck-filter"},
		{path: "/assets/profile.css", contentType: "text/css", contains: ".mt-profile-hero"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			AssetHandler().ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rr.Code)
			}
			if got := rr.Header().Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Fatalf("expected %s content type, got %q", tt.contentType, got)
			}
			if !strings.Contains(rr.Body.String(), tt.contains) {
				t.Fatalf("expected asset to contain %q", tt.contains)
			}
		})
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
