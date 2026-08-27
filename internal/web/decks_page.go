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
	GuestAuthNextPath     string
}

func deckWorkspacePageMeta(deck *decks.Deck) *PageMeta {
	meta := defaultPageMeta("deck_show")
	if meta == nil {
		meta = &PageMeta{Title: "Deck Editor"}
	}
	if deck != nil {
		if name := truncateShareText(deck.Name, 56); name != "" {
			meta.Title = name
			meta.Description = "Edit, organize, analyze, and playtest " + name + " on ManaTomb."
		}
	}
	return meta
}

func deckPlaytestPageMeta(deck *decks.Deck) *PageMeta {
	meta := defaultPageMeta("deck_playtest")
	if meta == nil {
		meta = &PageMeta{Title: "Deck Playtest"}
	}
	if deck != nil {
		if name := truncateShareText(deck.Name, 44); name != "" {
			meta.Title = name + " — Playtest"
			meta.Description = "Playtest " + name + " in the ManaTomb tabletop workspace."
		}
	}
	return meta
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

func (a *App) lookupCommanderCardPrinting(ctx context.Context, commanderName, printID string) *cards.Card {
	card := a.lookupCommanderCard(ctx, commanderName)
	if card == nil || strings.TrimSpace(printID) == "" {
		return card
	}

	selected, err := cards.GetCardPrintingByID(ctx, a.DB, card.OracleID, printID)
	if err != nil {
		return card
	}
	return selected
}
