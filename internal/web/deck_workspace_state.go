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
	PreferredPrintID     string           `json:"preferredPrintID,omitempty"`
	PrintID              string           `json:"printID,omitempty"`
	SetCode              string           `json:"setCode,omitempty"`
	SetName              string           `json:"setName,omitempty"`
	CollectorNumber      string           `json:"collectorNumber,omitempty"`
	Rarity               string           `json:"rarity,omitempty"`
	ReleasedAt           string           `json:"releasedAt,omitempty"`
	Artist               string           `json:"artist,omitempty"`
	IsCommanderCandidate bool             `json:"isCommanderCandidate,omitempty"`
	Faces                []cards.CardFace `json:"faces,omitempty"`
}

type workspaceDeckState struct {
	ID                  int64                                   `json:"id,omitempty"`
	Name                string                                  `json:"name"`
	Description         string                                  `json:"description"`
	Tags                string                                  `json:"tags"`
	Format              string                                  `json:"format"`
	CommanderName       string                                  `json:"commanderName,omitempty"`
	CommanderPrintID    string                                  `json:"commanderPrintID,omitempty"`
	IsPublic            bool                                    `json:"isPublic"`
	PublicSlug          string                                  `json:"publicSlug,omitempty"`
	Cards               map[string]int                          `json:"cards"`
	SideboardCards      map[string]int                          `json:"sideboardCards"`
	MaybeCards          map[string]int                          `json:"maybeCards"`
	CardMeta            map[string]workspaceCardMeta            `json:"cardMeta"`
	BoardCardMeta       map[string]map[string]workspaceCardMeta `json:"boardCardMeta,omitempty"`
	CommanderCandidates []string                                `json:"commanderCandidates,omitempty"`
	Analytics           deckAnalyticsData                       `json:"analytics"`
}

func wantsJSONResponse(r *http.Request) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Accept"))), "application/json")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func workspaceMetaFromDeckCard(dc decks.DeckCard, candidateSet map[string]bool) workspaceCardMeta {
	name := strings.TrimSpace(dc.CardName)
	return workspaceCardMeta{
		CardID:               strings.TrimSpace(dc.CardID),
		Name:                 name,
		ManaCost:             strings.TrimSpace(dc.ManaCost),
		TypeLine:             strings.TrimSpace(dc.TypeLine),
		OracleText:           strings.TrimSpace(dc.OracleText),
		CMC:                  dc.CMC,
		PriceUSD:             strings.TrimSpace(dc.PriceUSD),
		ImageURI:             strings.TrimSpace(dc.ImageURI),
		PreferredPrintID:     strings.TrimSpace(dc.PreferredPrintID),
		PrintID:              strings.TrimSpace(dc.PrintID),
		SetCode:              strings.TrimSpace(dc.SetCode),
		SetName:              strings.TrimSpace(dc.SetName),
		CollectorNumber:      strings.TrimSpace(dc.CollectorNumber),
		Rarity:               strings.TrimSpace(dc.Rarity),
		ReleasedAt:           strings.TrimSpace(dc.ReleasedAt),
		Artist:               strings.TrimSpace(dc.Artist),
		IsCommanderCandidate: candidateSet[name],
	}
}

func buildWorkspaceStateFromDeck(
	d *decks.Deck,
	deckCards []decks.DeckCard,
	sideboardDeckCards []decks.DeckCard,
	maybeDeckCards []decks.DeckCard,
	candidates []commanderCandidate,
	commanderCard *cards.Card,
) workspaceDeckState {
	state := workspaceDeckState{
		ID:               d.ID,
		Name:             d.Name,
		Description:      d.Description,
		Tags:             d.Tags,
		Format:           d.Format,
		CommanderName:    d.CommanderName,
		CommanderPrintID: strings.TrimSpace(d.CommanderPrintID),
		IsPublic:         d.IsPublic,
		PublicSlug:       d.PublicSlug,
		Cards:            map[string]int{},
		SideboardCards:   map[string]int{},
		MaybeCards:       map[string]int{},
		CardMeta:         map[string]workspaceCardMeta{},
		BoardCardMeta: map[string]map[string]workspaceCardMeta{
			"main":  {},
			"side":  {},
			"maybe": {},
		},
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
		meta := workspaceMetaFromDeckCard(dc, candidateSet)
		state.CardMeta[name] = meta
		state.BoardCardMeta["main"][name] = meta
	}

	for _, dc := range maybeDeckCards {
		name := strings.TrimSpace(dc.CardName)
		if name == "" || dc.Quantity <= 0 {
			continue
		}
		state.MaybeCards[name] += dc.Quantity
		meta := workspaceMetaFromDeckCard(dc, candidateSet)
		state.BoardCardMeta["maybe"][name] = meta
		if _, ok := state.CardMeta[name]; ok {
			continue
		}
		state.CardMeta[name] = meta
	}

	for _, dc := range sideboardDeckCards {
		name := strings.TrimSpace(dc.CardName)
		if name == "" || dc.Quantity <= 0 {
			continue
		}
		state.SideboardCards[name] += dc.Quantity
		meta := workspaceMetaFromDeckCard(dc, candidateSet)
		state.BoardCardMeta["side"][name] = meta
		if _, ok := state.CardMeta[name]; ok {
			continue
		}
		state.CardMeta[name] = meta
	}

	if commanderCard != nil {
		name := strings.TrimSpace(commanderCard.Name)
		if name != "" {
			meta := workspaceCardMeta{
				CardID:               strings.TrimSpace(commanderCard.OracleID),
				Name:                 name,
				ManaCost:             strings.TrimSpace(commanderCard.ManaCost),
				TypeLine:             strings.TrimSpace(commanderCard.TypeLine),
				OracleText:           strings.TrimSpace(commanderCard.OracleText),
				CMC:                  commanderCard.CMC,
				PriceUSD:             strings.TrimSpace(commanderCard.PriceUSD),
				ImageURI:             strings.TrimSpace(commanderCard.ImageURI),
				PreferredPrintID:     strings.TrimSpace(d.CommanderPrintID),
				PrintID:              strings.TrimSpace(commanderCard.ID),
				SetCode:              strings.TrimSpace(commanderCard.SetCode),
				SetName:              strings.TrimSpace(commanderCard.SetName),
				CollectorNumber:      strings.TrimSpace(commanderCard.CollectorNumber),
				Rarity:               strings.TrimSpace(commanderCard.Rarity),
				ReleasedAt:           strings.TrimSpace(commanderCard.ReleasedAt),
				Artist:               strings.TrimSpace(commanderCard.Artist),
				IsCommanderCandidate: isCommanderCandidateAllowed(commanderCard.IsCommanderCandidate, commanderCard.TypeLine),
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

	sideboardDeckCards, err := decks.ListDeckSideboardCards(ctx, a.DB, deckID)
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
			if ok && isCommanderCandidateAllowed(resolved.IsCommanderCandidate, resolved.TypeLine) {
				commanderCandidates = append(commanderCandidates, commanderCandidate{CardName: name})
			}
		}

		commanderCard = a.lookupCommanderCardPrinting(ctx, d.CommanderName, d.CommanderPrintID)
	}

	workspaceState := buildWorkspaceStateFromDeck(d, deckCards, sideboardDeckCards, maybeDeckCards, commanderCandidates, commanderCard)

	return deckPageData{
		Deck:                  d,
		DeckCards:             deckCards,
		SideboardDeckCards:    sideboardDeckCards,
		MaybeDeckCards:        maybeDeckCards,
		VisibleCardCount:      visibleDeckCardCount(deckCards, d.CommanderName),
		SideboardCardCount:    deckCardQuantityTotal(sideboardDeckCards),
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
