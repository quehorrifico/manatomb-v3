package web

import (
	"testing"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

func TestBuildWorkspaceStateKeepsExactCommanderPrinting(t *testing.T) {
	const (
		oracleID = "11111111-1111-4111-8111-111111111111"
		printID  = "22222222-2222-4222-8222-222222222222"
	)
	state := buildWorkspaceStateFromDeck(
		&decks.Deck{
			ID:               7,
			Name:             "Exact Commander",
			Format:           "Commander",
			CommanderName:    "Sharuum the Hegemon",
			CommanderPrintID: printID,
		},
		nil,
		nil,
		nil,
		nil,
		&cards.Card{
			ID:              printID,
			OracleID:        oracleID,
			Name:            "Sharuum the Hegemon",
			ImageURI:        "https://example.test/exact.jpg",
			PriceUSD:        "17.50",
			SetCode:         "arb",
			SetName:         "Alara Reborn",
			CollectorNumber: "26",
			Rarity:          "rare",
			ReleasedAt:      "2009-04-30",
			Artist:          "Izzy",
		},
	)

	if state.CommanderPrintID != printID {
		t.Fatalf("workspace commander print = %q, want %q", state.CommanderPrintID, printID)
	}
	meta, ok := state.CardMeta["Sharuum the Hegemon"]
	if !ok {
		t.Fatal("workspace commander metadata is missing")
	}
	if meta.CardID != oracleID || meta.PreferredPrintID != printID || meta.PrintID != printID {
		t.Fatalf("workspace commander identifiers = %#v", meta)
	}
	if meta.ImageURI != "https://example.test/exact.jpg" ||
		meta.PriceUSD != "17.50" ||
		meta.SetCode != "arb" ||
		meta.CollectorNumber != "26" {
		t.Fatalf("workspace lost exact commander printing metadata: %#v", meta)
	}
}
