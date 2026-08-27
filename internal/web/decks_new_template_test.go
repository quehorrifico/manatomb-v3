package web

import (
	"strings"
	"testing"

	"manatomb/app/internal/account"
)

func TestDeckBuilderStartIsFlatGuestFriendlyAndResumable(t *testing.T) {
	body := renderTemplate(t, "decks_new", TemplateData{
		Data: deckNewPageData{},
	})

	for _, needle := range []string{
		`data-theme="tomb"`,
		`<title>Build a Deck | ManaTomb</title>`,
		`How do you want to start?`,
		`Build and playtest without an account.`,
		`Support for more formats coming soon.`,
		`<h2 class="mt-builder-start__choice-title">Commander</h2>`,
		`100 card, singleton, multiplayer format`,
		`href="/decks/new/workbench?format=Standard&amp;reset=1"`,
		`<h2 class="mt-builder-start__choice-title">Standard</h2>`,
		`60 card, rotating format using cards from recent sets`,
		`<h2 class="mt-builder-start__choice-title">Sandbox</h2>`,
		`No format restrictions`,
		`class="mt-builder-start__import-section"`,
		`class="mt-builder-start__import"`,
		`aria-labelledby="builder-import-title"`,
		`aria-describedby="builder-import-copy"`,
		`Already have a decklist?`,
		`Upload or paste an existing deck list and continue working on it.`,
		`data-builder-resume-block`,
		`data-builder-resume`,
		`data-builder-resume-link`,
		`aria-labelledby="builder-resume-title"`,
		`aria-describedby="builder-resume-format builder-resume-warning"`,
		`data-builder-resume-format`,
		`formatLine.textContent = "Format: " + format`,
		`var draftName = String(draft.name || "").trim()`,
		`? "Resume " + draftName`,
		`Starting another deck will replace this device draft.`,
		`function draftHasCards(draft)`,
		`Array.isArray(cardMap)`,
		`String(name).trim() !== ""`,
		`cardMapHasCards(draft.sideboardCards)`,
		`cardMapHasCards(draft.maybeCards)`,
		`if (!draftHasCards(draft)) return;`,
		`manatomb.draftDeck.sandbox`,
		`commander_print_id`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck builder start missing %q: %s", needle, body)
		}
	}

	if got := strings.Count(body, "<main"); got != 1 {
		t.Fatalf("deck builder rendered %d main landmarks, want 1", got)
	}
	if strings.Contains(body, `class="mt-panel`) {
		t.Fatalf("deck builder landing should not render a containing panel: %s", body)
	}
	for _, removed := range []string{
		`<p class="mt-kicker">Deck builder</p>`,
		`mt-builder-start__choice--primary`,
		`mt-builder-start__choice-label`,
		`mt-builder-start__choice-action`,
		`Saved on this device`,
		`data-builder-resume-copy`,
		`Continue building without commander or format restrictions.`,
		`>Resume draft</a>`,
		`Import a decklist <span`,
	} {
		if strings.Contains(body, removed) {
			t.Fatalf("deck builder start retained removed content %q: %s", removed, body)
		}
	}

	resumeIndex := strings.Index(body, `data-builder-resume-block`)
	commanderIndex := strings.Index(body, `>Commander</h2>`)
	standardIndex := strings.Index(body, `>Standard</h2>`)
	sandboxIndex := strings.Index(body, `>Sandbox</h2>`)
	importIndex := strings.Index(body, `class="mt-builder-start__import-section"`)
	if resumeIndex < 0 || commanderIndex < 0 || standardIndex < 0 || sandboxIndex < 0 || importIndex < 0 ||
		!(resumeIndex < commanderIndex && commanderIndex < standardIndex && standardIndex < sandboxIndex && sandboxIndex < importIndex) {
		t.Fatalf("deck builder choices rendered in the wrong order: %s", body)
	}
}

func TestDeckBuilderStartUsesSignedInCopy(t *testing.T) {
	body := renderTemplate(t, "decks_new", TemplateData{
		CurrentUser: &account.User{ID: 42},
		Data:        deckNewPageData{},
	})

	if !strings.Contains(body, "shape and save the deck from one shared workspace") {
		t.Fatalf("signed-in deck builder copy is missing: %s", body)
	}
	if strings.Contains(body, "Your draft stays saved on this device until you sign in") {
		t.Fatalf("signed-in deck builder rendered guest-only copy: %s", body)
	}
	if strings.Contains(body, "Support for more formats coming soon.") {
		t.Fatalf("signed-in deck builder rendered guest-only format copy: %s", body)
	}
}

func TestCommanderPickerUsesOneClickChoicesWithoutLeavingForCardDetails(t *testing.T) {
	const printID = "223e4567-e89b-12d3-a456-426614174000"
	const detailPath = "/cards/view/123e4567-e89b-12d3-a456-426614174000?printing=" + printID
	body := renderTemplate(t, "decks_new_commander", TemplateData{
		Data: commanderDeckBuilderPageData{
			Query: "Atraxa",
			Results: []searchResult{{
				ScryfallID:    printID,
				DetailPath:    detailPath,
				Name:          "Atraxa, Grand Unifier",
				ImageURI:      "https://cards.example/atraxa.jpg",
				ColorIdentity: "White, Blue, Black, Green",
			}},
		},
	})

	for _, needle := range []string{
		`data-theme="tomb"`,
		`<title>Choose a Commander | ManaTomb</title>`,
		`role="search"`,
		`placeholder="Search commanders by name"`,
		`class="sr-only">Search commanders</button>`,
		`name="commander_name" value="Atraxa, Grand Unifier"`,
		`name="commander_print_id" value="` + printID + `"`,
		`aria-label="Build a deck with Atraxa, Grand Unifier"`,
		`1 matching commanders`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("commander picker missing %q: %s", needle, body)
		}
	}

	if got := strings.Count(body, "<main"); got != 1 {
		t.Fatalf("commander picker rendered %d main landmarks, want 1", got)
	}
	if strings.Contains(body, detailPath) {
		t.Fatalf("commander art should select the commander instead of navigating away: %s", body)
	}
	if strings.Contains(body, `class="mt-panel`) || strings.Contains(body, `class="mt-action-card`) {
		t.Fatalf("commander picker should not render nested panel/action-card chrome: %s", body)
	}
	for _, removed := range []string{
		`class="mt-commander-picker__back"`,
		`<p class="mt-kicker">Commander deck</p>`,
		`Ordered by EDHREC rank.`,
		`Color identity:`,
		`<small>Choose commander`,
	} {
		if strings.Contains(body, removed) {
			t.Fatalf("commander picker retained removed content %q: %s", removed, body)
		}
	}
}

func TestCommanderPickerLoadsSeededRandomBatches(t *testing.T) {
	const (
		seed       = "aa759df7-c324-4a72-a0a5-c86fcfe5d8d0"
		cursor     = "123e4567-e89b-12d3-a456-426614174000"
		secondID   = "223e4567-e89b-12d3-a456-426614174000"
		firstPrint = "323e4567-e89b-12d3-a456-426614174000"
	)
	body := renderTemplate(t, "decks_new_commander", TemplateData{
		Data: commanderDeckBuilderPageData{
			Recommended: []searchResult{
				{
					OracleID:   cursor,
					ScryfallID: firstPrint,
					Name:       "Atraxa, Grand Unifier",
					ImageURI:   "https://cards.example/atraxa.jpg",
				},
				{
					OracleID:   secondID,
					ScryfallID: "423e4567-e89b-12d3-a456-426614174000",
					Name:       "Muldrotha, the Gravetide",
					ImageURI:   "https://cards.example/muldrotha.jpg",
				},
			},
			RecommendationSeed:     seed,
			RecommendationCursor:   cursor,
			HasMoreRecommendations: true,
		},
	})

	for _, needle := range []string{
		`data-popular-commanders`,
		`data-commander-seed="` + seed + `"`,
		`data-commander-cursor="` + cursor + `"`,
		`data-commander-oracle-id="` + cursor + `"`,
		`data-load-more-commanders`,
		`/decks/new/commander/more`,
		`knownOracleIDs`,
		`hiddenField("commander_print_id", item.scryfall_id)`,
		`fetch(url.toString()`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("seeded commander picker missing %q: %s", needle, body)
		}
	}

	for _, removed := range []string{
		`data-popular-commander hidden`,
		`var chunkSize`,
		`Ordered by EDHREC rank.`,
		`Color identity:`,
		`<small>Choose commander`,
	} {
		if strings.Contains(body, removed) {
			t.Fatalf("seeded commander picker retained old behavior %q: %s", removed, body)
		}
	}
}

func TestCommanderPickerStylesStayTwoColumnAndMotionlessOnMobile(t *testing.T) {
	css := readThemeCSS(t)

	for _, needle := range []string{
		`body[data-page="decks-new-commander"] *`,
		`animation: none !important`,
		`transition: none !important`,
		`.mt-commander-grid`,
		`grid-template-columns: repeat(2, minmax(0, 1fr))`,
		`-webkit-line-clamp: 2`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("commander picker stylesheet missing %q", needle)
		}
	}
}
