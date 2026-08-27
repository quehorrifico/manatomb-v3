package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCardsSearchTemplateUsesTombFlatLayout(t *testing.T) {
	body := renderTemplate(t, "cards_search", TemplateData{
		Data: cardSearchPageData{
			SearchActionPath:    "/cards/search",
			ClearPath:           "/cards/search",
			Sort:                "relevance",
			TextMode:            "contains",
			ColorMode:           "includes",
			ColorsSelected:      map[string]bool{},
			RaritiesSelected:    map[string]bool{},
			LayoutOptions:       advancedCardLayoutOptions,
			StatOptions:         advancedCardStatOptions,
			StatOperatorOptions: advancedCardStatOperatorOptions,
			StatFilters: []cardSearchStatFilter{{
				Stat:     "mana_value",
				Operator: "eq",
			}},
			PriceOperatorOptions:  advancedCardStatOperatorOptions,
			PriceFilters:          []cardSearchPriceFilter{{Operator: "eq"}},
			SelectedStat:          "mana_value",
			SelectedStatOperator:  "eq",
			SelectedPriceOperator: "eq",
		},
	})

	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("guest advanced search is missing the Tomb palette: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="cards-search"`) {
		t.Fatalf("advanced search body is missing its page scope: %s", bodyTag)
	}

	themeIndex := strings.Index(body, `href="/assets/theme.css"`)
	pageCSSIndex := strings.Index(body, `href="/assets/cards_search.css"`)
	if themeIndex < 0 || pageCSSIndex <= themeIndex {
		t.Fatalf("advanced search stylesheet must load after the shared theme stylesheet")
	}
	if !strings.Contains(body, `<script src="/assets/cards_search.js" defer></script>`) {
		t.Fatal("advanced search page is missing the reusable filter behavior asset")
	}

	scriptSource, err := os.ReadFile(filepath.Join("internal", "web", "assets", "cards_search.js"))
	if err != nil {
		t.Fatalf("read advanced search behavior: %v", err)
	}
	searchSurface := body + "\n" + string(scriptSource)

	for _, needle := range []string{
		`class="mt-advanced-search-page"`,
		`class="mt-advanced-search-form" data-card-search-form`,
		`name="search_mode" value="advanced"`,
		`data-card-search-type-options`,
		`data-card-search-type-filters`,
		`data-card-search-set-options`,
		`class="mt-advanced-search-section mt-advanced-search-section--primary"`,
		`aria-label="Color filters"`,
		`aria-label="Printing filters"`,
		`aria-label="Stat and price filters"`,
		`for="advanced-card-name"`,
		`for="advanced-type-line"`,
		`class="mt-advanced-search-type-controls"`,
		`for="advanced-type-partial" class="mt-advanced-search-type-partial"`,
		`Allow partial type matches?`,
		`class="mt-advanced-search-mana-builder"`,
		`data-mana-symbol="{W}"`,
		`data-mana-symbol="{U}"`,
		`data-mana-symbol="{B}"`,
		`data-mana-symbol="{R}"`,
		`data-mana-symbol="{G}"`,
		`data-mana-symbol="{C}"`,
		`data-mana-symbol="{1}"`,
		`src="https://svgs.scryfall.io/card-symbols/1.svg"`,
		`readonly`,
		`data-mana-cost-undo`,
		`data-mana-cost-clear`,
		`name="layout"`,
		`data-card-autocomplete-submit="false"`,
		`role="combobox"`,
		`aria-live="polite"`,
		`class="mt-advanced-search-color-control"`,
		`class="mt-advanced-search-color-pips"`,
		`id="advanced-colors-label" class="mt-field-label">Colors</span>`,
		`class="mt-advanced-search-rarity-options"`,
		`value="any"`,
		`data-rarity-any`,
		`type="checkbox" name="rarity" value="common"`,
		`type="checkbox" name="rarity" value="uncommon"`,
		`type="checkbox" name="rarity" value="rare"`,
		`type="checkbox" name="rarity" value="mythic"`,
		`class="mt-advanced-search-stat-rows" data-stat-rows`,
		`data-stat-row-template`,
		`data-remove-stat-filter`,
		`class="mt-advanced-search-price-rows" data-price-rows`,
		`data-price-row-template`,
		`data-remove-price-filter`,
		`aria-label="Remove price filter">×</button>`,
		`data-search-range-error`,
		`hidden>Impossible range</p>`,
		`data-search-submit`,
		`data-search-summary-remove`,
		`chip.type = "button";`,
		`"aria-label", "Remove filter: " + item.label`,
		`items.push({ label: label, remove: remove });`,
		`addSummaryItem("Colors ("`,
		`addSummaryItem("Rarity: "`,
		`removeStatRow(row);`,
		`removePriceRow(row);`,
		`aria-describedby="advanced-search-range-error"`,
		`class="mt-advanced-search-metric-controls mt-advanced-search-metric-controls--stat"`,
		`class="mt-advanced-search-metric-controls mt-advanced-search-metric-controls--price"`,
		`class="mt-advanced-search-summary"`,
	} {
		if !strings.Contains(searchSurface, needle) {
			t.Fatalf("advanced search page missing %q", needle)
		}
	}

	if got := strings.Count(body, "<main"); got != 1 {
		t.Fatalf("advanced search page rendered %d main landmarks, want one", got)
	}
	for _, forbidden := range []string{
		`name="name_exact"`,
		`Exact card name only`,
		`Use mana notation such as`,
		`Press Enter or choose a suggestion`,
		`Rules text match`,
		`name="text_mode"`,
		`Color identity`,
		`Select colors, then choose how strictly they should match.`,
		`Printing details`,
		`Narrow results by rarity, set, artist, legality, or token status.`,
		`Stats &amp; price`,
		`Add optional comparisons for mana value, power, toughness, loyalty, or price.`,
		`Stat filter`,
		`Price filter`,
		`View Results opens the matching card list.`,
		`View Results will search with the filters below.`,
		`data-search-summary-line`,
		`<fieldset`,
		`<legend`,
		`<details`,
		`<summary`,
		`data-metrics-panel`,
		`type="radio"`,
		`Commander candidates only`,
		`Commander legal only`,
		`Include tokens`,
		`data-clear-stat-filter`,
		`data-clear-price-filter`,
		`advanced-price-comparison`,
		`advanced-price-value`,
		`data-stat-example`,
		`data-price-example`,
		`Example: Mana Value`,
		`Example: Price`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("advanced search page still contains removed interface %q", forbidden)
		}
	}
}

func TestCardsSearchPresentationUsesCentralThemeTokens(t *testing.T) {
	pageTemplateSource, err := os.ReadFile(filepath.Join("templates", "cards_search.html.tmpl"))
	if err != nil {
		t.Fatalf("read advanced search template: %v", err)
	}
	formTemplateSource, err := os.ReadFile(filepath.Join("templates", "cards_search_form.html.tmpl"))
	if err != nil {
		t.Fatalf("read shared advanced search form template: %v", err)
	}
	templateSource := string(pageTemplateSource) + "\n" + string(formTemplateSource)
	for _, forbidden := range []string{"text-slate-", "border-slate-", "bg-slate-", "text-sky-", "text-rose-", "mt-filter-group"} {
		if strings.Contains(templateSource, forbidden) {
			t.Fatalf("advanced search template still contains legacy presentation %q", forbidden)
		}
	}

	cssSource, err := os.ReadFile(filepath.Join("assets", "cards_search.css"))
	if err != nil {
		t.Fatalf("read advanced search stylesheet: %v", err)
	}
	css := string(cssSource)
	for _, needle := range []string{
		`var(--mt-surface)`,
		`var(--mt-text)`,
		`var(--mt-text-soft)`,
		`var(--mt-accent-soft)`,
		`var(--mt-focus)`,
		`.mt-advanced-search-form *`,
		`grid-template-columns: clamp(7rem, 21vw, 12.5rem) minmax(0, 1fr);`,
		`.mt-advanced-search-mana-pips`,
		`.mt-advanced-search-color-pips`,
		`.mt-advanced-search-rarity-options`,
		`.mt-advanced-search-stat-rows`,
		`.mt-advanced-search-price-rows`,
		`[data-stat-row]:not(:first-child) > .mt-field-label`,
		`[data-price-row]:not(:first-child) > .mt-field-label`,
		`.mt-advanced-search-metric-controls--stat`,
		`.mt-advanced-search-form .mt-control:focus-visible`,
		`.mt-advanced-search-remove`,
		`.mt-advanced-search-range-error`,
		`.mt-advanced-search-summary-chip:focus-visible`,
		`.mt-advanced-search-summary-chip__label`,
		`.mt-advanced-search-summary-chip__remove`,
		`.mt-advanced-search-submit:disabled`,
		`var(--mt-negative-soft)`,
		`animation: none;`,
		`transition: none;`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("advanced search stylesheet missing shared theme treatment %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "#", "@keyframes"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("advanced search stylesheet contains hardcoded or animated syntax %q", forbidden)
		}
	}
}

func TestCardsSearchBehaviorIsReusableAndTemplateFree(t *testing.T) {
	scriptSource, err := os.ReadFile(filepath.Join("assets", "cards_search.js"))
	if err != nil {
		t.Fatalf("read advanced search behavior: %v", err)
	}
	script := string(scriptSource)

	for _, needle := range []string{
		`function setupCardSearchForm(form)`,
		`document.querySelectorAll("[data-card-search-form]").forEach(setupCardSearchForm)`,
		`data-card-search-bound`,
		`readFormJSON(form, "[data-card-search-type-options]")`,
		`readFormJSON(form, "[data-card-search-type-filters]")`,
		`readFormJSON(form, "[data-card-search-set-options]")`,
		`ensureTrailingStatRow();`,
		`ensureTrailingPriceRow();`,
		`searchHasImpossibleRange()`,
		`renderSummaryChips(items)`,
		`syncRaritySelection`,
		`addManaSymbol`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("reusable advanced search behavior missing %q", needle)
		}
	}
	if strings.Contains(script, "{{") || strings.Contains(script, "}}") {
		t.Fatal("reusable advanced search behavior still contains template delimiters")
	}
}
