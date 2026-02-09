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
	MaybeDeckCards      []decks.DeckCard
	VisibleCardCount    int
	MaybeCardCount      int
	Analytics           deckAnalyticsData
	Commander           *cards.Card
	CommanderCandidates []commanderCandidate
	GuestMode           bool
}

func (a *App) lookupCommanderCard(ctx context.Context, commanderName string) *cards.Card {
	commanderName = strings.TrimSpace(commanderName)
	if commanderName == "" {
		return nil
	}

	card, err := cards.GetCardByName(ctx, a.DB, commanderName)
	if err != nil {
		return nil
	}
	return card
}
