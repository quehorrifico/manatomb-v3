package cards

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

var cardSearchOutcomeDriverSequence atomic.Uint64

type cardSearchOutcomeTestDriver struct {
	exact bool
	fuzzy bool
}

func (d *cardSearchOutcomeTestDriver) Open(string) (driver.Conn, error) {
	return &cardSearchOutcomeTestConn{driver: d}, nil
}

type cardSearchOutcomeTestConn struct {
	driver *cardSearchOutcomeTestDriver
}

func (c *cardSearchOutcomeTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (c *cardSearchOutcomeTestConn) Close() error {
	return nil
}

func (c *cardSearchOutcomeTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

func (c *cardSearchOutcomeTestConn) QueryContext(
	_ context.Context,
	query string,
	_ []driver.NamedValue,
) (driver.Rows, error) {
	lowerQuery := strings.ToLower(query)
	if strings.Contains(lowerQuery, "count(") {
		total := int64(0)
		if c.driver.exact || c.driver.fuzzy {
			total = 1
		}
		return &cardSearchOutcomeTestRows{
			columns: []string{"count"},
			values:  [][]driver.Value{{total}},
		}, nil
	}

	if strings.Contains(query, "oc.name_search = normalize_card_name") {
		return newCardSearchOutcomeTestRows(c.driver.exact, "Exact Card"), nil
	}
	if strings.Contains(query, "oc.name_search LIKE") {
		return newCardSearchOutcomeTestRows(c.driver.fuzzy, "Fuzzy Card"), nil
	}
	if strings.Contains(query, "WHERE 1=1") {
		return newCardSearchOutcomeTestRows(c.driver.fuzzy, "Filtered Card"), nil
	}
	return nil, errors.New("unexpected search query")
}

type cardSearchOutcomeTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func newCardSearchOutcomeTestRows(include bool, name string) *cardSearchOutcomeTestRows {
	rows := &cardSearchOutcomeTestRows{columns: canonicalCardSearchTestColumns()}
	if include {
		rows.values = [][]driver.Value{canonicalCardSearchTestRow(name)}
	}
	return rows
}

func (r *cardSearchOutcomeTestRows) Columns() []string {
	return r.columns
}

func (r *cardSearchOutcomeTestRows) Close() error {
	return nil
}

func (r *cardSearchOutcomeTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func canonicalCardSearchTestColumns() []string {
	return []string{
		"id",
		"oracle_id",
		"name",
		"mana_cost",
		"type_line",
		"oracle_text",
		"flavor_text",
		"image_uri",
		"art_crop_uri",
		"colors",
		"color_identity",
		"cmc",
		"power",
		"toughness",
		"loyalty",
		"layout",
		"legal_anywhere",
		"commander_legal",
		"is_commander_candidate",
		"price_usd",
		"artist",
		"edhrec_rank",
		"scryfall_uri",
		"set_code",
		"set_name",
		"collector_number",
		"rarity",
		"released_at",
		"lang",
		"faces_json",
		"legalities_json",
	}
}

func canonicalCardSearchTestRow(name string) []driver.Value {
	return []driver.Value{
		"223e4567-e89b-12d3-a456-426614174000",
		"123e4567-e89b-12d3-a456-426614174000",
		name,
		"{1}{U}",
		"Creature",
		"Draw a card.",
		"",
		"https://example.test/card.jpg",
		"https://example.test/art.jpg",
		"{U}",
		"{U}",
		float64(2),
		"2",
		"2",
		"",
		"normal",
		true,
		true,
		true,
		"$1.00",
		"Test Artist",
		int64(1),
		"https://example.test/card",
		"tst",
		"Test Set",
		"1",
		"rare",
		"2026-01-01",
		"en",
		"[]",
		"{}",
	}
}

func openCardSearchOutcomeTestDB(t *testing.T, exact, fuzzy bool) *sql.DB {
	t.Helper()

	driverName := "card-search-outcome-" + strconv.FormatUint(cardSearchOutcomeDriverSequence.Add(1), 10)
	sql.Register(driverName, &cardSearchOutcomeTestDriver{exact: exact, fuzzy: fuzzy})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func TestSearchCardsWithOutcomeReportsExactNameMatch(t *testing.T) {
	db := openCardSearchOutcomeTestDB(t, true, true)

	outcome, err := SearchCardsWithOutcome(context.Background(), db, CardSearchParams{
		Query: "Exact Card",
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("SearchCardsWithOutcome() error = %v", err)
	}
	if !outcome.ExactNameMatch {
		t.Fatal("SearchCardsWithOutcome().ExactNameMatch = false, want true")
	}
	if len(outcome.Cards) != 1 || outcome.Cards[0].Name != "Exact Card" {
		t.Fatalf("SearchCardsWithOutcome().Cards = %#v, want exact card", outcome.Cards)
	}

	legacy, err := SearchCards(context.Background(), db, CardSearchParams{
		Query: "Exact Card",
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("SearchCards() compatibility wrapper error = %v", err)
	}
	if len(legacy) != 1 || legacy[0].Name != "Exact Card" {
		t.Fatalf("SearchCards() = %#v, want exact card", legacy)
	}
}

func TestSearchCardsWithOutcomeDoesNotMarkFuzzySingletonExact(t *testing.T) {
	db := openCardSearchOutcomeTestDB(t, false, true)

	outcome, err := SearchCardsWithOutcome(context.Background(), db, CardSearchParams{
		Query: "Fuzzy",
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("SearchCardsWithOutcome() error = %v", err)
	}
	if outcome.ExactNameMatch {
		t.Fatal("SearchCardsWithOutcome().ExactNameMatch = true, want false")
	}
	if len(outcome.Cards) != 1 || outcome.Cards[0].Name != "Fuzzy Card" {
		t.Fatalf("SearchCardsWithOutcome().Cards = %#v, want fuzzy singleton", outcome.Cards)
	}
}

func TestSearchCardsWithOutcomeDoesNotMarkFilterOnlySingletonExact(t *testing.T) {
	db := openCardSearchOutcomeTestDB(t, false, true)

	outcome, err := SearchCardsWithOutcome(context.Background(), db, CardSearchParams{
		Layout: "normal",
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("SearchCardsWithOutcome() error = %v", err)
	}
	if outcome.ExactNameMatch {
		t.Fatal("SearchCardsWithOutcome().ExactNameMatch = true, want false")
	}
	if len(outcome.Cards) != 1 || outcome.Cards[0].Name != "Filtered Card" {
		t.Fatalf("SearchCardsWithOutcome().Cards = %#v, want filter-only singleton", outcome.Cards)
	}
}

func TestBuildCardSearchFiltersAdvanced(t *testing.T) {
	t.Parallel()

	minValue := 2.0
	maxValue := 5.0
	minPrice := 1.25
	maxPrice := 9.99
	sqlText, args := buildCardSearchFilters(CardSearchParams{
		OracleText:     "draw a card",
		OracleTextNot:  true,
		TypeFilters:    []CardTypeFilter{{Value: "Instant"}, {Value: "Legendary", Negated: true}},
		TypePartial:    true,
		Colors:         []string{"U", "W"},
		ColorMode:      "at_most",
		ManaValueMin:   &minValue,
		ManaValueMax:   &maxValue,
		PriceUSDMin:    &minPrice,
		PriceUSDMax:    &maxPrice,
		Rarity:         "rare",
		SetQuery:       "bro",
		ArtistQuery:    "Seb",
		Layout:         "transform",
		CommanderLegal: true,
		CommanderOnly:  true,
	}, 1)

	for _, snippet := range []string{
		"COALESCE(oc.legal_anywhere, TRUE) = TRUE",
		"oc.commander_legal = TRUE",
		"oc.is_commander_candidate = TRUE",
		"lower(COALESCE(oc.type_line, '')) NOT LIKE '%battle%'",
		"oc.oracle_text NOT ILIKE '%' || $1 || '%'",
		"oc.type_line ILIKE '%' || $2 || '%'",
		"oc.type_line NOT ILIKE '%' || $3 || '%'",
		"oc.color_identity <@ $4::text[]",
		"oc.cmc >= $5",
		"oc.cmc <= $6",
		"NULLIF(regexp_replace(COALESCE(oc.default_price_usd, ''), '[^0-9.]', '', 'g'), '')::double precision >= $7",
		"NULLIF(regexp_replace(COALESCE(oc.default_price_usd, ''), '[^0-9.]', '', 'g'), '')::double precision <= $8",
		"lower(COALESCE(cp.rarity, '')) = $9",
		"(oc.default_set_code ILIKE '%' || $10 || '%' OR oc.default_set_name ILIKE '%' || $10 || '%')",
		"oc.default_artist ILIKE '%' || $11 || '%'",
		"lower(COALESCE(oc.layout, '')) = $12",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("buildCardSearchFilters() missing SQL snippet %q in %q", snippet, sqlText)
		}
	}

	if len(args) != 12 {
		t.Fatalf("buildCardSearchFilters() returned %d args, want 12", len(args))
	}
	if args[0] != "draw a card" {
		t.Fatalf("args[0] = %#v, want %q", args[0], "draw a card")
	}
	if args[1] != "Instant" {
		t.Fatalf("args[1] = %#v, want %q", args[1], "Instant")
	}
	if args[2] != "Legendary" {
		t.Fatalf("args[2] = %#v, want %q", args[2], "Legendary")
	}
	if got := arrayArgString(t, args[3]); got != "{\"U\",\"W\"}" {
		t.Fatalf("args[3] = %q, want %q", got, "{\"U\",\"W\"}")
	}
	if args[4] != minValue {
		t.Fatalf("args[4] = %#v, want %v", args[4], minValue)
	}
	if args[5] != maxValue {
		t.Fatalf("args[5] = %#v, want %v", args[5], maxValue)
	}
	if args[6] != minPrice {
		t.Fatalf("args[6] = %#v, want %v", args[6], minPrice)
	}
	if args[7] != maxPrice {
		t.Fatalf("args[7] = %#v, want %v", args[7], maxPrice)
	}
	if args[8] != "rare" {
		t.Fatalf("args[8] = %#v, want %q", args[8], "rare")
	}
	if args[9] != "bro" {
		t.Fatalf("args[9] = %#v, want %q", args[9], "bro")
	}
	if args[10] != "Seb" {
		t.Fatalf("args[10] = %#v, want %q", args[10], "Seb")
	}
	if args[11] != "transform" {
		t.Fatalf("args[11] = %#v, want %q", args[11], "transform")
	}
}

func TestNormalizeColorFiltersDeduplicatesAndUppercases(t *testing.T) {
	t.Parallel()

	got := normalizeColorFilters([]string{"w", "U", "w", " ", "x", "b", "c"})
	want := []string{"W", "U", "B", "C"}
	if len(got) != len(want) {
		t.Fatalf("normalizeColorFilters() length = %d, want %d", len(got), len(want))
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("normalizeColorFilters()[%d] = %q, want %q", idx, got[idx], want[idx])
		}
	}
}

func TestBuildCardSearchFiltersColorless(t *testing.T) {
	t.Parallel()

	sqlText, args := buildCardSearchFilters(CardSearchParams{
		Colors:    []string{"C"},
		ColorMode: "exact",
	}, 1)

	if !strings.Contains(sqlText, "COALESCE(array_length(oc.color_identity, 1), 0) = 0") {
		t.Fatalf("buildCardSearchFilters() missing colorless SQL in %q", sqlText)
	}
	if len(args) != 0 {
		t.Fatalf("buildCardSearchFilters() returned %d args, want 0", len(args))
	}
}

func TestBuildCardSearchFiltersIncludesTokensWhenRequested(t *testing.T) {
	t.Parallel()

	sqlText, _ := buildCardSearchFilters(CardSearchParams{
		IncludeTokens: true,
	}, 1)

	if strings.Contains(sqlText, "lower(btrim(COALESCE(oc.layout, ''))) <> 'token'") {
		t.Fatalf("buildCardSearchFilters() unexpectedly excluded tokens in %q", sqlText)
	}
	if !strings.Contains(sqlText, "COALESCE(oc.legal_anywhere, TRUE) = TRUE OR lower(btrim(COALESCE(oc.layout, ''))) IN ('token', 'double_faced_token')") {
		t.Fatalf("buildCardSearchFilters() missing token-aware legality SQL in %q", sqlText)
	}
}

func TestLegalAnywhereRequiresLegalOrRestrictedFormat(t *testing.T) {
	t.Parallel()

	if !legalAnywhere(map[string]string{"commander": "legal"}) {
		t.Fatal("legalAnywhere() should accept legal cards")
	}
	if !legalAnywhere(map[string]string{"vintage": "restricted"}) {
		t.Fatal("legalAnywhere() should accept restricted cards")
	}
	if legalAnywhere(map[string]string{"commander": "not_legal", "legacy": "banned"}) {
		t.Fatal("legalAnywhere() should reject cards without a legal or restricted format")
	}
}

func TestDecodeLegalitiesJSONNormalizesAndRejectsInvalidData(t *testing.T) {
	t.Parallel()

	got := decodeLegalitiesJSON(`{"Commander":"LEGAL","vintage":"restricted"}`)
	if got["commander"] != "legal" || got["vintage"] != "restricted" {
		t.Fatalf("decodeLegalitiesJSON() = %#v, want normalized legalities", got)
	}
	if invalid := decodeLegalitiesJSON(`{"commander":`); len(invalid) != 0 {
		t.Fatalf("decodeLegalitiesJSON(invalid) = %#v, want empty map", invalid)
	}
	if empty := decodeLegalitiesJSON(""); len(empty) != 0 {
		t.Fatalf("decodeLegalitiesJSON(empty) = %#v, want empty map", empty)
	}
}

func TestCardSearchLimitSupportsAllMatches(t *testing.T) {
	t.Parallel()

	limit, unlimited := cardSearchLimit(CardSearchParams{})
	if unlimited {
		t.Fatal("cardSearchLimit() unexpectedly marked default search unlimited")
	}
	if limit != 120 {
		t.Fatalf("cardSearchLimit() default limit = %d, want 120", limit)
	}

	limit, unlimited = cardSearchLimit(CardSearchParams{Limit: 500})
	if unlimited {
		t.Fatal("cardSearchLimit() unexpectedly marked capped search unlimited")
	}
	if limit != 300 {
		t.Fatalf("cardSearchLimit() capped limit = %d, want 300", limit)
	}

	limit, unlimited = cardSearchLimit(CardSearchParams{Limit: 120, AllMatches: true})
	if !unlimited {
		t.Fatal("cardSearchLimit() did not mark all-matches search unlimited")
	}
	if limit != 0 {
		t.Fatalf("cardSearchLimit() unlimited limit = %d, want 0", limit)
	}
}

func TestCardNameSearchMatchSQLIncludesContainsAndFuzzy(t *testing.T) {
	t.Parallel()

	sqlText := cardNameSearchMatchSQL()
	for _, snippet := range []string{
		"oc.name_search LIKE '%' || $1 || '%'",
		"oc.name_search % $1",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("cardNameSearchMatchSQL() missing SQL snippet %q in %q", snippet, sqlText)
		}
	}
}

func TestCanonicalCardSelectCarriesDefaultPrintingIdentity(t *testing.T) {
	t.Parallel()

	sqlText := canonicalCardSelectSQL()
	for _, snippet := range []string{
		"COALESCE(oc.default_print_id::text, '') AS id",
		"LEFT JOIN card_prints cp ON cp.scryfall_id = oc.default_print_id",
		"COALESCE(cp.flavor_text, '') AS flavor_text",
		"NULLIF(cp.card_faces_json::text, '[]')",
		"COALESCE(oc.legalities, '{}'::jsonb)::text AS legalities_json",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("canonicalCardSelectSQL() missing %q in %q", snippet, sqlText)
		}
	}
}

func TestCardPrintingSelectKeepsPrintingSpecificData(t *testing.T) {
	t.Parallel()

	sqlText := cardPrintingSelectSQL()
	for _, snippet := range []string{
		"cp.scryfall_id::text AS id",
		"COALESCE(cp.flavor_text, '') AS flavor_text",
		"COALESCE(cp.image_uri, '') AS image_uri",
		"NULLIF(cp.card_faces_json::text, '[]')",
		"COALESCE(oc.legalities, '{}'::jsonb)::text AS legalities_json",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("cardPrintingSelectSQL() missing %q in %q", snippet, sqlText)
		}
	}
	if strings.Contains(sqlText, "COALESCE(cp.flavor_text, COALESCE(oc.flavor_text, ''))") {
		t.Fatalf("cardPrintingSelectSQL() still borrows flavor text from another printing: %q", sqlText)
	}
}

func TestBuildCardSearchFiltersSupportsManaCostExactTypeAndStatOperator(t *testing.T) {
	t.Parallel()

	powerValue := 2.0
	sqlText, args := buildCardSearchFilters(CardSearchParams{
		ManaCost:     "{W}{W}",
		TypeFilters:  []CardTypeFilter{{Value: "Artifact"}},
		Stat:         "power",
		StatOperator: "gte",
		StatValue:    &powerValue,
	}, 1)

	for _, snippet := range []string{
		"COALESCE(oc.mana_cost, '') ILIKE '%' || $1 || '%'",
		"oc.type_line ~* $2",
		"oc.power_value >= $3",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("buildCardSearchFilters() missing SQL snippet %q in %q", snippet, sqlText)
		}
	}
	if strings.Contains(sqlText, "oc.power_value <=") {
		t.Fatalf("buildCardSearchFilters() unexpectedly included upper-bound SQL in %q", sqlText)
	}

	if len(args) != 3 {
		t.Fatalf("buildCardSearchFilters() returned %d args, want 3", len(args))
	}
	if args[0] != "{W}{W}" {
		t.Fatalf("args[0] = %#v, want %q", args[0], "{W}{W}")
	}
	if args[1] != exactTypePattern("Artifact") {
		t.Fatalf("args[1] = %#v, want %q", args[1], exactTypePattern("Artifact"))
	}
	if args[2] != powerValue {
		t.Fatalf("args[2] = %#v, want %v", args[2], powerValue)
	}
}

func TestBuildCardSearchFiltersSupportsMultipleStatsAndRarities(t *testing.T) {
	t.Parallel()

	sqlText, args := buildCardSearchFilters(CardSearchParams{
		StatFilters: []CardStatFilter{
			{Stat: "mana_value", Operator: "gte", Value: 2},
			{Stat: "power", Operator: "lte", Value: 4},
		},
		Rarities: []string{"rare", "mythic", "rare", "invalid"},
	}, 1)

	for _, snippet := range []string{
		"oc.cmc >= $1",
		"oc.power_value <= $2",
		"lower(COALESCE(cp.rarity, '')) = ANY($3::text[])",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("buildCardSearchFilters() missing SQL snippet %q in %q", snippet, sqlText)
		}
	}
	if len(args) != 3 {
		t.Fatalf("buildCardSearchFilters() returned %d args, want 3", len(args))
	}
	if args[0] != 2.0 || args[1] != 4.0 {
		t.Fatalf("buildCardSearchFilters() stat args = %#v, want [2 4]", args[:2])
	}
	if got := arrayArgString(t, args[2]); got != `{"rare","mythic"}` {
		t.Fatalf("buildCardSearchFilters() rarity arg = %q, want %q", got, `{"rare","mythic"}`)
	}
}

func TestBuildCardSearchFiltersSupportsMultiplePricesWithSlicePrecedence(t *testing.T) {
	t.Parallel()

	scalarValue := 99.0
	minValue := 50.0
	maxValue := 100.0
	sqlText, args := buildCardSearchFilters(CardSearchParams{
		PriceOperator: "eq",
		PriceValue:    &scalarValue,
		PriceUSDMin:   &minValue,
		PriceUSDMax:   &maxValue,
		PriceFilters: []CardPriceFilter{
			{Operator: "gte", Value: 1},
			{Operator: "lte", Value: 25},
		},
	}, 1)

	priceExpr := "NULLIF(regexp_replace(COALESCE(oc.default_price_usd, ''), '[^0-9.]', '', 'g'), '')::double precision"
	for _, snippet := range []string{
		priceExpr + " >= $1",
		priceExpr + " <= $2",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("buildCardSearchFilters() missing SQL snippet %q in %q", snippet, sqlText)
		}
	}
	if len(args) != 2 || args[0] != 1.0 || args[1] != 25.0 {
		t.Fatalf("buildCardSearchFilters() price args = %#v, want [1 25]", args)
	}
}

func TestBuildCardSearchFiltersSupportsPriceOperator(t *testing.T) {
	t.Parallel()

	priceValue := 9.99
	sqlText, args := buildCardSearchFilters(CardSearchParams{
		PriceOperator: "lte",
		PriceValue:    &priceValue,
	}, 1)

	if !strings.Contains(sqlText, "NULLIF(regexp_replace(COALESCE(oc.default_price_usd, ''), '[^0-9.]', '', 'g'), '')::double precision <= $1") {
		t.Fatalf("buildCardSearchFilters() missing price operator SQL in %q", sqlText)
	}
	if strings.Contains(sqlText, "NULLIF(regexp_replace(COALESCE(oc.default_price_usd, ''), '[^0-9.]', '', 'g'), '')::double precision >=") {
		t.Fatalf("buildCardSearchFilters() unexpectedly included legacy lower-bound price SQL in %q", sqlText)
	}
	if len(args) != 1 {
		t.Fatalf("buildCardSearchFilters() returned %d args, want 1", len(args))
	}
	if args[0] != priceValue {
		t.Fatalf("args[0] = %#v, want %v", args[0], priceValue)
	}
}

func TestNormalizeCardSearchSortDefaultsToRelevance(t *testing.T) {
	t.Parallel()

	if got := normalizeCardSearchSort(""); got != "relevance" {
		t.Fatalf("normalizeCardSearchSort(\"\") = %q, want %q", got, "relevance")
	}
	if got := normalizeCardSearchSort("unknown"); got != "relevance" {
		t.Fatalf("normalizeCardSearchSort(\"unknown\") = %q, want %q", got, "relevance")
	}
}

func TestNormalizeCardSearchSortDirectionUsesSortDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sortMode     string
		hasNameQuery bool
		raw          string
		want         string
	}{
		{
			name:         "relevance with query",
			sortMode:     "relevance",
			hasNameQuery: true,
			want:         "desc",
		},
		{
			name:         "relevance without query",
			sortMode:     "relevance",
			hasNameQuery: false,
			want:         "asc",
		},
		{
			name:         "newest printing",
			sortMode:     "newest_printing",
			hasNameQuery: false,
			want:         "desc",
		},
		{
			name:         "mana value",
			sortMode:     "mana_value",
			hasNameQuery: false,
			want:         "asc",
		},
		{
			name:         "explicit desc",
			sortMode:     "alphabetical",
			hasNameQuery: false,
			raw:          "desc",
			want:         "desc",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeCardSearchSortDirection(tt.sortMode, tt.hasNameQuery, tt.raw); got != tt.want {
				t.Fatalf("normalizeCardSearchSortDirection(%q, %t, %q) = %q, want %q", tt.sortMode, tt.hasNameQuery, tt.raw, got, tt.want)
			}
		})
	}
}

func TestCardSearchOrderBySQLRelevanceWithNameQuery(t *testing.T) {
	t.Parallel()

	sqlText := cardSearchOrderBySQL("relevance", "desc", true, true)

	for _, snippet := range []string{
		"(oc.name_search = $1) DESC",
		"(oc.name_search LIKE $1 || '%') DESC",
		"similarity(oc.name_search, $1) DESC",
		"COALESCE(oc.edhrec_rank, 999999) ASC",
		"lower(oc.name) ASC",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("cardSearchOrderBySQL() missing SQL snippet %q in %q", snippet, sqlText)
		}
	}
}

func TestCardSearchOrderBySQLRelevanceWithoutNameQueryFallsBackAlphabetical(t *testing.T) {
	t.Parallel()

	sqlText := cardSearchOrderBySQL("relevance", "asc", false, false)

	for _, snippet := range []string{
		"lower(oc.name) ASC",
		"oc.name ASC",
		"oc.oracle_id ASC",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("cardSearchOrderBySQL() missing SQL snippet %q in %q", snippet, sqlText)
		}
	}
	for _, snippet := range []string{
		"similarity(oc.name_search, $1) DESC",
		"COALESCE(oc.edhrec_rank, 999999) ASC",
	} {
		if strings.Contains(sqlText, snippet) {
			t.Fatalf("cardSearchOrderBySQL() unexpectedly included SQL snippet %q in %q", snippet, sqlText)
		}
	}
}

func TestCardSearchOrderBySQLSupportsAlternateSorts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sortMode string
		dir      string
		snippets []string
	}{
		{
			name:     "alphabetical",
			sortMode: "alphabetical",
			dir:      "asc",
			snippets: []string{"lower(oc.name) ASC", "oc.oracle_id ASC"},
		},
		{
			name:     "mana value",
			sortMode: "mana_value",
			dir:      "asc",
			snippets: []string{"COALESCE(oc.cmc, 0) ASC", "lower(oc.name) ASC"},
		},
		{
			name:     "newest printing",
			sortMode: "newest_printing",
			dir:      "desc",
			snippets: []string{"oc.default_released_at DESC NULLS LAST", "lower(oc.name) ASC"},
		},
		{
			name:     "oldest printing",
			sortMode: "oldest_printing",
			dir:      "asc",
			snippets: []string{"SELECT MIN(cp_oldest.released_at)", "lower(COALESCE(cp_oldest.lang, 'en')) = 'en'"},
		},
		{
			name:     "rarity",
			sortMode: "rarity",
			dir:      "asc",
			snippets: []string{"CASE lower(COALESCE(cp.rarity, ''))", "WHEN 'common' THEN 0", "ELSE 4", "CASE WHEN CASE lower(COALESCE(cp.rarity, ''))"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sqlText := cardSearchOrderBySQL(tt.sortMode, tt.dir, true, true)
			for _, snippet := range tt.snippets {
				if !strings.Contains(sqlText, snippet) {
					t.Fatalf("cardSearchOrderBySQL(%q) missing SQL snippet %q in %q", tt.sortMode, snippet, sqlText)
				}
			}
		})
	}
}

func TestCardSearchOrderBySQLSupportsDescendingAlternates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sortMode string
		snippets []string
	}{
		{
			name:     "alphabetical desc",
			sortMode: "alphabetical",
			snippets: []string{"lower(oc.name) DESC", "oc.oracle_id DESC"},
		},
		{
			name:     "mana value desc",
			sortMode: "mana_value",
			snippets: []string{"COALESCE(oc.cmc, 0) DESC", "lower(oc.name) ASC"},
		},
		{
			name:     "oldest printing desc",
			sortMode: "oldest_printing",
			snippets: []string{"DESC NULLS LAST", "lower(oc.name) ASC"},
		},
		{
			name:     "rarity desc",
			sortMode: "rarity",
			snippets: []string{"THEN 1 ELSE 0 END ASC", "ELSE 4", "END DESC", "lower(COALESCE(cp.rarity, '')) DESC"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sqlText := cardSearchOrderBySQL(tt.sortMode, "desc", true, true)
			for _, snippet := range tt.snippets {
				if !strings.Contains(sqlText, snippet) {
					t.Fatalf("cardSearchOrderBySQL(%q, desc) missing SQL snippet %q in %q", tt.sortMode, snippet, sqlText)
				}
			}
		})
	}
}

func TestCardSearchOrderBySQLRelevanceAscendingReversesRanking(t *testing.T) {
	t.Parallel()

	sqlText := cardSearchOrderBySQL("relevance", "asc", true, true)

	for _, snippet := range []string{
		"(oc.name_search = $1) ASC",
		"(oc.name_search LIKE $1 || '%') ASC",
		"similarity(oc.name_search, $1) ASC",
		"COALESCE(oc.edhrec_rank, 999999) DESC",
		"lower(oc.name) ASC",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("cardSearchOrderBySQL(relevance, asc) missing SQL snippet %q in %q", snippet, sqlText)
		}
	}
}

func arrayArgString(t *testing.T, arg any) string {
	t.Helper()

	valuer, ok := arg.(driver.Valuer)
	if !ok {
		t.Fatalf("arg %#v does not implement driver.Valuer", arg)
	}
	value, err := valuer.Value()
	if err != nil {
		t.Fatalf("valuer.Value() error = %v", err)
	}
	out, ok := value.(string)
	if !ok {
		t.Fatalf("valuer.Value() = %#v, want string", value)
	}
	return out
}
