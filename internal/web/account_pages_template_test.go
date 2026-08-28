package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountPagesStylesheetUsesSharedThemeTokens(t *testing.T) {
	withRendererRoot(t)

	body, err := os.ReadFile(filepath.Join("internal", "web", "assets", "account_pages.css"))
	if err != nil {
		t.Fatalf("read account pages stylesheet: %v", err)
	}
	css := string(body)

	for _, needle := range []string{
		`body[data-page="home"] .mt-account-home`,
		`.mt-deck-library__grid`,
		`color: var(--mt-text);`,
		`color: var(--mt-text-muted);`,
		`background: var(--mt-surface);`,
		`border: 1px solid var(--mt-border-subtle);`,
		`outline: 2px solid var(--mt-focus);`,
		`@media (max-width: 560px)`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("account pages stylesheet missing %q: %s", needle, css)
		}
	}
	if strings.Contains(css, `.mt-deck-library-card`) {
		t.Fatal("My Decks retained a duplicate deck-tile component instead of the shared deck tile")
	}

	for _, forbidden := range []string{`rgb(`, `rgba(`, `text-slate-`, `bg-slate-`, `border-slate-`} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("account pages stylesheet contains hardcoded color %q: %s", forbidden, css)
		}
	}
}

func TestAccountPageStylesheetIsScopedAwayFromGuestHome(t *testing.T) {
	guest := renderTemplate(t, "home", TemplateData{HideHeader: true})
	if strings.Contains(guest, `href="/assets/account_pages.css"`) {
		t.Fatalf("guest home unexpectedly loaded signed account styles: %s", guest)
	}

	emptyDecks := renderTemplate(t, "decks_list", TemplateData{})
	for _, needle := range []string{
		`href="/assets/account_pages.css"`,
		`class="mt-deck-library__empty"`,
		`id="empty-deck-library-title">No decks yet</h2>`,
		`href="/decks/new" class="mt-btn mt-btn--primary">Build a Deck</a>`,
		`href="/decks/public" class="mt-btn mt-btn--secondary">Browse Public Decks</a>`,
	} {
		if !strings.Contains(emptyDecks, needle) {
			t.Fatalf("empty My Decks state missing %q: %s", needle, emptyDecks)
		}
	}
}
