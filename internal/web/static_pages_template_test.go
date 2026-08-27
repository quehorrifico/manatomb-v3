package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func staticPageMain(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `<div class="mt-static-page`)
	if start == -1 {
		t.Fatalf("could not isolate static-page content: %s", body)
	}
	end := strings.Index(body[start:], `<footer id="site-footer"`)
	if end < 0 {
		t.Fatalf("could not isolate static-page content: %s", body)
	}
	return body[start : start+end]
}

func TestStaticPagesUseSharedFlatLayoutAndMetadata(t *testing.T) {
	tests := []struct {
		name      string
		pageID    string
		title     string
		headingID string
	}{
		{name: "privacy", pageID: "privacy", title: "Privacy Notice", headingID: ""},
		{name: "terms", pageID: "terms", title: "Terms of Use", headingID: ""},
		{name: "rules_home", pageID: "rules-home", title: "Rules", headingID: `id="rules-title"`},
		{name: "error", pageID: "error", title: "Something Went Wrong", headingID: `id="error-title"`},
		{name: "not_found", pageID: "not-found", title: "Page Not Found", headingID: `id="not-found-title"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := renderTemplate(t, tt.name, TemplateData{})
			main := staticPageMain(t, body)

			for _, needle := range []string{
				`data-page="` + tt.pageID + `"`,
				`href="/assets/static_pages.css"`,
				`<title>` + tt.title + ` | ManaTomb</title>`,
				`<meta name="description" content=`,
				`class="mt-static-page`,
				`class="mt-static-title"`,
			} {
				if !strings.Contains(body, needle) {
					t.Fatalf("%s page missing %q: %s", tt.name, needle, body)
				}
			}
			if tt.headingID != "" && !strings.Contains(main, tt.headingID) {
				t.Fatalf("%s page missing accessible heading id %q: %s", tt.name, tt.headingID, main)
			}
			if got := strings.Count(body, `href="/assets/static_pages.css"`); got != 1 {
				t.Fatalf("%s page rendered %d static stylesheets, want one", tt.name, got)
			}
			for _, forbidden := range []string{"mt-panel", "mt-kicker", "text-slate-", "bg-slate-", "border-slate-"} {
				if strings.Contains(main, forbidden) {
					t.Fatalf("%s page retained legacy or nested presentation %q: %s", tt.name, forbidden, main)
				}
			}
		})
	}
}

func TestStaticDocumentsRetainContentWithoutNestedPanels(t *testing.T) {
	privacy := staticPageMain(t, renderTemplate(t, "privacy", TemplateData{}))
	for _, needle := range []string{
		`class="mt-static-document"`,
		`What ManaTomb stores`,
		`Cookies and browser storage`,
		`Stay signed in on this device`,
		`up to 30 days`,
		`Third-party services`,
		`What this project does not currently do`,
		`Your controls`,
		`href="/settings" class="mt-static-link"`,
	} {
		if !strings.Contains(privacy, needle) {
			t.Fatalf("privacy page lost %q: %s", needle, privacy)
		}
	}

	terms := staticPageMain(t, renderTemplate(t, "terms", TemplateData{}))
	for _, needle := range []string{
		`Project status`,
		`Accounts and public decks`,
		`Card data and intellectual property`,
		`No warranty`,
		`Support`,
		`target="_blank" rel="noopener noreferrer"`,
		`opens in a new tab`,
	} {
		if !strings.Contains(terms, needle) {
			t.Fatalf("terms page lost %q: %s", needle, terms)
		}
	}
}

func TestStaticMessagePagesKeepOnlyContextualActions(t *testing.T) {
	rules := staticPageMain(t, renderTemplate(t, "rules_home", TemplateData{}))
	if strings.Contains(rules, `<nav`) || strings.Contains(rules, `<a `) || strings.Contains(rules, `mt-btn`) {
		t.Fatalf("rules placeholder duplicates shared navigation: %s", rules)
	}

	for _, name := range []string{"error", "not_found"} {
		main := staticPageMain(t, renderTemplate(t, name, TemplateData{}))
		for _, needle := range []string{
			`class="mt-static-actions"`,
			`onclick="window.history.back()"`,
			`>Go back</button>`,
			`href="/" class="mt-btn mt-btn--primary">Return home</a>`,
		} {
			if !strings.Contains(main, needle) {
				t.Fatalf("%s recovery page missing %q: %s", name, needle, main)
			}
		}
	}
}

func TestStaticPagesStylesheetIsServedAndTokenDriven(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/static_pages.css", nil)
	rec := httptest.NewRecorder()
	AssetHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("static stylesheet status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/css") {
		t.Fatalf("static stylesheet content type = %q", got)
	}
	css := rec.Body.String()
	for _, needle := range []string{
		`.mt-static-page {`,
		`.mt-static-section {`,
		`color: var(--mt-text-soft);`,
		`color: var(--mt-accent-text);`,
		`outline: 2px solid var(--mt-focus);`,
		`@media (max-width: 639px)`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("static stylesheet missing %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "#", "animation:", "transition:"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("static stylesheet hardcodes color or motion %q", forbidden)
		}
	}
}
