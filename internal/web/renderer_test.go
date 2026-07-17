package web

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"manatomb/app/internal/account"
	"manatomb/app/internal/decks"
)

func withRendererRoot(t *testing.T) {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}
}

func renderTemplate(t *testing.T, name string, data TemplateData) string {
	t.Helper()
	withRendererRoot(t)

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, name, data)
	if rec.Code != 200 {
		t.Fatalf("render %s returned status %d: %s", name, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func TestRendererParsesTemplates(t *testing.T) {
	withRendererRoot(t)
	if renderer := NewRenderer(); renderer == nil {
		t.Fatal("NewRenderer returned nil")
	}
}

func TestDeckShowTemplateIncludesCardDetailModal(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:   "Inline Card View",
				Format: "Commander",
			},
			WorkbenchMode: true,
		},
	})

	if !strings.Contains(body, `id="card-detail-modal"`) {
		t.Fatalf("deck_show template did not render the card detail modal shell: %s", body)
	}
	if !strings.Contains(body, `data-field="open-page-link"`) {
		t.Fatalf("deck_show template did not render the full-page modal action: %s", body)
	}
	if !strings.Contains(body, `window.mtCardDetailModal =`) {
		t.Fatalf("deck_show template did not render the shared card detail modal script: %s", body)
	}
	if !strings.Contains(body, `src="/assets/deck_browser.js"`) {
		t.Fatalf("deck_show template did not load the shared deck browser core: %s", body)
	}
}

func TestDeckShowTemplateSeedsGuestWorkbenchCommanderState(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:          "Guest Commander",
				Format:        "Commander",
				CommanderName: "Atraxa, Grand Unifier",
			},
			WorkspaceState: workspaceDeckState{
				Name:          "New Guest Deck",
				Format:        "Commander",
				CommanderName: "Atraxa, Grand Unifier",
				Cards:         map[string]int{},
				MaybeCards:    map[string]int{},
				CardMeta:      map[string]workspaceCardMeta{},
			},
			WorkbenchMode: true,
		},
	})

	if !strings.Contains(body, `<option value="Commander" selected>Commander</option>`) {
		t.Fatalf("deck_show template did not select Commander for guest workbench: %s", body)
	}
	if !strings.Contains(body, `"format":"Commander"`) {
		t.Fatalf("deck_show template did not seed guest workspace format: %s", body)
	}
	if !strings.Contains(body, `"commanderName":"Atraxa, Grand Unifier"`) {
		t.Fatalf("deck_show template did not seed guest workspace commander: %s", body)
	}
}

func TestDeckShowTemplateIncludesGuestSaveAuthCTAs(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:   "Guest Save",
				Format: "Commander",
			},
			WorkbenchMode: true,
		},
	})

	if !strings.Contains(body, `id="guest-save-auth-panel"`) {
		t.Fatalf("deck_show template did not render guest save auth panel: %s", body)
	}
}

func TestDeckListTemplateIncludesPrimaryBuildActionAndMobileMenu(t *testing.T) {
	body := renderTemplate(t, "decks_list", TemplateData{
		Data: []deckListItem{{ID: 1, Name: "Header Test"}},
	})

	if !strings.Contains(body, `href="/decks/new"`) || !strings.Contains(body, `Build New Deck`) {
		t.Fatalf("decks_list template did not render the primary build action: %s", body)
	}
	if !strings.Contains(body, `data-site-menu-toggle`) || !strings.Contains(body, `aria-controls="site-primary-navigation"`) {
		t.Fatalf("layout header did not render the mobile menu control: %s", body)
	}
}

func TestDefaultActiveNav(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     TemplateData
		want     string
	}{
		{name: "builder", template: "decks_new", want: "builder"},
		{name: "workbench", template: "deck_show", data: TemplateData{Data: deckPageData{WorkbenchMode: true}}, want: "builder"},
		{name: "saved deck", template: "deck_show", data: TemplateData{Data: deckPageData{}}, want: "my-decks"},
		{name: "public deck", template: "decks_public_show", want: "public-decks"},
		{name: "cards", template: "cards_search", want: "cards"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultActiveNav(tt.template, tt.data); got != tt.want {
				t.Fatalf("defaultActiveNav(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}

func TestPublicDeckTemplateUsesSharedBrowserAndExportActions(t *testing.T) {
	body := renderTemplate(t, "decks_public_show", TemplateData{
		Data: publicDeckPageData{
			Deck: &decks.Deck{
				Name:       "Public Browser",
				Format:     "Commander",
				PublicSlug: "public-browser",
			},
			DeckCards: []decks.DeckCard{{
				CardID:   "card-one",
				CardName: "Sol Ring",
				Quantity: 1,
			}},
		},
	})

	for _, needle := range []string{
		`href="/assets/public_deck.css"`,
		`src="/assets/deck_browser.js"`,
		`src="/assets/public_deck.js"`,
		`id="public-deck-share"`,
		`id="public-deck-copy-list"`,
		`id="public-deck-export-txt"`,
		`id="public-deck-export-csv"`,
		`aria-current="page"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("public deck template missing %q: %s", needle, body)
		}
	}
	if strings.Count(body, `id="card-detail-modal"`) != 1 {
		t.Fatalf("public deck template should render one shared card detail modal")
	}
	if strings.Contains(body, "function groupedCards") {
		t.Fatal("public deck template still contains the inline browser controller")
	}
}

func TestProfileTemplateRendersOverhauledDecksView(t *testing.T) {
	body := renderTemplate(t, "profile_show", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Deck Brewer"},
		Data: profilePageData{
			Profile: account.PublicProfile{ID: 42, DisplayName: "Deck Brewer"},
			Items: []deckListItem{{
				Name:                "Featured Deck",
				Format:              "Commander",
				CommanderName:       "Meren of Clan Nel Toth",
				CommanderImageURI:   "https://example.test/meren-card.jpg",
				CommanderArtCropURI: "https://example.test/meren-art.jpg",
				PublicSlug:          "featured-deck",
			}},
			Stats: profileStatsData{
				CanEdit:                  true,
				JoinedLabel:              "Mar 2025",
				UsuallyPlays:             "Commander",
				AveragePowerBracket:      "3 / 5",
				FavoriteColorCombination: "Golgari",
			},
			ActiveTab:          "decks",
			AwardTotal:         2,
			FavoritePagination: profilePagination{Total: 3},
		},
	})

	for _, needle := range []string{
		`href="/assets/profile.css"`,
		`src="/assets/profile.js"`,
		`class="mt-profile-hero"`,
		`id="profile-deck-filter"`,
		`data-profile-deck`,
		`data-profile-customize-open`,
		`id="profile-customize-modal"`,
		`src="https://example.test/meren-art.jpg"`,
		`mt-profile-deck__image--art`,
		`aria-current="page"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("profile decks view missing %q: %s", needle, body)
		}
	}
}

func TestHomeTemplateUsesStandardSiteHeader(t *testing.T) {
	body := renderTemplate(t, "home", TemplateData{})

	for _, needle := range []string{
		`class="mt-site-header__inner max-w-7xl`,
		`id="site-card-search"`,
		`data-site-menu-toggle`,
		`href="/decks/new"`,
		`href="/cards/search"`,
		`href="/decks/public"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("home page standard header missing %q: %s", needle, body)
		}
	}
}

func TestProfileTemplateRendersFavoriteTableWithCardModal(t *testing.T) {
	body := renderTemplate(t, "profile_show", TemplateData{
		Data: profilePageData{
			Profile:      account.PublicProfile{ID: 42, DisplayName: "Collector"},
			Stats:        profileStatsData{JoinedLabel: "Mar 2025", FavoriteColorCombination: "Colorless"},
			ActiveTab:    "favorites",
			FavoriteView: "table",
			FavoritePrintings: []profileFavoritePrintingView{{
				ScryfallID: "printing-one",
				OracleID:   "oracle-one",
				Name:       "Sol Ring",
				SetLabel:   "Commander Masters (CMM)",
				DetailPath: "/cards/view/oracle-one?printing=printing-one",
			}},
			FavoritePagination: profilePagination{Total: 1},
		},
	})

	for _, needle := range []string{
		`mt-profile-favorites--table`,
		`data-card-detail-root`,
		`data-scryfall-id="printing-one"`,
		`id="card-detail-modal"`,
		`window.mtCardDetailModal =`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("profile favorites view missing %q: %s", needle, body)
		}
	}
}
