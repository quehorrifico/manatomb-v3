package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"manatomb/app/internal/cards"

	"github.com/lib/pq"
)

// sharedGameEligiblePoolPredicateSQL is the common daily-game card pool.
// Keep the oracle_cards alias as "oc" at every call site.
const sharedGameEligiblePoolPredicateSQL = `
	COALESCE(oc.legal_anywhere, true) = true
	AND COALESCE(oc.default_image_uri, '') <> ''
	AND oc.default_print_id IS NOT NULL
	AND COALESCE(oc.edhrec_rank, 0) BETWEEN 1 AND 250
	AND lower(COALESCE(oc.default_set_code, '')) <> 'unk'`

// Guess the Card excludes every card with Land in its type line. This same
// predicate selects targets and calculates the live possible-card count, so a
// round can never advertise options that were not eligible to be chosen.
const guessCardEligiblePoolPredicateSQL = sharedGameEligiblePoolPredicateSQL + `
	AND lower(COALESCE(oc.type_line, '')) NOT LIKE '%land%'`

const guessCardEligibleCandidatesSQL = `
	SELECT
		oc.oracle_id::text,
		oc.name,
		COALESCE(oc.mana_cost, ''),
		COALESCE(oc.cmc, 0),
		COALESCE(oc.type_line, ''),
		COALESCE(oc.oracle_text, ''),
		COALESCE(oc.flavor_text, ''),
		COALESCE(oc.colors, ARRAY[]::TEXT[]),
		COALESCE(oc.color_identity, ARRAY[]::TEXT[]),
		COALESCE(oc.power_text, ''),
		COALESCE(oc.toughness_text, ''),
		COALESCE(oc.loyalty_text, ''),
		COALESCE(oc.layout, ''),
		COALESCE(oc.card_faces, '[]'::jsonb),
		COALESCE(oc.commander_legal, false)
	FROM oracle_cards oc
	WHERE ` + guessCardEligiblePoolPredicateSQL + `
	ORDER BY COALESCE(oc.edhrec_rank, 1000000), oc.name, oc.oracle_id::text`

// guessCardPossibilityCounts is the display-ready numerator and denominator
// for "possible cards X / Y". Total is the complete eligible target pool;
// Possible applies the round's answered questions and resolved wrong guesses.
type guessCardPossibilityCounts struct {
	Possible int
	Total    int
}

// loadGuessCardPossibilityCounts computes a round's live possible-card count.
// The target must be the exact card already loaded for the round so every
// candidate is compared with the same answer logic used by gameplay.
func loadGuessCardPossibilityCounts(
	ctx context.Context,
	db *sql.DB,
	game guessCardGame,
	target cards.Card,
) (guessCardPossibilityCounts, error) {
	candidates, err := loadGuessCardEligibleCandidates(ctx, db)
	if err != nil {
		return guessCardPossibilityCounts{}, err
	}
	return calculateGuessCardPossibilityCounts(candidates, game, target), nil
}

func loadGuessCardEligibleCandidates(ctx context.Context, db *sql.DB) ([]cards.Card, error) {
	rows, err := db.QueryContext(ctx, guessCardEligibleCandidatesSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]cards.Card, 0, 250)
	for rows.Next() {
		var (
			candidate cards.Card
			facesJSON []byte
		)
		if err := rows.Scan(
			&candidate.OracleID,
			&candidate.Name,
			&candidate.ManaCost,
			&candidate.CMC,
			&candidate.TypeLine,
			&candidate.OracleText,
			&candidate.FlavorText,
			pq.Array(&candidate.Colors),
			pq.Array(&candidate.ColorIdentity),
			&candidate.Power,
			&candidate.Toughness,
			&candidate.Loyalty,
			&candidate.Layout,
			&facesJSON,
			&candidate.CommanderLegal,
		); err != nil {
			return nil, err
		}
		if len(facesJSON) > 0 {
			if err := json.Unmarshal(facesJSON, &candidate.Faces); err != nil {
				return nil, fmt.Errorf("decode guess-card candidate %s faces: %w", candidate.OracleID, err)
			}
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func calculateGuessCardPossibilityCounts(
	candidates []cards.Card,
	game guessCardGame,
	target cards.Card,
) guessCardPossibilityCounts {
	counts := guessCardPossibilityCounts{Total: len(candidates)}
	wrongGuesses := normalizedGuessCardOracleIDSet(game.WrongGuessOracleIDs)

	type answerConstraint struct {
		questionID string
		answer     bool
	}
	constraints := make([]answerConstraint, 0, len(game.AskedQuestions))
	seenQuestions := make(map[string]struct{}, len(game.AskedQuestions))
	for _, rawQuestionID := range game.AskedQuestions {
		questionID := strings.TrimSpace(rawQuestionID)
		if questionID == "" || questionByID(questionID) == nil {
			continue
		}
		if _, exists := seenQuestions[questionID]; exists {
			continue
		}
		seenQuestions[questionID] = struct{}{}
		constraints = append(constraints, answerConstraint{
			questionID: questionID,
			answer:     guessQuestionAnswer(target, questionID),
		})
	}

	for _, candidate := range candidates {
		candidateID := normalizedGuessCardOracleID(candidate.OracleID)
		if _, excluded := wrongGuesses[candidateID]; excluded && candidateID != "" {
			continue
		}

		possible := true
		for _, constraint := range constraints {
			if guessQuestionAnswer(candidate, constraint.questionID) != constraint.answer {
				possible = false
				break
			}
		}
		if possible {
			counts.Possible++
		}
	}
	return counts
}

func appendResolvedWrongGuessOracleID(existing []string, guessedOracleID, targetOracleID string) []string {
	targetID := normalizedGuessCardOracleID(targetOracleID)
	guessedID := normalizedGuessCardOracleID(guessedOracleID)
	out := make([]string, 0, len(existing)+1)
	seen := make(map[string]struct{}, len(existing)+1)
	for _, rawID := range existing {
		id := normalizedGuessCardOracleID(rawID)
		if id == "" || id == targetID {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if guessedID == "" || guessedID == targetID {
		return out
	}
	if _, duplicate := seen[guessedID]; !duplicate {
		out = append(out, guessedID)
	}
	return out
}

func normalizedGuessCardOracleIDSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, rawID := range ids {
		if id := normalizedGuessCardOracleID(rawID); id != "" {
			set[id] = struct{}{}
		}
	}
	return set
}

func normalizedGuessCardOracleID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
