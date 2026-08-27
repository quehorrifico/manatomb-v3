package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRobotsTXTUsesConfiguredOriginAndProtectsPrivateTools(t *testing.T) {
	app := &App{PublicBaseURL: "https://manatomb.app"}
	req := httptest.NewRequest(http.MethodGet, "https://attacker.example/robots.txt", nil)
	rec := httptest.NewRecorder()

	app.HandleRobotsTXT(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("robots status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("robots content type = %q", got)
	}
	body := rec.Body.String()
	for _, needle := range []string{
		"User-agent: *",
		"Allow: /",
		"Disallow: /settings",
		"Disallow: /decks/new",
		"Disallow: /cards/autocomplete",
		"Sitemap: https://manatomb.app/sitemap.xml",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("robots response missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, "attacker.example") {
		t.Fatalf("robots response trusted request host: %s", body)
	}
}

func TestSitemapXMLListsStablePublicPages(t *testing.T) {
	app := &App{PublicBaseURL: "https://manatomb.app"}
	req := httptest.NewRequest(http.MethodGet, "https://attacker.example/sitemap.xml", nil)
	rec := httptest.NewRecorder()

	app.HandleSitemapXML(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("sitemap status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/xml") {
		t.Fatalf("sitemap content type = %q", got)
	}
	body := rec.Body.String()
	for _, path := range sitemapPublicPaths {
		needle := "<loc>https://manatomb.app" + path + "</loc>"
		if !strings.Contains(body, needle) {
			t.Fatalf("sitemap missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, "/settings") || strings.Contains(body, "attacker.example") {
		t.Fatalf("sitemap exposed a private or untrusted URL: %s", body)
	}
}

func TestSEOEndpointsRejectWrites(t *testing.T) {
	app := &App{PublicBaseURL: "https://manatomb.app"}
	for _, handler := range []http.HandlerFunc{app.HandleRobotsTXT, app.HandleSitemapXML} {
		req := httptest.NewRequest(http.MethodPost, "https://manatomb.app/", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("SEO write response = %d Allow=%q", rec.Code, rec.Header().Get("Allow"))
		}
	}
}
