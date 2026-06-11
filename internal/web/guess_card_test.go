package web

import (
	"strings"
	"testing"
	"time"

	"manatomb/app/internal/cards"
)

func TestBuildGuessCardPageDataFiltersAskedQuestionsAndBuildsClues(t *testing.T) {
	t.Parallel()

	game := guessCardGame{
		ID:             42,
		Status:         "active",
		QuestionCount:  2,
		IsDaily:        true,
		AskedQuestions: []string{"color_w", "creature"},
	}
	card := cards.Card{
		OracleID:      "123e4567-e89b-12d3-a456-426614174000",
		Name:          "Sample Flyer",
		ManaCost:      "{2}{W}",
		TypeLine:      "Creature - Bird Horse",
		OracleText:    "Flying",
		FlavorText:    "It crossed the skyline at dawn.",
		ColorIdentity: []string{"W"},
		CMC:           3,
		Power:         "2",
		Toughness:     "3",
		SetName:       "Sample Set",
	}

	got := buildGuessCardPageData(game, card)
	if len(got.Clues) != 2 {
		t.Fatalf("buildGuessCardPageData() clues = %d, want 2", len(got.Clues))
	}
	if len(got.History) != 2 {
		t.Fatalf("buildGuessCardPageData() history = %d, want 2", len(got.History))
	}
	for _, question := range got.AvailableQuestions {
		if question.ID == "color_w" || question.ID == "creature" {
			t.Fatalf("asked question %q remained available", question.ID)
		}
	}
}

func TestAvailableGuessCardQuestionsRemovesQuestionsAnsweredByTypeClue(t *testing.T) {
	t.Parallel()

	got := availableGuessCardQuestions(nil, []guessClueView{{
		Label: "Card type",
		Value: "Creature - Otter Wizard",
	}})
	for _, question := range got {
		switch question.ID {
		case "permanent", "nonpermanent", "creature", "instant", "sorcery", "artifact", "enchantment", "planeswalker", "land", "legendary":
			t.Fatalf("type clue left redundant question %q available", question.ID)
		}
	}
}

func TestAvailableGuessCardQuestionsRemovesQuestionsAnsweredByCastCostClue(t *testing.T) {
	t.Parallel()

	got := availableGuessCardQuestions(nil, []guessClueView{{
		Label: "Cast Cost",
		Value: "{2}{G}",
	}})
	for _, question := range got {
		switch question.ID {
		case "mv_le_2", "mv_le_3", "mv_ge_5":
			t.Fatalf("cast cost clue left redundant mana value question %q available", question.ID)
		}
	}
}

func TestGuessCardCluePoolDoesNotUseEDHRankOrLayout(t *testing.T) {
	t.Parallel()

	clues := guessCardCluePool(cards.Card{
		Name:       "Sample Card",
		TypeLine:   "Instant",
		OracleText: "Draw a card.",
		EDHRecRank: 12,
		Layout:     "normal",
	})
	for _, clue := range clues {
		if clue.Label == "EDHREC rank" || clue.Label == "Layout" {
			t.Fatalf("guessCardCluePool() included unwanted clue label %q", clue.Label)
		}
	}
}

func TestGuessCardRulesTextClueRendersManaPips(t *testing.T) {
	t.Parallel()

	clues := guessCardCluePool(cards.Card{
		Name:       "Sample Card",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {G}.",
	})
	var rules guessClueView
	for _, clue := range clues {
		if clue.Label == "Rules text" {
			rules = clue
			break
		}
	}
	if rules.Value != "{T}: Add {G}." {
		t.Fatalf("rules clue value = %q, want raw rules text", rules.Value)
	}
	html := string(rules.ValueHTML)
	if !strings.Contains(html, "card-symbols/T.svg") || !strings.Contains(html, "card-symbols/G.svg") {
		t.Fatalf("rules clue html = %q, want real mana pip image URLs", html)
	}
}

func TestGuessCardCastCostClueRendersManaPips(t *testing.T) {
	t.Parallel()

	clues := guessCardCluePool(cards.Card{
		Name:     "Sample Card",
		ManaCost: "{1}{G}",
		TypeLine: "Creature",
	})
	var cost guessClueView
	for _, clue := range clues {
		if clue.Label == "Cast Cost" {
			cost = clue
			break
		}
	}
	if cost.Value != "{1}{G}" {
		t.Fatalf("cast cost clue value = %q, want raw mana cost", cost.Value)
	}
	html := string(cost.ValueHTML)
	if !strings.Contains(html, "card-symbols/1.svg") || !strings.Contains(html, "card-symbols/G.svg") {
		t.Fatalf("cast cost clue html = %q, want real mana pip image URLs", html)
	}
}

func TestGuessQuestionAnswerHandlesManaDamageAndProtection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		card       cards.Card
		questionID string
	}{
		{
			name: "generates mana",
			card: cards.Card{
				OracleText: "{T}: Add {G}.",
			},
			questionID: "generates_mana",
		},
		{
			name: "deals damage",
			card: cards.Card{
				OracleText: "Lightning Bolt deals 3 damage to any target.",
			},
			questionID: "deals_damage",
		},
		{
			name: "protects with hexproof",
			card: cards.Card{
				OracleText: "Target creature gains hexproof until end of turn.",
			},
			questionID: "protects",
		},
		{
			name: "protects by phasing",
			card: cards.Card{
				OracleText: "Target creature phases out.",
			},
			questionID: "protects",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !guessQuestionAnswer(tc.card, tc.questionID) {
				t.Fatalf("guessQuestionAnswer(%q) = false, want true", tc.questionID)
			}
		})
	}
}

func TestGuessQuestionAnswerDoesNotTreatPreventedDamageAsDealingDamage(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleText: "Prevent all combat damage that would be dealt this turn.",
	}
	if guessQuestionAnswer(card, "deals_damage") {
		t.Fatalf("guessQuestionAnswer(deals_damage) = true for prevention text, want false")
	}
}

func TestBuildGuessCardPageDataUsesFinalGuessBadgeCutoff(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Sample Card",
		TypeLine: "Instant",
	}

	active := buildGuessCardPageData(guessCardGame{Status: "active", GuessCount: 8, IsDaily: true}, card)
	if active.AwardGuessesLeft != 1 || active.AwardStatus != "Eligible" {
		t.Fatalf("active badge data = left %d status %q, want left 1 status Eligible", active.AwardGuessesLeft, active.AwardStatus)
	}

	practice := buildGuessCardPageData(guessCardGame{Status: "active", GuessCount: 9, IsDaily: true}, card)
	if practice.AwardGuessesLeft != 0 || practice.AwardStatus != "Practice" {
		t.Fatalf("practice badge data = left %d status %q, want left 0 status Practice", practice.AwardGuessesLeft, practice.AwardStatus)
	}

	won := buildGuessCardPageData(guessCardGame{Status: "won", GuessCount: 9, IsDaily: true}, card)
	if won.AwardStatus != "Earned" {
		t.Fatalf("won badge status = %q, want Earned", won.AwardStatus)
	}
}

func TestBuildGuessCardPageDataMarksReplayGamesPractice(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Sample Card",
		TypeLine: "Instant",
	}

	got := buildGuessCardPageData(guessCardGame{Status: "active", GuessCount: 0, IsDaily: false}, card)
	if got.AwardStatus != "Practice" {
		t.Fatalf("replay award status = %q, want Practice", got.AwardStatus)
	}
	if got.GameModeLabel != "Practice Game" {
		t.Fatalf("replay game mode label = %q, want Practice Game", got.GameModeLabel)
	}
}

func TestGuessCardDailyEligibilityAndExpiry(t *testing.T) {
	t.Parallel()

	today := guessCardDailyKey(time.Now().UTC())
	yesterday := guessCardDailyKey(time.Now().UTC().Add(-24 * time.Hour))

	if !guessCardGameAwardEligible(guessCardGame{IsDaily: true, DailyKey: today}) {
		t.Fatal("today's daily game should be award eligible")
	}
	if guessCardGameAwardEligible(guessCardGame{IsDaily: false, DailyKey: today}) {
		t.Fatal("practice game should not be award eligible")
	}
	if !guessCardActiveGameExpired(guessCardGame{Status: "active", IsDaily: true, DailyKey: yesterday}) {
		t.Fatal("yesterday's active game should expire")
	}
	if guessCardActiveGameExpired(guessCardGame{Status: "won", IsDaily: true, DailyKey: yesterday}) {
		t.Fatal("completed game should not be treated as an expired active game")
	}
}
