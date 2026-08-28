package web

import (
	"os"
	"strings"
	"testing"

	"manatomb/app/internal/account"
)

func TestDeckLibrariesRenderTheSharedDeckTile(t *testing.T) {
	item := deckListItem{
		ID:                  7,
		Name:                "Shared Tile Test",
		Description:         "One component on both pages.",
		Tags:                "Combo, Control",
		Format:              "Commander",
		CommanderName:       "Muldrotha, the Gravetide",
		CommanderArtCropURI: "https://example.test/muldrotha-art.jpg",
		ColorPips:           manaPipsForColorIdentity("UBG"),
		ColorIdentityName:   "Sultai",
		PublicSlug:          "shared-tile-test",
		PowerBracket:        "3 - Upgraded",
		PublishedLabel:      "Aug 26, 2026",
		ProfileTile:         true,
	}

	profile := renderTemplate(t, "profile_show", TemplateData{Data: profilePageData{
		Profile:   account.PublicProfile{ID: 42, DisplayName: "Deck Builder"},
		Items:     []deckListItem{item},
		ActiveTab: "decks",
	}})
	browse := renderTemplate(t, "decks_public", TemplateData{Data: publicDeckListPageData{
		Items: []deckListItem{item},
		Sort:  "recent",
	}})
	mine := renderTemplate(t, "decks_list", TemplateData{Data: []deckListItem{{
		ID:                  item.ID,
		DeckPath:            "/decks/7",
		Name:                item.Name,
		Description:         item.Description,
		Tags:                item.Tags,
		Format:              item.Format,
		CommanderName:       item.CommanderName,
		CommanderArtCropURI: item.CommanderArtCropURI,
		ColorPips:           item.ColorPips,
		ColorIdentityName:   item.ColorIdentityName,
		PowerBracket:        item.PowerBracket,
	}}})

	profileTile := renderedDeckTile(t, profile)
	browseTile := renderedDeckTile(t, browse)
	if profileTile != browseTile {
		t.Fatalf("profile and browse deck tiles drifted apart:\nprofile: %s\nbrowse: %s", profileTile, browseTile)
	}
	myTile := renderedDeckTile(t, mine)
	for _, needle := range []string{
		`href="/decks/7"`,
		`<span>Commander:</span> Muldrotha, the Gravetide`,
		`<span>Bracket:</span> 3 - Upgraded`,
	} {
		if !strings.Contains(myTile, needle) {
			t.Fatalf("My Decks shared tile missing %q: %s", needle, myTile)
		}
	}
	if strings.Contains(myTile, `mt-deck-tile__meta`) {
		t.Fatalf("shared deck tile retained the old format pill: %s", myTile)
	}

	for _, page := range []string{profile, browse, mine} {
		if !strings.Contains(page, `href="/assets/deck_tile.css"`) {
			t.Fatal("page using the shared deck tile is missing its shared stylesheet")
		}
	}
}

func TestDeckTileMarkupAndStylesHaveOneOwner(t *testing.T) {
	profileTemplate, err := os.ReadFile("templates/profile_show.html.tmpl")
	if err != nil {
		t.Fatalf("read profile template: %v", err)
	}
	browseTemplate, err := os.ReadFile("templates/decks_public.html.tmpl")
	if err != nil {
		t.Fatalf("read public decks template: %v", err)
	}
	sharedTemplate, err := os.ReadFile("templates/deck_tile.html.tmpl")
	if err != nil {
		t.Fatalf("read shared deck tile template: %v", err)
	}

	myDecksTemplate, err := os.ReadFile("templates/decks_list.html.tmpl")
	if err != nil {
		t.Fatalf("read My Decks template: %v", err)
	}
	for name, source := range map[string]string{
		"profile":  string(profileTemplate),
		"browse":   string(browseTemplate),
		"my decks": string(myDecksTemplate),
	} {
		if !strings.Contains(source, `{{ template "deck_tile" . }}`) {
			t.Fatalf("%s template does not call the shared deck tile", name)
		}
		if strings.Contains(source, `<article class="mt-deck-tile"`) {
			t.Fatalf("%s template duplicates the shared deck tile markup", name)
		}
	}
	if !strings.Contains(string(sharedTemplate), `{{ define "deck_tile" }}`) ||
		!strings.Contains(string(sharedTemplate), `<article class="mt-deck-tile"`) {
		t.Fatal("shared deck tile template is missing its component definition")
	}

	for _, path := range []string{"assets/profile.css", "assets/decks_public.css"} {
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if strings.Contains(string(body), ".mt-deck-tile") {
			t.Fatalf("%s duplicates shared deck tile styles", path)
		}
	}
}

func renderedDeckTile(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, `<article class="mt-deck-tile"`)
	if start < 0 {
		t.Fatal("rendered page is missing shared deck tile")
	}
	end := strings.Index(page[start:], `</article>`)
	if end < 0 {
		t.Fatal("rendered shared deck tile is malformed")
	}
	return page[start : start+end+len(`</article>`)]
}
