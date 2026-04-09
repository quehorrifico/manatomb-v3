package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type workspaceCardMeta struct {
	CardID               string           `json:"cardID,omitempty"`
	Name                 string           `json:"name,omitempty"`
	ManaCost             string           `json:"manaCost,omitempty"`
	TypeLine             string           `json:"typeLine,omitempty"`
	OracleText           string           `json:"oracleText,omitempty"`
	CMC                  float64          `json:"cmc"`
	PriceUSD             string           `json:"priceUSD,omitempty"`
	ImageURI             string           `json:"imageURI,omitempty"`
	IsCommanderCandidate bool             `json:"isCommanderCandidate,omitempty"`
	Faces                []cards.CardFace `json:"faces,omitempty"`
}

type workspaceDeckState struct {
	ID                  int64                        `json:"id,omitempty"`
	Name                string                       `json:"name"`
	Description         string                       `json:"description"`
	Tags                string                       `json:"tags"`
	Format              string                       `json:"format"`
	CommanderName       string                       `json:"commanderName,omitempty"`
	IsPublic            bool                         `json:"isPublic"`
	PublicSlug          string                       `json:"publicSlug,omitempty"`
	Cards               map[string]int               `json:"cards"`
	MaybeCards          map[string]int               `json:"maybeCards"`
	CardMeta            map[string]workspaceCardMeta `json:"cardMeta"`
	CommanderCandidates []string                     `json:"commanderCandidates,omitempty"`
	Analytics           deckAnalyticsData            `json:"analytics"`
}

func wantsJSONResponse(r *http.Request) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Accept"))), "application/json")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func buildWorkspaceStateFromDeck(
	d *decks.Deck,
	deckCards []decks.DeckCard,
	maybeDeckCards []decks.DeckCard,
	candidates []commanderCandidate,
	commanderCard *cards.Card,
) workspaceDeckState {
	state := workspaceDeckState{
		ID:                  d.ID,
		Name:                d.Name,
		Description:         d.Description,
		Tags:                d.Tags,
		Format:              d.Format,
		CommanderName:       d.CommanderName,
		IsPublic:            d.IsPublic,
		PublicSlug:          d.PublicSlug,
		Cards:               map[string]int{},
		MaybeCards:          map[string]int{},
		CardMeta:            map[string]workspaceCardMeta{},
		CommanderCandidates: []string{},
		Analytics:           computeDeckAnalyticsFromDeckCards(d.Format, d.CommanderName, deckCards),
	}

	candidateSet := buildCommanderCandidateSet(candidates)
	for _, candidate := range candidates {
		name := strings.TrimSpace(candidate.CardName)
		if name == "" {
			continue
		}
		state.CommanderCandidates = append(state.CommanderCandidates, name)
	}

	for _, dc := range deckCards {
		name := strings.TrimSpace(dc.CardName)
		if name == "" || dc.Quantity <= 0 {
			continue
		}

		state.Cards[name] += dc.Quantity
		state.CardMeta[name] = workspaceCardMeta{
			CardID:               strings.TrimSpace(dc.CardID),
			Name:                 name,
			ManaCost:             strings.TrimSpace(dc.ManaCost),
			TypeLine:             strings.TrimSpace(dc.TypeLine),
			OracleText:           strings.TrimSpace(dc.OracleText),
			CMC:                  dc.CMC,
			PriceUSD:             strings.TrimSpace(dc.PriceUSD),
			ImageURI:             strings.TrimSpace(dc.ImageURI),
			IsCommanderCandidate: candidateSet[name],
		}
	}

	for _, dc := range maybeDeckCards {
		name := strings.TrimSpace(dc.CardName)
		if name == "" || dc.Quantity <= 0 {
			continue
		}
		state.MaybeCards[name] += dc.Quantity
		if _, ok := state.CardMeta[name]; ok {
			continue
		}
		state.CardMeta[name] = workspaceCardMeta{
			CardID:               strings.TrimSpace(dc.CardID),
			Name:                 name,
			ManaCost:             strings.TrimSpace(dc.ManaCost),
			TypeLine:             strings.TrimSpace(dc.TypeLine),
			OracleText:           strings.TrimSpace(dc.OracleText),
			CMC:                  dc.CMC,
			PriceUSD:             strings.TrimSpace(dc.PriceUSD),
			ImageURI:             strings.TrimSpace(dc.ImageURI),
			IsCommanderCandidate: candidateSet[name],
		}
	}

	if commanderCard != nil {
		name := strings.TrimSpace(commanderCard.Name)
		if name != "" {
			meta := workspaceCardMeta{
				Name:                 name,
				ManaCost:             strings.TrimSpace(commanderCard.ManaCost),
				TypeLine:             strings.TrimSpace(commanderCard.TypeLine),
				OracleText:           strings.TrimSpace(commanderCard.OracleText),
				CMC:                  commanderCard.CMC,
				PriceUSD:             strings.TrimSpace(commanderCard.PriceUSD),
				ImageURI:             strings.TrimSpace(commanderCard.ImageURI),
				IsCommanderCandidate: true,
				Faces:                commanderCard.Faces,
			}
			state.CardMeta[name] = meta
			if commanderName := strings.TrimSpace(d.CommanderName); commanderName != "" && !strings.EqualFold(commanderName, name) {
				state.CardMeta[commanderName] = meta
			}
		}
	}

	return state
}

func (a *App) loadSavedDeckWorkspace(ctx context.Context, userID, deckID int64) (deckPageData, error) {
	d, err := decks.GetDeck(ctx, a.DB, deckID, userID)
	if err != nil {
		return deckPageData{}, err
	}

	deckCards, err := decks.ListDeckCards(ctx, a.DB, deckID)
	if err != nil {
		return deckPageData{}, err
	}

	maybeDeckCards, err := decks.ListDeckMaybeCards(ctx, a.DB, deckID)
	if err != nil {
		return deckPageData{}, err
	}

	commanderCandidates := make([]commanderCandidate, 0)
	var commanderCard *cards.Card
	if decks.FormatRequiresCommander(d.Format) {
		deckCardNames := make([]string, 0, len(deckCards))
		for _, dc := range deckCards {
			name := strings.TrimSpace(dc.CardName)
			if name == "" {
				continue
			}
			deckCardNames = append(deckCardNames, name)
		}
		deckCardsByName, err := cards.LookupCardsByNames(ctx, a.DB, deckCardNames)
		if err != nil {
			return deckPageData{}, err
		}

		for _, dc := range deckCards {
			name := strings.TrimSpace(dc.CardName)
			if name == "" {
				continue
			}
			resolved, ok := deckCardsByName[strings.ToLower(name)]
			if ok && resolved.IsCommanderCandidate {
				commanderCandidates = append(commanderCandidates, commanderCandidate{CardName: name})
			}
		}

		commanderCard = a.lookupCommanderCard(ctx, d.CommanderName)
	}

	workspaceState := buildWorkspaceStateFromDeck(d, deckCards, maybeDeckCards, commanderCandidates, commanderCard)

	return deckPageData{
		Deck:                  d,
		DeckCards:             deckCards,
		MaybeDeckCards:        maybeDeckCards,
		VisibleCardCount:      visibleDeckCardCount(deckCards, d.CommanderName),
		MaybeCardCount:        deckCardQuantityTotal(maybeDeckCards),
		Analytics:             workspaceState.Analytics,
		Commander:             commanderCard,
		CommanderCandidates:   commanderCandidates,
		CommanderCandidateSet: buildCommanderCandidateSet(commanderCandidates),
		WorkbenchMode:         false,
		WorkspaceState:        workspaceState,
	}, nil
}

func (a *App) renderSavedDeckWorkspaceJSON(w http.ResponseWriter, r *http.Request, userID, deckID int64) {
	pageData, err := a.loadSavedDeckWorkspace(r.Context(), userID, deckID)
	if err != nil {
		a.RenderNotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, pageData.WorkspaceState)
}
