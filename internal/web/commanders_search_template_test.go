package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func commanderSearchTemplateData(query, returnTo string, results, recommended []searchResult) any {
	return struct {
		Query       string
		Results     []searchResult
		Recommended []searchResult
		ReturnTo    string
		ReturnLabel string
	}{
		Query:       query,
		Results:     results,
		Recommended: recommended,
		ReturnTo:    returnTo,
		ReturnLabel: "Back to Deck Settings",
	}
}

func TestCommanderSearchUsesFlatTokenDrivenPicker(t *testing.T) {
	result := searchResult{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		ScryfallID: "223e4567-e89b-12d3-a456-426614174000",
		Name:       "Atraxa, Grand Unifier",
		TypeLine:   "Legendary Creature — Phyrexian Angel",
		ImageURI:   "https://example.test/atraxa.jpg",
	}
	body := renderTemplate(t, "commanders_search", TemplateData{
		Data: commanderSearchTemplateData("", "/decks/settings?id=42", nil, []searchResult{result}),
	})

	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("commander search did not receive the Tomb default: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="commanders-search"`) {
		t.Fatalf("commander search body is missing its page scope: %s", bodyTag)
	}

	themeIndex := strings.Index(body, `href="/assets/theme.css"`)
	pageCSSIndex := strings.Index(body, `href="/assets/commanders_search.css"`)
	if themeIndex < 0 || pageCSSIndex <= themeIndex {
		t.Fatal("commander search stylesheet must load after the shared theme stylesheet")
	}

	for _, needle := range []string{
		`class="mt-commander-browser"`,
		`href="/decks/settings?id=42">← Back to Deck Settings</a>`,
		`action="/commanders/search" class="mt-commander-browser__search" role="search"`,
		`name="return_to" value="/decks/settings?id=42"`,
		`id="commander-browser-search"`,
		`data-card-autocomplete-commander-only="true"`,
		`id="commander-browser-recommended-title">Recommended commanders</h2>`,
		`class="mt-commander-browser__grid"`,
		`data-scryfall-id="223e4567-e89b-12d3-a456-426614174000"`,
		`aria-label="View Atraxa, Grand Unifier and choose a printing"`,
		`id="card-detail-modal"`,
		`name="commander_print_id" value="" data-field="sticky-print-input"`,
		`name="return_to" value="/decks/settings?id=42"`,
		`class="mt-btn mt-btn--primary mt-btn--sm">`,
		`Select Commander`,
		`stickyPrintInputEl.value = trimValue(version.scryfall_id)`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("commander search missing %q", needle)
		}
	}

	if got := strings.Count(body, "<main"); got != 1 {
		t.Fatalf("commander search rendered %d main landmarks, want one", got)
	}
	for _, forbidden := range []string{
		`class="mt-panel`,
		`class="mt-action-card`,
		`text-slate-`,
		`border-slate-`,
		`shadow-slate-`,
		`href="/" class="mt-btn`,
		`>Home</a>`,
		`>Commander</span>`,
		`>Duel Commander</span>`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("commander search retained legacy presentation %q", forbidden)
		}
	}
}

func TestCommanderSearchKeepsResultsRecommendationsAndReturnState(t *testing.T) {
	result := searchResult{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		ScryfallID: "223e4567-e89b-12d3-a456-426614174000",
		Name:       "Atraxa, Grand Unifier",
	}
	body := renderTemplate(t, "commanders_search", TemplateData{
		Data: commanderSearchTemplateData("Atraxa", "/decks/settings?id=42", []searchResult{result}, []searchResult{result}),
	})

	for _, needle := range []string{
		`id="commander-browser-results-title">Search results</h2>`,
		`1 matching commander</p>`,
		`href="/commanders/search?return_to=%2Fdecks%2Fsettings%3Fid%3D42">Clear search</a>`,
		`id="commander-browser-recommended-title">Recommended commanders</h2>`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("commander search state missing %q: %s", needle, body)
		}
	}
	if got := strings.Count(body, `class="mt-commander-browser__grid"`); got != 2 {
		t.Fatalf("commander search rendered %d result grids, want search and recommendations", got)
	}
}

func TestCommanderSearchAssetIsServedAndUsesOnlyThemeTokens(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/commanders_search.css", nil)
	rr := httptest.NewRecorder()
	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("commander search asset returned status %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/css") {
		t.Fatalf("commander search asset content type = %q", got)
	}
	css := rr.Body.String()
	for _, needle := range []string{
		`.mt-commander-browser {`,
		`.mt-commander-browser__search .mt-control:focus-visible {`,
		`.mt-commander-browser-card__media {`,
		`color: var(--mt-text);`,
		`border: 1px solid var(--mt-border-subtle);`,
		`box-shadow: var(--mt-shadow-card);`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("commander search asset missing %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "#", "slate", "translateY("} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("commander search asset contains hard-coded or moving presentation %q", forbidden)
		}
	}
}
