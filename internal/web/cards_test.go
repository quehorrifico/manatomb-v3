package web

import (
	"net/http/httptest"
	"net/url"
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
