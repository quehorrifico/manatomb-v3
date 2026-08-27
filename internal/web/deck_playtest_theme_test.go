package web

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"manatomb/app/internal/decks"
)

func TestDeckPlaytestChromeUsesSemanticThemeTokens(t *testing.T) {
	withRendererRoot(t)

	templateBody, err := os.ReadFile(filepath.Join("internal", "web", "templates", "deck_playtest.html.tmpl"))
	if err != nil {
		t.Fatalf("read playtest template: %v", err)
	}
	assetBody, err := os.ReadFile(filepath.Join("internal", "web", "assets", "playtest.js"))
	if err != nil {
		t.Fatalf("read playtest asset: %v", err)
	}

	markup := string(templateBody)
	script := string(assetBody)
	for _, want := range []string{
		`body[data-page="deck-playtest"]`,
		`--pt-canvas: var(--mt-bg);`,
		`--pt-surface: var(--mt-surface);`,
		`--pt-accent: var(--mt-accent);`,
		`--pt-text: var(--mt-text);`,
		`class="pt-overlay-backdrop absolute inset-0"`,
		`class="pt-drop-zone pt-zone-surface pt-battlefield-zone`,
	} {
		if !strings.Contains(markup, want) {
			t.Fatalf("playtest template is missing semantic theme hook %q", want)
		}
	}

	for _, legacy := range []string{
		"bg-slate-",
		"text-slate-",
		"border-slate-",
		"bg-sky-",
		"text-sky-",
		"border-sky-",
		"rgb(2 6 23)",
		"rgba(2, 6, 23",
		"rgb(15 23 42)",
		"rgba(15, 23, 42",
		"rgba(30, 41, 59",
		"rgba(51, 65, 85",
		"rgba(56, 189, 248",
		"rgba(71, 85, 105",
		"rgb(8 47 73",
		"rgba(8, 47, 73",
		"rgb(14 116 144",
		"rgba(14, 165, 233",
		"rgb(100 116 139)",
		"rgba(100, 116, 139",
		"rgb(148 163 184)",
		"rgba(148, 163, 184",
		"rgb(203 213 225)",
		"rgba(203, 213, 225",
		"rgb(226 232 240)",
		"rgba(226, 232, 240",
		"rgb(240 249 255)",
		"rgb(248 250 252)",
	} {
		if strings.Contains(markup, legacy) {
			t.Fatalf("playtest template still contains legacy chrome color %q", legacy)
		}
		if strings.Contains(script, legacy) {
			t.Fatalf("playtest asset still creates legacy chrome color %q", legacy)
		}
	}

	if strings.Contains(markup, `class="pt-battlefield-section mt-panel`) {
		t.Fatal("playtest battlefield still nests the board inside a shared panel")
	}
	for _, want := range []string{
		`pt-empty-message pt-empty-message--padded`,
		`pt-empty-message pt-empty-message--centered`,
		`pt-card-note`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("playtest asset is missing semantic generated class %q", want)
		}
	}
}

func TestDeckPlaytestPropagatesSelectedTheme(t *testing.T) {
	body := renderTemplate(t, "deck_playtest", TemplateData{
		PageID: "deck-playtest",
		Theme:  "verdigris",
		Data: playtestData{
			Deck:          &decks.Deck{Format: "Commander"},
			CardsJSON:     template.JS("[]"),
			CommanderJSON: template.JS("null"),
		},
		HideHeader: true,
		HideFooter: true,
	})

	if htmlTag := renderedOpeningTag(t, body, "html"); !strings.Contains(htmlTag, `data-theme="verdigris"`) {
		t.Fatalf("playtest html did not preserve the selected theme: %s", htmlTag)
	}
	if bodyTag := renderedOpeningTag(t, body, "body"); !strings.Contains(bodyTag, `data-page="deck-playtest"`) {
		t.Fatalf("playtest body did not expose its scoped page id: %s", bodyTag)
	}
}
