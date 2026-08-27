package web

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

func TestNormalizeWorkbenchDraftSeedGeneratesMissingName(t *testing.T) {
	got := normalizeWorkbenchDraftSeed(workbenchPlaytestPayload{Format: "Sandbox", Sandbox: true})

	if !regexp.MustCompile(`^[A-Za-z0-9_]{6,32}$`).MatchString(got.Name) {
		t.Fatalf("expected a generated gamertag deck name, got %q", got.Name)
	}
}

func TestNormalizeWorkbenchPlaytestCardsAppliesMetadata(t *testing.T) {
	got := normalizeWorkbenchPlaytestCards(
		[]workbenchPlaytestCard{
			{Name: "sol ring", Qty: 1},
			{Name: "Sol Ring", Qty: 1},
			{Name: "Island", Qty: 2},
		},
		map[string]workbenchPlaytestCardMeta{
			"sol ring": {
				Name:            "Sol Ring",
				ManaCost:        "{1}",
				TypeLine:        "Artifact",
				OracleText:      "{T}: Add {C}{C}.",
				FlavorText:      "The ring hums.",
				CMC:             1,
				Colors:          playtestColorList{},
				ColorIdentity:   playtestColorList{"C"},
				PriceUSD:        "1.25",
				SetCode:         "LTC",
				SetName:         "The Lost Caverns of Ixalan Commander",
				CollectorNumber: "400",
				Artist:          "Mark Tedin",
				ImageURI:        "https://example.test/sol-ring.jpg",
			},
		},
	)

	if len(got) != 2 {
		t.Fatalf("expected 2 normalized cards, got %d", len(got))
	}
	if got[0].Name != "Sol Ring" {
		t.Fatalf("expected canonical metadata name, got %q", got[0].Name)
	}
	if got[0].Qty != 2 {
		t.Fatalf("expected merged Sol Ring quantity 2, got %d", got[0].Qty)
	}
	if got[0].ManaCost != "{1}" {
		t.Fatalf("expected metadata mana cost, got %q", got[0].ManaCost)
	}
	if got[0].ImageURI == "" || got[0].TypeLine == "" || got[0].OracleText == "" {
		t.Fatalf("expected metadata fields to be preserved, got %#v", got[0])
	}
	if got[0].ManaValue != 1 || got[0].PriceUSD != "1.25" || got[0].Artist != "Mark Tedin" {
		t.Fatalf("expected detail metadata to be preserved, got %#v", got[0])
	}
	if len(got[0].ColorIdentity) != 1 || got[0].ColorIdentity[0] != "C" {
		t.Fatalf("expected color identity metadata to be preserved, got %#v", got[0].ColorIdentity)
	}
	if got[0].SetCode != "LTC" || got[0].SetName == "" || got[0].CollectorNumber != "400" {
		t.Fatalf("expected printing metadata to be preserved, got %#v", got[0])
	}
	if got[1].Name != "Island" || got[1].Qty != 2 {
		t.Fatalf("expected Island row to be preserved, got %#v", got[1])
	}
}

func TestDeckRowsToPlaytestCardsPreservesManaCost(t *testing.T) {
	got := deckRowsToPlaytestCards([]decks.DeckCard{{
		CardName:        "Arcane Signet",
		ManaCost:        "{2}",
		ImageURI:        "https://example.test/arcane-signet.jpg",
		TypeLine:        "Artifact",
		OracleText:      "{T}: Add one mana of any color in your commander's color identity.",
		FlavorText:      "Every leader needs a symbol.",
		CMC:             2,
		PriceUSD:        "0.50",
		Colors:          "",
		ColorIdentity:   "W,U,B,R,G",
		SetCode:         "CMM",
		SetName:         "Commander Masters",
		CollectorNumber: "700",
		Artist:          "Dan Scott",
		Quantity:        1,
	}})

	if len(got) != 1 {
		t.Fatalf("expected 1 playtest card, got %d", len(got))
	}
	if got[0].ManaCost != "{2}" {
		t.Fatalf("expected mana cost to be preserved, got %q", got[0].ManaCost)
	}
	if got[0].ManaValue != 2 || got[0].PriceUSD != "0.50" || got[0].Artist != "Dan Scott" {
		t.Fatalf("expected card detail metadata to be preserved, got %#v", got[0])
	}
	if len(got[0].ColorIdentity) != 5 {
		t.Fatalf("expected color identity to be split, got %#v", got[0].ColorIdentity)
	}
	if got[0].FlavorText == "" || got[0].SetCode != "CMM" || got[0].CollectorNumber != "700" {
		t.Fatalf("expected flavor and print metadata to be preserved, got %#v", got[0])
	}
}

func TestNormalizeWorkbenchDraftSeedCommander(t *testing.T) {
	const commanderPrintID = "223e4567-e89b-12d3-a456-426614174000"
	in := workbenchPlaytestPayload{
		CommanderName:    "Atraxa, Grand Unifier",
		CommanderPrintID: commanderPrintID,
		Format:           "Commander",
		Name:             "Ignored",
		Description:      "Test deck",
		Tags:             "Ramp, Midrange",
		Cards: []workbenchPlaytestCard{
			{Name: "Atraxa, Grand Unifier", Qty: 1},
			{Name: "Sol Ring", Qty: 1},
			{Name: "Island", Qty: 3},
		},
		MaybeCards: []workbenchPlaytestCard{
			{Name: "Arcane Signet", Qty: 1},
		},
		CommanderCandidates: []string{"Atraxa, Grand Unifier", "The Goose Mother"},
		CardMeta: map[string]workbenchPlaytestCardMeta{
			"Atraxa, Grand Unifier": {
				Name:                 "Atraxa, Grand Unifier",
				ManaCost:             "{3}{G}{W}{U}",
				IsCommanderCandidate: true,
			},
		},
	}

	got := normalizeWorkbenchDraftSeed(in)

	if got.Name != "Ignored" {
		t.Fatalf("expected workbench deck name to be preserved, got %q", got.Name)
	}
	if got.Format != "Commander" {
		t.Fatalf("expected Commander format, got %q", got.Format)
	}
	if got.CommanderName != "Atraxa, Grand Unifier" {
		t.Fatalf("expected commander preserved, got %q", got.CommanderName)
	}
	if got.CommanderPrintID != commanderPrintID {
		t.Fatalf("expected commander printing preserved, got %q", got.CommanderPrintID)
	}
	if got.Cards["Island"] != 3 {
		t.Fatalf("expected Island count 3, got %d", got.Cards["Island"])
	}
	if got.MaybeCards["Arcane Signet"] != 1 {
		t.Fatalf("expected Arcane Signet maybe count 1, got %d", got.MaybeCards["Arcane Signet"])
	}
	if len(got.CommanderCandidates) != 2 {
		t.Fatalf("expected commander candidates to be preserved, got %v", got.CommanderCandidates)
	}
	if !got.CardMeta["Atraxa, Grand Unifier"].IsCommanderCandidate {
		t.Fatalf("expected commander metadata to be preserved")
	}
	if got.CardMeta["Atraxa, Grand Unifier"].ManaCost != "{3}{G}{W}{U}" {
		t.Fatalf("expected mana cost to be preserved, got %q", got.CardMeta["Atraxa, Grand Unifier"].ManaCost)
	}
}

func TestWorkbenchPlaytestPreservesPerBoardPrintingMetadata(t *testing.T) {
	const (
		mainPrint  = "11111111-1111-1111-1111-111111111111"
		sidePrint  = "22222222-2222-2222-2222-222222222222"
		maybePrint = "33333333-3333-3333-3333-333333333333"
	)

	raw := []byte(`{
		"format":"Sandbox",
		"sandbox":true,
		"cards":[{"name":"Sol Ring","qty":1}],
		"sideboard_cards":[{"name":"Sol Ring","qty":1}],
		"maybe_cards":[{"name":"Arcane Signet","qty":1}],
		"board_card_meta":{
			"main":{"Sol Ring":{"name":"Sol Ring","preferredPrintID":"` + mainPrint + `","setCode":"CMM"}},
			"side":{"Sol Ring":{"name":"Sol Ring","preferredPrintID":"` + sidePrint + `","setCode":"LTC"}},
			"maybe":{"Arcane Signet":{"name":"Arcane Signet","preferredPrintID":"` + maybePrint + `","setCode":"MKC"}}
		}
	}`)

	var payload workbenchPlaytestPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode browser playtest payload: %v", err)
	}

	seed := normalizeWorkbenchDraftSeed(payload)
	if got := seed.BoardCardMeta["main"]["Sol Ring"].PreferredPrintID; got != mainPrint {
		t.Fatalf("mainboard printing changed during normalization: got %q", got)
	}
	if got := seed.BoardCardMeta["side"]["Sol Ring"].PreferredPrintID; got != sidePrint {
		t.Fatalf("sideboard printing changed during normalization: got %q", got)
	}
	if seed.BoardCardMeta["main"]["Sol Ring"].PreferredPrintID == seed.BoardCardMeta["side"]["Sol Ring"].PreferredPrintID {
		t.Fatal("same card on different boards collapsed to one printing")
	}
	if got := seed.BoardCardMeta["maybe"]["Arcane Signet"].PreferredPrintID; got != maybePrint {
		t.Fatalf("maybeboard printing changed during normalization: got %q", got)
	}

	encoded, err := json.Marshal(seed)
	if err != nil {
		t.Fatalf("encode playtest workbench seed: %v", err)
	}
	for _, needle := range []string{
		`"boardCardMeta"`,
		`"` + mainPrint + `"`,
		`"` + sidePrint + `"`,
		`"` + maybePrint + `"`,
	} {
		if !strings.Contains(string(encoded), needle) {
			t.Fatalf("rendered workbench seed is missing %s: %s", needle, encoded)
		}
	}
}

func TestNormalizeWorkbenchDraftSeedCommanderFormatVariantsKeepCommander(t *testing.T) {
	t.Parallel()

	in := workbenchPlaytestPayload{
		CommanderName:    "Atraxa, Grand Unifier",
		CommanderPrintID: "223e4567-e89b-12d3-a456-426614174000",
		Format:           "Historic Brawl",
		Name:             "Atraxa Brawl",
	}

	got := normalizeWorkbenchDraftSeed(in)
	if got.Format != "Historic Brawl" {
		t.Fatalf("expected Historic Brawl format, got %q", got.Format)
	}
	if got.CommanderName != in.CommanderName || got.CommanderPrintID != in.CommanderPrintID {
		t.Fatalf("expected commander and printing to survive, got %#v", got)
	}
}

func TestNormalizeWorkbenchDraftSeedSandboxClearsCommander(t *testing.T) {
	in := workbenchPlaytestPayload{
		CommanderName:    "Atraxa, Grand Unifier",
		CommanderPrintID: "223e4567-e89b-12d3-a456-426614174000",
		Format:           "Sandbox",
		Sandbox:          true,
		Cards: []workbenchPlaytestCard{
			{Name: "Sol Ring", Qty: 1},
		},
	}

	got := normalizeWorkbenchDraftSeed(in)

	if got.Format != "Sandbox" {
		t.Fatalf("expected Sandbox format, got %q", got.Format)
	}
	if got.CommanderName != "" {
		t.Fatalf("expected commander to be cleared for sandbox, got %q", got.CommanderName)
	}
	if got.CommanderPrintID != "" {
		t.Fatalf("expected commander printing to be cleared for sandbox, got %q", got.CommanderPrintID)
	}
	if got.Sandbox != true {
		t.Fatalf("expected sandbox flag to be preserved")
	}
}

func TestPlaytestCommanderFromCardUsesExactPrinting(t *testing.T) {
	got := playtestCommanderFromCard(&cards.Card{
		Name:            "Atraxa, Grand Unifier",
		ImageURI:        "https://example.test/exact.jpg",
		PriceUSD:        "42.00",
		SetCode:         "mul",
		SetName:         "Multiverse Legends",
		CollectorNumber: "196",
		Artist:          "Marta Nael",
	})

	if got.ImageURI != "https://example.test/exact.jpg" ||
		got.PriceUSD != "42.00" ||
		got.SetCode != "mul" ||
		got.CollectorNumber != "196" {
		t.Fatalf("exact commander printing was not preserved in playtest: %#v", got)
	}
}
