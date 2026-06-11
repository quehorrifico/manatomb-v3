package web

import (
	"testing"
	"time"

	"manatomb/app/internal/cards"
)

func TestSpellifyNormalizeGuessChar(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"G":     "g",
		"{G}":   "g",
		"  2  ": "2",
		"/":     "",
		"":      "",
	}
	for input, want := range cases {
		input, want := input, want
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if got := spellifyNormalizeGuessChar(input); got != want {
				t.Fatalf("spellifyNormalizeGuessChar(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestSpellifyMaskTextRevealsOnlyGuessedCharacters(t *testing.T) {
	t.Parallel()

	got := spellifyMaskText("Sol Ring {T}: Add {C}.", []string{"s", "g", "t"})
	want := "S__ ___g {T}: ___ {_}."
	if got != want {
		t.Fatalf("spellifyMaskText() = %q, want %q", got, want)
	}
}

func TestBuildSpellifyPageDataRevealsCompletedCard(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Sol Ring",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}{C}.",
		FlavorText: "The ring hums.",
	}
	got := buildSpellifyPageData(spellifyGame{
		ID:           7,
		Status:       "won",
		GuessCount:   3,
		GuessedChars: []string{"s"},
	}, card)
	if got.MaskedName != "Sol Ring" || got.MaskedRulesText != "{T}: Add {C}{C}." || got.MaskedFlavorText != "The ring hums." {
		t.Fatalf("completed Spellify data masked answer as name %q rules %q flavor %q", got.MaskedName, got.MaskedRulesText, got.MaskedFlavorText)
	}
	if got.CanGuess || got.CanRevealChar {
		t.Fatalf("completed Spellify game allowed actions")
	}
}

func TestBuildSpellifyPageDataMasksFlavorText(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:        "123e4567-e89b-12d3-a456-426614174000",
		Name:            "Sol Ring",
		OracleText:      "{T}: Add {C}{C}.",
		FlavorText:      "The ring hums.",
		ImageURI:        "https://example.test/card.jpg",
		ReleasedAt:      "2024-01-01",
		SetName:         "Sample Set",
		SetCode:         "smp",
		CollectorNumber: "1",
	}
	got := buildSpellifyPageData(spellifyGame{
		ID:           8,
		Status:       "active",
		GuessCount:   1,
		GuessedChars: []string{"r"},
	}, card)
	if got.MaskedFlavorText != "___ r___ ____." {
		t.Fatalf("masked flavor = %q, want %q", got.MaskedFlavorText, "___ r___ ____.")
	}
	if got.RemainingGuesses != spellifyMaxGuesses-1 {
		t.Fatalf("remaining guesses = %d, want %d", got.RemainingGuesses, spellifyMaxGuesses-1)
	}
}

func TestBuildSpellifyPageDataUsesDailyAwardCutoff(t *testing.T) {
	t.Parallel()

	card := cards.Card{OracleID: "123e4567-e89b-12d3-a456-426614174000", Name: "Sol Ring"}
	eligible := buildSpellifyPageData(spellifyGame{Status: "active", GuessCount: 6, IsDaily: true}, card)
	if eligible.AwardStatus != "Eligible" || eligible.GameModeLabel != "Daily Tombscript" {
		t.Fatalf("eligible daily data = status %q mode %q", eligible.AwardStatus, eligible.GameModeLabel)
	}
	practice := buildSpellifyPageData(spellifyGame{Status: "active", GuessCount: 7, IsDaily: true}, card)
	if practice.AwardStatus != "Practice" {
		t.Fatalf("daily after cutoff status = %q, want Practice", practice.AwardStatus)
	}
	won := buildSpellifyPageData(spellifyGame{Status: "won", GuessCount: 6, IsDaily: true}, card)
	if won.AwardStatus != "Earned" {
		t.Fatalf("daily win status = %q, want Earned", won.AwardStatus)
	}
	replay := buildSpellifyPageData(spellifyGame{Status: "active", GuessCount: 0, IsDaily: false}, card)
	if replay.AwardStatus != "Practice" || replay.GameModeLabel != "Practice Tombscript" {
		t.Fatalf("replay data = status %q mode %q", replay.AwardStatus, replay.GameModeLabel)
	}
}

func TestSpellifyDailyEligibilityAndExpiry(t *testing.T) {
	t.Parallel()

	today := guessCardDailyKey(time.Now().UTC())
	yesterday := guessCardDailyKey(time.Now().UTC().Add(-24 * time.Hour))

	if !spellifyGameAwardEligible(spellifyGame{IsDaily: true, DailyKey: today}) {
		t.Fatal("today's daily Tombscript game should be award eligible")
	}
	if spellifyGameAwardEligible(spellifyGame{IsDaily: false, DailyKey: today}) {
		t.Fatal("practice Tombscript game should not be award eligible")
	}
	if !spellifyActiveGameExpired(spellifyGame{Status: "active", IsDaily: true, DailyKey: yesterday}) {
		t.Fatal("yesterday's active Tombscript game should expire")
	}
	if spellifyActiveGameExpired(spellifyGame{Status: "won", IsDaily: true, DailyKey: yesterday}) {
		t.Fatal("completed Tombscript game should not be treated as an expired active game")
	}
}
