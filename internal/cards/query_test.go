package cards

import (
	"database/sql/driver"
	"strings"
	"testing"
)

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
		"oc.commander_legal = TRUE",
		"oc.is_commander_candidate = TRUE",
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
