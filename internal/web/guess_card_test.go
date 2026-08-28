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

func TestBuildGuessCardPageDataInterleavesWrongGuessesWithQuestions(t *testing.T) {
	t.Parallel()

	game := guessCardGame{
		Status:         "active",
		AskedQuestions: []string{"color_w", "creature", "flying"},
		HistoryEvents: []string{
			"question:color_w",
			"question:creature",
			"question:flying",
			"guess:Polluted Delta",
		},
	}
	card := cards.Card{
		Name:          "Sample Flyer",
		TypeLine:      "Creature — Bird",
		OracleText:    "Flying",
		ColorIdentity: []string{"W"},
	}

	got := buildGuessCardPageData(game, card)
	if len(got.History) != 4 {
		t.Fatalf("history = %#v, want four ordered events", got.History)
	}
	wrongGuess := got.History[3]
	if wrongGuess.Number != 4 || wrongGuess.Kind != "guess" || wrongGuess.Question != "Polluted Delta?" || wrongGuess.Answer != "No" || wrongGuess.Yes {
		t.Fatalf("wrong guess history = %#v, want Q4 Polluted Delta? No", wrongGuess)
	}
	if got.History[0].Kind != "question" || got.History[0].Number != 1 || got.History[0].Answer != "Yes" {
		t.Fatalf("first question history = %#v", got.History[0])
	}
}

func TestBuildGuessCardPageDataReconstructsLegacyQuestionHistory(t *testing.T) {
	t.Parallel()

	got := buildGuessCardPageData(guessCardGame{
		Status:         "active",
		AskedQuestions: []string{"color_w", "creature"},
	}, cards.Card{TypeLine: "Creature", ColorIdentity: []string{"W"}})

	if len(got.History) != 2 || got.History[0].Number != 1 || got.History[1].Number != 2 {
		t.Fatalf("legacy history = %#v, want two reconstructed questions", got.History)
	}
	for _, event := range got.History {
		if event.Kind != "question" {
			t.Fatalf("legacy event kind = %q, want question", event.Kind)
		}
	}
}

func TestGuessCardPersistedGuessNameIsBoundedAndRemovesNUL(t *testing.T) {
	t.Parallel()

	input := "  Polluted\x00 Delta  " + strings.Repeat("x", guessCardHistoryGuessMaxRunes)
	got := guessCardPersistedGuessName(input)
	if strings.ContainsRune(got, 0) {
		t.Fatalf("persisted guess retained NUL: %q", got)
	}
	if len([]rune(got)) != guessCardHistoryGuessMaxRunes {
		t.Fatalf("persisted guess length = %d, want %d", len([]rune(got)), guessCardHistoryGuessMaxRunes)
	}
	if !strings.HasPrefix(got, "Polluted Delta") {
		t.Fatalf("persisted guess = %q, want trimmed visible name", got)
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
		case "permanent", "nonpermanent", "creature", "instant", "sorcery", "artifact", "enchantment", "planeswalker", "land", "battle":
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
		case "mv_ge_5", "mv_le_5", "mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4", "mv_eq_5":
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

func TestGuessQuestionAnswerHandlesNewCatalogQuestions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		card       cards.Card
		questionID string
		want       bool
	}{
		{name: "battle type", card: cards.Card{TypeLine: "Battle — Siege"}, questionID: "battle", want: true},
		{name: "exile rules text", card: cards.Card{OracleText: "Exile target permanent."}, questionID: "exiles", want: true},
		{name: "exact mana value", card: cards.Card{CMC: 5}, questionID: "mv_eq_5", want: true},
		{name: "different exact mana value", card: cards.Card{CMC: 5}, questionID: "mv_eq_4", want: false},
		{name: "five or less boundary", card: cards.Card{CMC: 5}, questionID: "mv_le_5", want: true},
		{name: "more than five", card: cards.Card{CMC: 6}, questionID: "mv_le_5", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := guessQuestionAnswer(tt.card, tt.questionID); got != tt.want {
				t.Fatalf("guessQuestionAnswer(%q) = %t, want %t", tt.questionID, got, tt.want)
			}
		})
	}
}

func TestBuildGuessCardPageDataKeepsFirstGamePrizeEligibleWithoutGuessCutoff(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Sample Card",
		TypeLine: "Instant",
	}

	active := buildGuessCardPageData(guessCardGame{Status: "active", GuessCount: 99, IsDaily: true}, card)
	if active.AwardStatus != "Eligible" || active.GameModeLabel != "First game today" {
		t.Fatalf("first-game prize data = status %q mode %q", active.AwardStatus, active.GameModeLabel)
	}

	won := buildGuessCardPageData(guessCardGame{Status: "won", GuessCount: 99, IsDaily: true, AwardEarned: true}, card)
	if won.AwardStatus != "Earned" {
		t.Fatalf("won badge status = %q, want Earned", won.AwardStatus)
	}
	if !won.AwardEarned {
		t.Fatal("persisted award state was not exposed by the page data")
	}

	solved := buildGuessCardPageData(guessCardGame{Status: "won", GuessCount: 1, IsDaily: true}, card)
	if solved.AwardStatus != "Solved" || solved.AwardEarned {
		t.Fatalf("non-awarded win = status %q earned %t, want Solved false", solved.AwardStatus, solved.AwardEarned)
	}
	if active.NextGuessNumber != 100 {
		t.Fatalf("next guess number = %d, want 100", active.NextGuessNumber)
	}
}

func TestGuessCardDetailPrintingPrefersResolvedExactPrinting(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		ID:       "323e4567-e89b-12d3-a456-426614174000",
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Resolved Printing",
		TypeLine: "Instant",
	}
	page := buildGuessCardPageData(guessCardGame{
		Status:           "won",
		TargetScryfallID: "223e4567-e89b-12d3-a456-426614174000",
	}, card)
	if !strings.Contains(page.TargetDetailPath, card.ID) {
		t.Fatalf("detail path = %q, want resolved printing %q", page.TargetDetailPath, card.ID)
	}
}

func TestGuessCardOwnerLockKeySeparatesUsersAndGuests(t *testing.T) {
	t.Parallel()

	if got := guessCardOwnerLockKey(gamePlayer{UserID: 42}); got != "guess-card:user:42" {
		t.Fatalf("user lock key = %q", got)
	}
	if got := guessCardOwnerLockKey(gamePlayer{GuestID: "ABC"}); got != "guess-card:guest:abc" {
		t.Fatalf("guest lock key = %q", got)
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
	if got.AwardStatus != "Just for fun" {
		t.Fatalf("replay award status = %q, want Just for fun", got.AwardStatus)
	}
	if got.GameModeLabel != "Just for fun" {
		t.Fatalf("replay game mode label = %q, want Just for fun", got.GameModeLabel)
	}
}

func TestBuildGuessCardPageDataKeepsCompleteEvidenceAndFinalAwardState(t *testing.T) {
	t.Parallel()

	card := guessCardRichTestCard()
	game := guessCardGame{
		Status:         "lost",
		IsDaily:        true,
		QuestionCount:  2,
		AskedQuestions: []string{"color_w", "creature"},
	}
	got := buildGuessCardPageData(game, card)
	if got.AwardStatus != "Not earned" {
		t.Fatalf("lost daily award status = %q, want Not earned", got.AwardStatus)
	}
	if len(got.PreviousReveals) != 2 {
		t.Fatalf("completed evidence trail = %d reveals, want all 2", len(got.PreviousReveals))
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

func TestGuessCardQuestionCatalogBuildsCompleteOneTapGroups(t *testing.T) {
	t.Parallel()

	wantIDs := []string{
		"color_w", "color_u", "color_b", "color_r", "color_g", "colorless", "monocolored", "multicolor",
		"permanent", "nonpermanent", "creature", "instant", "sorcery", "artifact", "enchantment", "planeswalker", "battle",
		"mv_ge_5", "mv_le_5", "mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4", "mv_eq_5",
		"draws_cards", "makes_tokens", "destroys", "exiles", "searches_library", "graveyard", "flying", "generates_mana", "deals_damage", "protects",
	}
	if len(guessCardQuestions) != len(wantIDs) {
		t.Fatalf("question catalog length = %d, want %d", len(guessCardQuestions), len(wantIDs))
	}
	for index, wantID := range wantIDs {
		question := guessCardQuestions[index]
		if question.ID != wantID {
			t.Fatalf("question %d ID = %q, want stable ID %q", index, question.ID, wantID)
		}
		if question.Label == "" || question.Text == "" || question.Symbol == "" {
			t.Fatalf("question %q is missing one-tap metadata: %#v", question.ID, question)
		}
	}

	groups := groupGuessCardQuestions(guessCardQuestions)
	if len(groups) != len(guessCardQuestionGroupDefinitions) {
		t.Fatalf("question groups = %d, want %d", len(groups), len(guessCardQuestionGroupDefinitions))
	}
	seen := map[string]bool{}
	for _, group := range groups {
		if group.Name == "" || len(group.Questions) == 0 {
			t.Fatalf("invalid question group: %#v", group)
		}
		for _, question := range group.Questions {
			if seen[question.ID] {
				t.Fatalf("question %q appeared in more than one group", question.ID)
			}
			seen[question.ID] = true
		}
	}
	if len(seen) != len(guessCardQuestions) {
		t.Fatalf("grouped questions = %d, want all %d catalog questions", len(seen), len(guessCardQuestions))
	}

	if got := questionByID("monocolored"); got == nil || got.Label != "Monocolored" {
		t.Fatalf("monocolored label = %#v, want Monocolored", got)
	}
	for _, group := range groups {
		if group.Name == "Format" {
			t.Fatal("retired Format group remained in the current question catalog")
		}
	}
	legacyIDs := []string{"mv_le_2", "mv_le_3", "commander_legal", "legendary"}
	for _, legacyID := range legacyIDs {
		if questionByID(legacyID) == nil {
			t.Errorf("legacy question %q is no longer resolvable", legacyID)
		}
		if seen[legacyID] {
			t.Errorf("legacy question %q remained available for new rounds", legacyID)
		}
	}
}

func TestBuildGuessCardPageDataPairsRevealsAndEnforcesLegacyQuestionBudget(t *testing.T) {
	t.Parallel()

	card := guessCardRichTestCard()
	game := guessCardGame{
		ID:             42,
		Status:         "active",
		QuestionCount:  3,
		MaxQuestions:   0,
		AskedQuestions: []string{"color_w", "creature", "flying"},
	}
	got := buildGuessCardPageData(game, card)
	if got.MaxQuestions != 8 || got.QuestionsLeft != 5 || !got.CanAsk {
		t.Fatalf("question budget = max %d left %d can ask %t, want 8, 5, true", got.MaxQuestions, got.QuestionsLeft, got.CanAsk)
	}
	if !got.HasLatestReveal || got.LatestReveal.Number != 3 || got.LatestReveal.Question != "Does its rules text mention flying?" || !got.LatestReveal.Yes {
		t.Fatalf("latest reveal = %#v", got.LatestReveal)
	}
	if !got.LatestReveal.HasClue || got.LatestReveal.Clue.Label != "Color identity" {
		t.Fatalf("latest reveal clue = %#v, want staged color identity", got.LatestReveal.Clue)
	}
	if len(got.PreviousReveals) != 2 || got.PreviousReveals[0].Number != 1 || got.PreviousReveals[1].Number != 2 {
		t.Fatalf("previous reveals = %#v, want turns 1 and 2", got.PreviousReveals)
	}

	game.QuestionCount = 8
	game.AskedQuestions = []string{"color_w", "creature", "flying", "artifact", "legendary", "draws_cards", "deals_damage", "protects"}
	exhausted := buildGuessCardPageData(game, card)
	if exhausted.QuestionsLeft != 0 || exhausted.CanAsk {
		t.Fatalf("exhausted budget = left %d can ask %t, want 0, false", exhausted.QuestionsLeft, exhausted.CanAsk)
	}
}

func TestGuessCardCluesStageBroadToSpecificAndMaskNamesWhileActive(t *testing.T) {
	t.Parallel()

	card := guessCardRichTestCard()
	card.Name = "Secret Duo // Hidden Face"
	card.OracleText = "When Secret Duo enters, Hidden Face deals 3 damage."
	card.FlavorText = "The legend of Secret Duo lives on."
	card.Faces = []cards.CardFace{{Name: "Secret Duo"}, {Name: "Hidden Face"}}
	game := guessCardGame{
		Status:         "active",
		QuestionCount:  8,
		AskedQuestions: []string{"color_w", "creature", "flying", "artifact", "legendary", "draws_cards", "deals_damage", "protects"},
	}

	clues := buildGuessCardClues(game, card)
	wantLabels := []string{"Rarity", "Mana value", "Color identity", "Card type", "Power/Toughness", "Cast Cost", "Default set", "Rules text"}
	if len(clues) != len(wantLabels) {
		t.Fatalf("staged clues = %d, want %d", len(clues), len(wantLabels))
	}
	for index, wantLabel := range wantLabels {
		if clues[index].Label != wantLabel {
			t.Fatalf("clue %d label = %q, want %q", index, clues[index].Label, wantLabel)
		}
	}
	rules := clues[len(clues)-1]
	for _, secret := range []string{"secret duo", "hidden face"} {
		if strings.Contains(strings.ToLower(rules.Value), secret) || strings.Contains(strings.ToLower(string(rules.ValueHTML)), secret) {
			t.Fatalf("active rules clue leaked %q: value=%q html=%q", secret, rules.Value, rules.ValueHTML)
		}
	}
	if !strings.Contains(rules.Value, "[card name]") {
		t.Fatalf("active rules clue did not mark a masked name: %q", rules.Value)
	}

	game.Status = "won"
	completed := buildGuessCardClues(game, card)
	if !strings.Contains(completed[len(completed)-1].Value, "Secret Duo") {
		t.Fatalf("completed rules clue remained masked: %q", completed[len(completed)-1].Value)
	}
}

func TestGuessCardQuestionAvailabilityHonorsCluesAndBudget(t *testing.T) {
	t.Parallel()

	card := guessCardRichTestCard()
	game := guessCardGame{
		Status:         "active",
		QuestionCount:  2,
		AskedQuestions: []string{"color_w", "creature"},
	}
	if guessCardQuestionAvailable(game, card, "color_w") {
		t.Fatal("already asked question remained available")
	}
	if guessCardQuestionAvailable(game, card, "mv_ge_5") {
		t.Fatal("mana-value question remained available after the staged mana-value clue")
	}
	if !guessCardQuestionAvailable(game, card, "draws_cards") {
		t.Fatal("unasked, unrevealed rules-text question should remain available")
	}
	if guessCardQuestionAvailable(game, card, "not_a_question") {
		t.Fatal("unknown question was accepted")
	}

	game.QuestionCount = 8
	if guessCardQuestionAvailable(game, card, "draws_cards") {
		t.Fatal("question was accepted after the round budget was exhausted")
	}
}

func guessCardRichTestCard() cards.Card {
	return cards.Card{
		OracleID:       "123e4567-e89b-12d3-a456-426614174000",
		Name:           "Sample Flyer",
		ManaCost:       "{2}{W}",
		TypeLine:       "Legendary Creature - Bird Horse",
		OracleText:     "Flying. Sample Flyer deals 3 damage to any target.",
		FlavorText:     "Sample Flyer crossed the skyline at dawn.",
		ColorIdentity:  []string{"W"},
		CMC:            3,
		Power:          "2",
		Toughness:      "3",
		CommanderLegal: true,
		Rarity:         "rare",
		SetName:        "Sample Set",
		ReleasedAt:     "2026-01-01",
		Artist:         "Sample Artist",
	}
}

func TestGuessCardAnswerEliminationsRemoveBroadColorSiblingAfterEitherAnswer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		facts     map[string]bool
		removed   string
		available []string
	}{
		{
			name:      "monocolored yes removes multicolor",
			facts:     map[string]bool{"monocolored": true},
			removed:   "multicolor",
			available: []string{"color_w"},
		},
		{
			name:      "monocolored no still removes multicolor sibling",
			facts:     map[string]bool{"monocolored": false},
			removed:   "multicolor",
			available: []string{"color_w", "colorless"},
		},
		{
			name:      "multicolor yes removes monocolored",
			facts:     map[string]bool{"multicolor": true},
			removed:   "monocolored",
			available: []string{"color_w"},
		},
		{
			name:      "multicolor no still removes monocolored sibling",
			facts:     map[string]bool{"multicolor": false},
			removed:   "monocolored",
			available: []string{"color_w", "colorless"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := guessCardAnswerEliminations(tt.facts)
			if !got[tt.removed] {
				t.Fatalf("eliminations = %#v, want sibling %q removed", got, tt.removed)
			}
			for _, questionID := range tt.available {
				if got[questionID] {
					t.Fatalf("eliminations = %#v, want %q to remain useful", got, questionID)
				}
			}
		})
	}
}

func TestGuessCardAnswerEliminationsDeduceColorIdentityFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		facts     map[string]bool
		removed   []string
		available []string
	}{
		{
			name:      "colorless settles every colored and cardinality question",
			facts:     map[string]bool{"colorless": true},
			removed:   []string{"color_w", "color_u", "color_b", "color_r", "color_g", "monocolored", "multicolor"},
			available: nil,
		},
		{
			name:      "two known colors settle multicolor cardinality",
			facts:     map[string]bool{"color_w": true, "color_u": true},
			removed:   []string{"colorless", "monocolored", "multicolor"},
			available: []string{"color_b", "color_r", "color_g"},
		},
		{
			name: "one known color and four exclusions settle monocolored",
			facts: map[string]bool{
				"color_w": true,
				"color_u": false,
				"color_b": false,
				"color_r": false,
				"color_g": false,
			},
			removed: []string{"colorless", "monocolored", "multicolor"},
		},
		{
			name: "monocolored plus one known color settles every other color",
			facts: map[string]bool{
				"monocolored": true,
				"color_w":     true,
			},
			removed: []string{"color_u", "color_b", "color_r", "color_g", "colorless", "multicolor"},
		},
		{
			name: "five excluded colors settle colorless identity",
			facts: map[string]bool{
				"color_w": false,
				"color_u": false,
				"color_b": false,
				"color_r": false,
				"color_g": false,
			},
			removed: []string{"colorless", "monocolored", "multicolor"},
		},
		{
			name:      "multicolor no plus one color settles all other colors",
			facts:     map[string]bool{"multicolor": false, "color_w": true},
			removed:   []string{"color_u", "color_b", "color_r", "color_g", "colorless", "monocolored"},
			available: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := guessCardAnswerEliminations(tt.facts)
			assertGuessCardQuestionEliminations(t, got, tt.removed, tt.available)
		})
	}
}

func TestGuessCardAnswerEliminationsDeduceManaValueFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		facts     map[string]bool
		removed   []string
		available []string
	}{
		{
			name:      "exactly zero settles every other current mana question",
			facts:     map[string]bool{"mv_eq_0": true},
			removed:   []string{"mv_ge_5", "mv_le_5", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4", "mv_eq_5"},
			available: nil,
		},
		{
			name:      "exactly five settles both boundary questions",
			facts:     map[string]bool{"mv_eq_5": true},
			removed:   []string{"mv_ge_5", "mv_le_5", "mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4"},
			available: nil,
		},
		{
			name:      "five or greater preserves the exact-five boundary",
			facts:     map[string]bool{"mv_ge_5": true},
			removed:   []string{"mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4"},
			available: []string{"mv_le_5", "mv_eq_5"},
		},
		{
			name:      "below five settles the upper range but preserves exact zero through four",
			facts:     map[string]bool{"mv_ge_5": false},
			removed:   []string{"mv_le_5", "mv_eq_5"},
			available: []string{"mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4"},
		},
		{
			name:      "five or less preserves both the lower values and exact-five boundary",
			facts:     map[string]bool{"mv_le_5": true},
			removed:   nil,
			available: []string{"mv_ge_5", "mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4", "mv_eq_5"},
		},
		{
			name:      "more than five settles every other current mana question",
			facts:     map[string]bool{"mv_le_5": false},
			removed:   []string{"mv_ge_5", "mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4", "mv_eq_5"},
			available: nil,
		},
		{
			name:      "one rejected exact value leaves both ranges and other exact values useful",
			facts:     map[string]bool{"mv_eq_4": false},
			removed:   nil,
			available: []string{"mv_ge_5", "mv_le_5", "mv_eq_0", "mv_eq_3", "mv_eq_5"},
		},
		{
			name:      "legacy three-or-less answer prunes the new catalog",
			facts:     map[string]bool{"mv_le_3": true},
			removed:   []string{"mv_ge_5", "mv_le_5", "mv_eq_4", "mv_eq_5"},
			available: []string{"mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := guessCardAnswerEliminations(tt.facts)
			assertGuessCardQuestionEliminations(t, got, tt.removed, tt.available)
		})
	}
}

func TestGuessCardAnswerEliminationsDeduceTypesConservatively(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		facts     map[string]bool
		removed   []string
		available []string
	}{
		{
			name:      "creature entails permanent but not other card types",
			facts:     map[string]bool{"creature": true},
			removed:   []string{"permanent"},
			available: []string{"artifact", "nonpermanent", "instant", "sorcery"},
		},
		{
			name:      "not permanent settles each visible permanent type",
			facts:     map[string]bool{"permanent": false},
			removed:   []string{"creature", "artifact", "enchantment", "planeswalker", "land", "battle"},
			available: []string{"nonpermanent", "instant", "sorcery"},
		},
		{
			name:      "battle entails permanent",
			facts:     map[string]bool{"battle": true},
			removed:   []string{"permanent"},
			available: []string{"creature", "nonpermanent", "instant", "sorcery"},
		},
		{
			name:      "instant entails nonpermanent but remains compatible with permanent faces",
			facts:     map[string]bool{"instant": true},
			removed:   []string{"nonpermanent"},
			available: []string{"permanent", "creature", "sorcery"},
		},
		{
			name:      "not nonpermanent settles instant and sorcery only",
			facts:     map[string]bool{"nonpermanent": false},
			removed:   []string{"instant", "sorcery"},
			available: []string{"permanent", "creature", "artifact"},
		},
		{
			name:      "nonpermanent plus not instant entails sorcery",
			facts:     map[string]bool{"nonpermanent": true, "instant": false},
			removed:   []string{"sorcery"},
			available: []string{"permanent", "creature"},
		},
		{
			name: "all visible permanent types absent still leaves battle possibility",
			facts: map[string]bool{
				"creature":     false,
				"artifact":     false,
				"enchantment":  false,
				"planeswalker": false,
				"land":         false,
			},
			available: []string{"permanent"},
		},
		{
			name: "all permanent types absent settles permanent",
			facts: map[string]bool{
				"creature":     false,
				"artifact":     false,
				"enchantment":  false,
				"planeswalker": false,
				"land":         false,
				"battle":       false,
			},
			removed: []string{"permanent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := guessCardAnswerEliminations(tt.facts)
			assertGuessCardQuestionEliminations(t, got, tt.removed, tt.available)
		})
	}
}

func TestGuessCardAnswerEliminationsPreserveMultifacePermanentAndNonpermanent(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		TypeLine: "Creature // Sorcery",
		Faces: []cards.CardFace{
			{TypeLine: "Creature"},
			{TypeLine: "Sorcery"},
		},
	}
	if !guessQuestionAnswer(card, "permanent") || !guessQuestionAnswer(card, "nonpermanent") {
		t.Fatal("multiface fixture must be both permanent and nonpermanent")
	}

	permanentFacts := guessCardAnswerFacts([]string{"permanent"}, card)
	if got := guessCardAnswerEliminations(permanentFacts); got["nonpermanent"] {
		t.Fatalf("permanent answer incorrectly removed nonpermanent for a multiface card: %#v", got)
	}
	nonpermanentFacts := guessCardAnswerFacts([]string{"nonpermanent"}, card)
	if got := guessCardAnswerEliminations(nonpermanentFacts); got["permanent"] {
		t.Fatalf("nonpermanent answer incorrectly removed permanent for a multiface card: %#v", got)
	}
}

func assertGuessCardQuestionEliminations(t *testing.T, got map[string]bool, removed, available []string) {
	t.Helper()
	for _, questionID := range removed {
		if !got[questionID] {
			t.Errorf("eliminations = %#v, want %q removed", got, questionID)
		}
	}
	for _, questionID := range available {
		if got[questionID] {
			t.Errorf("eliminations = %#v, want %q available", got, questionID)
		}
	}
}
