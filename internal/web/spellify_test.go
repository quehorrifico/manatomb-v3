package web

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"manatomb/app/internal/cards"
)

func TestParseSpellifyGameID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want int64
		ok   bool
	}{
		{raw: "42", want: 42, ok: true},
		{raw: " 7 ", want: 7, ok: true},
		{raw: "", ok: false},
		{raw: "0", ok: false},
		{raw: "-3", ok: false},
		{raw: "another-round", ok: false},
	}

	for _, tt := range tests {
		got, ok := parseSpellifyGameID(tt.raw)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("parseSpellifyGameID(%q) = (%d, %t), want (%d, %t)", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSpellifyRoundPathScopesRenderedGame(t *testing.T) {
	t.Parallel()

	if got := spellifyRoundPath(42); got != "/games/spellify?game_id=42" {
		t.Fatalf("spellifyRoundPath(42) = %q", got)
	}
}

func TestSpellifyOwnerLockKeySeparatesUsersAndGuests(t *testing.T) {
	t.Parallel()

	if got := spellifyOwnerLockKey(gamePlayer{UserID: 42}); got != "spellify:user:42" {
		t.Fatalf("user lock key = %q", got)
	}
	if got := spellifyOwnerLockKey(gamePlayer{GuestID: "ABC"}); got != "spellify:guest:abc" {
		t.Fatalf("guest lock key = %q", got)
	}
}

func TestLoadSpellifyGameForMutationRequiresRoundID(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "not-a-round", "0"} {
		_, err := loadSpellifyGameForMutation(t.Context(), nil, gamePlayer{}, raw)
		if !errors.Is(err, errSpellifyRoundStale) {
			t.Fatalf("loadSpellifyGameForMutation(%q) error = %v, want stale round", raw, err)
		}
	}
}

func TestSpellifyNormalizeGuessChar(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"G":       "g",
		"{G}":     "{G}",
		" {w/p} ": "{W/P}",
		"  2  ":   "2",
		"GG":      "",
		"{W/U}":   "",
		"{P}":     "",
		"/":       "",
		"":        "",
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

	got := spellifyMaskText("Sol Ring {T}: Add {C}.", []string{"s", "g", "t", "{T}"})
	want := "S__ ___g {T}: ___ {_}."
	if got != want {
		t.Fatalf("spellifyMaskText() = %q, want %q", got, want)
	}
}

func TestSpellifyMaskTextTreatsSymbolsAtomically(t *testing.T) {
	t.Parallel()

	value := "{2}{W/P}, then {W} or {CHAOS}."
	if got, want := spellifyMaskText(value, []string{"w", "p", "2"}), "{_}{_}, ____ {_} __ {_}."; got != want {
		t.Fatalf("letter guesses exposed brace symbols: got %q, want %q", got, want)
	}
	if got, want := spellifyMaskText(value, []string{"{2}", "{W/P}"}), "{2}{W/P}, ____ {_} __ {_}."; got != want {
		t.Fatalf("symbol guesses did not reveal exact tokens: got %q, want %q", got, want)
	}
}

func TestSpellifyNormalizedGuessedCharsPreservesLegacySessionOrder(t *testing.T) {
	t.Parallel()

	got := spellifyNormalizedGuessedChars([]string{"R", "{G}", "r", "{g}", "/", " 2 "})
	want := []string{"r", "{G}", "2"}
	if len(got) != len(want) {
		t.Fatalf("normalized guessed chars = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("normalized guessed chars = %#v, want %#v", got, want)
		}
	}
}

func TestSpellifyCharacterRevealFeedbackCountsMatches(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		Name:       "Sol Ring",
		OracleText: "Add green.",
		FlavorText: "A golden ring.",
	}
	if got := spellifyCharacterRevealCount(card, "g"); got != 4 {
		t.Fatalf("g reveal count = %d, want 4", got)
	}
	if got := spellifyRevealMessage("g", 4); got != "G revealed 4 characters." {
		t.Fatalf("match feedback = %q", got)
	}
	if got := spellifyRevealMessage("z", 0); got != "Z is not in the card." {
		t.Fatalf("miss feedback = %q", got)
	}
	if got := spellifyRevealMessage("r", 1); got != "R revealed 1 character." {
		t.Fatalf("single-match feedback = %q", got)
	}
}

func TestSpellifyCharacterRevealCountSeparatesTextAndSymbols(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		Name:       "Talisman",
		ManaCost:   "{2}{W/P}",
		OracleText: "{T}: Add {C}{C}. The value is 2.",
		FlavorText: "Tap into tomorrow.",
	}
	if got := spellifyCharacterRevealCount(card, "t"); got != 5 {
		t.Fatalf("plain t count = %d, want 5 without counting {T}", got)
	}
	if got := spellifyCharacterRevealCount(card, "2"); got != 1 {
		t.Fatalf("plain 2 count = %d, want only the rules-text digit", got)
	}
	if got := spellifyCharacterRevealCount(card, "{2}"); got != 1 {
		t.Fatalf("generic symbol count = %d, want 1", got)
	}
	if got := spellifyCharacterRevealCount(card, "{W/P}"); got != 1 {
		t.Fatalf("Phyrexian symbol count = %d, want 1", got)
	}
	if got := spellifyCharacterRevealCount(card, "{C}"); got != 2 {
		t.Fatalf("colorless symbol count = %d, want 2", got)
	}
	if got := spellifyCharacterRevealCount(card, "{T}"); got != 1 {
		t.Fatalf("tap symbol count = %d, want 1", got)
	}
}

func TestBuildSpellifyPageDataRevealsCompletedCard(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Sol Ring",
		ManaCost:   "{1}",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}{C}.",
		FlavorText: "The ring hums.",
		ImageURI:   "https://example.test/sol-ring.jpg",
	}
	got := buildSpellifyPageData(spellifyGame{
		ID:           7,
		Status:       "won",
		GuessCount:   3,
		GuessedChars: []string{"s"},
	}, card)
	if got.MaskedName != "Sol Ring" || got.MaskedManaCost != "{1}" || got.MaskedRulesText != "{T}: Add {C}{C}." || got.MaskedFlavorText != "The ring hums." {
		t.Fatalf("completed Spellify data masked answer as name %q mana %q rules %q flavor %q", got.MaskedName, got.MaskedManaCost, got.MaskedRulesText, got.MaskedFlavorText)
	}
	if !got.HasManaCost {
		t.Fatal("completed card mana cost was not marked available")
	}
	if got.CanGuess || got.CanRevealChar {
		t.Fatalf("completed Spellify game allowed actions")
	}
	if got.TargetName != card.Name || got.TargetImageURI != card.ImageURI || got.TargetDescription != card.TypeLine {
		t.Fatalf("completed target identity = name %q image %q description %q", got.TargetName, got.TargetImageURI, got.TargetDescription)
	}
	if got.TargetDetailPath != cardPrintingDetailPath(card.OracleID, "") {
		t.Fatalf("completed target detail path = %q", got.TargetDetailPath)
	}
}

func TestBuildSpellifyPageDataDoesNotSerializeActiveTargetIdentity(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Secret Identity",
		TypeLine:   "Legendary Artifact",
		OracleText: "Secret Identity has indestructible.",
		ImageURI:   "https://example.test/secret-identity.jpg",
	}
	got := buildSpellifyPageData(spellifyGame{
		Status:           "active",
		TargetScryfallID: "223e4567-e89b-12d3-a456-426614174000",
	}, card)
	if got.TargetName != "" || got.TargetImageURI != "" || got.TargetDetailPath != "" || got.TargetDescription != "" {
		t.Fatalf("active target identity leaked through result fields: %#v", got)
	}
	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal active Tombscript state: %v", err)
	}
	for _, secret := range []string{card.Name, card.TypeLine, card.ImageURI, card.OracleID} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("active Tombscript JSON leaked %q: %s", secret, payload)
		}
	}
}

func TestBuildSpellifyPageDataProvidesCompactRoundState(t *testing.T) {
	t.Parallel()

	got := buildSpellifyPageData(spellifyGame{
		Status:       "active",
		GuessCount:   4,
		GuessedChars: []string{"R", "{G}", "r", "/", "2"},
		IsDaily:      true,
	}, cards.Card{
		Name:       "Sol Ring",
		OracleText: "{T}: Add {C}{C}.",
	})

	if got.MaxGuesses != spellifyMaxGuesses || got.RemainingGuesses != spellifyMaxGuesses-4 {
		t.Fatalf("reveal budget = %d max, %d left", got.MaxGuesses, got.RemainingGuesses)
	}
	if got.AwardGuessesLeft != spellifyAwardGuessLimit-1-4 {
		t.Fatalf("award-safe reveals left = %d", got.AwardGuessesLeft)
	}
	if got.HasManaCost || !got.HasRulesText || got.HasFlavorText {
		t.Fatalf("clue availability = mana %t rules %t flavor %t", got.HasManaCost, got.HasRulesText, got.HasFlavorText)
	}
	if got.MaskedManaCost != "" {
		t.Fatalf("card without a mana cost produced masked cost %q", got.MaskedManaCost)
	}
	if got.LastGuessedChar != "2" || got.LastRevealCount != 0 {
		t.Fatalf("latest reveal = %q with %d matches", got.LastGuessedChar, got.LastRevealCount)
	}
	wantChars := []string{"r", "{G}", "2"}
	if len(got.GuessedChars) != len(wantChars) {
		t.Fatalf("guessed chars = %#v, want %#v", got.GuessedChars, wantChars)
	}
	for index := range wantChars {
		if got.GuessedChars[index] != wantChars[index] {
			t.Fatalf("guessed chars = %#v, want %#v", got.GuessedChars, wantChars)
		}
	}
}

func TestBuildSpellifyPageDataProvidesSafeSymbolKeyboardState(t *testing.T) {
	t.Parallel()

	got := buildSpellifyPageData(spellifyGame{
		Status:       "active",
		GuessCount:   2,
		GuessedChars: []string{"{T}", "{W/P}"},
	}, cards.Card{
		Name:       "Sample Relic",
		ManaCost:   "{2}{W/P}",
		OracleText: "{T}: Add {C}.",
	})

	if !got.HasManaCost || got.MaskedManaCost != "{_}{W/P}" {
		t.Fatalf("masked mana cost = %q (available %t), want %q", got.MaskedManaCost, got.HasManaCost, "{_}{W/P}")
	}
	if got.MaskedRulesText != "{T}: ___ {_}." {
		t.Fatalf("masked rules text = %q", got.MaskedRulesText)
	}
	if len(got.SymbolKeys) != len(spellifySymbolKeyDefinitions) {
		t.Fatalf("symbol key count = %d, want fixed %d", len(got.SymbolKeys), len(spellifySymbolKeyDefinitions))
	}

	keys := make(map[string]spellifySymbolKey, len(got.SymbolKeys))
	for _, key := range got.SymbolKeys {
		keys[key.Value] = key
	}
	for value, assetName := range map[string]string{
		"{T}":   "T",
		"{W/P}": "WP",
	} {
		key := keys[value]
		if !key.Guessed || !key.Hit || key.AssetName != assetName {
			t.Fatalf("guessed symbol key %s = %#v", value, key)
		}
	}
	for _, value := range []string{"{2}", "{C}"} {
		key := keys[value]
		if key.Guessed || key.Hit {
			t.Fatalf("unguessed target symbol leaked through key %s: %#v", value, key)
		}
	}
	if key := keys["{P}"]; key.Value != "" {
		t.Fatalf("standalone P pawprint must not be exposed as a Phyrexian key: %#v", key)
	}
	if got.LastGuessedChar != "{W/P}" || got.LastRevealCount != 1 {
		t.Fatalf("latest symbol reveal = %q with %d matches", got.LastGuessedChar, got.LastRevealCount)
	}
}

func TestSpellifyTargetManaCostCombinesCardFaces(t *testing.T) {
	t.Parallel()

	card := cards.Card{Faces: []cards.CardFace{
		{ManaCost: "{1}{U}"},
		{ManaCost: "{2}{B}"},
	}}
	if got, want := spellifyTargetManaCost(card), "{1}{U} // {2}{B}"; got != want {
		t.Fatalf("combined face mana cost = %q, want %q", got, want)
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
	eligible := buildSpellifyPageData(spellifyGame{Status: "active", GuessCount: 5, IsDaily: true}, card)
	if eligible.AwardStatus != "Eligible" || eligible.GameModeLabel != "Daily Tombscript" {
		t.Fatalf("eligible daily data = status %q mode %q", eligible.AwardStatus, eligible.GameModeLabel)
	}
	solveNow := buildSpellifyPageData(spellifyGame{Status: "active", GuessCount: 6, IsDaily: true}, card)
	if solveNow.AwardStatus != "Solve now" || solveNow.AwardGuessesLeft != 0 {
		t.Fatalf("final award-safe state = status %q, safe reveals %d", solveNow.AwardStatus, solveNow.AwardGuessesLeft)
	}
	closed := buildSpellifyPageData(spellifyGame{Status: "active", GuessCount: 7, IsDaily: true}, card)
	if closed.AwardStatus != "Award closed" || closed.AwardGuessesLeft != 0 {
		t.Fatalf("daily after cutoff = status %q, safe reveals %d", closed.AwardStatus, closed.AwardGuessesLeft)
	}
	won := buildSpellifyPageData(spellifyGame{Status: "won", GuessCount: 6, IsDaily: true}, card)
	if won.AwardStatus != "Earned" {
		t.Fatalf("daily win status = %q, want Earned", won.AwardStatus)
	}
	replay := buildSpellifyPageData(spellifyGame{Status: "active", GuessCount: 0, IsDaily: false}, card)
	if replay.AwardStatus != "Practice" || replay.GameModeLabel != "Practice Tombscript" {
		t.Fatalf("replay data = status %q mode %q", replay.AwardStatus, replay.GameModeLabel)
	}
	guestReplay := buildSpellifyPageData(spellifyGame{Status: "active", GuestID: "guest", IsDaily: false}, card)
	if guestReplay.AwardStatus != "Practice" {
		t.Fatalf("guest practice status = %q, want Practice", guestReplay.AwardStatus)
	}
	lost := buildSpellifyPageData(spellifyGame{Status: "lost", GuessCount: 2, IsDaily: true}, card)
	if lost.AwardStatus != "Not earned" {
		t.Fatalf("completed daily loss status = %q, want Not earned", lost.AwardStatus)
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
