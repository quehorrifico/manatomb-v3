package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"manatomb/app/internal/account"
)

func TestProfileCustomizeModalIsHiddenUntilOpened(t *testing.T) {
	body := renderTemplate(t, "profile_show", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Deck Brewer"},
		Data: profilePageData{
			Profile:   account.PublicProfile{ID: 42, DisplayName: "Deck Brewer"},
			Stats:     profileStatsData{CanEdit: true},
			ActiveTab: "decks",
		},
	})

	modalID := `id="profile-customize-modal"`
	idStart := strings.Index(body, modalID)
	if idStart < 0 {
		t.Fatalf("rendered profile missing customize modal")
	}
	tagStart := strings.LastIndex(body[:idStart], "<")
	tagEnd := strings.Index(body[idStart:], ">")
	if tagStart < 0 || tagEnd < 0 {
		t.Fatalf("rendered profile has malformed customize modal opening tag")
	}
	modalTag := body[tagStart : idStart+tagEnd+1]
	for _, attribute := range []string{`class="mt-modal mt-profile-customize-modal hidden"`, `aria-hidden="true"`, `hidden`} {
		if !strings.Contains(modalTag, attribute) {
			t.Fatalf("profile customize modal should initially contain %q: %s", attribute, modalTag)
		}
	}
}

func TestSharedModalStacksAboveSiteHeader(t *testing.T) {
	withRendererRoot(t)

	layoutBytes, err := os.ReadFile(filepath.Join("internal", "web", "templates", "layout_header.html.tmpl"))
	if err != nil {
		t.Fatalf("read shared layout: %v", err)
	}
	layout := string(layoutBytes)
	headerZ := cssZIndex(t, renderedCSSRule(t, layout, ".mt-site-header"))
	modalZ := cssZIndex(t, renderedCSSRule(t, layout, ".mt-modal"))
	if modalZ <= headerZ {
		t.Fatalf("shared modal z-index = %d, must be above header z-index %d", modalZ, headerZ)
	}
}

func TestProfileCustomizeModalIsCenteredAndScrollsInternally(t *testing.T) {
	withRendererRoot(t)

	cssBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "profile.css"))
	if err != nil {
		t.Fatalf("read profile stylesheet: %v", err)
	}
	css := string(cssBytes)
	modalRule := renderedCSSRule(t, css, ".mt-profile-customize-modal")
	if !strings.Contains(modalRule, "align-items: center;") {
		t.Fatalf("profile customize modal should center its panel: %s", modalRule)
	}
	panelRule := renderedCSSRule(t, css, ".mt-profile-customize-panel")
	for _, declaration := range []string{
		"max-height: calc(100dvh - 2rem);",
		"margin-block: auto;",
		"overflow-y: auto;",
	} {
		if !strings.Contains(panelRule, declaration) {
			t.Fatalf("centered profile modal must retain bounded internal scrolling %q: %s", declaration, panelRule)
		}
	}
}

func cssZIndex(t *testing.T, rule string) int {
	t.Helper()
	match := regexp.MustCompile(`z-index:\s*(-?\d+)`).FindStringSubmatch(rule)
	if len(match) != 2 {
		t.Fatalf("CSS rule has no numeric z-index: %s", rule)
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("parse z-index %q: %v", match[1], err)
	}
	return value
}

func TestSharedModalHiddenStateOverridesModalDisplay(t *testing.T) {
	withRendererRoot(t)

	layoutBytes, err := os.ReadFile(filepath.Join("internal", "web", "templates", "layout_header.html.tmpl"))
	if err != nil {
		t.Fatalf("read shared layout: %v", err)
	}
	layout := string(layoutBytes)
	modalRule := strings.Index(layout, ".mt-modal {")
	hiddenRule := strings.Index(layout, ".mt-modal.hidden,")
	if modalRule < 0 || hiddenRule < 0 || hiddenRule < modalRule {
		t.Fatalf("shared modal hidden override must follow the base modal display rule")
	}
	if !strings.Contains(layout[hiddenRule:], ".mt-modal[hidden] {") || !strings.Contains(layout[hiddenRule:], "display: none;") {
		t.Fatalf("shared modal hidden override must hide class- and attribute-based states")
	}

	jsBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "profile.js"))
	if err != nil {
		t.Fatalf("read profile JavaScript: %v", err)
	}
	js := string(jsBytes)
	for _, stateChange := range []string{
		"customizeModal.hidden = false;",
		"customizeModal.hidden = true;",
		"document.body.appendChild(customizeModal);",
	} {
		if !strings.Contains(js, stateChange) {
			t.Fatalf("profile modal controller missing native hidden state change %q", stateChange)
		}
	}
}

func TestProfileArtSearchRequiresSuggestionAndOffersExactPrintings(t *testing.T) {
	body := renderTemplate(t, "profile_show", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Deck Brewer"},
		Data: profilePageData{
			Profile:   account.PublicProfile{ID: 42, DisplayName: "Deck Brewer"},
			Stats:     profileStatsData{CanEdit: true},
			ActiveTab: "decks",
		},
	})

	for _, needle := range []string{
		`data-profile-art-target="picture"`,
		`data-profile-art-target="background"`,
		`data-profile-art-search data-card-autocomplete-root`,
		`name="oracle_id" value="" data-card-autocomplete-oracle-id`,
		`data-card-autocomplete-submit="false"`,
		`role="listbox"`,
		`data-profile-art-random>Random Card Art</button>`,
		`Choose a card suggestion, then choose its printing.`,
		`data-profile-art-printings hidden`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("profile avatar picker missing %q: %s", needle, body)
		}
	}
	for _, forbidden := range []string{`>Use Art</button>`, `>Reset Art</button>`, `>Account Settings</a>`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("profile avatar picker retained removed control %q: %s", forbidden, body)
		}
	}

	withRendererRoot(t)
	jsBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "profile.js"))
	if err != nil {
		t.Fatalf("read profile JavaScript: %v", err)
	}
	js := string(jsBytes)
	for _, needle := range []string{
		`profileArtSearchForm.addEventListener("card-autocomplete-selected"`,
		`/cards/versions?oracle_id=`,
		`fetch("/profile/art"`,
		`renderProfileArtPrintings`,
	} {
		if !strings.Contains(js, needle) {
			t.Fatalf("profile avatar picker behavior missing %q", needle)
		}
	}
}

func TestProfileAchievementsLabelAllCompletedTombscriptWins(t *testing.T) {
	body := renderTemplate(t, "profile_show", TemplateData{Data: profilePageData{
		Profile:            account.PublicProfile{ID: 42, DisplayName: "Deck Builder"},
		ActiveTab:          "achievements",
		AwardTotal:         3,
		SpellifyPagination: profilePagination{Total: 3},
	}})
	if !strings.Contains(body, `id="tombscript-awards-title">Tombscript Wins</h3>`) {
		t.Fatalf("profile achievements still describe Tombscript wins as card rewards: %s", body)
	}
}

func TestProfileSectionsArePreloadedAndTabsStayInPage(t *testing.T) {
	body := renderTemplate(t, "profile_show", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Deck Brewer"},
		Data: profilePageData{
			Profile:   account.PublicProfile{ID: 42, DisplayName: "Deck Brewer"},
			Stats:     profileStatsData{CanEdit: true},
			ActiveTab: "decks",
		},
	})

	for _, needle := range []string{
		`data-profile-tab="decks"`,
		`data-profile-tab="favorites"`,
		`data-profile-tab="achievements"`,
		`data-profile-panel="decks"`,
		`data-profile-panel="favorites" aria-labelledby="profile-favorites-title" hidden`,
		`data-profile-panel="achievements" aria-labelledby="profile-achievements-title" hidden`,
		`No game wins yet.`,
		`Completed game wins will appear here.`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("preloaded profile tabs missing %q", needle)
		}
	}

	withRendererRoot(t)
	jsBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "profile.js"))
	if err != nil {
		t.Fatalf("read profile JavaScript: %v", err)
	}
	js := string(jsBytes)
	for _, needle := range []string{
		`event.preventDefault();`,
		`panel.hidden = panel.dataset.profilePanel !== tab;`,
		`window.history.pushState({ profileTab: tab }`,
		`window.addEventListener("popstate"`,
		`focus({ preventScroll: true })`,
	} {
		if !strings.Contains(js, needle) {
			t.Fatalf("in-page profile tab behavior missing %q", needle)
		}
	}
}
