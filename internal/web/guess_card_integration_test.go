package web

import (
	"strings"
	"testing"
)

func TestGuessCardUsesSharedPageDefaults(t *testing.T) {
	got, ok := applyTemplateDefaults("guess_card", TemplateData{}).(TemplateData)
	if !ok {
		t.Fatal("guess_card defaults did not return TemplateData")
	}
	if got.Theme != siteThemeTomb {
		t.Fatalf("guess_card theme = %q, want %q", got.Theme, siteThemeTomb)
	}
	if got.PageID != "guess-card" {
		t.Fatalf("guess_card page ID = %q, want guess-card", got.PageID)
	}
	if !got.WideLayout {
		t.Fatal("guess_card should use the wide shared page shell")
	}
	if got.Meta == nil {
		t.Fatal("guess_card is missing default page metadata")
	}
	if got.Meta.Title != "Guess the Card" {
		t.Fatalf("guess_card metadata title = %q", got.Meta.Title)
	}
}

func TestGuessCardPageScopeLoadsDedicatedStylesheet(t *testing.T) {
	body := renderTemplate(t, "home", TemplateData{
		HideHeader: true,
		PageID:     "guess-card",
	})
	if !strings.Contains(body, `data-page="guess-card"`) {
		t.Fatalf("guess-card page scope is missing: %s", body)
	}
	if !strings.Contains(body, `href="/assets/guess_card.css"`) {
		t.Fatalf("guess-card stylesheet is missing from the shared layout: %s", body)
	}
	if !strings.Contains(body, `src="/assets/guess_card.js"`) {
		t.Fatalf("guess-card behavior script is missing from the shared layout: %s", body)
	}
}
