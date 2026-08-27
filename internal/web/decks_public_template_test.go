package web

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicDeckBrowseTemplateUsesFlatTombFilterLayout(t *testing.T) {
	body := renderTemplate(t, "decks_public", TemplateData{
		Data: publicDeckListPageData{
			DeckName:          "Silver",
			CommanderName:     "Karn",
			Archetypes:        []string{"Combo", "Voltron"},
			ArchetypeSelected: map[string]bool{"Combo": true, "Voltron": true},
			ColorFilters:      []string{"C"},
			ColorSelected:     map[string]bool{"C": true},
			ColorMode:         "exact",
			Sort:              "updated",
			ClearPath:         "/decks/public?deck_name=Silver&sort=updated",
			ActiveFilters:     4,
			AppliedFilters: []publicDeckAppliedFilter{
				{Label: "Commander: Karn", RemovePath: "/decks/public?archetype=Combo&archetype=Voltron&deck_name=Silver&sort=updated"},
				{Label: "Archetype: Combo", RemovePath: "/decks/public?archetype=Voltron&commander=Karn&deck_name=Silver&sort=updated"},
				{Label: "Archetype: Voltron", RemovePath: "/decks/public?archetype=Combo&commander=Karn&deck_name=Silver&sort=updated"},
				{Label: "Colors: Colorless", RemovePath: "/decks/public?archetype=Combo&archetype=Voltron&commander=Karn&deck_name=Silver&sort=updated"},
			},
			Page:         2,
			HasPrevious:  true,
			HasNext:      true,
			PreviousPath: "/decks/public?archetype=Combo&archetype=Voltron&commander=Karn&deck_name=Silver&sort=updated",
			NextPath:     "/decks/public?archetype=Combo&archetype=Voltron&commander=Karn&deck_name=Silver&page=3&sort=updated",
			Items: []deckListItem{{
				OwnerID:           42,
				OwnerDisplayName:  "Tomb Keeper",
				Name:              "The Silver Golem",
				Description:       "A colorless artifact deck.",
				Tags:              "Combo, Voltron",
				Format:            "Commander",
				CommanderName:     "Karn, Silver Golem",
				CommanderImageURI: "https://example.test/karn.jpg",
				ColorPips:         manaPipsForColorIdentity("C"),
				ColorIdentityName: "Colorless",
				PublicSlug:        "the-silver-golem",
				PowerBracket:      "3 - Upgraded",
				PublishedLabel:    "Jul 27, 2026",
			}},
		},
	})

	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("guest public deck browser is missing the Tomb palette: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="decks-public"`) {
		t.Fatalf("public deck browser is missing its page scope: %s", bodyTag)
	}
	if strings.Count(body, "<main") != 1 {
		t.Fatalf("public deck browser rendered %d main landmarks, want the shared one only", strings.Count(body, "<main"))
	}

	pageStart := strings.Index(body, `<div class="mt-public-decks-page">`)
	pageEnd := strings.Index(body, `<script src="/assets/decks_public.js"`)
	if pageStart == -1 || pageEnd <= pageStart {
		t.Fatalf("could not isolate rendered public deck browser: %s", body)
	}
	page := body[pageStart:pageEnd]

	for _, needle := range []string{
		`href="/assets/decks_public.css"`,
		`<h1>Browse Public Decks</h1>`,
		`id="public-decks-name-search"`,
		`name="deck_name"`,
		`value="Silver"`,
		`placeholder="Search public decks by name"`,
		`enterkeyhint="search"`,
		`id="public-decks-sort"`,
		`name="sort"`,
		`value="updated" selected`,
		`data-public-decks-sort`,
		`class="mt-public-decks-filters"`,
		`4 active`,
		`id="public-decks-commander"`,
		`name="format"`,
		`name="power_bracket"`,
		`class="mt-public-decks-archetypes"`,
		`Matches any selected archetype.`,
		`name="archetype"`,
		`class="mt-public-decks-archetype-input"`,
		`type="hidden" name="archetype" value="Combo"`,
		`type="hidden" name="archetype" value="Voltron"`,
		`name="color_mode"`,
		`value="exact"`,
		`value="includes"`,
		`value="at_most"`,
		`Commander color identity`,
		`value="C"`,
		`aria-label="Colorless"`,
		`aria-label="Applied filters"`,
		`Commander: Karn`,
		`Archetype: Combo`,
		`Archetype: Voltron`,
		`Colors: Colorless`,
		`aria-label="Remove filter: Commander: Karn"`,
		`href="/decks/public?deck_name=Silver&amp;sort=updated"`,
		`>Apply filters</button>`,
		`loading="lazy"`,
		`<span>Commander:</span> Karn, Silver Golem`,
		`class="mt-deck-tile__identity"`,
		`class="mt-deck-tile__footer"`,
		`aria-label="Deck archetypes"`,
		`href="/decks/public?archetype=Combo"`,
		`aria-label="Browse public decks with the Combo archetype">#Combo</a>`,
		`href="/decks/public?archetype=Voltron"`,
		`class="mt-deck-tile__published">Published Jul 27, 2026`,
		`aria-label="Public deck pages"`,
		`href="/decks/public?archetype=Combo&amp;archetype=Voltron&amp;commander=Karn&amp;deck_name=Silver&amp;sort=updated"`,
		`href="/decks/public?archetype=Combo&amp;archetype=Voltron&amp;commander=Karn&amp;deck_name=Silver&amp;page=3&amp;sort=updated"`,
		`Page 2`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("public deck browser missing %q: %s", needle, body)
		}
	}
	if got := strings.Count(page, `class="mt-public-decks-color-input"`); got != 6 {
		t.Fatalf("public deck browser rendered %d color controls, want WUBRGC", got)
	}
	if got := strings.Count(page, `class="mt-public-decks-archetype-input"`); got != 12 {
		t.Fatalf("public deck browser rendered %d archetype controls, want all supported archetypes", got)
	}
	for _, archetype := range []string{"Combo", "Voltron"} {
		labelIndex := strings.Index(page, `aria-label="`+archetype+`"`)
		if labelIndex == -1 {
			t.Fatalf("public deck browser is missing %s archetype control: %s", archetype, page)
		}
		inputStart := strings.LastIndex(page[:labelIndex], "<input")
		inputEnd := strings.Index(page[labelIndex:], ">")
		if inputStart == -1 || inputEnd == -1 {
			t.Fatalf("could not isolate %s archetype input: %s", archetype, page)
		}
		inputTag := page[inputStart : labelIndex+inputEnd+1]
		if !strings.Contains(inputTag, " checked") {
			t.Fatalf("%s archetype control is not selected: %s", archetype, inputTag)
		}
	}
	detailsStart := strings.Index(page, `<details class="mt-public-decks-filters"`)
	detailsEnd := strings.Index(page[detailsStart:], ">")
	if detailsStart == -1 || detailsEnd == -1 {
		t.Fatalf("public deck browser is missing its filter disclosure: %s", page)
	}
	detailsTag := page[detailsStart : detailsStart+detailsEnd+1]
	if strings.Contains(detailsTag, " open") {
		t.Fatalf("public deck filters should be collapsed by default: %s", detailsTag)
	}
	searchIndex := strings.Index(page, `id="public-decks-name-search"`)
	filterIndex := strings.Index(page, `<details class="mt-public-decks-filters"`)
	appliedIndex := strings.Index(page, `class="mt-public-decks-applied"`)
	resultsIndex := strings.Index(page, `id="public-decks-results-title"`)
	sortIndex := strings.Index(page, `id="public-decks-sort"`)
	if searchIndex == -1 || filterIndex <= searchIndex {
		t.Fatalf("deck-name search is not before the filter drawer: %s", page)
	}
	if appliedIndex <= filterIndex || resultsIndex <= appliedIndex {
		t.Fatalf("applied filters are not between the filter drawer and results heading: %s", page)
	}
	if sortIndex <= resultsIndex {
		t.Fatalf("sort control is not in the public-decks results heading: %s", page)
	}
	cardHeadingIndex := strings.Index(page, `class="mt-deck-tile__heading"`)
	identityIndex := strings.Index(page, `class="mt-deck-tile__identity"`)
	commanderIndex := strings.Index(page, `class="mt-deck-tile__commander"`)
	cardFooterIndex := strings.Index(page, `class="mt-deck-tile__footer"`)
	publishedIndex := strings.Index(page, `class="mt-deck-tile__published"`)
	if cardHeadingIndex == -1 || identityIndex <= cardHeadingIndex || commanderIndex <= identityIndex {
		t.Fatalf("deck color identity is not anchored in the tile heading above commander metadata: %s", page)
	}
	if cardFooterIndex == -1 || publishedIndex <= cardFooterIndex {
		t.Fatalf("published date is not anchored in the deck tile footer: %s", page)
	}

	for _, forbidden := range []string{
		`<main`,
		`mt-panel`,
		`mt-action-card`,
		`text-slate-`,
		`text-sky-`,
		`publicTagChipClass`,
		`<select id="public-decks-archetype"`,
		`Showing `,
		`>Update</button>`,
	} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("public deck browser unexpectedly contains %q: %s", forbidden, page)
		}
	}
}

func TestPublicDeckColorNamesAreReadable(t *testing.T) {
	got := publicDeckColorNames([]string{"W", "U", "B", "R", "G", "C", "invalid"})
	want := []string{"White", "Blue", "Black", "Red", "Green", "Colorless"}
	if len(got) != len(want) {
		t.Fatalf("publicDeckColorNames() = %v, want %v", got, want)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("publicDeckColorNames()[%d] = %q, want %q", idx, got[idx], want[idx])
		}
	}
}

func TestPublicDeckBrowsePaginationNormalizesAndPreservesQuery(t *testing.T) {
	for raw, want := range map[string]int{
		"":        1,
		"0":       1,
		"-2":      1,
		"invalid": 1,
		"3":       3,
		"10001":   10000,
	} {
		if got := publicDeckBrowsePage(raw); got != want {
			t.Fatalf("publicDeckBrowsePage(%q) = %d, want %d", raw, got, want)
		}
	}

	path := publicDeckBrowsePath(url.Values{
		"color":      {"W", "U"},
		"color_mode": {"exact"},
		"commander":  {"Atraxa"},
		"deck_name":  {"Friends"},
		"archetype":  {"Combo", "Voltron"},
		"sort":       {"updated"},
	}, 3)
	for _, needle := range []string{
		"/decks/public?",
		"color=W",
		"color=U",
		"color_mode=exact",
		"commander=Atraxa",
		"deck_name=Friends",
		"archetype=Combo",
		"archetype=Voltron",
		"page=3",
		"sort=updated",
	} {
		if !strings.Contains(path, needle) {
			t.Fatalf("public deck pagination path %q is missing %q", path, needle)
		}
	}
}

func TestPublicDeckAppliedFiltersAreConciseAndIndividuallyRemovable(t *testing.T) {
	values := url.Values{
		"deck_name":     {"Friends"},
		"commander":     {"Atraxa"},
		"format":        {"Commander"},
		"power_bracket": {"3 - Upgraded"},
		"archetype":     {"Combo", "Voltron"},
		"color":         {"W", "U"},
		"color_mode":    {"exact"},
		"sort":          {"updated"},
		"page":          {"4"},
	}
	filters := publicDeckAppliedFilters(values, []string{"Combo", "Voltron"}, []string{"W", "U"}, "exact")
	wantLabels := []string{
		"Commander: Atraxa",
		"Format: Commander",
		"Power bracket: 3 - Upgraded",
		"Archetype: Combo",
		"Archetype: Voltron",
		"Colors: exactly White / Blue",
	}
	if len(filters) != len(wantLabels) {
		t.Fatalf("publicDeckAppliedFilters() returned %d filters, want %d", len(filters), len(wantLabels))
	}
	for idx, want := range wantLabels {
		if filters[idx].Label != want {
			t.Fatalf("publicDeckAppliedFilters()[%d].Label = %q, want %q", idx, filters[idx].Label, want)
		}
		if !strings.Contains(filters[idx].RemovePath, "deck_name=Friends") ||
			!strings.Contains(filters[idx].RemovePath, "sort=updated") ||
			strings.Contains(filters[idx].RemovePath, "page=") {
			t.Fatalf("filter removal path did not preserve search/sort and reset pagination: %q", filters[idx].RemovePath)
		}
	}
	if strings.Contains(filters[0].RemovePath, "commander=") {
		t.Fatalf("commander removal path still contains commander: %q", filters[0].RemovePath)
	}
	removeComboURL, err := url.Parse(filters[3].RemovePath)
	if err != nil {
		t.Fatalf("parse Combo removal path: %v", err)
	}
	if got := removeComboURL.Query()["archetype"]; len(got) != 1 || got[0] != "Voltron" {
		t.Fatalf("Combo removal path archetypes = %v, want [Voltron]", got)
	}
	removeVoltronURL, err := url.Parse(filters[4].RemovePath)
	if err != nil {
		t.Fatalf("parse Voltron removal path: %v", err)
	}
	if got := removeVoltronURL.Query()["archetype"]; len(got) != 1 || got[0] != "Combo" {
		t.Fatalf("Voltron removal path archetypes = %v, want [Combo]", got)
	}
	if strings.Contains(filters[5].RemovePath, "color=") || strings.Contains(filters[5].RemovePath, "color_mode=") {
		t.Fatalf("color removal path still contains color state: %q", filters[5].RemovePath)
	}
}

func TestPublicDeckBrowseStylesUseCentralThemeTokens(t *testing.T) {
	withRendererRoot(t)

	body, err := os.ReadFile(filepath.Join("internal", "web", "assets", "decks_public.css"))
	if err != nil {
		t.Fatalf("read public deck browse stylesheet: %v", err)
	}
	sharedTileBody, err := os.ReadFile(filepath.Join("internal", "web", "assets", "deck_tile.css"))
	if err != nil {
		t.Fatalf("read shared deck tile stylesheet: %v", err)
	}
	css := string(body) + "\n" + string(sharedTileBody)

	for _, needle := range []string{
		`.mt-public-decks-page`,
		`.mt-public-decks-browser`,
		`width: 100%;`,
		`.mt-public-decks-applied ul`,
		`flex-wrap: wrap;`,
		`.mt-public-decks-applied li a`,
		`border: 1px solid var(--mt-border-subtle);`,
		`.mt-public-decks-results-sort`,
		`.mt-public-decks-filter-grid`,
		`grid-template-columns: repeat(3, minmax(0, 1fr));`,
		`.mt-public-decks-archetype-options`,
		`.mt-public-decks-archetype-input:checked + span`,
		`.mt-public-decks-archetype-input:focus-visible + span`,
		`.mt-deck-tile__heading`,
		`grid-template-columns: minmax(0, 1fr) auto;`,
		`.mt-deck-tile__footer`,
		`margin-top: auto;`,
		`.mt-deck-tile__tags a`,
		`.mt-deck-tile__tags a:focus-visible`,
		`.mt-deck-tile__published`,
		`text-align: right;`,
		`color: var(--mt-text);`,
		`border-bottom: 1px solid var(--mt-border-subtle);`,
		`filter: saturate(1) drop-shadow`,
		`body[data-page="decks-public"] *`,
		`animation: none;`,
		`transition: none;`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("public deck browse stylesheet missing %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "#"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("public deck browse stylesheet contains hardcoded color syntax %q", forbidden)
		}
	}

	colorSectionRule := renderedCSSRule(t, css, ".mt-public-decks-color-section")
	if strings.Contains(colorSectionRule, "border-top") || strings.Contains(colorSectionRule, "padding-top") {
		t.Fatalf("color filter section still renders a horizontal divider: %s", colorSectionRule)
	}
	filterActionsRule := renderedCSSRule(t, css, ".mt-public-decks-filter-actions")
	if strings.Contains(filterActionsRule, "border-top") || strings.Contains(filterActionsRule, "padding-top") {
		t.Fatalf("filter actions still render a divider above Clear filters: %s", filterActionsRule)
	}
	resultsRule := renderedCSSRule(t, css, ".mt-public-decks-results")
	if !strings.Contains(resultsRule, "margin-top: 0.75rem;") {
		t.Fatalf("public deck results still leave an oversized gap below filters: %s", resultsRule)
	}
	if strings.Contains(css, ".mt-public-decks-filters {") {
		t.Fatalf("filter disclosure wrapper still carries a box or horizontal divider")
	}
	if strings.Contains(css, ".mt-public-decks-applied li + li::before") {
		t.Fatalf("applied filters still use ambiguous separator dots instead of individual boxes")
	}

	script, err := os.ReadFile(filepath.Join("internal", "web", "assets", "decks_public.js"))
	if err != nil {
		t.Fatalf("read public deck browse script: %v", err)
	}
	for _, needle := range []string{
		`[data-public-decks-sort]`,
		`addEventListener("change"`,
		`requestSubmit()`,
		`[data-public-decks-color-options]`,
	} {
		if !strings.Contains(string(script), needle) {
			t.Fatalf("public deck browse script missing %q", needle)
		}
	}
}
