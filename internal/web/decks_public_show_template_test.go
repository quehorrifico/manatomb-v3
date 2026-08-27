package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

func publicDeckShowFixture() publicDeckPageData {
	return publicDeckPageData{
		Deck: &decks.Deck{
			ID:            73,
			UserID:        42,
			Name:          "Tomb of the Infinite",
			Description:   "A patient artifact-combo deck with several overlapping win conditions.",
			Tags:          "Combo, Control",
			Format:        "Commander",
			CommanderName: "Sharuum the Hegemon",
			PublicSlug:    "tomb-of-the-infinite",
			PowerBracket:  "4 - Optimized",
		},
		DeckCards: []decks.DeckCard{{
			CardID:   "sol-ring",
			CardName: "Sol Ring",
			TypeLine: "Artifact",
			Quantity: 1,
		}},
		SideboardDeckCards: []decks.DeckCard{{
			CardID:   "negate",
			CardName: "Negate",
			TypeLine: "Instant",
			Quantity: 1,
		}},
		MaybeDeckCards: []decks.DeckCard{{
			CardID:   "ponder",
			CardName: "Ponder",
			TypeLine: "Sorcery",
			Quantity: 1,
		}},
		Analytics: deckAnalyticsData{
			TotalCards:        100,
			AverageCMCDisplay: "2.74",
			LandCount:         36,
			NonLandCount:      64,
			CreatureCount:     18,
			ArtifactCount:     24,
			EnchantmentCount:  6,
			InstantCount:      12,
			SorceryCount:      9,
			PlaneswalkerCount: 2,
			RampCount:         11,
			CardDrawCount:     10,
			TutorCount:        5,
			InteractionCount:  14,
			BoardWipeCount:    3,
			ProtectionCount:   4,
			ValidationWarnings: []string{
				"Example deck warning.",
			},
		},
		Commander: &cards.Card{
			ID:              "commander-print-id",
			Name:            "Sharuum the Hegemon",
			OracleID:        "sharuum-oracle-id",
			ImageURI:        "https://example.test/sharuum.jpg",
			ArtCropURI:      "https://example.test/sharuum-art.jpg",
			SetCode:         "CMM",
			CollectorNumber: "231",
			PriceUSD:        "0.75",
		},
		Owner: &account.PublicProfile{
			ID:          42,
			DisplayName: "Tomb Keeper",
		},
		ColorPips:               manaPipsForColorIdentity("W,U,B"),
		ColorIdentityName:       colorCombinationName("W,U,B"),
		PublishedLabel:          "Jul 27, 2026",
		UpdatedLabel:            "Jul 30, 2026",
		DeckCostDisplay:         "$1,234.56+",
		DeckCostNote:            "Minimum known cost; 1 card has no USD price.",
		SampleExcludesCommander: true,
		SampleHand: []decks.DeckCard{
			{CardID: "sample-1", PrintID: "print-1", CardName: "Sample One", ImageURI: "https://example.test/sample-1.jpg"},
			{CardID: "sample-2", PrintID: "print-2", CardName: "Sample Two", ImageURI: "https://example.test/sample-2.jpg"},
			{CardID: "sample-3", PrintID: "print-3", CardName: "Sample Three", ImageURI: "https://example.test/sample-3.jpg"},
			{CardID: "sample-4", PrintID: "print-4", CardName: "Sample Four", ImageURI: "https://example.test/sample-4.jpg"},
			{CardID: "sample-5", PrintID: "print-5", CardName: "Sample Five", ImageURI: "https://example.test/sample-5.jpg"},
			{CardID: "sample-6", PrintID: "print-6", CardName: "Sample Six", ImageURI: "https://example.test/sample-6.jpg"},
			{CardID: "sample-7", PrintID: "print-7", CardName: "Sample Seven", ImageURI: "https://example.test/sample-7.jpg"},
		},
	}
}

func TestPublicDeckShowUsesEditorialTombShowcase(t *testing.T) {
	body := renderTemplate(t, "decks_public_show", TemplateData{
		Data: publicDeckShowFixture(),
	})

	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("guest public deck detail is missing the Tomb palette: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="decks-public-show"`) {
		t.Fatalf("public deck detail is missing its page scope: %s", bodyTag)
	}
	if strings.Count(body, "<main") != 1 {
		t.Fatalf("public deck detail rendered %d main landmarks, want the shared one only", strings.Count(body, "<main"))
	}
	if strings.Count(body, "<h1") != 1 {
		t.Fatalf("public deck detail rendered %d level-one headings, want one", strings.Count(body, "<h1"))
	}
	if !strings.Contains(body, `class="mt-page-shell w-full px-0`) {
		t.Fatalf("public deck detail did not opt into the wide shared shell: %s", body)
	}

	stylesheetIndex := strings.Index(body, `href="/assets/public_deck.css"`)
	bodyIndex := strings.Index(body, "<body")
	if stylesheetIndex == -1 || bodyIndex == -1 || stylesheetIndex >= bodyIndex {
		t.Fatal("public deck stylesheet is not loaded from the document head")
	}

	pageStart := strings.Index(body, `<div class="mt-public-deck-page">`)
	pageEnd := strings.Index(body, `id="card-detail-modal"`)
	if pageStart == -1 || pageEnd <= pageStart {
		t.Fatalf("could not isolate rendered public deck detail: %s", body)
	}
	page := body[pageStart:pageEnd]

	for _, needle := range []string{
		`class="mt-public-showcase"`,
		`class="mt-public-showcase__art"`,
		`class="mt-public-showcase__content"`,
		`id="public-deck-title">Tomb of the Infinite</h1>`,
		`aria-label="Deck summary"`,
		`Commander</li>`,
		`4 - Optimized`,
		`aria-label="Esper color identity"`,
		`https://svgs.scryfall.io/card-symbols/W.svg`,
		`Tomb Keeper</a>`,
		`Published Jul 27, 2026`,
		`Updated Jul 30, 2026`,
		`Sign In to Copy`,
		`id="public-deck-action-menu"`,
		`aria-label="More deck actions"`,
		`id="public-deck-copy-link"`,
		`Copy deck link`,
		`id="public-deck-copy-list"`,
		`Copy decklist`,
		`id="public-deck-export-txt"`,
		`id="public-deck-export-csv"`,
		`data-card-detail`,
		`data-detail-path="/cards/view/sharuum-oracle-id"`,
		`aria-label="View Sharuum the Hegemon card details"`,
		`A patient artifact-combo deck`,
		`href="/decks/public?archetype=Combo"`,
		`aria-label="Browse public decks with the Combo archetype">#Combo</a>`,
		`class="mt-public-showcase__metrics"`,
		`<dt>Cost</dt>`,
		`$1,234.56`,
		`Minimum known cost; 1 card has no USD price.`,
		`Average mana value`,
		`<dt>Nonlands</dt>`,
		`class="mt-public-deck-analysis"`,
		`aria-label="Deck analysis"`,
		`Card types`,
		`Deck roles`,
		`Sample hand`,
		`id="public-deck-sample-cards"`,
		`data-sample-index="0"`,
		`https://example.test/sample-7.jpg`,
		`id="public-deck-sample-refresh"`,
		`aria-describedby="public-deck-sample-status"`,
		`mt-public-sample-refresh__icon`,
		`Another hand`,
		`id="public-deck-sample-status"`,
		`sampleHand:`,
		`"PrintID":"print-1"`,
		`sampleExcludesCommander: true`,
		`commander: {`,
		`printID: "commander-print-id"`,
		`setCode: "CMM"`,
		`collectorNumber: "231"`,
		`id="public-deck-browser-title">Decklist</h2>`,
		`role="group" aria-label="Deck sections"`,
		`<span>Deck</span>`,
		`data-board="side"`,
		`data-board="maybe"`,
		`aria-pressed="false"`,
		`class="mt-public-deck-controls"`,
		`for="public-deck-group"`,
		`<span>Group cards by</span>`,
		`<option value="type" selected>Card type</option>`,
		`for="public-deck-sort"`,
		`<span>Sort cards by</span>`,
		`<option value="mv" selected>Mana value</option>`,
		`for="public-deck-view"`,
		`<span>Card view</span>`,
		`<option value="text" selected>Text list</option>`,
		`src="/assets/deck_browser.js"`,
		`src="/assets/public_deck.js"`,
	} {
		if !strings.Contains(page, needle) && !strings.Contains(body, needle) {
			t.Fatalf("public deck detail missing %q", needle)
		}
	}

	showcaseIndex := strings.Index(page, `class="mt-public-showcase"`)
	analysisIndex := strings.Index(page, `class="mt-public-deck-analysis"`)
	browserIndex := strings.Index(page, `class="mt-public-deck-browser"`)
	if showcaseIndex == -1 || analysisIndex <= showcaseIndex || browserIndex <= analysisIndex {
		t.Fatalf("public deck detail sections are not ordered showcase, analysis, decklist: %s", page)
	}
	if got := strings.Count(page, `class="mt-public-sample-card"`); got != 7 {
		t.Fatalf("public deck detail rendered %d sample-hand cards, want 7", got)
	}

	outputIndex := strings.Index(page, `id="public-deck-output"`)
	if outputIndex == -1 {
		t.Fatalf("public deck detail is missing its browser output: %s", page)
	}
	outputStart := strings.LastIndex(page[:outputIndex], "<div")
	outputEnd := strings.Index(page[outputIndex:], ">")
	if outputStart == -1 || outputEnd == -1 {
		t.Fatalf("could not isolate public deck output tag: %s", page)
	}
	outputTag := page[outputStart : outputIndex+outputEnd+1]
	if strings.Contains(outputTag, "aria-live") {
		t.Fatalf("deck output should not announce the entire rerendered collection: %s", outputTag)
	}

	for _, forbidden := range []string{
		`<main`,
		`mt-panel`,
		`mt-public-deck-hero`,
		`mt-public-stats-disclosure`,
		`id="public-deck-share"`,
		`Share deck`,
		`<details class="mt-public-deck-analysis"`,
		`Deck checks`,
		`Example deck warning.`,
		`Explore deck analysis`,
		`Card types, deck roles, and a sample opening hand`,
		`The deck`,
		`100 cards`,
		`id="public-deck-summary"`,
		`id="public-deck-filter"`,
		`Filter cards`,
		`mt-public-view-options`,
		`View options`,
		`Mainboard`,
		`>Home</a>`,
		`>Browse</a>`,
		`role="tablist"`,
		`role="tab"`,
		`text-slate-`,
		`bg-slate-`,
		`text-sky-`,
		`text-amber-`,
		`border-amber-`,
		`divide-slate-`,
		`>Edit Deck</a>`,
	} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("public deck detail unexpectedly contains %q", forbidden)
		}
	}
}

func TestPublicDeckShowPrimaryActionMatchesViewer(t *testing.T) {
	tests := []struct {
		name      string
		user      *account.User
		required  []string
		forbidden []string
	}{
		{
			name: "owner gets edit only",
			user: &account.User{ID: 42},
			required: []string{
				`href="/decks/73"`,
				`>Edit Deck</a>`,
			},
			forbidden: []string{
				`action="/decks/public/fork"`,
				`Copy to My Decks`,
				`Sign In to Copy`,
			},
		},
		{
			name: "signed in non-owner gets copy",
			user: &account.User{ID: 99},
			required: []string{
				`action="/decks/public/fork"`,
				`Copy to My Decks`,
			},
			forbidden: []string{
				`href="/decks/73"`,
				`>Edit Deck</a>`,
				`Sign In to Copy`,
			},
		},
		{
			name: "guest gets sign in",
			required: []string{
				`Sign In to Copy`,
			},
			forbidden: []string{
				`href="/decks/73"`,
				`>Edit Deck</a>`,
				`action="/decks/public/fork"`,
				`Copy to My Decks`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := renderTemplate(t, "decks_public_show", TemplateData{
				CurrentUser: tt.user,
				Data:        publicDeckShowFixture(),
			})
			pageStart := strings.Index(body, `<div class="mt-public-deck-page">`)
			pageEnd := strings.Index(body, `id="card-detail-modal"`)
			if pageStart == -1 || pageEnd <= pageStart {
				t.Fatalf("could not isolate rendered public deck detail")
			}
			page := body[pageStart:pageEnd]

			for _, needle := range tt.required {
				if !strings.Contains(page, needle) {
					t.Fatalf("primary action missing %q", needle)
				}
			}
			for _, needle := range tt.forbidden {
				if strings.Contains(page, needle) {
					t.Fatalf("primary action unexpectedly contains %q", needle)
				}
			}
		})
	}
}

func TestPublicDeckShowStylesUseCentralThemeTokens(t *testing.T) {
	withRendererRoot(t)

	body, err := os.ReadFile(filepath.Join("internal", "web", "assets", "public_deck.css"))
	if err != nil {
		t.Fatalf("read public deck detail stylesheet: %v", err)
	}
	css := string(body)

	for _, needle := range []string{
		`.mt-public-deck-page`,
		`width: min(100%, 104rem);`,
		`.mt-public-showcase`,
		`grid-template-areas:`,
		`grid-template-areas: "commander content";`,
		`.mt-public-showcase__content`,
		`display: flex;`,
		`flex-direction: column;`,
		`gap: 0.85rem;`,
		`.mt-public-showcase__commander`,
		`.mt-public-showcase__metrics`,
		`.mt-public-action-status:empty`,
		`.mt-public-showcase__description + .mt-public-showcase__tags`,
		`display: contents;`,
		`grid-template-columns: repeat(4, minmax(0, 1fr));`,
		`.mt-public-download-split`,
		`.mt-public-deck-analysis__content`,
		`.mt-public-sample-hand`,
		`.mt-public-sample-hand__cards`,
		`.mt-public-sample-refresh`,
		`.mt-public-deck-toolbar`,
		`.mt-public-deck-controls`,
		`grid-template-columns: repeat(3, minmax(0, 1fr));`,
		`.mt-deck-view-root--text.mt-deck-view-root--grouped`,
		`grid-template-columns: repeat(auto-fit, minmax(min(100%, 19rem), 1fr));`,
		`.mt-deck-view-root--grouped .mt-deck-view-section + .mt-deck-view-section`,
		`.mt-public-single-heading`,
		`.mt-public-single-type`,
		`width: min(100%, 32rem);`,
		`min-height: 2.75rem;`,
		`.mt-public-single-peek`,
		`display: none;`,
		`grid-template-columns: repeat(2, minmax(0, 1fr));`,
		`grid-template-columns: minmax(0, 1fr);`,
		`body[data-page="decks-public-show"] .mt-board-tab--active`,
		`body[data-page="decks-public-show"] .mt-deck-view-section`,
		`color: var(--mt-text);`,
		`color: var(--mt-text-soft);`,
		`outline: 2px solid var(--mt-focus);`,
		`body[data-page="decks-public-show"] *`,
		`animation: none;`,
		`transition: none;`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("public deck detail stylesheet missing %q", needle)
		}
	}

	for _, forbidden := range []string{"rgb(", "rgba(", "#", "@keyframes"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("public deck detail stylesheet contains hardcoded or animated syntax %q", forbidden)
		}
	}
	if strings.Contains(css, "grid-template-rows: auto auto 1fr;") {
		t.Fatal("public deck showcase still reserves an empty flexible row between actions, tags, and metrics")
	}

	for _, selector := range []string{
		".mt-public-showcase",
		".mt-public-showcase__metrics",
		".mt-public-deck-toolbar",
	} {
		rule := renderedCSSRule(t, css, selector)
		for _, forbidden := range []string{"border:", "background:", "box-shadow:"} {
			if strings.Contains(rule, forbidden) {
				t.Fatalf("%s still behaves like a containing panel (%q): %s", selector, forbidden, rule)
			}
		}
	}
}

func TestPublicDeckShowScriptUsesSemanticThemeClassesAndButtonState(t *testing.T) {
	withRendererRoot(t)

	body, err := os.ReadFile(filepath.Join("internal", "web", "assets", "public_deck.js"))
	if err != nil {
		t.Fatalf("read public deck detail script: %v", err)
	}
	script := string(body)

	for _, needle := range []string{
		`mt-public-card-price`,
		`mt-public-table-meta`,
		`<h3 class="mt-deck-view-section__title">`,
		`setAttribute("aria-pressed"`,
		`state.board !== "main"`,
		`state.board = "main"`,
		`params.get("view") || "text"`,
		`core.normalizeGroup(params.get("group"), "type")`,
		`params.get("sort") || "mv"`,
		`mt-public-text-card__qty`,
		`class="mt-list-row mt-public-text-card mt-public-card-button"`,
		`data-single-current`,
		`mt-public-single-quantity`,
		`mt-public-single-type`,
		`function drawSampleHand()`,
		`config.sampleExcludesCommander`,
		`pool.slice(0, 7)`,
		`data-sample-index`,
		`openCardObject(currentSample[index])`,
		`A new seven-card sample hand is ready.`,
		`function canonicalDeckURL()`,
		`document.querySelector('link[rel="canonical"]')`,
		`url.search = "";`,
		`url.hash = "";`,
		`function copyDeckLink()`,
		`Deck link copied to clipboard.`,
		`Could not copy the deck link.`,
		`document.getElementById("public-deck-copy-link").addEventListener("click", copyDeckLink)`,
		`Deck list copied to clipboard.`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("public deck detail script missing %q", needle)
		}
	}
	for _, forbidden := range []string{
		"text-slate-",
		`setAttribute("aria-selected"`,
		`public-deck-share`,
		`public-deck-filter`,
		`public-deck-summary`,
		`filterField`,
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("public deck detail script unexpectedly contains %q", forbidden)
		}
	}
}
