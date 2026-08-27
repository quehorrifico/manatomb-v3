package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cardListTestFilterForm() cardSearchPageData {
	return cardSearchPageData{
		LayoutOptions:         advancedCardLayoutOptions,
		StatOptions:           advancedCardStatOptions,
		StatOperatorOptions:   advancedCardStatOperatorOptions,
		StatFilters:           []cardSearchStatFilter{{Stat: "mana_value", Operator: "eq"}},
		PriceOperatorOptions:  advancedCardStatOperatorOptions,
		PriceFilters:          []cardSearchPriceFilter{{Operator: "eq"}},
		ColorsSelected:        map[string]bool{},
		RaritiesSelected:      map[string]bool{},
		ColorMode:             "includes",
		Sort:                  "relevance",
		SortDirection:         "desc",
		SearchActionPath:      "/cards",
		ClearPath:             "/cards?search_mode=advanced&sort=relevance",
		ResultsMode:           cardResultsModeAdvanced,
		InlineResults:         true,
		SubmitLabel:           "Apply filters",
		SelectedStat:          "mana_value",
		SelectedStatOperator:  "eq",
		SelectedPriceOperator: "eq",
	}
}

func TestCardsListTemplateUsesSharedTombResultsSurface(t *testing.T) {
	page := cardListPageData{
		NameQuery:             "lightning",
		ResultsMode:           cardResultsModeStandard,
		FilterForm:            cardListTestFilterForm(),
		SortOptions:           advancedCardSortOptions,
		DirectionOptions:      advancedCardSortDirectionOptions,
		SelectedSort:          "relevance",
		SelectedSortDirection: "desc",
		TotalResults:          76,
		Page:                  1,
		TotalPages:            2,
		ResultStart:           1,
		ResultEnd:             48,
		NextPath:              "/cards?page=2&q=lightning&sort=relevance",
		Results: []searchResult{{
			OracleID:   "123e4567-e89b-12d3-a456-426614174000",
			ScryfallID: "223e4567-e89b-12d3-a456-426614174000",
			DetailPath: "/cards/view/123e4567-e89b-12d3-a456-426614174000?printing=223e4567-e89b-12d3-a456-426614174000",
			Name:       "Lightning Bolt",
			ImageURI:   "https://example.com/bolt.jpg",
			SetName:    "Magic 2010",
			SetCode:    "M10",
			PriceUSD:   "$1.25",
		}},
	}

	body := renderTemplate(t, "cards_list", TemplateData{Data: page})
	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")

	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("guest results page is missing the Tomb palette: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="cards-list"`) {
		t.Fatalf("results body is missing its page scope: %s", bodyTag)
	}
	if got := strings.Count(body, "<main"); got != 1 {
		t.Fatalf("results page rendered %d main elements, want exactly one", got)
	}
	for _, needle := range []string{
		`Results for “lightning”`,
		`Showing 1–48 of 76 cards`,
		`class="mt-card-results-grid"`,
		`class="mt-card-result__name"`,
		`>Lightning Bolt</span>`,
		`loading="lazy"`,
		`href="/cards/view/123e4567-e89b-12d3-a456-426614174000?printing=223e4567-e89b-12d3-a456-426614174000"`,
		`Refine with filters`,
		`Page 1 of 2`,
		`href="/cards?page=2&amp;q=lightning&amp;sort=relevance"`,
		`href="/assets/cards_list.css"`,
		`src="/assets/cards_list.js"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("results page missing %q", needle)
		}
	}
	for _, forbidden := range []string{
		`Advanced Search Results`,
		`>Home</a>`,
		`>Decks</a>`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("results page still contains removed presentation %q", forbidden)
		}
	}
}

func TestCardsListAdvancedModeShowsRemovableFiltersAndInlineForm(t *testing.T) {
	page := cardListPageData{
		NameQuery:             "elf",
		ResultsMode:           cardResultsModeAdvanced,
		AppliedFilters:        []cardSearchFilterChip{{Label: "Name: elf", RemovePath: "/cards?search_mode=advanced&sort=relevance"}},
		ClearPath:             "/cards?search_mode=advanced&sort=relevance",
		FilterForm:            cardListTestFilterForm(),
		SortOptions:           advancedCardSortOptions,
		DirectionOptions:      advancedCardSortDirectionOptions,
		SelectedSort:          "relevance",
		SelectedSortDirection: "desc",
		TotalResults:          0,
		FiltersOpen:           true,
	}

	body := renderTemplate(t, "cards_list", TemplateData{Data: page})
	for _, needle := range []string{
		`Cards matching “elf”`,
		`aria-label="Applied filters"`,
		`Remove filter: Name: elf`,
		`<details class="mt-card-results-refine" open>`,
		`<form method="GET" action="/cards" class="mt-advanced-search-form" data-card-search-form>`,
		`name="search_mode" value="advanced"`,
		`Apply filters`,
		`No cards matched`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("advanced results page missing %q", needle)
		}
	}

	formStart := strings.Index(body, `<form method="GET" action="/cards" class="mt-advanced-search-form"`)
	if formStart < 0 {
		t.Fatal("inline filter form was not rendered")
	}
	formEnd := strings.Index(body[formStart:], "</form>")
	if formEnd < 0 {
		t.Fatal("inline filter form was not closed")
	}
	inlineForm := body[formStart : formStart+formEnd]
	if strings.Contains(inlineForm, `name="view"`) {
		t.Fatal("inline results form should submit directly without the standalone view marker")
	}
}

func TestCardsListAssetsUseOnlyCentralThemeRoles(t *testing.T) {
	templateSource, err := os.ReadFile(filepath.Join("templates", "cards_list.html.tmpl"))
	if err != nil {
		t.Fatalf("read results template: %v", err)
	}
	for _, forbidden := range []string{"text-slate-", "border-slate-", "bg-slate-", "text-sky-", "mt-subpanel", "<main"} {
		if strings.Contains(string(templateSource), forbidden) {
			t.Fatalf("results template contains legacy or nested presentation %q", forbidden)
		}
	}

	cssSource, err := os.ReadFile(filepath.Join("assets", "cards_list.css"))
	if err != nil {
		t.Fatalf("read results stylesheet: %v", err)
	}
	css := string(cssSource)
	for _, needle := range []string{
		`var(--mt-surface)`,
		`var(--mt-text)`,
		`var(--mt-text-muted)`,
		`var(--mt-accent-soft)`,
		`var(--mt-focus)`,
		`animation: none`,
		`transition: none`,
		`-webkit-line-clamp: 2`,
		`line-clamp: 2`,
		`white-space: normal`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("results stylesheet missing shared theme treatment %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "#", "@keyframes"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("results stylesheet contains hardcoded or animated syntax %q", forbidden)
		}
	}
}
