package web

import (
	"strings"
	"testing"

	"manatomb/app/internal/cards"
)

func TestCalculateGuessCardPossibilityCountsMatchesEveryAskedAnswer(t *testing.T) {
	t.Parallel()

	target := cards.Card{
		OracleID:      "target",
		TypeLine:      "Legendary Creature - Bird",
		OracleText:    "When this enters, draw a card.",
		ColorIdentity: []string{"W"},
		CMC:           3,
	}
	candidates := []cards.Card{
		target,
		{
			OracleID:      "matching",
			TypeLine:      "Artifact Creature - Construct",
			OracleText:    "When this enters, draw two cards.",
			ColorIdentity: []string{"W"},
			CMC:           2,
		},
		{
			OracleID:      "wrong-color",
			TypeLine:      "Creature - Wizard",
			OracleText:    "Draw a card.",
			ColorIdentity: []string{"U"},
		},
		{
			OracleID:      "wrong-type",
			TypeLine:      "Instant",
			OracleText:    "Draw a card.",
			ColorIdentity: []string{"W"},
		},
		{
			OracleID:      "wrong-text",
			TypeLine:      "Creature - Human",
			OracleText:    "Scry 2.",
			ColorIdentity: []string{"W"},
		},
	}
	game := guessCardGame{AskedQuestions: []string{"color_w", "creature", "draws_cards"}}

	got := calculateGuessCardPossibilityCounts(candidates, game, target)
	if got.Total != 5 || got.Possible != 2 {
		t.Fatalf("counts = %#v, want 2 possible of 5 total", got)
	}
}

func TestCalculateGuessCardPossibilityCountsMatchesNegativeAnswers(t *testing.T) {
	t.Parallel()

	target := cards.Card{
		OracleID:      "target",
		TypeLine:      "Sorcery",
		OracleText:    "Create a token.",
		ColorIdentity: []string{"R"},
	}
	candidates := []cards.Card{
		target,
		{
			OracleID:      "also-no",
			TypeLine:      "Instant",
			OracleText:    "Destroy target artifact.",
			ColorIdentity: []string{"G"},
		},
		{
			OracleID:      "blue",
			TypeLine:      "Instant",
			ColorIdentity: []string{"U"},
		},
		{
			OracleID:      "flying",
			TypeLine:      "Creature - Bird",
			OracleText:    "Flying",
			ColorIdentity: []string{"R"},
		},
	}
	game := guessCardGame{AskedQuestions: []string{"color_u", "flying"}}

	got := calculateGuessCardPossibilityCounts(candidates, game, target)
	if got.Total != 4 || got.Possible != 2 {
		t.Fatalf("counts = %#v, want 2 possible of 4 total", got)
	}
}

func TestCalculateGuessCardPossibilityCountsUsesCardFaces(t *testing.T) {
	t.Parallel()

	target := cards.Card{
		OracleID: "target",
		Faces: []cards.CardFace{{
			TypeLine:   "Creature - Wizard",
			OracleText: "Flying",
		}},
	}
	candidates := []cards.Card{
		target,
		{
			OracleID: "face-match",
			Faces: []cards.CardFace{{
				TypeLine:   "Creature - Bird",
				OracleText: "Flying",
			}},
		},
		{OracleID: "no-face-match", TypeLine: "Sorcery", OracleText: "Draw a card."},
	}
	game := guessCardGame{AskedQuestions: []string{"creature", "flying"}}

	got := calculateGuessCardPossibilityCounts(candidates, game, target)
	if got.Total != 3 || got.Possible != 2 {
		t.Fatalf("counts = %#v, want both face-only matches retained", got)
	}
}

func TestResolvedWrongGuessTrackingIsUniqueAndIgnoresUnmatchedOrCorrect(t *testing.T) {
	t.Parallel()

	got := appendResolvedWrongGuessOracleID(
		[]string{" WRONG-A ", "wrong-a", "target", ""},
		"WRONG-B",
		"TARGET",
	)
	if len(got) != 2 || got[0] != "wrong-a" || got[1] != "wrong-b" {
		t.Fatalf("wrong guesses = %#v, want unique normalized wrong-a and wrong-b", got)
	}

	got = appendResolvedWrongGuessOracleID(got, "wrong-b", "target")
	if len(got) != 2 {
		t.Fatalf("repeated resolved guess changed list: %#v", got)
	}

	got = appendResolvedWrongGuessOracleID(got, "", "target")
	if len(got) != 2 {
		t.Fatalf("unmatched guess changed list: %#v", got)
	}

	got = appendResolvedWrongGuessOracleID(got, "target", "target")
	if len(got) != 2 {
		t.Fatalf("correct guess changed wrong-guess list: %#v", got)
	}
}

func TestCalculateGuessCardPossibilityCountsExcludesEachWrongCardOnce(t *testing.T) {
	t.Parallel()

	candidates := []cards.Card{
		{OracleID: "target"},
		{OracleID: "wrong-a"},
		{OracleID: "wrong-b"},
	}
	game := guessCardGame{WrongGuessOracleIDs: []string{"WRONG-A", "wrong-a", "", "outside-pool"}}

	got := calculateGuessCardPossibilityCounts(candidates, game, candidates[0])
	if got.Total != 3 || got.Possible != 2 {
		t.Fatalf("counts = %#v, want only wrong-a excluded from 3-card pool", got)
	}
}

func TestGuessCardEligiblePoolPredicateMatchesTargetSelectionContract(t *testing.T) {
	t.Parallel()

	for _, required := range []string{
		"COALESCE(oc.legal_anywhere, true) = true",
		"COALESCE(oc.default_image_uri, '') <> ''",
		"oc.default_print_id IS NOT NULL",
		"COALESCE(oc.edhrec_rank, 0) BETWEEN 1 AND 250",
		"lower(COALESCE(oc.default_set_code, '')) <> 'unk'",
		"lower(COALESCE(oc.type_line, '')) NOT LIKE '%land%'",
	} {
		if !strings.Contains(guessCardEligiblePoolPredicateSQL, required) {
			t.Fatalf("eligible-pool predicate is missing %q: %s", required, guessCardEligiblePoolPredicateSQL)
		}
	}
	if strings.Contains(sharedGameEligiblePoolPredicateSQL, "NOT LIKE '%land%'") {
		t.Fatal("the shared Tombscript pool unexpectedly inherited the Guess the Card land exclusion")
	}
}

func TestGuessCardCurrentQuestionsDoNotOfferLand(t *testing.T) {
	t.Parallel()

	for _, question := range guessCardQuestions {
		if question.ID == "land" {
			t.Fatal("Land remains available even though lands cannot be selected as Guess the Card targets")
		}
	}
	if legacy := questionByID("land"); legacy == nil {
		t.Fatal("older rounds should still be able to render a previously asked Land question")
	}
}
