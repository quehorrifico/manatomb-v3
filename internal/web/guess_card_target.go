package web

import (
	"context"
	"errors"
	"strings"

	"manatomb/app/internal/cards"
)

// loadGuessCardTarget keeps a round pinned to the printing chosen when the
// game was created. Older rows without a printing ID gracefully fall back to
// the oracle card's current default printing.
func (a *App) loadGuessCardTarget(ctx context.Context, game guessCardGame) (*cards.Card, error) {
	if printingID := strings.TrimSpace(game.TargetScryfallID); printingID != "" {
		card, err := cards.GetCardPrintingByID(ctx, a.DB, game.TargetOracleID, printingID)
		if err == nil {
			return card, nil
		}
		if !errors.Is(err, cards.ErrCardNotFound) {
			return nil, err
		}
	}
	return cards.GetCardByOracleID(ctx, a.DB, game.TargetOracleID)
}
