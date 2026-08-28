package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

func readThemeCSS(t *testing.T) string {
	t.Helper()
	withRendererRoot(t)

	body, err := os.ReadFile(filepath.Join("internal", "web", "assets", "theme.css"))
	if err != nil {
		t.Fatalf("read theme stylesheet: %v", err)
	}
	return string(body)
}

func renderedOpeningTag(t *testing.T, body, element string) string {
	t.Helper()
	start := strings.Index(body, "<"+element)
	if start == -1 {
		t.Fatalf("rendered page missing <%s> opening tag", element)
	}
	end := strings.Index(body[start:], ">")
	if end == -1 {
		t.Fatalf("rendered page has unterminated <%s> opening tag", element)
	}
	return body[start : start+end+1]
}

func renderedCSSRule(t *testing.T, body, selector string) string {
	t.Helper()
	start := strings.Index(body, selector+" {")
	if start == -1 {
		t.Fatalf("shared theme missing %q rule", selector)
	}
	end := strings.Index(body[start:], "}")
	if end == -1 {
		t.Fatalf("shared theme has unterminated %q rule", selector)
	}
	return body[start : start+end+1]
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
	if !strings.Contains(body, `detailURL.searchParams.set('printing', selectedPrintingID);`) ||
		!strings.Contains(body, `version.scryfall_id`) {
		t.Fatalf("deck_show modal does not preserve the selected printing on its full-page link: %s", body)
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

func TestDeckShowCommanderVersionUsesDeckLevelPersistence(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		CurrentUser: &account.User{ID: 7},
		Data: deckPageData{
			Deck: &decks.Deck{
				ID:               42,
				Name:             "Saved Commander",
				Format:           "Commander",
				CommanderName:    "Atraxa, Grand Unifier",
				CommanderPrintID: "223e4567-e89b-12d3-a456-426614174000",
			},
			WorkspaceState: workspaceDeckState{
				ID:               42,
				Name:             "Saved Commander",
				Format:           "Commander",
				CommanderName:    "Atraxa, Grand Unifier",
				CommanderPrintID: "223e4567-e89b-12d3-a456-426614174000",
				Cards:            map[string]int{},
				SideboardCards:   map[string]int{},
				MaybeCards:       map[string]int{},
				CardMeta:         map[string]workspaceCardMeta{},
			},
		},
	})

	for _, needle := range []string{
		`params.set('action', 'set_commander_print')`,
		`setSavedStatus('Commander version updated.')`,
		`"commanderPrintID":"223e4567-e89b-12d3-a456-426614174000"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("saved commander printing flow missing %q", needle)
		}
	}
	if strings.Contains(body, "Could not add commander card.") {
		t.Fatal("saved commander printing still creates a phantom mainboard row")
	}
}

func TestDeckShowKeepsGuestAuthInSharedHeaderOnly(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:   "Guest Save",
				Format: "Commander",
			},
			WorkbenchMode: true,
		},
	})

	if strings.Contains(body, `id="guest-save-auth-panel"`) ||
		strings.Contains(body, `id="guest-panel-login-to-save"`) ||
		strings.Contains(body, `id="guest-panel-signup-to-save"`) {
		t.Fatalf("deck_show template rendered duplicate guest auth controls: %s", body)
	}
	if !strings.Contains(body, `data-guest-auth-save-link`) {
		t.Fatalf("deck_show template lost the shared-header auth entry point: %s", body)
	}
}

func TestGuestWorkbenchAuthEntryPreservesAndResumesDraftState(t *testing.T) {
	withRendererRoot(t)
	app := &App{Renderer: NewRenderer()}
	req := httptest.NewRequest(http.MethodGet, "/decks/new/workbench?format=Sandbox&sandbox=1", nil)
	rec := httptest.NewRecorder()

	app.HandleDeckWorkbench(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest workbench returned status %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	nameMatch := regexp.MustCompile(`(?s)id="guest-deck-name-input".*?value="([A-Za-z0-9_]+)"`).FindStringSubmatch(body)
	if len(nameMatch) != 2 {
		t.Fatalf("guest workbench did not render a generated deck name: %s", body)
	}
	generatedName := nameMatch[1]
	if !regexp.MustCompile(`^[A-Za-z0-9_]{6,32}$`).MatchString(generatedName) {
		t.Fatalf("guest workbench deck name is not a generated gamertag: %q", generatedName)
	}
	if !strings.Contains(body, `"name":"`+generatedName+`"`) {
		t.Fatalf("workspace seed did not preserve rendered deck name %q", generatedName)
	}
	returnPath := deckWorkbenchPath(deckWorkbenchOptions{
		Format:        "Sandbox",
		Sandbox:       true,
		SaveWorkbench: true,
	})
	encodedReturn := url.QueryEscape(returnPath)
	for _, needle := range []string{
		`href="/login?next=` + encodedReturn + `"`,
		`data-guest-auth-save-link`,
		`guestHeaderAuthSaveLink.href = loginHref`,
		`function persistGuestOverviewBeforeAuth()`,
		`guestTagsField`,
		`link.addEventListener('pointerdown', persistGuestOverviewBeforeAuth)`,
		`link.addEventListener('click', persistGuestOverviewBeforeAuth)`,
		`dd.description = String(descEl`,
		`dd.tags = String(guestTagsField`,
		`saveDraft(dd)`,
		`isSignedIn && shouldAutoSave`,
		`importDraftToAccount();`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("guest workbench auth-resume flow missing %q: %s", needle, body)
		}
	}
	if got := strings.Count(body, `href="/login?next=`+encodedReturn+`"`); got != 1 {
		t.Fatalf("guest workbench rendered %d state-preserving sign-in links, want the shared header only: %s", got, body)
	}
}

func TestDeckListTemplateUsesFlatThemeAwareLibraryAndMobileMenu(t *testing.T) {
	body := renderTemplate(t, "decks_list", TemplateData{
		Data: []deckListItem{{
			ID:                1,
			DeckPath:          "/decks/1",
			Name:              "Header Test",
			Description:       "A saved deck description.",
			Format:            "Commander",
			CommanderName:     "Alela, Artful Provocateur",
			CommanderImageURI: "https://example.test/alela.jpg",
			IsPublic:          true,
			PublicSlug:        "header-test",
			PowerBracket:      "3 - Upgraded",
		}},
	})

	for _, needle := range []string{
		`href="/assets/account_pages.css"`,
		`href="/assets/deck_tile.css"`,
		`class="mt-account-page mt-deck-library"`,
		`class="mt-deck-library__grid" aria-label="Saved decks"`,
		`class="mt-deck-tile"`,
		`href="/decks/1"`,
		`loading="lazy"`,
		`Commander:</span>`,
		`Alela, Artful Provocateur`,
		`Bracket:</span> 3 - Upgraded`,
		`href="/decks/new"`,
		`Build New Deck`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("decks_list template missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, `href="/decks/public/header-test"`) || strings.Contains(body, `mt-deck-tile__meta`) {
		t.Fatalf("My Decks should use the editor path and no format pill: %s", body)
	}
	if !strings.Contains(body, `data-site-menu-toggle`) || !strings.Contains(body, `aria-controls="site-primary-navigation"`) {
		t.Fatalf("layout header did not render the mobile menu control: %s", body)
	}

	pageStart := strings.Index(body, `<div class="mt-account-page mt-deck-library">`)
	if pageStart == -1 {
		t.Fatalf("could not isolate My Decks content: %s", body)
	}
	pageEnd := strings.Index(body[pageStart:], `<footer id="site-footer"`)
	if pageEnd == -1 {
		t.Fatalf("could not isolate My Decks content: %s", body)
	}
	page := body[pageStart : pageStart+pageEnd]
	for _, forbidden := range []string{`mt-panel`, `mt-action-card`, `text-slate-`, `bg-slate-`, `border-slate-`} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("My Decks retained legacy or nested-panel styling %q: %s", forbidden, page)
		}
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
		`id="public-deck-copy-link"`,
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
				ProfileTile:         true,
			}},
			Stats: profileStatsData{
				CanEdit:                  true,
				JoinedLabel:              "Mar 2025",
				UsuallyPlays:             "Commander",
				AveragePowerBracket:      "3 / 5",
				FavoriteColorCombination: "Golgari",
				AvatarChoices: []profileAvatarChoice{{
					Name:       "Meren of Clan Nel Toth",
					ImageURI:   "https://example.test/meren-art.jpg",
					IsSelected: true,
				}},
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
		`id="profile-share"`,
		`href="/settings" class="mt-btn mt-btn--secondary mt-btn--sm">Settings</a>`,
		`id="profile-customize-modal"`,
		`aria-hidden="true"`,
		`aria-describedby="profile-customize-copy"`,
		`role="document" tabindex="-1"`,
		`aria-pressed="true"`,
		`src="https://example.test/meren-art.jpg"`,
		`mt-deck-tile__image--art`,
		`aria-current="page"`,
		`data-profile-avatar-search`,
		`data-card-autocomplete-input`,
		`data-card-autocomplete-oracle-id`,
		`aria-controls="profile-avatar-card-results"`,
		`>Random Card Art</button>`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("profile decks view missing %q: %s", needle, body)
		}
	}
	for _, removed := range []string{
		`>Player Profile<`,
		`>Published Collection<`,
		`>Card Collection<`,
		`>Trophy Case<`,
		`>Profile Art<`,
		`>Use Art</button>`,
		`>Reset Art</button>`,
		`>Account Settings</a>`,
		`<dt>Public Decks</dt>`,
		`<dt>Favorite Printings</dt>`,
		`<dt>Cards Won</dt>`,
		`<dt>Published Deck Average</dt>`,
	} {
		if strings.Contains(body, removed) {
			t.Fatalf("profile refresh still renders decorative label %q: %s", removed, body)
		}
	}

	customizeIndex := strings.Index(body, `data-profile-customize-open`)
	shareIndex := strings.Index(body, `id="profile-share"`)
	settingsIndex := strings.Index(body, `href="/settings" class="mt-btn`)
	if customizeIndex < 0 || shareIndex < customizeIndex || settingsIndex < shareIndex {
		t.Fatalf("profile actions should render in Customize, Share Profile, Settings order: %s", body)
	}
}

func TestProfileAssetsUseThemeTokensAndTrapCustomizeFocus(t *testing.T) {
	withRendererRoot(t)

	cssBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "profile.css"))
	if err != nil {
		t.Fatalf("read profile CSS: %v", err)
	}
	css := string(cssBytes)
	for _, needle := range []string{
		`color: var(--mt-text-soft);`,
		`background: var(--mt-surface);`,
		`border-color: var(--mt-border-strong);`,
		`-webkit-line-clamp: 2;`,
		`.mt-profile-avatar-choices button[aria-pressed="true"]`,
		`.mt-profile-customize-modal`,
		`max-height: calc(100dvh - 2rem);`,
		`overflow-y: auto;`,
		`scrollbar-gutter: stable;`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("profile stylesheet missing %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "#", "translateY(", "transition:"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("profile stylesheet still contains hard-coded color or lift motion %q", forbidden)
		}
	}

	jsBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "profile.js"))
	if err != nil {
		t.Fatalf("read profile JavaScript: %v", err)
	}
	js := string(jsBytes)
	for _, needle := range []string{
		`function customizeModalFocusables()`,
		`if (event.key !== "Tab") return;`,
		`last.focus();`,
		`first.focus();`,
		`customizeModal.setAttribute("aria-hidden", "false");`,
		`customizeModal.setAttribute("aria-hidden", "true");`,
		`document.addEventListener("focusin"`,
		`restoreFocus.focus({ preventScroll: true })`,
	} {
		if !strings.Contains(js, needle) {
			t.Fatalf("profile modal keyboard support missing %q", needle)
		}
	}
}

func TestGuestHomeHandlerIsSearchFirst(t *testing.T) {
	withRendererRoot(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	app := &App{Renderer: NewRenderer()}
	app.HandleHome(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest home returned status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("guest home is missing the Tomb palette: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="guest-home"`) {
		t.Fatalf("guest home body is missing its page scope: %s", bodyTag)
	}

	for _, needle := range []string{
		`href="/assets/theme.css"`,
		`class="mt-guest-home max-w-3xl`,
		`id="home-card-search"`,
		`type="search"`,
		`autofocus`,
		`class="mt-home-search-shell"`,
		`class="mt-control flex-1 mt-home-search-input"`,
		`class="mt-autocomplete-menu hidden absolute`,
		`min-height: calc(100svh - 3rem)`,
		`class="grid grid-cols-2 gap-3 md:grid-cols-4"`,
		`href="/login"`,
		`id="home-games-title"`,
		`href="/games/guess-card"`,
		`href="/games/spellify"`,
		`href="/games/pack-opening"`,
		`href="/decks/new" class="mt-btn mt-btn--secondary mt-btn--sm mt-home-action">Build a Deck</a>`,
		`href="/cards/random" class="mt-home-search-random">Random Card</a>`,
		`body[data-page="guest-home"] *`,
		`animation: none;`,
		`transition: none;`,
		`classList.toggle('is-active', active)`,
		`button.className = 'mt-autocomplete-option'`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("guest home page missing %q: %s", needle, body)
		}
	}

	actionNeedles := []string{
		`href="/decks/public"`,
		`href="/cards/search"`,
		`href="/decks/new"`,
		`href="/login"`,
	}
	previousActionIndex := -1
	for _, needle := range actionNeedles {
		actionIndex := strings.Index(body, needle)
		if actionIndex <= previousActionIndex {
			t.Fatalf("guest home action %q is missing or out of order: %s", needle, body)
		}
		previousActionIndex = actionIndex
	}
	if got := strings.Count(body, `mt-home-action"`); got != 4 {
		t.Fatalf("guest home has %d equal guest actions, want 4: %s", got, body)
	}
	if got := strings.Count(body, `href="/cards/random"`); got != 1 {
		t.Fatalf("guest home has %d Random Card links, want 1: %s", got, body)
	}
	searchIndex := strings.Index(body, `id="home-card-search"`)
	randomIndex := strings.Index(body, `href="/cards/random"`)
	primaryActionIndex := strings.Index(body, `href="/decks/public"`)
	if searchIndex == -1 || randomIndex <= searchIndex || primaryActionIndex <= randomIndex {
		t.Fatalf("guest Random Card action is not directly between search and primary actions: %s", body)
	}

	for _, needle := range []string{
		`class="mt-site-header__inner max-w-7xl`,
		`id="site-card-search"`,
		`data-site-menu-toggle`,
		`<button type="submit" class="mt-btn mt-btn--primary">Search</button>`,
		`href="/decks/new" class="mt-btn mt-btn--primary mt-btn--sm mt-home-action">Build a Deck</a>`,
		`>Builder</a>`,
		`>Browse Decks</a>`,
		`text-slate-500`,
		`>Play now<`,
		`>Open a pack<`,
		`mt-random-card-cta`,
		`border-slate-700 bg-slate-950 shadow-2xl shadow-slate-950/70`,
		`classList.toggle('bg-slate-800', active)`,
		`classList.toggle('text-sky-200', active)`,
		`.mt-home-search-shell:focus-within {`,
		`@keyframes mtRandomCardGlow`,
		`animation: mtRandomCardGlow`,
		`prefers-reduced-motion`,
		`--mt-random-`,
	} {
		if strings.Contains(body, needle) {
			t.Fatalf("guest home page unexpectedly contains %q: %s", needle, body)
		}
	}
}

func TestSharedThemeDefinesReadableSemanticDefaults(t *testing.T) {
	body := renderTemplate(t, "home", TemplateData{HideHeader: true})
	themeCSS := readThemeCSS(t)

	if !strings.Contains(body, `href="/assets/theme.css"`) {
		t.Fatalf("layout does not load the shared theme stylesheet: %s", body)
	}
	if !strings.Contains(body, `href="/assets/tailwind.css"`) {
		t.Fatalf("layout does not load the compiled Tailwind stylesheet: %s", body)
	}
	for _, forbidden := range []string{`cdn.tailwindcss.com`, `tailwind.config =`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("layout still contains browser-side Tailwind runtime %q: %s", forbidden, body)
		}
	}

	classicPaletteRule := renderedCSSRule(t, themeCSS, ":root,\nhtml[data-theme=\"classic\"],\n[data-theme-preview=\"classic\"]")
	for _, needle := range []string{
		`--mt-palette-bg: rgb(2 6 23);`,
		`--mt-palette-surface: rgb(15 23 42);`,
		`--mt-palette-raised: rgb(30 41 59);`,
		`--mt-palette-border: rgb(100 116 139);`,
		`--mt-palette-text: rgb(248 250 252);`,
		`--mt-palette-text-soft: rgb(203 213 225);`,
		`--mt-palette-muted: rgb(148 163 184);`,
		`--mt-palette-accent: rgb(14 165 233);`,
		`--mt-palette-positive: rgb(52 211 153);`,
		`--mt-palette-negative: rgb(248 113 113);`,
	} {
		if !strings.Contains(classicPaletteRule, needle) {
			t.Fatalf("classic theme missing %q: %s", needle, classicPaletteRule)
		}
	}

	rootRule := renderedCSSRule(t, themeCSS, ":root")
	for _, needle := range []string{
		`--mt-font-sans:`,
		`--mt-bg: var(--mt-palette-bg);`,
		`--mt-border: color-mix(`,
		`--mt-text: var(--mt-palette-text);`,
		`--mt-positive: var(--mt-palette-positive);`,
		`--mt-positive-soft: color-mix(`,
		`--mt-positive-border: color-mix(`,
		`--mt-negative: var(--mt-palette-negative);`,
		`--mt-negative-soft: color-mix(`,
		`--mt-negative-border: color-mix(`,
		`--mt-focus: var(--mt-palette-accent-strong);`,
		`--mt-control-font-size: 1rem;`,
		`--mt-shadow-panel:`,
		`--mt-shadow-card:`,
	} {
		if !strings.Contains(rootRule, needle) {
			t.Fatalf("root theme missing %q: %s", needle, rootRule)
		}
	}

	for selector, needles := range map[string][]string{
		"body.mt-app": {
			`background: var(--mt-bg);`,
			`color: var(--mt-text);`,
			`font-family: var(--mt-font-sans);`,
		},
		".mt-btn--secondary": {
			`border: 1px solid var(--mt-border-control);`,
			`background: var(--mt-surface-strong);`,
			`color: var(--mt-text);`,
		},
		".mt-panel": {
			`border: 1px solid var(--mt-border);`,
			`background: var(--mt-surface);`,
			`box-shadow: var(--mt-shadow-panel);`,
		},
		".mt-action-card": {
			`border: 1px solid var(--mt-border);`,
			`background: var(--mt-surface);`,
			`box-shadow: var(--mt-shadow-card);`,
		},
		".mt-control": {
			`border: 1px solid var(--mt-border-control);`,
			`background: var(--mt-surface-strong);`,
			`font-size: var(--mt-control-font-size);`,
			`color: var(--mt-text);`,
		},
		".mt-control::placeholder": {
			`color: var(--mt-text-muted);`,
			`opacity: 1;`,
		},
		".mt-control:focus-visible": {
			`outline: 2px solid var(--mt-focus);`,
			`border-color: var(--mt-focus-border);`,
		},
	} {
		rule := renderedCSSRule(t, body, selector)
		for _, needle := range needles {
			if !strings.Contains(rule, needle) {
				t.Fatalf("%s missing %q: %s", selector, needle, rule)
			}
		}
	}
}

func TestGuestHomeThemeIsScopedAndTokenDriven(t *testing.T) {
	body := renderTemplate(t, "home", TemplateData{HideHeader: true})
	themeCSS := readThemeCSS(t)

	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("guest palette marker missing from html: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="guest-home"`) {
		t.Fatalf("guest page marker missing from body: %s", bodyTag)
	}

	themeRule := renderedCSSRule(t, themeCSS, "html[data-theme=\"tomb\"],\n[data-theme-preview=\"tomb\"]")
	for _, needle := range []string{
		`--mt-palette-bg: #100d13;`,
		`--mt-palette-surface: #1a151f;`,
		`--mt-palette-raised: #2b2230;`,
		`--mt-palette-border: rgb(154 126 147 / 0.8);`,
		`--mt-palette-text: #f5f0e8;`,
		`--mt-palette-text-soft: #d8cec3;`,
		`--mt-palette-muted: #b3a6a8;`,
		`--mt-palette-accent: #c89a55;`,
		`--mt-palette-accent-strong: #e0b56f;`,
		`--mt-palette-accent-text: #e8c98f;`,
		`--mt-palette-positive: #8fc79a;`,
		`--mt-palette-negative: #e58b86;`,
	} {
		if !strings.Contains(themeRule, needle) {
			t.Fatalf("guest home theme missing %q: %s", needle, themeRule)
		}
	}

	for selector, needles := range map[string][]string{
		".mt-home-search-shell,\n    .mt-site-search-shell": {
			`border: 1px solid var(--mt-border-control);`,
			`background: var(--mt-surface-strong);`,
		},
		".mt-home-search-random,\n    .mt-site-search-random": {
			`border-left: 1px solid var(--mt-border-strong);`,
			`border-radius: 0 0.68rem 0.68rem 0;`,
			`background: var(--mt-accent-soft);`,
			`color: var(--mt-accent-text);`,
		},
		".mt-autocomplete-menu": {
			`border: 1px solid var(--mt-border);`,
			`background: var(--mt-surface-strong);`,
			`box-shadow: var(--mt-shadow-popover);`,
		},
		".mt-autocomplete-option": {
			`color: var(--mt-text-soft);`,
		},
		".mt-kicker": {
			`color: var(--mt-accent-strong);`,
		},
	} {
		rule := renderedCSSRule(t, body, selector)
		for _, needle := range needles {
			if !strings.Contains(rule, needle) {
				t.Fatalf("%s missing %q: %s", selector, needle, rule)
			}
		}
	}

	randomRule := renderedCSSRule(t, body, ".mt-home-search-random,\n    .mt-site-search-random")
	for _, forbidden := range []string{`animation:`, `transition:`, `rgb(`, `rgba(`, `#`, `--mt-random-`} {
		if strings.Contains(randomRule, forbidden) {
			t.Fatalf("guest Random Card rule contains non-theme or motion value %q: %s", forbidden, randomRule)
		}
	}

	inputFocusRule := renderedCSSRule(t, body, ".mt-home-search-shell .mt-home-search-input:focus-visible,\n    .mt-site-search-shell .mt-site-search-input:focus-visible")
	if !strings.Contains(inputFocusRule, `outline: none;`) {
		t.Fatalf("guest search focus rule should remain visually static: %s", inputFocusRule)
	}
	for _, forbidden := range []string{
		`.mt-home-search-shell:focus-within {`,
		`@keyframes mtRandomCardGlow`,
		`animation: mtRandomCardGlow`,
		`prefers-reduced-motion`,
		`--mt-random-`,
	} {
		if strings.Contains(body, forbidden) || strings.Contains(themeCSS, forbidden) {
			t.Fatalf("guest theme unexpectedly contains %q", forbidden)
		}
	}
}

func TestSharedFooterRendersCompactNavigationAndAttribution(t *testing.T) {
	body := renderTemplate(t, "home", TemplateData{HideHeader: true})

	for _, needle := range []string{
		`<body id="top"`,
		`<footer id="site-footer" class="mt-site-footer mt-8">`,
		`<nav aria-label="Footer navigation">`,
		`class="mt-footer-links flex flex-wrap`,
		`href="#top" class="mt-footer-link shrink-0">Back to top</a>`,
		`Card data &copy;`,
		`card images &copy;`,
		`Changelog <span aria-hidden="true">&middot; v1.0</span>`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("shared footer missing %q: %s", needle, body)
		}
	}

	anchorFor := func(label string) string {
		t.Helper()
		labelIndex := strings.Index(body, label)
		if labelIndex == -1 {
			t.Fatalf("shared footer missing link label %q: %s", label, body)
		}
		start := strings.LastIndex(body[:labelIndex], "<a ")
		endOffset := strings.Index(body[labelIndex:], "</a>")
		if start == -1 || endOffset == -1 {
			t.Fatalf("could not isolate shared footer link %q: %s", label, body)
		}
		return body[start : labelIndex+endOffset+len("</a>")]
	}

	for label, href := range map[string]string{
		"Changelog":            "/changelog",
		"Privacy":              "/privacy",
		"Terms":                "/terms",
		"GitHub":               projectRepoURL,
		"Report an Issue":      projectIssuesURL,
		"Scryfall":             "https://scryfall.com",
		"Wizards of the Coast": "https://company.wizards.com/en",
	} {
		anchor := anchorFor(label)
		if !strings.Contains(anchor, `href="`+href+`"`) {
			t.Fatalf("%s footer link has wrong destination: %s", label, anchor)
		}
		if strings.HasPrefix(href, "http") {
			for _, needle := range []string{`target="_blank"`, `rel="noopener noreferrer"`, `opens in a new tab`} {
				if !strings.Contains(anchor, needle) {
					t.Fatalf("%s external footer link missing %q: %s", label, needle, anchor)
				}
			}
		}
	}

	for _, oldCopy := range []string{`>Legal</p>`, `>Support</p>`, `>Data</p>`, `Contact / Issues`, `Ko-fi`} {
		if strings.Contains(body, oldCopy) {
			t.Fatalf("shared footer unexpectedly contains %q: %s", oldCopy, body)
		}
	}
}

func TestSharedFooterCanBeHidden(t *testing.T) {
	body := renderTemplate(t, "home", TemplateData{HideHeader: true, HideFooter: true})
	if strings.Contains(body, `<footer id="site-footer"`) || strings.Contains(body, `aria-label="Footer navigation"`) {
		t.Fatalf("shared footer rendered despite HideFooter: %s", body)
	}
}

func TestSignedInHomeMatchesGuestHomeWithAccountAction(t *testing.T) {
	body := renderTemplate(t, "home", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Deck Brewer", SiteTheme: string(siteThemeVerdigris)},
		HideHeader:  true,
	})

	for _, needle := range []string{
		`class="mt-guest-home max-w-3xl`,
		`<p class="mt-kicker">Build MTG Decks</p>`,
		`Across every format.`,
		`Absolutely free.`,
		`class="mt-home-search-shell"`,
		`class="mt-control flex-1 mt-home-search-input"`,
		`href="/cards/random" class="mt-home-search-random">Random Card</a>`,
		`<button type="submit" class="sr-only">Search cards</button>`,
		`href="/decks/public"`,
		`href="/cards/search"`,
		`href="/decks/new"`,
		`href="/decks" class="mt-btn mt-btn--secondary mt-btn--sm mt-home-action">My Decks</a>`,
		`href="/users/42" class="mt-btn mt-btn--secondary mt-btn--sm mt-home-action">Profile</a>`,
		`id="home-games-title"`,
		`href="/games/guess-card"`,
		`href="/games/spellify"`,
		`href="/games/pack-opening"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("signed-in home page missing %q: %s", needle, body)
		}
	}

	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="verdigris"`) {
		t.Fatalf("signed-in home did not keep the account theme: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="guest-home"`) {
		t.Fatalf("signed-in home is missing the shared landing-page scope: %s", bodyTag)
	}
	if !strings.Contains(body, `id="home-card-search"`) || !strings.Contains(body, `autofocus`) {
		t.Fatalf("signed-in home search should autofocus: %s", body)
	}
	for _, forbidden := range []string{
		`class="mt-site-header`,
		`href="/assets/account_pages.css"`,
		`Sign In/Sign Up`,
		`Recent Decks`,
		`mt-account-home`,
		`<button type="submit" class="mt-btn mt-btn--primary">Search</button>`,
		`text-slate-`,
		`bg-slate-`,
		`border-slate-`,
		`Solve the daily card in fewer than 10 guesses`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("signed-in home retained legacy markup %q: %s", forbidden, body)
		}
	}
}

func TestSignedInHomeHandlerDoesNotRequireDeckDashboardData(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(context.WithValue(request.Context(), ctxKeyUser, &account.User{
		ID:          42,
		DisplayName: "Deck Brewer",
		SiteTheme:   string(siteThemeMoonlit),
	}))

	app := &App{Renderer: NewRenderer()}
	app.HandleHome(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("signed-in home returned %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, needle := range []string{
		`data-theme="moonlit"`,
		`data-page="guest-home"`,
		`href="/decks" class="mt-btn mt-btn--secondary mt-btn--sm mt-home-action">My Decks</a>`,
		`href="/users/42" class="mt-btn mt-btn--secondary mt-btn--sm mt-home-action">Profile</a>`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("signed-in home handler missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, `class="mt-site-header`) || strings.Contains(body, "Recent Decks") {
		t.Fatalf("signed-in home handler retained the old dashboard/header: %s", body)
	}
}

func TestPackOpeningReceivesSharedTombPageDefaults(t *testing.T) {
	got, ok := applyTemplateDefaults("pack_opening", TemplateData{}).(TemplateData)
	if !ok {
		t.Fatal("pack-opening defaults did not return TemplateData")
	}
	if got.Theme != siteThemeTomb {
		t.Fatalf("pack-opening theme = %q, want %q", got.Theme, siteThemeTomb)
	}
	if !got.WideLayout {
		t.Fatal("pack-opening should use the wide shared layout")
	}
	if got.PageID != "pack-opening" {
		t.Fatalf("pack-opening page ID = %q, want pack-opening", got.PageID)
	}
	if got.Meta == nil || got.Meta.Title != "Pack Crack" || got.Meta.Description == "" {
		t.Fatalf("pack-opening metadata = %#v", got.Meta)
	}
}

func TestSharedHeaderUsesFlatThemeDrivenAccountSlot(t *testing.T) {
	headerHTML := func(body string) string {
		t.Helper()
		start := strings.Index(body, `<header class="mt-site-header">`)
		end := strings.Index(body, `</header>`)
		if start == -1 || end <= start {
			t.Fatalf("could not isolate shared header: %s", body)
		}
		return body[start : end+len(`</header>`)]
	}

	guestBody := renderTemplate(t, "privacy", TemplateData{})
	guestHeader := headerHTML(guestBody)
	for _, needle := range []string{
		`class="mt-site-brand shrink-0"`,
		`role="search" aria-label="Card search"`,
		`id="site-card-search"`,
		`type="search"`,
		`aria-controls="site-card-search-results"`,
		`id="site-card-search-results"`,
		`class="mt-site-search-shell"`,
		`class="mt-control mt-control--sm mt-site-search-input flex-1"`,
		`href="/cards/random" class="mt-site-search-random">Random Card</a>`,
		`class="sr-only">Search cards</button>`,
		`>Build a Deck</a>`,
		`>Advanced Search</a>`,
		`>Browse Public Decks</a>`,
		`href="/login" class="mt-btn mt-btn--primary mt-btn--sm mt-site-account-action"`,
		`Sign In/Sign Up`,
		`data-site-menu-toggle`,
		`data-open="false" data-site-nav`,
	} {
		if !strings.Contains(guestHeader, needle) {
			t.Fatalf("guest shared header missing %q: %s", needle, guestHeader)
		}
	}
	if got := strings.Count(guestHeader, "mt-site-account-action"); got != 1 {
		t.Fatalf("guest shared header has %d account actions, want 1: %s", got, guestHeader)
	}
	searchIndex := strings.Index(guestHeader, `id="site-card-search"`)
	randomIndex := strings.Index(guestHeader, `class="mt-site-search-random"`)
	submitIndex := strings.Index(guestHeader, `class="sr-only">Search cards</button>`)
	if searchIndex == -1 || randomIndex <= searchIndex || submitIndex <= randomIndex {
		t.Fatalf("header search, Random Card, and submit fallback are out of order: %s", guestHeader)
	}
	focusRuleStart := strings.Index(guestBody, `.mt-site-search [data-card-autocomplete-input]:focus,`)
	if focusRuleStart == -1 {
		t.Fatalf("shared header is missing its search focus override: %s", guestBody)
	}
	focusRuleEnd := strings.Index(guestBody[focusRuleStart:], `}`)
	if focusRuleEnd == -1 {
		t.Fatalf("shared header search focus override is unterminated: %s", guestBody[focusRuleStart:])
	}
	focusRule := guestBody[focusRuleStart : focusRuleStart+focusRuleEnd+1]
	for _, needle := range []string{
		`.mt-site-search [data-card-autocomplete-input]:focus-visible`,
		`border-color: transparent;`,
		`outline: none;`,
		`box-shadow: inset 0 1px 0 var(--mt-control-highlight);`,
	} {
		if !strings.Contains(focusRule, needle) {
			t.Fatalf("shared header search focus override missing %q: %s", needle, focusRule)
		}
	}
	for _, forbidden := range []string{
		`>My Decks</a>`,
		`mt-header-search-submit`,
		`border-slate-`,
		`bg-slate-`,
		`text-sky-`,
		`rounded-full border`,
		`>Builder</a>`,
		`>Browse Decks</a>`,
		`>Log In</a>`,
	} {
		if strings.Contains(guestHeader, forbidden) {
			t.Fatalf("guest shared header unexpectedly contains %q: %s", forbidden, guestHeader)
		}
	}

	signedHeader := headerHTML(renderTemplate(t, "decks_list", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Deck Brewer"},
	}))
	for _, needle := range []string{
		`class="mt-site-search-shell"`,
		`href="/cards/random" class="mt-site-search-random">Random Card</a>`,
		`href="/decks" class="mt-btn mt-btn--primary mt-btn--sm mt-site-account-action" aria-current="page"`,
		`My Decks`,
		`>Profile</a>`,
	} {
		if !strings.Contains(signedHeader, needle) {
			t.Fatalf("signed-in shared header missing %q: %s", needle, signedHeader)
		}
	}
	if got := strings.Count(signedHeader, "mt-site-account-action"); got != 1 {
		t.Fatalf("signed-in shared header has %d account actions, want 1: %s", got, signedHeader)
	}
	if strings.Contains(signedHeader, "Sign In/Sign Up") {
		t.Fatalf("signed-in shared header still contains guest account action: %s", signedHeader)
	}
	if strings.Contains(signedHeader, `href="/settings"`) {
		t.Fatalf("signed-in shared header still contains Settings: %s", signedHeader)
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
	if strings.Contains(body, `id="profile-customize-modal"`) {
		t.Fatalf("profile customization must remain owner-only: %s", body)
	}
}
