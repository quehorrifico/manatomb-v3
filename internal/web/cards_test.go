package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"manatomb/app/internal/cards"
)

func TestCardDetailPath(t *testing.T) {
	t.Parallel()

	got := cardDetailPath("123e4567-e89b-12d3-a456-426614174000")
	want := "/cards/view/123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("cardDetailPath() = %q, want %q", got, want)
	}
}

func TestSingleCardResultPath(t *testing.T) {
	t.Parallel()

	got := singleCardResultPath([]cards.Card{{
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Mystic Card",
	}})
	want := "/cards/view/123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("singleCardResultPath() = %q, want %q", got, want)
	}
}

func TestSingleCardResultPathMultipleResults(t *testing.T) {
	t.Parallel()

	got := singleCardResultPath([]cards.Card{
		{OracleID: "123e4567-e89b-12d3-a456-426614174000"},
		{OracleID: "223e4567-e89b-12d3-a456-426614174000"},
	})
	if got != "" {
		t.Fatalf("singleCardResultPath() = %q, want empty string", got)
	}
}

func TestParseCardOracleIDFromPath(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest("GET", "/cards/view/123e4567-e89b-12d3-a456-426614174000", nil)
	got := parseCardOracleIDFromPath(r)
	want := "123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("parseCardOracleIDFromPath() = %q, want %q", got, want)
	}
}

func TestFormatCardColorNamesColorless(t *testing.T) {
	t.Parallel()

	if got := formatCardColorNames(nil); got != "Colorless" {
		t.Fatalf("formatCardColorNames(nil) = %q, want %q", got, "Colorless")
	}
}

func TestBuildCardDetailPageDataUsesFallbackPrinting(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Mystic Card",
		TypeLine:   "Instant",
		OracleText: "Draw a card.",
		CMC:        1,
	}

	got := buildCardDetailPageData(card, nil)
	if got.Card.Name != "Mystic Card" {
		t.Fatalf("buildCardDetailPageData().Card.Name = %q, want %q", got.Card.Name, "Mystic Card")
	}
	if len(got.Printings) != 1 {
		t.Fatalf("buildCardDetailPageData() printings = %d, want 1", len(got.Printings))
	}
	if got.Printings[0].SetName != "Unknown set" {
		t.Fatalf("buildCardDetailPageData() fallback set = %q, want %q", got.Printings[0].SetName, "Unknown set")
	}
}

func TestBuildCardDetailPageDataBuildsRichPrintingPayload(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Sample Card",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}.",
		FlavorText: "A quiet hum filled the vault.",
		CMC:        2,
		Faces: []cards.CardFace{{
			Name:       "Front Face",
			ManaCost:   "{2}",
			TypeLine:   "Artifact",
			OracleText: "{T}: Add {C}.",
			FlavorText: "Its song never ends.",
			ImageURI:   "https://example.com/front.jpg",
			Artist:     "Face Artist",
		}},
	}

	printings := []cards.Card{{
		Name:            "Sample Card",
		ManaCost:        "{2}",
		TypeLine:        "Artifact",
		OracleText:      "{T}: Add {C}.",
		FlavorText:      "Printed in the first vault run.",
		SetName:         "Foundations",
		SetCode:         "fdn",
		CollectorNumber: "321",
		ReleasedAt:      "2026-01-15",
		PriceUSD:        "4.25",
		Artist:          "Print Artist",
		Lang:            "en",
	}}

	got := buildCardDetailPageData(card, printings)
	if len(got.Printings) != 1 {
		t.Fatalf("buildCardDetailPageData() printings = %d, want 1", len(got.Printings))
	}

	printing := got.Printings[0]
	if printing.PriceUSD != "$4.25" {
		t.Fatalf("buildCardDetailPageData().Printings[0].PriceUSD = %q, want %q", printing.PriceUSD, "$4.25")
	}
	if printing.PriceSort != 4.25 {
		t.Fatalf("buildCardDetailPageData().Printings[0].PriceSort = %v, want %v", printing.PriceSort, 4.25)
	}
	if printing.SetCode != "FDN" {
		t.Fatalf("buildCardDetailPageData().Printings[0].SetCode = %q, want %q", printing.SetCode, "FDN")
	}
	if len(printing.Faces) != 1 {
		t.Fatalf("buildCardDetailPageData().Printings[0].Faces = %d, want 1", len(printing.Faces))
	}
	if printing.Faces[0].Name != "Front Face" {
		t.Fatalf("buildCardDetailPageData().Printings[0].Faces[0].Name = %q, want %q", printing.Faces[0].Name, "Front Face")
	}
	if got.Card.FlavorText != "A quiet hum filled the vault." {
		t.Fatalf("buildCardDetailPageData().Card.FlavorText = %q, want %q", got.Card.FlavorText, "A quiet hum filled the vault.")
	}
	if printing.FlavorText != "Printed in the first vault run." {
		t.Fatalf("buildCardDetailPageData().Printings[0].FlavorText = %q, want %q", printing.FlavorText, "Printed in the first vault run.")
	}
	if printing.Faces[0].FlavorText != "Its song never ends." {
		t.Fatalf("buildCardDetailPageData().Printings[0].Faces[0].FlavorText = %q, want %q", printing.Faces[0].FlavorText, "Its song never ends.")
	}
}

func TestBuildCardDetailPageDataFlavorFallbackStaysServerSide(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Fallback Card",
		TypeLine:   "Creature",
		OracleText: "Flying",
		CMC:        3,
	}

	printings := []cards.Card{
		{
			Name:       "Fallback Card",
			TypeLine:   "Creature",
			OracleText: "Flying",
			SetName:    "No Flavor Set",
			SetCode:    "nfs",
			ReleasedAt: "2026-01-01",
		},
		{
			Name:       "Fallback Card",
			TypeLine:   "Creature",
			OracleText: "Flying",
			SetName:    "Face Flavor Set",
			SetCode:    "ffs",
			ReleasedAt: "2026-02-01",
			Faces: []cards.CardFace{{
				Name:       "Front Face",
				TypeLine:   "Creature",
				OracleText: "Flying",
				FlavorText: "The sky remembers its name.",
			}},
		},
	}

	got := buildCardDetailPageData(card, printings)
	if got.Card.FlavorText != "The sky remembers its name." {
		t.Fatalf("buildCardDetailPageData().Card.FlavorText = %q, want %q", got.Card.FlavorText, "The sky remembers its name.")
	}
	if got.Printings[0].FlavorText != "" {
		t.Fatalf("buildCardDetailPageData().Printings[0].FlavorText = %q, want empty string", got.Printings[0].FlavorText)
	}
	if got.Printings[1].FlavorText != "The sky remembers its name." {
		t.Fatalf("buildCardDetailPageData().Printings[1].FlavorText = %q, want %q", got.Printings[1].FlavorText, "The sky remembers its name.")
	}
}

func TestAuditCardFlavorFlow(t *testing.T) {
	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Audit Card",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}.",
		CMC:        2,
	}
	printings := []cards.Card{{
		OracleID:        card.OracleID,
		Name:            "Audit Card",
		ManaCost:        "{2}",
		TypeLine:        "Artifact",
		OracleText:      "{T}: Add {C}.",
		FlavorText:      "Echoes still linger in the chamber.",
		SetName:         "Foundations",
		SetCode:         "fdn",
		CollectorNumber: "321",
		ReleasedAt:      "2026-01-15",
		PriceUSD:        "4.25",
		Artist:          "Print Artist",
		Lang:            "en",
	}}

	page := buildCardDetailPageData(card, printings)
	fmt.Printf("AUDIT canonical_card.FlavorText=%q\n", card.FlavorText)
	fmt.Printf("AUDIT first_printing.FlavorText=%q\n", printings[0].FlavorText)
	fmt.Printf("AUDIT page.Card.FlavorText=%q\n", page.Card.FlavorText)
	fmt.Printf("AUDIT page.Printings[0].FlavorText=%q\n", page.Printings[0].FlavorText)

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "card_show", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	fmt.Printf("AUDIT rendered_html_contains_flavor=%t\n", strings.Contains(body, "Echoes still linger in the chamber."))
	fmt.Printf("AUDIT rendered_html_has_hidden_flavor_section=%t\n", strings.Contains(body, "data-card-flavor-group") && strings.Contains(body, "hidden"))

	if page.Card.FlavorText == "" {
		t.Fatal("page card flavor text should not be empty after printing fallback")
	}
	if !strings.Contains(body, "Echoes still linger in the chamber.") {
		t.Fatal("rendered card_show HTML did not contain flavor text")
	}
}

func TestCardShowTemplateDoesNotShipFlavorFetchScript(t *testing.T) {
	page := buildCardDetailPageData(cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "No Fetch Card",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}.",
		CMC:        2,
	}, []cards.Card{{
		OracleID:        "123e4567-e89b-12d3-a456-426614174000",
		Name:            "No Fetch Card",
		TypeLine:        "Artifact",
		OracleText:      "{T}: Add {C}.",
		SetName:         "Foundations",
		SetCode:         "FDN",
		CollectorNumber: "321",
		ReleasedAt:      "2026-01-15",
	}})

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "card_show", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	if strings.Contains(body, "https://api.scryfall.com/cards/") {
		t.Fatal("rendered card_show HTML still contains a direct browser fetch to the Scryfall card API")
	}
}

func TestParseCardSearchRequestAdvancedFiltersMarkSearched(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"type_value": []string{"Instant"},
		"type_mode":  []string{"is"},
		"color":      []string{"U"},
		"set":        []string{"bro"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if !req.HasSearched {
		t.Fatalf("parseCardSearchRequest().HasSearched = false, want true")
	}
	if req.SetQuery != "bro" {
		t.Fatalf("parseCardSearchRequest().SetQuery = %q, want %q", req.SetQuery, "bro")
	}
	if len(req.TypeFilters) != 1 || req.TypeFilters[0].Value != "Instant" {
		t.Fatalf("parseCardSearchRequest().TypeFilters = %#v, want Instant filter", req.TypeFilters)
	}
}

func TestParseCardSearchRequestSupportsAdvancedOptions(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"q":               []string{"Atraxa"},
		"name_exact":      []string{"1"},
		"mana_cost":       []string{"{W}{W}"},
		"text":            []string{"draw"},
		"type_value":      []string{"Angel"},
		"type_mode":       []string{"is"},
		"type_partial":    []string{"1"},
		"stat":            []string{"power"},
		"stat_op":         []string{"gte"},
		"stat_value":      []string{"2"},
		"price_op":        []string{"lte"},
		"price_value":     []string{"9.99"},
		"commander_legal": []string{"1"},
		"include_tokens":  []string{"1"},
		"sort":            []string{"mana_value"},
		"sort_dir":        []string{"desc"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if !req.HasSearched {
		t.Fatalf("parseCardSearchRequest().HasSearched = false, want true")
	}
	if !req.NameExact {
		t.Fatalf("parseCardSearchRequest().NameExact = false, want true")
	}
	if req.ManaCostQuery != "{W}{W}" {
		t.Fatalf("parseCardSearchRequest().ManaCostQuery = %q, want %q", req.ManaCostQuery, "{W}{W}")
	}
	if len(req.TypeFilters) != 1 || req.TypeFilters[0].Value != "Angel" {
		t.Fatalf("parseCardSearchRequest().TypeFilters = %#v, want Angel filter", req.TypeFilters)
	}
	if !req.TypePartial {
		t.Fatalf("parseCardSearchRequest().TypePartial = false, want true")
	}
	if req.Stat != "power" {
		t.Fatalf("parseCardSearchRequest().Stat = %q, want %q", req.Stat, "power")
	}
	if req.StatOperator != "gte" {
		t.Fatalf("parseCardSearchRequest().StatOperator = %q, want %q", req.StatOperator, "gte")
	}
	if req.StatValue == nil || *req.StatValue != 2 {
		t.Fatalf("parseCardSearchRequest().StatValue = %#v, want 2", req.StatValue)
	}
	if req.PriceOperator != "lte" {
		t.Fatalf("parseCardSearchRequest().PriceOperator = %q, want %q", req.PriceOperator, "lte")
	}
	if req.PriceValue == nil || *req.PriceValue != 9.99 {
		t.Fatalf("parseCardSearchRequest().PriceValue = %#v, want 9.99", req.PriceValue)
	}
	if !req.CommanderLegal {
		t.Fatalf("parseCardSearchRequest().CommanderLegal = false, want true")
	}
	if !req.IncludeTokens {
		t.Fatalf("parseCardSearchRequest().IncludeTokens = false, want true")
	}
	if req.Sort != "mana_value" {
		t.Fatalf("parseCardSearchRequest().Sort = %q, want %q", req.Sort, "mana_value")
	}
	if req.SortDirection != "desc" {
		t.Fatalf("parseCardSearchRequest().SortDirection = %q, want %q", req.SortDirection, "desc")
	}
	if !req.SortDirectionExplicit {
		t.Fatalf("parseCardSearchRequest().SortDirectionExplicit = false, want true")
	}
}

func TestParseCardSearchRequestSupportsMultipleTypeModes(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"type_value": []string{"Artifact", "Creature", "Artifact"},
		"type_mode":  []string{"is", "not", "not"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if len(req.TypeFilters) != 2 {
		t.Fatalf("parseCardSearchRequest().TypeFilters length = %d, want 2", len(req.TypeFilters))
	}
	if req.TypeFilters[0].Value != "Artifact" || req.TypeFilters[0].Mode != "not" {
		t.Fatalf("parseCardSearchRequest().TypeFilters[0] = %#v, want Artifact/not", req.TypeFilters[0])
	}
	if req.TypeFilters[1].Value != "Creature" || req.TypeFilters[1].Mode != "not" {
		t.Fatalf("parseCardSearchRequest().TypeFilters[1] = %#v, want Creature/not", req.TypeFilters[1])
	}
}

func TestParseCardSearchRequestInvalidSortDefaultsToRelevance(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"sort": []string{"mystery"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if req.Sort != "relevance" {
		t.Fatalf("parseCardSearchRequest().Sort = %q, want %q", req.Sort, "relevance")
	}
	if req.SortDirection != "asc" {
		t.Fatalf("parseCardSearchRequest().SortDirection = %q, want %q", req.SortDirection, "asc")
	}
	if req.SortDirectionExplicit {
		t.Fatalf("parseCardSearchRequest().SortDirectionExplicit = true, want false")
	}
}

func TestParseCardSearchRequestRelevanceQueryDefaultsDescendingDirection(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"q":    []string{"Atraxa"},
		"sort": []string{"relevance"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if req.SortDirection != "desc" {
		t.Fatalf("parseCardSearchRequest().SortDirection = %q, want %q", req.SortDirection, "desc")
	}
	if req.SortDirectionExplicit {
		t.Fatalf("parseCardSearchRequest().SortDirectionExplicit = true, want false")
	}
}

func TestCardSearchQueryValuesIncludeSort(t *testing.T) {
	t.Parallel()

	values := cardSearchQueryValues(cardSearchRequest{
		NameQuery:             "Atraxa",
		Sort:                  "mana_value",
		SortDirection:         "desc",
		SortDirectionExplicit: true,
	})
	if got := values.Get("q"); got != "Atraxa" {
		t.Fatalf("cardSearchQueryValues() q = %q, want %q", got, "Atraxa")
	}
	if got := values.Get("sort"); got != "mana_value" {
		t.Fatalf("cardSearchQueryValues() sort = %q, want %q", got, "mana_value")
	}
	if got := values.Get("sort_dir"); got != "desc" {
		t.Fatalf("cardSearchQueryValues() sort_dir = %q, want %q", got, "desc")
	}
}

func TestCardSearchQueryValuesOmitImplicitSortDirection(t *testing.T) {
	t.Parallel()

	values := cardSearchQueryValues(cardSearchRequest{
		NameQuery:     "Atraxa",
		Sort:          "relevance",
		SortDirection: "desc",
		HasSearched:   true,
	})
	if got := values.Get("sort"); got != "relevance" {
		t.Fatalf("cardSearchQueryValues() sort = %q, want %q", got, "relevance")
	}
	if got := values.Get("sort_dir"); got != "" {
		t.Fatalf("cardSearchQueryValues() sort_dir = %q, want empty", got)
	}
}

func TestBuildSearchResultsIncludesDeckbuilderMetadata(t *testing.T) {
	t.Parallel()

	results := buildSearchResults([]cards.Card{{
		OracleID:             "123e4567-e89b-12d3-a456-426614174000",
		Name:                 "Atraxa, Grand Unifier",
		ManaCost:             "{3}{G}{W}{U}{B}",
		TypeLine:             "Legendary Creature — Phyrexian Angel",
		OracleText:           "Flying, vigilance, deathtouch, lifelink",
		ImageURI:             "https://example.com/atraxa.jpg",
		ColorIdentity:        []string{"U", "W", "B", "G"},
		CMC:                  7,
		IsCommanderCandidate: true,
	}})

	if len(results) != 1 {
		t.Fatalf("buildSearchResults() len = %d, want 1", len(results))
	}

	got := results[0]
	if got.ColorIdentity != "White, Blue, Black, Green" {
		t.Fatalf("buildSearchResults().ColorIdentity = %q, want %q", got.ColorIdentity, "White, Blue, Black, Green")
	}
	if got.ManaValue != "7" {
		t.Fatalf("buildSearchResults().ManaValue = %q, want %q", got.ManaValue, "7")
	}
	if !got.IsCommanderCandidate {
		t.Fatalf("buildSearchResults().IsCommanderCandidate = false, want true")
	}
}

func TestBuildCardSearchFilterChipsPreserveOtherFilters(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		NameQuery:     "Atraxa",
		TypeFilters:   []cardSearchTypeFilter{{Value: "Creature", Mode: "is"}},
		ColorParams:   []string{"W", "U"},
		ColorMode:     "exact",
		Stat:          "mana_value",
		StatOperator:  "gte",
		StatValueRaw:  "3",
		SetQuery:      "bro",
		CommanderOnly: true,
		HasSearched:   true,
	}

	chips := buildCardSearchFilterChips(req)
	chipMap := map[string]string{}
	for _, chip := range chips {
		chipMap[chip.Label] = chip.RemovePath
	}

	removeType, ok := chipMap["Type: Creature"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing type chip: %#v", chips)
	}
	if parsed, err := url.Parse(removeType); err != nil {
		t.Fatalf("url.Parse(%q) error = %v", removeType, err)
	} else if parsed.Path != "/cards" {
		t.Fatalf("type remove path = %q, want %q", parsed.Path, "/cards")
	}
	typeQuery := mustParseQuery(t, removeType)
	if got := typeQuery.Get("q"); got != "Atraxa" {
		t.Fatalf("type remove query q = %q, want %q", got, "Atraxa")
	}
	if got := typeQuery.Get("set"); got != "bro" {
		t.Fatalf("type remove query set = %q, want %q", got, "bro")
	}
	if got := typeQuery.Get("stat"); got != "mana_value" {
		t.Fatalf("type remove query stat = %q, want %q", got, "mana_value")
	}
	if got := typeQuery.Get("stat_op"); got != "gte" {
		t.Fatalf("type remove query stat_op = %q, want %q", got, "gte")
	}
	if got := typeQuery.Get("stat_value"); got != "3" {
		t.Fatalf("type remove query stat_value = %q, want %q", got, "3")
	}
	if got := typeQuery.Get("commander"); got != "1" {
		t.Fatalf("type remove query commander = %q, want %q", got, "1")
	}
	if got := typeQuery["type_value"]; len(got) != 0 {
		t.Fatalf("type remove query type_value = %#v, want empty", got)
	}

	removeWhite, ok := chipMap["Color: White"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing white chip: %#v", chips)
	}
	colorQuery := mustParseQuery(t, removeWhite)
	if got := colorQuery["color"]; len(got) != 1 || got[0] != "U" {
		t.Fatalf("white remove query color = %#v, want [U]", got)
	}
	if got := colorQuery.Get("color_mode"); got != "exact" {
		t.Fatalf("white remove query color_mode = %q, want %q", got, "exact")
	}
}

func TestBuildCardSearchFilterChipsSupportAdvancedFilterRemovals(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		NameQuery:             "Atraxa",
		NameExact:             true,
		ManaCostQuery:         "{W}{W}",
		TextQuery:             "draw",
		TypeFilters:           []cardSearchTypeFilter{{Value: "Angel", Mode: "is"}},
		TypePartial:           true,
		Stat:                  "power",
		StatOperator:          "gte",
		StatValueRaw:          "2",
		PriceOperator:         "lte",
		PriceValueRaw:         "1.25",
		CommanderLegal:        true,
		IncludeTokens:         true,
		Sort:                  "rarity",
		SortDirection:         "desc",
		SortDirectionExplicit: true,
		HasSearched:           true,
	}

	chips := buildCardSearchFilterChips(req)
	chipMap := map[string]string{}
	for _, chip := range chips {
		chipMap[chip.Label] = chip.RemovePath
	}

	exactPath, ok := chipMap["Exact name match"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing exact-name chip: %#v", chips)
	}
	exactQuery := mustParseQuery(t, exactPath)
	if got := exactQuery.Get("name_exact"); got != "" {
		t.Fatalf("exact remove query name_exact = %q, want empty", got)
	}
	if got := exactQuery.Get("q"); got != "Atraxa" {
		t.Fatalf("exact remove query q = %q, want %q", got, "Atraxa")
	}
	if got := exactQuery.Get("sort"); got != "rarity" {
		t.Fatalf("exact remove query sort = %q, want %q", got, "rarity")
	}
	if got := exactQuery.Get("sort_dir"); got != "desc" {
		t.Fatalf("exact remove query sort_dir = %q, want %q", got, "desc")
	}

	manaCostPath, ok := chipMap["Mana Cost: {W}{W}"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing mana-cost chip: %#v", chips)
	}
	manaCostQuery := mustParseQuery(t, manaCostPath)
	if got := manaCostQuery.Get("mana_cost"); got != "" {
		t.Fatalf("mana-cost remove query mana_cost = %q, want empty", got)
	}
	if got := manaCostQuery.Get("commander_legal"); got != "1" {
		t.Fatalf("mana-cost remove query commander_legal = %q, want %q", got, "1")
	}

	typePartialPath, ok := chipMap["Allow partial type matches"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing type-partial chip: %#v", chips)
	}
	typePartialQuery := mustParseQuery(t, typePartialPath)
	if got := typePartialQuery.Get("type_partial"); got != "" {
		t.Fatalf("type-partial remove query type_partial = %q, want empty", got)
	}
	if got := typePartialQuery.Get("type_value"); got != "Angel" {
		t.Fatalf("type-partial remove query type_value = %q, want %q", got, "Angel")
	}

	pricePath, ok := chipMap["Price <= $1.25"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing price chip: %#v", chips)
	}
	priceQuery := mustParseQuery(t, pricePath)
	if got := priceQuery.Get("price_value"); got != "" {
		t.Fatalf("price remove query price_value = %q, want empty", got)
	}
	if got := priceQuery.Get("price_op"); got != "" {
		t.Fatalf("price remove query price_op = %q, want empty", got)
	}
	if got := priceQuery.Get("stat_value"); got != "2" {
		t.Fatalf("price remove query stat_value = %q, want %q", got, "2")
	}
	if got := priceQuery.Get("sort"); got != "rarity" {
		t.Fatalf("price remove query sort = %q, want %q", got, "rarity")
	}
	if got := priceQuery.Get("sort_dir"); got != "desc" {
		t.Fatalf("price remove query sort_dir = %q, want %q", got, "desc")
	}

	commanderLegalPath, ok := chipMap["Commander legal"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing commander-legal chip: %#v", chips)
	}
	commanderLegalQuery := mustParseQuery(t, commanderLegalPath)
	if got := commanderLegalQuery.Get("commander_legal"); got != "" {
		t.Fatalf("commander-legal remove query commander_legal = %q, want empty", got)
	}
	if got := commanderLegalQuery.Get("include_tokens"); got != "1" {
		t.Fatalf("commander-legal remove query include_tokens = %q, want %q", got, "1")
	}
	if got := commanderLegalQuery.Get("stat"); got != "power" {
		t.Fatalf("commander-legal remove query stat = %q, want %q", got, "power")
	}
	if got := commanderLegalQuery.Get("stat_op"); got != "gte" {
		t.Fatalf("commander-legal remove query stat_op = %q, want %q", got, "gte")
	}
	if got := commanderLegalQuery.Get("stat_value"); got != "2" {
		t.Fatalf("commander-legal remove query stat_value = %q, want %q", got, "2")
	}
	if got := commanderLegalQuery.Get("price_op"); got != "lte" {
		t.Fatalf("commander-legal remove query price_op = %q, want %q", got, "lte")
	}
	if got := commanderLegalQuery.Get("price_value"); got != "1.25" {
		t.Fatalf("commander-legal remove query price_value = %q, want %q", got, "1.25")
	}
	if got := commanderLegalQuery.Get("sort"); got != "rarity" {
		t.Fatalf("commander-legal remove query sort = %q, want %q", got, "rarity")
	}
	if got := commanderLegalQuery.Get("sort_dir"); got != "desc" {
		t.Fatalf("commander-legal remove query sort_dir = %q, want %q", got, "desc")
	}
}

func TestBuildCardSearchFilterChipsRemoveOneTypeKeepsOtherTypeModes(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		TypeFilters: []cardSearchTypeFilter{
			{Value: "Artifact", Mode: "is"},
			{Value: "Creature", Mode: "not"},
		},
		TypePartial: true,
		HasSearched: true,
	}

	chips := buildCardSearchFilterChips(req)
	chipMap := map[string]string{}
	for _, chip := range chips {
		chipMap[chip.Label] = chip.RemovePath
	}

	removeArtifact, ok := chipMap["Type: Artifact"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing artifact chip: %#v", chips)
	}

	artifactQuery := mustParseQuery(t, removeArtifact)
	if got := artifactQuery["type_value"]; len(got) != 1 || got[0] != "Creature" {
		t.Fatalf("artifact remove query type_value = %#v, want [Creature]", got)
	}
	if got := artifactQuery["type_mode"]; len(got) != 1 || got[0] != "not" {
		t.Fatalf("artifact remove query type_mode = %#v, want [not]", got)
	}
	if got := artifactQuery.Get("type_partial"); got != "1" {
		t.Fatalf("artifact remove query type_partial = %q, want %q", got, "1")
	}

	removeCreature, ok := chipMap["Type: NOT Creature"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing creature chip: %#v", chips)
	}

	creatureQuery := mustParseQuery(t, removeCreature)
	if got := creatureQuery["type_value"]; len(got) != 1 || got[0] != "Artifact" {
		t.Fatalf("creature remove query type_value = %#v, want [Artifact]", got)
	}
	if got := creatureQuery["type_mode"]; len(got) != 1 || got[0] != "is" {
		t.Fatalf("creature remove query type_mode = %#v, want [is]", got)
	}
	if got := creatureQuery.Get("type_partial"); got != "1" {
		t.Fatalf("creature remove query type_partial = %q, want %q", got, "1")
	}
}

func TestBuildCardSearchFilterChipsDropsColorModeWhenRemovingLastColor(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		ColorParams: []string{"C"},
		ColorMode:   "at_most",
		HasSearched: true,
	}

	chips := buildCardSearchFilterChips(req)
	chipMap := map[string]string{}
	for _, chip := range chips {
		chipMap[chip.Label] = chip.RemovePath
	}

	removeColor, ok := chipMap["Color: Colorless"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing colorless chip: %#v", chips)
	}
	query := mustParseQuery(t, removeColor)
	if got := query["color"]; len(got) != 0 {
		t.Fatalf("last color remove query color = %#v, want empty", got)
	}
	if got := query.Get("color_mode"); got != "" {
		t.Fatalf("last color remove query color_mode = %q, want empty", got)
	}
}

func TestCardSearchEditPathUsesSearchEditorRoute(t *testing.T) {
	t.Parallel()

	got := cardSearchEditPath(cardSearchRequest{
		NameQuery:             "Atraxa",
		NameExact:             true,
		TextQuery:             "draw",
		TextMode:              "not",
		Sort:                  "oldest_printing",
		SortDirection:         "desc",
		SortDirectionExplicit: true,
		HasSearched:           true,
	})

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", got, err)
	}
	if parsed.Path != "/cards/search" {
		t.Fatalf("cardSearchEditPath() path = %q, want %q", parsed.Path, "/cards/search")
	}

	query := parsed.Query()
	if got := query.Get("edit"); got != "1" {
		t.Fatalf("cardSearchEditPath() edit = %q, want %q", got, "1")
	}
	if got := query.Get("q"); got != "Atraxa" {
		t.Fatalf("cardSearchEditPath() q = %q, want %q", got, "Atraxa")
	}
	if got := query.Get("name_exact"); got != "1" {
		t.Fatalf("cardSearchEditPath() name_exact = %q, want %q", got, "1")
	}
	if got := query.Get("text_mode"); got != "not" {
		t.Fatalf("cardSearchEditPath() text_mode = %q, want %q", got, "not")
	}
	if got := query.Get("sort"); got != "oldest_printing" {
		t.Fatalf("cardSearchEditPath() sort = %q, want %q", got, "oldest_printing")
	}
	if got := query.Get("sort_dir"); got != "desc" {
		t.Fatalf("cardSearchEditPath() sort_dir = %q, want %q", got, "desc")
	}
}

func TestHandleCardSearchRedirectsToResultsWithoutFiltersWhenViewRequested(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodGet, "/cards/search?view=1", nil)
	rr := httptest.NewRecorder()

	app.HandleCardSearch(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("HandleCardSearch() status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	parsed, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("url.Parse(Location) error = %v", err)
	}
	if parsed.Path != "/cards" {
		t.Fatalf("HandleCardSearch() redirect path = %q, want %q", parsed.Path, "/cards")
	}
	if got := parsed.Query().Get("sort"); got != "relevance" {
		t.Fatalf("HandleCardSearch() redirect sort = %q, want %q", got, "relevance")
	}
	if got := parsed.Query().Get("sort_dir"); got != "" {
		t.Fatalf("HandleCardSearch() redirect sort_dir = %q, want empty", got)
	}
}

func TestHandleCardSearchRedirectsPreserveSelectedSort(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodGet, "/cards/search?view=1&q=Atraxa&sort=mana_value&sort_dir=desc", nil)
	rr := httptest.NewRecorder()

	app.HandleCardSearch(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("HandleCardSearch() status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	parsed, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("url.Parse(Location) error = %v", err)
	}
	if parsed.Path != "/cards" {
		t.Fatalf("HandleCardSearch() redirect path = %q, want %q", parsed.Path, "/cards")
	}
	if got := parsed.Query().Get("q"); got != "Atraxa" {
		t.Fatalf("HandleCardSearch() redirect q = %q, want %q", got, "Atraxa")
	}
	if got := parsed.Query().Get("sort"); got != "mana_value" {
		t.Fatalf("HandleCardSearch() redirect sort = %q, want %q", got, "mana_value")
	}
	if got := parsed.Query().Get("sort_dir"); got != "desc" {
		t.Fatalf("HandleCardSearch() redirect sort_dir = %q, want %q", got, "desc")
	}
}

func TestHandleCardSearchRedirectsRelevanceQueryWithoutImplicitDirection(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodGet, "/cards/search?view=1&q=Atraxa&sort=relevance", nil)
	rr := httptest.NewRecorder()

	app.HandleCardSearch(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("HandleCardSearch() status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	parsed, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("url.Parse(Location) error = %v", err)
	}
	if got := parsed.Query().Get("sort_dir"); got != "" {
		t.Fatalf("HandleCardSearch() redirect sort_dir = %q, want empty", got)
	}
}

func TestCardsListTemplateShowsSelectedSort(t *testing.T) {
	page := cardListPageData{
		Results: []searchResult{{
			OracleID:   "123e4567-e89b-12d3-a456-426614174000",
			DetailPath: cardDetailPath("123e4567-e89b-12d3-a456-426614174000"),
			Name:       "Atraxa, Grand Unifier",
		}},
		HasSearched:            true,
		CurrentPath:            "/cards?q=Atraxa&sort=mana_value&sort_dir=desc",
		ClearPath:              "/cards/search",
		EditFiltersPath:        "/cards/search?edit=1&q=Atraxa&sort=mana_value&sort_dir=desc",
		SortOptions:            advancedCardSortOptions,
		DirectionOptions:       advancedCardSortDirectionOptions,
		SelectedSort:           "mana_value",
		SelectedSortDirection:  "desc",
		CurrentSortLabel:       "Mana Value",
		CurrentDirectionLabel:  "Descending",
		SortFields:             []cardSearchQueryField{{Name: "q", Value: "Atraxa"}},
		ShowOldestPrintingNote: false,
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "cards_list", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	if !strings.Contains(body, `<option value="mana_value" selected>Mana Value</option>`) {
		t.Fatalf("cards_list template did not render the selected sort option: %s", body)
	}
	if !strings.Contains(body, `<option value="desc" selected>Descending</option>`) {
		t.Fatalf("cards_list template did not render the selected direction option: %s", body)
	}
	if !strings.Contains(body, `name="q" value="Atraxa"`) {
		t.Fatalf("cards_list template did not preserve hidden query fields: %s", body)
	}
	if !strings.Contains(body, `onchange="this.form.requestSubmit()"`) {
		t.Fatalf("cards_list template did not render auto-submit sort controls: %s", body)
	}
	if !strings.Contains(body, `<noscript>`) {
		t.Fatalf("cards_list template did not render a noscript fallback: %s", body)
	}
}

func TestCardsSearchTemplateOmitsImplicitSortDirectionHiddenField(t *testing.T) {
	page := cardSearchPageData{
		Sort:                  "relevance",
		SortDirection:         "desc",
		SortDirectionExplicit: false,
		CurrentSortLabel:      "Relevance",
		CurrentDirectionLabel: "Descending",
		ShowCurrentSort:       true,
		SearchActionPath:      "/cards/search",
		ClearPath:             "/cards/search",
		ColorsSelected: map[string]bool{
			"W": false,
			"U": false,
			"B": false,
			"R": false,
			"G": false,
			"C": false,
		},
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "cards_search", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	if !strings.Contains(body, `name="sort" value="relevance"`) {
		t.Fatalf("cards_search template did not preserve hidden sort field: %s", body)
	}
	searchFormMarker := `<form method="GET" action="/cards/search" class="space-y-6" data-card-search-form>`
	searchFormStart := strings.Index(body, searchFormMarker)
	if searchFormStart < 0 {
		t.Fatalf("cards_search template did not render the advanced search form: %s", body)
	}
	searchFormBody := body[searchFormStart:]
	if nextForm := strings.Index(searchFormBody[len(searchFormMarker):], "</form>"); nextForm >= 0 {
		searchFormBody = searchFormBody[:len(searchFormMarker)+nextForm]
	}
	if strings.Contains(searchFormBody, `name="sort_dir"`) {
		t.Fatalf("cards_search template unexpectedly rendered hidden sort_dir field in the advanced search form: %s", searchFormBody)
	}
}

func mustParseQuery(t *testing.T, rawPath string) url.Values {
	t.Helper()

	parsed, err := url.Parse(rawPath)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", rawPath, err)
	}
	return parsed.Query()
}
