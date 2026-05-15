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
	Deck                  *decks.Deck
	DeckCards             []decks.DeckCard
	SideboardDeckCards    []decks.DeckCard
	MaybeDeckCards        []decks.DeckCard
	VisibleCardCount      int
	SideboardCardCount    int
	MaybeCardCount        int
	Analytics             deckAnalyticsData
	Commander             *cards.Card
	CommanderCandidates   []commanderCandidate
	CommanderCandidateSet map[string]bool
	WorkspaceState        workspaceDeckState
	WorkbenchMode         bool
	WorkbenchSandbox      bool
}

func buildCommanderCandidateSet(candidates []commanderCandidate) map[string]bool {
	if len(candidates) == 0 {
		return nil
	}

	out := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		name := strings.TrimSpace(candidate.CardName)
		if name == "" {
			continue
		}
		out[name] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
