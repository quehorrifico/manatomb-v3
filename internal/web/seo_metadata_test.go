package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRendererAddsCanonicalSocialAndRobotsMetadata(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewRenderer("https://manatomb.app").Render(recorder, "home", TemplateData{HideHeader: true})
	body := recorder.Body.String()

	for _, needle := range []string{
		`<title>ManaTomb</title>`,
		`<meta name="description" content=`,
		`<meta name="robots" content="index,follow">`,
		`<link rel="canonical" href="https://manatomb.app/">`,
		`<meta property="og:site_name" content="ManaTomb">`,
		`<meta property="og:locale" content="en_US">`,
		`<meta property="og:image" content="https://manatomb.app/assets/manatomb-square-logo.png">`,
		`<meta name="twitter:card" content="summary_large_image">`,
		`<meta name="twitter:image:alt" content="ManaTomb logo">`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("home metadata missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, `<title>ManaTomb | ManaTomb</title>`) {
		t.Fatalf("home title duplicates the brand: %s", body)
	}
}

func TestPrivatePagesAreNoIndexWithoutShareFallback(t *testing.T) {
	got, ok := applyTemplateDefaultsWithPublicBaseURL("settings", TemplateData{}, "https://manatomb.app").(TemplateData)
	if !ok || got.Meta == nil {
		t.Fatalf("settings metadata defaults missing: %#v", got)
	}
	if got.Meta.Robots != "noindex,follow" {
		t.Fatalf("settings robots = %q", got.Meta.Robots)
	}
	if got.Meta.CanonicalURL != "" || got.Meta.ImageURL != "" {
		t.Fatalf("private settings page received public share metadata: %#v", got.Meta)
	}
}

func TestStablePublicPagesUseConfiguredCanonicalOrigin(t *testing.T) {
	tests := map[string]string{
		"home":         "/",
		"cards_search": "/cards/search",
		"decks_public": "/decks/public",
		"guess_card":   "/games/guess-card",
		"spellify":     "/games/spellify",
		"pack_opening": "/games/pack-opening",
		"changelog":    "/changelog",
		"privacy":      "/privacy",
		"terms":        "/terms",
	}
	for name, path := range tests {
		got, ok := applyTemplateDefaultsWithPublicBaseURL(name, TemplateData{}, "https://manatomb.app").(TemplateData)
		if !ok || got.Meta == nil {
			t.Fatalf("%s metadata defaults missing", name)
		}
		if got.Meta.CanonicalURL != "https://manatomb.app"+path {
			t.Fatalf("%s canonical = %q", name, got.Meta.CanonicalURL)
		}
		if got.Meta.Robots != "index,follow" || got.Meta.ImageURL != "https://manatomb.app/assets/manatomb-square-logo.png" {
			t.Fatalf("%s public SEO defaults = %#v", name, got.Meta)
		}
	}
}

func TestPlaceholderRulesPageIsNoIndexUntilReferenceIsPublished(t *testing.T) {
	got, ok := applyTemplateDefaultsWithPublicBaseURL("rules_home", TemplateData{}, "https://manatomb.app").(TemplateData)
	if !ok || got.Meta == nil {
		t.Fatalf("rules metadata defaults missing: %#v", got)
	}
	if got.Meta.Robots != "noindex,follow" || got.Meta.CanonicalURL != "" || got.Meta.ImageURL != "" {
		t.Fatalf("placeholder rules page should not be advertised to search or social crawlers: %#v", got.Meta)
	}
}

func TestDefaultPageTitlesAreConciseAndUnbranded(t *testing.T) {
	pages := []string{
		"decks_list", "deck_show", "deck_playtest", "decks_public", "settings", "profile_show",
		"login", "signup", "forgot_password", "reset_password", "cards_search", "cards_list",
		"decks_new", "decks_new_commander", "commanders_search", "decks_import",
		"decks_workbench_import_seed", "guess_card", "spellify", "pack_opening", "changelog", "privacy",
		"terms", "rules_home", "not_found", "error",
	}
	for _, name := range pages {
		meta := defaultPageMeta(name)
		if meta == nil || strings.TrimSpace(meta.Title) == "" || strings.TrimSpace(meta.Description) == "" {
			t.Fatalf("%s has incomplete tab/SEO metadata: %#v", name, meta)
		}
		if strings.Contains(meta.Title, "|") || strings.Contains(meta.Title, "ManaTomb") {
			t.Fatalf("%s title should contain only the page name: %q", name, meta.Title)
		}
	}
}
