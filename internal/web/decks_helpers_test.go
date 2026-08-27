package web

import (
	"context"
	"testing"

	"manatomb/app/internal/decks"
)

func TestVisibleDeckCardCountSubtractsExactlyOneCommander(t *testing.T) {
	t.Parallel()

	cards := []decks.DeckCard{
		{CardName: "Atraxa, Grand Unifier", Quantity: 2},
		{CardName: "Sol Ring", Quantity: 1},
	}
	if got := visibleDeckCardCount(cards, "atraxa, grand unifier"); got != 2 {
		t.Fatalf("visibleDeckCardCount() = %d, want the extra commander copy plus Sol Ring", got)
	}
}

func TestDefaultDeckFormatPreservesSupportedFormats(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"Commander":      "Commander",
		"Sandbox":        "Sandbox",
		"Standard":       "Standard",
		"Modern":         "Modern",
		"Historic Brawl": "Historic Brawl",
		"Duel Commander": "Duel Commander",
		"Oathbreaker":    "Oathbreaker",
		"unknown format": "Sandbox",
	}
	for input, want := range tests {
		if got := defaultDeckFormat(input, "", ""); got != want {
			t.Errorf("defaultDeckFormat(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDefaultDeckFormatFallsBackToCommanderWhenCommanderIsPresent(t *testing.T) {
	t.Parallel()

	if got := defaultDeckFormat("unknown format", "Atraxa, Grand Unifier", ""); got != "Commander" {
		t.Fatalf("defaultDeckFormat() = %q, want Commander", got)
	}
}

func TestCommanderForFormatChangeUsesFormatRegistry(t *testing.T) {
	t.Parallel()

	app := &App{}
	const commander = "Atraxa, Grand Unifier"

	got, err := app.commanderForFormatChange(
		context.Background(),
		1,
		"Commander",
		"Historic Brawl",
		commander,
		"",
	)
	if err != nil {
		t.Fatalf("commanderForFormatChange: %v", err)
	}
	if got != commander {
		t.Fatalf("commander-format transition returned %q, want %q", got, commander)
	}

	got, err = app.commanderForFormatChange(
		context.Background(),
		1,
		"Standard",
		"Oathbreaker",
		commander,
		"",
	)
	if err != nil {
		t.Fatalf("commanderForFormatChange entering commander format: %v", err)
	}
	if got != "" {
		t.Fatalf("entering a commander format kept %q, want a fresh commander selection", got)
	}
}
