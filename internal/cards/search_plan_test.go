package cards

import (
	"context"
	"strings"
	"testing"
)

func TestBuildCardSearchSourcePlanUsesDefaultPrintingForOracleFilters(t *testing.T) {
	t.Parallel()

	plan := buildCardSearchSourcePlan(CardSearchParams{
		Query:       "lightning bolt",
		OracleText:  "damage",
		TypeFilters: []CardTypeFilter{{Value: "Instant"}},
	}, 1)

	if plan.MatchingPrintings {
		t.Fatal("oracle-only search unexpectedly selected matching printings")
	}
	for _, snippet := range []string{
		"COALESCE(oc.default_print_id::text, '') AS id",
		"LEFT JOIN card_prints cp ON cp.scryfall_id = oc.default_print_id",
	} {
		if !strings.Contains(plan.SelectSQL, snippet) {
			t.Fatalf("default search plan missing %q in %q", snippet, plan.SelectSQL)
		}
	}
	if strings.Contains(plan.SelectSQL, "JOIN LATERAL") {
		t.Fatalf("default search plan unexpectedly contains lateral printing selection: %q", plan.SelectSQL)
	}
}

func TestBuildCardSearchSourcePlanSelectsDeterministicMatchingPrinting(t *testing.T) {
	t.Parallel()

	minPrice := 1.0
	maxPrice := 25.0
	plan := buildCardSearchSourcePlan(CardSearchParams{
		TypeFilters: []CardTypeFilter{{Value: "Creature"}},
		PriceFilters: []CardPriceFilter{
			{Operator: "gte", Value: minPrice},
			{Operator: "lte", Value: maxPrice},
		},
		Rarities:    []string{"rare", "mythic"},
		SetQuery:    "bro",
		ArtistQuery: "Seb",
	}, 1)

	if !plan.MatchingPrintings {
		t.Fatal("printing-filtered search did not select matching printings")
	}
	for _, snippet := range []string{
		"cp.scryfall_id::text AS id",
		"JOIN LATERAL",
		"FROM card_prints cp_match",
		"cp_match.oracle_id = oc.oracle_id",
		matchingPrintingPriceSQL("cp_match") + " >= $1",
		matchingPrintingPriceSQL("cp_match") + " <= $2",
		"lower(COALESCE(cp_match.rarity, '')) = ANY($3::text[])",
		"cp_match.set_code ILIKE '%' || $4 || '%'",
		"cp_match.artist ILIKE '%' || $5 || '%'",
		"CASE WHEN lower(COALESCE(cp_match.lang, 'en')) = 'en' THEN 0 ELSE 1 END ASC",
		"cp_match.released_at DESC NULLS LAST",
		"cp_match.scryfall_id ASC",
	} {
		if !strings.Contains(plan.SelectSQL, snippet) {
			t.Fatalf("matching-printing search plan missing %q in %q", snippet, plan.SelectSQL)
		}
	}
	if !strings.Contains(plan.FilterSQL, "oc.type_line ~* $6") {
		t.Fatalf("oracle filter placeholders did not follow printing filters: %q", plan.FilterSQL)
	}
	if len(plan.Args) != 6 {
		t.Fatalf("matching-printing search plan args = %d, want 6: %#v", len(plan.Args), plan.Args)
	}
	if plan.Args[0] != minPrice || plan.Args[1] != maxPrice {
		t.Fatalf("matching-printing price args = %#v, want [%v %v]", plan.Args[:2], minPrice, maxPrice)
	}
	if plan.Args[3] != "bro" || plan.Args[4] != "Seb" {
		t.Fatalf("matching-printing set/artist args = %#v, want [bro Seb]", plan.Args[3:5])
	}
}

func TestPrintingSpecificCardSearchDetectionIgnoresInvalidEmptyValues(t *testing.T) {
	t.Parallel()

	if hasPrintingSpecificCardSearchFilters(CardSearchParams{
		Rarity:      "special",
		Rarities:    []string{"", "bonus"},
		SetQuery:    " ",
		ArtistQuery: "\t",
	}) {
		t.Fatal("invalid empty printing filters unexpectedly activated matching-printing mode")
	}

	price := 0.0
	for _, params := range []CardSearchParams{
		{PriceValue: &price},
		{Rarity: "rare"},
		{SetQuery: "lea"},
		{ArtistQuery: "Rebecca Guay"},
	} {
		if !hasPrintingSpecificCardSearchFilters(params) {
			t.Fatalf("printing-specific filters were not detected: %#v", params)
		}
	}
}

func TestNormalizeCardSearchPageBoundsAndMetadata(t *testing.T) {
	t.Parallel()

	window := normalizeCardSearchPage(3, 24, 70)
	if window.Page != 3 || window.PageSize != 24 || window.Offset != 48 || window.TotalPages != 3 || window.Total != 70 {
		t.Fatalf("normalizeCardSearchPage(3, 24, 70) = %#v", window)
	}

	clamped := normalizeCardSearchPage(99, 24, 70)
	if clamped.Page != 3 || clamped.Offset != 48 {
		t.Fatalf("normalizeCardSearchPage() did not clamp past the last page: %#v", clamped)
	}

	empty := normalizeCardSearchPage(5, 24, 0)
	if empty.Page != 1 || empty.Offset != 0 || empty.TotalPages != 0 {
		t.Fatalf("normalizeCardSearchPage(empty) = %#v", empty)
	}
}

func TestSearchCardsWithOutcomeExposesPaginationMetadata(t *testing.T) {
	db := openCardSearchOutcomeTestDB(t, false, true)

	outcome, err := SearchCardsWithOutcome(context.Background(), db, CardSearchParams{
		Layout: "normal",
		Limit:  24,
		Page:   7,
	})
	if err != nil {
		t.Fatalf("SearchCardsWithOutcome() error = %v", err)
	}
	if outcome.Total != 1 || outcome.Page != 1 || outcome.PageSize != 24 || outcome.TotalPages != 1 {
		t.Fatalf("SearchCardsWithOutcome() pagination = total %d, page %d, size %d, pages %d; want 1, 1, 24, 1",
			outcome.Total,
			outcome.Page,
			outcome.PageSize,
			outcome.TotalPages,
		)
	}
}

func TestSearchCardsWithOutcomeReportsMatchingPrintingSelection(t *testing.T) {
	db := openCardSearchOutcomeTestDB(t, false, true)

	outcome, err := SearchCardsWithOutcome(context.Background(), db, CardSearchParams{
		SetQuery: "tst",
		Limit:    24,
	})
	if err != nil {
		t.Fatalf("SearchCardsWithOutcome() error = %v", err)
	}
	if !outcome.MatchingPrintings {
		t.Fatal("SearchCardsWithOutcome().MatchingPrintings = false, want true")
	}
	if len(outcome.Cards) != 1 || outcome.Cards[0].ID == "" {
		t.Fatalf("SearchCardsWithOutcome().Cards = %#v, want selected matching printing", outcome.Cards)
	}
}
