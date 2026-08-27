package web

import (
	"math/rand"
	"strings"
	"testing"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

func TestBuildPublicDeckShareMetaUsesStableDeckURL(t *testing.T) {
	meta := buildPublicDeckShareMeta("https://manatomb.example", &decks.Deck{
		Name:        "Tomb",
		Description: "A public deck.",
		PublicSlug:  "tomb",
	}, &cards.Card{
		Name:       "Sharuum",
		ImageURI:   "https://example.test/sharuum.jpg",
		ArtCropURI: "https://example.test/sharuum-art.jpg",
	})

	if meta.CanonicalURL != "https://manatomb.example/decks/public/tomb" {
		t.Fatalf("canonical URL = %q", meta.CanonicalURL)
	}
	if meta.Title != "Tomb" || meta.Description != "A public deck." {
		t.Fatalf("unexpected public deck metadata: %#v", meta)
	}
	if meta.ImageURL != "https://example.test/sharuum-art.jpg" || meta.Type != "article" {
		t.Fatalf("unexpected public deck preview metadata: %#v", meta)
	}
}

func TestBuildPublicDeckShareMetaUsesTrustedOriginAndUsefulFallback(t *testing.T) {
	meta := buildPublicDeckShareMeta("https://manatomb.app", &decks.Deck{
		Name:          "Cats in Hats",
		Format:        "Commander",
		CommanderName: "Arahbo, Roar of the World",
		PublicSlug:    "cats",
	}, nil)

	if meta.CanonicalURL != "https://manatomb.app/decks/public/cats" {
		t.Fatalf("canonical URL = %q, want trusted public origin", meta.CanonicalURL)
	}
	if !strings.Contains(meta.Description, "Commander deck led by Arahbo") {
		t.Fatalf("fallback description = %q", meta.Description)
	}
	if meta.Type != "article" {
		t.Fatalf("deck share type = %q, want article", meta.Type)
	}
}

func TestBuildPublicDeckCostUsesPrintingSpecificBoardPrices(t *testing.T) {
	got := buildPublicDeckCost(
		[]decks.DeckCard{
			{CardName: "Sol Ring", PriceUSD: "$1.25", Quantity: 2, PreferredPrintID: "selected-sol-ring"},
			{CardName: "Arcane Signet", PriceUSD: "2.00", Quantity: 1, PreferredPrintID: "selected-signet"},
		},
		[]decks.DeckCard{
			{CardName: "Negate", PriceUSD: "0.50", Quantity: 3, PreferredPrintID: "selected-negate"},
		},
		"Sharuum the Hegemon",
		&cards.Card{Name: "Sharuum the Hegemon", PriceUSD: "10.00"},
		true,
	)

	if got.Display != "$16.00" {
		t.Fatalf("buildPublicDeckCost().Display = %q, want $16.00", got.Display)
	}
	if got.Note != "" {
		t.Fatalf("buildPublicDeckCost().Note = %q, want empty", got.Note)
	}
}

func TestBuildPublicDeckCostUsesDurableCommanderPrintingOverLegacyRow(t *testing.T) {
	got := buildPublicDeckCost(
		[]decks.DeckCard{
			{CardName: "SHARUUM THE HEGEMON", PriceUSD: "3.00", Quantity: 2, PreferredPrintID: "selected-commander"},
			{CardName: "Island", PriceUSD: "1.00", Quantity: 1},
		},
		nil,
		"Sharuum the Hegemon",
		&cards.Card{Name: "Sharuum the Hegemon", PriceUSD: "100.00"},
		true,
	)

	if got.Display != "$104.00" {
		t.Fatalf("buildPublicDeckCost().Display = %q, want $104.00", got.Display)
	}
}

func TestBuildPublicDeckCostMarksMissingPrices(t *testing.T) {
	got := buildPublicDeckCost(
		[]decks.DeckCard{
			{CardName: "Known Card", PriceUSD: "2.00", Quantity: 1},
			{CardName: "Unknown Card", Quantity: 2},
		},
		nil,
		"Missing Commander",
		nil,
		true,
	)

	if got.Display != "$2.00+" {
		t.Fatalf("buildPublicDeckCost().Display = %q, want $2.00+", got.Display)
	}
	if got.Note != "Minimum known cost; 3 cards have no USD price." {
		t.Fatalf("buildPublicDeckCost().Note = %q", got.Note)
	}

	unavailable := buildPublicDeckCost(
		[]decks.DeckCard{{CardName: "Unknown Card", Quantity: 2}},
		nil,
		"",
		nil,
		false,
	)
	if unavailable.Display != "—" || unavailable.Note != "Price unavailable for 2 cards." {
		t.Fatalf("unavailable price summary = %#v", unavailable)
	}
}

func TestPublicDeckPriceParsingAndFormatting(t *testing.T) {
	for _, tt := range []struct {
		raw   string
		cents int64
		ok    bool
	}{
		{raw: "$1,234.56", cents: 123456, ok: true},
		{raw: "0", cents: 0, ok: true},
		{raw: "1.999", cents: 200, ok: true},
		{raw: "", ok: false},
		{raw: "unknown", ok: false},
		{raw: "-1.00", ok: false},
	} {
		got, ok := parsePublicDeckPriceCents(tt.raw)
		if got != tt.cents || ok != tt.ok {
			t.Fatalf("parsePublicDeckPriceCents(%q) = (%d, %t), want (%d, %t)", tt.raw, got, ok, tt.cents, tt.ok)
		}
	}
	if got := formatPublicDeckCost(123456); got != "$1,234.56" {
		t.Fatalf("formatPublicDeckCost(123456) = %q", got)
	}
}

func TestBuildPublicDeckSampleHandUsesWeightedSelectedPrintings(t *testing.T) {
	mainboard := []decks.DeckCard{
		{CardName: "Commander Card", CardID: "commander", PrintID: "commander-print", ImageURI: "commander.jpg", Quantity: 1},
		{CardName: "Island", CardID: "island", PrintID: "selected-island", ImageURI: "island.jpg", Quantity: 4},
		{CardName: "One", CardID: "one", PrintID: "print-one", ImageURI: "one.jpg", Quantity: 1},
		{CardName: "Two", CardID: "two", PrintID: "print-two", ImageURI: "two.jpg", Quantity: 1},
		{CardName: "Three", CardID: "three", PrintID: "print-three", ImageURI: "three.jpg", Quantity: 1},
		{CardName: "Four", CardID: "four", PrintID: "print-four", ImageURI: "four.jpg", Quantity: 1},
		{CardName: "Five", CardID: "five", PrintID: "print-five", ImageURI: "five.jpg", Quantity: 1},
	}

	hand := buildPublicDeckSampleHand(
		mainboard,
		"Commander Card",
		true,
		7,
		rand.New(rand.NewSource(42)),
	)
	if len(hand) != 7 {
		t.Fatalf("sample hand length = %d, want 7", len(hand))
	}
	islands := 0
	for _, card := range hand {
		if strings.EqualFold(card.CardName, "Commander Card") {
			t.Fatal("sample hand included the commander")
		}
		if card.Quantity != 1 {
			t.Fatalf("sample card %q quantity = %d, want 1", card.CardName, card.Quantity)
		}
		if card.PrintID == "" || card.ImageURI == "" {
			t.Fatalf("sample card lost selected printing metadata: %#v", card)
		}
		if card.CardName == "Island" {
			islands++
			if card.PrintID != "selected-island" {
				t.Fatalf("sample Island print = %q, want selected-island", card.PrintID)
			}
		}
	}
	if islands > 4 {
		t.Fatalf("sample hand contains %d Islands, deck has 4", islands)
	}
}

func TestBuildPublicDeckSampleHandHandlesShortAndNonCommanderDecks(t *testing.T) {
	mainboard := []decks.DeckCard{{
		CardName: "Named Commander",
		CardID:   "same-card",
		PrintID:  "selected-print",
		ImageURI: "selected.jpg",
		Quantity: 2,
	}}

	hand := buildPublicDeckSampleHand(mainboard, "Named Commander", false, 7, rand.New(rand.NewSource(1)))
	if len(hand) != 2 {
		t.Fatalf("non-Commander sample hand length = %d, want 2", len(hand))
	}
	for _, card := range hand {
		if card.PrintID != "selected-print" || card.Quantity != 1 {
			t.Fatalf("short sample hand lost printing or copy semantics: %#v", card)
		}
	}
}
