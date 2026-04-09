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
	sqlText, args := buildCardSearchFilters(CardSearchParams{
		OracleText:    "draw a card",
		TypeFilters:   []CardTypeFilter{{Value: "Instant"}, {Value: "Legendary", Negated: true}},
		Colors:        []string{"U", "W"},
		ColorMode:     "at_most",
		ManaValueMin:  &minValue,
		ManaValueMax:  &maxValue,
		Rarity:        "rare",
		SetQuery:      "bro",
		ArtistQuery:   "Seb",
		CommanderOnly: true,
	}, 1)

	for _, snippet := range []string{
		"oc.is_commander_candidate = TRUE",
		"oc.oracle_text ILIKE '%' || $1 || '%'",
		"oc.type_line ILIKE '%' || $2 || '%'",
		"oc.type_line NOT ILIKE '%' || $3 || '%'",
		"oc.color_identity <@ $4::text[]",
		"oc.cmc >= $5",
		"oc.cmc <= $6",
		"lower(COALESCE(cp.rarity, '')) = $7",
		"(oc.default_set_code ILIKE '%' || $8 || '%' OR oc.default_set_name ILIKE '%' || $8 || '%')",
		"oc.default_artist ILIKE '%' || $9 || '%'",
	} {
		if !strings.Contains(sqlText, snippet) {
			t.Fatalf("buildCardSearchFilters() missing SQL snippet %q in %q", snippet, sqlText)
		}
	}

	if len(args) != 9 {
		t.Fatalf("buildCardSearchFilters() returned %d args, want 9", len(args))
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
	if args[6] != "rare" {
		t.Fatalf("args[6] = %#v, want %q", args[6], "rare")
	}
	if args[7] != "bro" {
		t.Fatalf("args[7] = %#v, want %q", args[7], "bro")
	}
	if args[8] != "Seb" {
		t.Fatalf("args[8] = %#v, want %q", args[8], "Seb")
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
