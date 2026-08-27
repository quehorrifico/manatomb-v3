package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errGuessCardRoundStale = errors.New("guess card round is no longer active")

func parseGuessCardGameID(raw string) (int64, bool) {
	gameID, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || gameID <= 0 {
		return 0, false
	}
	return gameID, true
}

func guessCardRoundPath(gameID int64) string {
	return fmt.Sprintf("/games/guess-card?game_id=%d", gameID)
}

func guessCardRoundRefreshPath(rawGameID string) string {
	gameID, ok := parseGuessCardGameID(rawGameID)
	if !ok {
		return "/games/guess-card"
	}
	return guessCardRoundPath(gameID)
}

// loadGuessCardGameForMutation binds every action to the round rendered in
// the submitting tab. Without this check, an older tab could accidentally
// ask a question or submit a guess against a newly-created round.
func loadGuessCardGameForMutation(
	ctx context.Context,
	db *sql.DB,
	player gamePlayer,
	rawGameID string,
) (*guessCardGame, error) {
	gameID, ok := parseGuessCardGameID(rawGameID)
	if !ok {
		return nil, errGuessCardRoundStale
	}
	game, err := loadGuessCardGameByID(ctx, db, player, gameID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errGuessCardRoundStale
	}
	if err != nil {
		return nil, err
	}
	if game.Status != "active" || guessCardActiveGameExpired(*game) {
		return nil, errGuessCardRoundStale
	}
	return game, nil
}
