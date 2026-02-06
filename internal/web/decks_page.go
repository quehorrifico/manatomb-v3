package web

import (
	"context"
	"strings"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type commanderCandidate struct {
	CardName string
}

type deckPageData struct {
	Deck                *decks.Deck
	DeckCards           []decks.DeckCard
	VisibleCardCount    int
	Commander           *cards.Card
	CommanderCandidates []commanderCandidate
	GuestMode           bool
}

func (a *App) lookupCommanderCard(ctx context.Context, commanderName string) *cards.Card {
	commanderName = strings.TrimSpace(commanderName)
	if commanderName == "" {
		return nil
	}

	scry := cards.NewScryfallClient()
	results, err := scry.SearchByName(ctx, commanderName+" is:commander")
	if err != nil || len(results) == 0 {
		return nil
	}

	return &results[0]
}
