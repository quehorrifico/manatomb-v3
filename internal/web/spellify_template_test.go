package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"manatomb/app/internal/cards"
)

func spellifyTemplateFixture(status string) spellifyPageData {
	return buildSpellifyPageData(spellifyGame{
		ID:                   73,
		Status:               status,
		GuessCount:           2,
		GuessedChars:         []string{"q", "z"},
		IsDaily:              true,
		DailyKey:             "2026-08-12",
		GuestID:              "guest-token",
		PreviousWrongGuesses: []string{"Definitely Not This Card"},
	}, cards.Card{
		OracleID:   "11111111-1111-1111-1111-111111111111",
		ID:         "22222222-2222-2222-2222-222222222222",
		Name:       "Secret Puzzle Card",
		TypeLine:   "Legendary Artifact",
		ManaCost:   "{2}{U}",
		OracleText: "Q is present in this secret card.",
		FlavorText: "Keep this hidden.",
		ImageURI:   "https://example.test/secret-puzzle.jpg",
	})
}

func TestSpellifyTemplateUsesFlatTombGameBoard(t *testing.T) {
	page := spellifyTemplateFixture("active")
	body := renderTemplate(t, "spellify", TemplateData{
		Data:  page,
		Flash: "One Tombscript notice.",
	})

	if !strings.Contains(renderedOpeningTag(t, body, "html"), `data-theme="tomb"`) {
		t.Fatal("guest Tombscript page did not receive the Tomb palette")
	}
	if !strings.Contains(renderedOpeningTag(t, body, "body"), `data-page="spellify"`) {
		t.Fatal("Tombscript page is missing its page scope")
	}
	if got := strings.Count(body, "<main"); got != 1 {
		t.Fatalf("Tombscript rendered %d main landmarks, want the shared one only", got)
	}
	if got := strings.Count(body, "One Tombscript notice."); got != 1 {
		t.Fatalf("Tombscript notice rendered %d times, want one screen-reader announcement", got)
	}
	if strings.Contains(body, `class="mt-flash`) {
		t.Fatal("Tombscript must not render page-wide notice bars")
	}

	for _, needle := range []string{
		`href="/assets/spellify.css"`,
		`src="/assets/spellify.js"`,
		`class="mt-tombscript"`,
		`aria-label="Game status"`,
		`aria-live="polite"`,
		`Remaining Guesses <strong data-tombscript-remaining>11</strong>`,
		`class="mt-tombscript__game"`,
		`class="mt-tombscript__name-row"`,
		`data-tombscript-name`,
		`data-tombscript-mana-cost`,
		`data-tombscript-character-form`,
		`data-tombscript-key="q"`,
		`data-tombscript-symbol-key`,
		`data-tombscript-key="{9}"`,
		`data-tombscript-key="{W/P}"`,
		`card-symbols/WP.svg`,
		`data-tombscript-guess-open`,
		`id="tombscript-guess-modal"`,
		`data-tombscript-guess-form`,
		`data-tombscript-guess-history`,
		`Previous incorrect card guesses`,
		`Definitely Not This Card`,
		`type="text"`,
		`autocomplete="off"`,
		`spellcheck="false"`,
		`data-tombscript-give-up-form`,
		`name="game_id" value="73"`,
		`data-tombscript-help-open`,
		`id="tombscript-help-modal"`,
		`aria-label="Close How to Play"`,
		`data-auto-open="true"`,
		`Mystery cards come from Scryfall’s EDHREC ranks 1–250`,
		`This is your first game today, but guest rounds cannot win cards. Sign in before starting your first game tomorrow to be eligible.`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("Tombscript page missing %q", needle)
		}
	}
	wantKeyCount := 26 + 10 + len(spellifySymbolKeyDefinitions)
	if got := strings.Count(body, `data-tombscript-key=`); got != wantKeyCount {
		t.Fatalf("Tombscript rendered %d keys, want %d letters/digits/symbols", got, wantKeyCount)
	}
	for _, digit := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"} {
		plainKey := `data-tombscript-key="` + digit + `"`
		if !strings.Contains(body, plainKey) {
			t.Fatalf("Tombscript omitted plain numeric keyboard key %q", digit)
		}
	}
	for _, removedStatus := range []string{
		`<dt>Result</dt>`,
		`<dt>Reveals</dt>`,
		`<dt>Remaining</dt>`,
		`<dt>Card guesses</dt>`,
		`<dt>Award</dt>`,
		`data-tombscript-count`,
		`data-tombscript-card-guesses-left`,
		`data-tombscript-award`,
	} {
		if strings.Contains(body, removedStatus) {
			t.Fatalf("Tombscript retained removed under-title status %q", removedStatus)
		}
	}
	if got := strings.Count(body, `name="game_id" value="73"`); got != 3 {
		t.Fatalf("active Tombscript rendered %d scoped game IDs, want reveal, guess, and give-up", got)
	}
	for _, secret := range []string{
		"Secret Puzzle Card",
		"Legendary Artifact",
		"secret-puzzle.jpg",
		"11111111-1111-1111-1111-111111111111",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("active Tombscript HTML leaked hidden target data %q", secret)
		}
	}
}

func TestSpellifyCompletedTemplateShowsResultAndScopedReplay(t *testing.T) {
	page := spellifyTemplateFixture("won")
	body := renderTemplate(t, "spellify", TemplateData{Data: page})

	for _, needle := range []string{
		`class="mt-tombscript__game mt-tombscript__game--completed"`,
		`aria-label="View Secret Puzzle Card card details"`,
		`id="tombscript-result-title">Secret Puzzle Card</h2>`,
		`name="action" value="new"`,
		`name="game_id" value="73"`,
		`Play Again`,
		`View Card`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("completed Tombscript page missing %q", needle)
		}
	}
	for _, forbidden := range []string{
		`data-tombscript-character-form`,
		`data-tombscript-guess-form`,
		`data-tombscript-give-up-form`,
		`data-tombscript-guess-modal`,
		`renderPreviousWrongGuesses`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("completed Tombscript retained active control %q", forbidden)
		}
	}
}

func TestSpellifyAssetsUseSharedTokensAndSafeRoundState(t *testing.T) {
	withRendererRoot(t)

	cssBody, err := os.ReadFile(filepath.Join("internal", "web", "assets", "spellify.css"))
	if err != nil {
		t.Fatalf("read Tombscript stylesheet: %v", err)
	}
	css := string(cssBody)
	for _, needle := range []string{
		`body[data-page="spellify"]`,
		`var(--mt-bg)`,
		`var(--mt-surface)`,
		`var(--mt-text)`,
		`var(--mt-accent-soft)`,
		`var(--mt-positive)`,
		`var(--mt-negative)`,
		`.mt-tombscript__text-clues img`,
		`width: 1.15em`,
		`@media (max-width: 43rem)`,
		`animation: none !important`,
		`transition: none !important`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("Tombscript stylesheet missing %q", needle)
		}
	}
	for _, forbidden := range []string{"slate-", "amber-", "emerald-", "#100d13", "rgb("} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("Tombscript stylesheet hard-coded palette value %q", forbidden)
		}
	}

	jsBody, err := os.ReadFile(filepath.Join("internal", "web", "assets", "spellify.js"))
	if err != nil {
		t.Fatalf("read Tombscript script: %v", err)
	}
	js := string(jsBody)
	for _, needle := range []string{
		`body.set("game_id"`,
		`var helpStorageKey = "manatomb.tombscript.howToPlaySeen.v2"`,
		`window.localStorage.getItem(helpStorageKey) !== "1"`,
		`window.localStorage.setItem(helpStorageKey, "1")`,
		`element.setAttribute("inert", "")`,
		`event.key === "Escape"`,
		`window.confirm("Reveal the card and end this round?")`,
		`[\p{L}\p{N}]`,
		`window.location.assign(payload.Redirect)`,
		`data-tombscript-guess-modal`,
		`raw === "_"`,
	} {
		if !strings.Contains(js, needle) {
			t.Fatalf("Tombscript script missing %q", needle)
		}
	}

	templateBody, err := os.ReadFile(filepath.Join("internal", "web", "templates", "spellify.html.tmpl"))
	if err != nil {
		t.Fatalf("read Tombscript template: %v", err)
	}
	for _, forbidden := range []string{"<main", "slate-", "amber-", "emerald-", "data-tombscript-card-back"} {
		if strings.Contains(string(templateBody), forbidden) {
			t.Fatalf("Tombscript template retained legacy treatment %q", forbidden)
		}
	}
}
