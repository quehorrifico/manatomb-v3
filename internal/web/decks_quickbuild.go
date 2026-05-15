package web

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"manatomb/app/internal/decks"
	"manatomb/app/internal/quickbuild"
)

type quickBuildRequestCard struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

type quickBuildRequest struct {
	DeckID        int64                   `json:"deck_id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	Tags          string                  `json:"tags"`
	CommanderName string                  `json:"commander_name"`
	Format        string                  `json:"format"`
	Seed          int64                   `json:"seed"`
	Cards         []quickBuildRequestCard `json:"cards"`
	Sideboard     []quickBuildRequestCard `json:"sideboard_cards"`
	MaybeCards    []quickBuildRequestCard `json:"maybe_cards"`
}

type quickBuildResponse struct {
	Workspace workspaceDeckState `json:"workspace"`
	Summary   quickbuild.Summary `json:"summary"`
}

func recommendedQuickBuildTags(summary quickbuild.Summary) []string {
	out := make([]string, 0, 2)
	seen := map[string]struct{}{}
	add := func(raw string) {
		if len(out) >= 2 {
			return
		}
		tag := decks.NormalizeDeckTag(raw)
		if tag == "" {
			return
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}

	add(summary.PrimaryTheme)

	strategy := decks.NormalizeDeckTag(summary.Strategy)
	if strategy != "" {
		if !(strings.EqualFold(strategy, "Midrange") && len(out) > 0) {
			add(strategy)
		}
	}

	for _, theme := range summary.Themes {
		if len(out) >= 2 {
			break
		}
		add(theme)
	}

	return out
}

func mergeQuickBuildTags(existing string, summary quickbuild.Summary) string {
	out := decks.SplitTags(existing)
	seen := make(map[string]struct{}, len(out))
	for _, tag := range out {
		seen[strings.ToLower(tag)] = struct{}{}
	}

	added := 0
	for _, tag := range recommendedQuickBuildTags(summary) {
		if added >= 2 {
			break
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
		added++
	}

	return decks.NormalizeTags(strings.Join(out, ", "))
}

func (a *App) HandleDeckQuickBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req quickBuildRequest
	if err := parseJSONBody(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	service := quickbuild.NewService(a.DB)
	if req.DeckID > 0 {
		a.handleSavedDeckQuickBuild(w, r, service, req)
		return
	}
	a.handleGuestDeckQuickBuild(w, r, service, req)
}

func (a *App) handleSavedDeckQuickBuild(w http.ResponseWriter, r *http.Request, service *quickbuild.Service, req quickBuildRequest) {
	user := CurrentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	deck, err := decks.GetDeck(r.Context(), a.DB, req.DeckID, user.ID)
	if err != nil {
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	deckCards, err := decks.ListDeckCards(r.Context(), a.DB, deck.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	maybeCards, err := decks.ListDeckMaybeCards(r.Context(), a.DB, deck.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	sideboardCards, err := decks.ListDeckSideboardCards(r.Context(), a.DB, deck.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	if !deckEligibleForQuickBuild(deck.Format, deck.CommanderName, deckCards, sideboardCards, maybeCards) {
		http.Error(w, "Quick Build is only available for empty commander decks with a commander selected.", http.StatusBadRequest)
		return
	}

	result, err := service.Build(r.Context(), quickbuild.Request{
		CommanderName: deck.CommanderName,
		Seed:          req.Seed,
	})
	if err != nil {
		writeQuickBuildError(w, err)
		return
	}

	if err := decks.SetMainboard(r.Context(), a.DB, deck.ID, quickBuildResultToDeckCardInputs(result)); err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	mergedTags := mergeQuickBuildTags(deck.Tags, result.Summary)
	if mergedTags != decks.NormalizeTags(deck.Tags) {
		if err := decks.UpdateDeckWithOptions(r.Context(), a.DB, deck.ID, decks.DeckInput{
			Name:          deck.Name,
			Description:   deck.Description,
			Tags:          mergedTags,
			Format:        deck.Format,
			CommanderName: deck.CommanderName,
			IsPublic:      deck.IsPublic,
			PublicSlug:    deck.PublicSlug,
			PowerBracket:  deck.PowerBracket,
		}); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
	}

	pageData, err := a.loadSavedDeckWorkspace(r.Context(), user.ID, deck.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, quickBuildResponse{
		Workspace: pageData.WorkspaceState,
		Summary:   result.Summary,
	})
}

func (a *App) handleGuestDeckQuickBuild(w http.ResponseWriter, r *http.Request, service *quickbuild.Service, req quickBuildRequest) {
	commanderName := strings.TrimSpace(req.CommanderName)
	format := defaultDeckFormat(req.Format, commanderName, "")
	if format != "Commander" {
		writeQuickBuildError(w, quickbuild.ErrUnsupportedRequest)
		return
	}
	if !guestRequestEligibleForQuickBuild(commanderName, format, req.Cards, req.Sideboard, req.MaybeCards) {
		http.Error(w, "Quick Build is only available for empty commander decks with a commander selected.", http.StatusBadRequest)
		return
	}

	result, err := service.Build(r.Context(), quickbuild.Request{
		CommanderName: commanderName,
		Seed:          req.Seed,
	})
	if err != nil {
		writeQuickBuildError(w, err)
		return
	}

	deckName := strings.TrimSpace(req.Name)
	if deckName == "" {
		deckName = "New Guest Deck"
	}
	mergedTags := mergeQuickBuildTags(req.Tags, result.Summary)

	fakeDeck := &decks.Deck{
		ID:            0,
		UserID:        0,
		Name:          deckName,
		Description:   strings.TrimSpace(req.Description),
		Tags:          mergedTags,
		Format:        "Commander",
		CommanderName: result.Commander.Name,
	}
	workspace := buildWorkspaceStateFromDeck(
		fakeDeck,
		quickBuildResultToDeckCards(result),
		nil,
		nil,
		[]commanderCandidate{{CardName: result.Commander.Name}},
		nil,
	)

	writeJSON(w, http.StatusOK, quickBuildResponse{
		Workspace: workspace,
		Summary:   result.Summary,
	})
}

func writeQuickBuildError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, quickbuild.ErrUnsupportedRequest):
		http.Error(w, "Quick Build is only available for commander decks.", http.StatusBadRequest)
	case errors.Is(err, quickbuild.ErrCommanderRequired):
		http.Error(w, "Quick Build needs a commander first.", http.StatusBadRequest)
	case errors.Is(err, quickbuild.ErrCommanderNotFound):
		http.Error(w, "That commander could not be found.", http.StatusNotFound)
	case errors.Is(err, quickbuild.ErrCommanderIllegal):
		http.Error(w, "Quick Build requires a commander-legal commander card.", http.StatusBadRequest)
	case errors.Is(err, quickbuild.ErrBuildNotPossible):
		http.Error(w, "Could not build a starter shell for that commander yet.", http.StatusBadRequest)
	default:
		http.Error(w, "Quick Build failed.", http.StatusInternalServerError)
	}
}

func deckEligibleForQuickBuild(format, commanderName string, deckCards, sideboardCards, maybeCards []decks.DeckCard) bool {
	if defaultDeckFormat(format, commanderName, "") != "Commander" {
		return false
	}
	if strings.TrimSpace(commanderName) == "" {
		return false
	}
	if deckCardQuantityTotal(sideboardCards) > 0 {
		return false
	}
	if deckCardQuantityTotal(maybeCards) > 0 {
		return false
	}
	return visibleDeckCardCount(deckCards, commanderName) == 0
}

func guestRequestEligibleForQuickBuild(commanderName, format string, cardsIn, sideboardCardsIn, maybeCardsIn []quickBuildRequestCard) bool {
	if defaultDeckFormat(format, commanderName, "") != "Commander" {
		return false
	}
	commanderName = strings.TrimSpace(commanderName)
	if commanderName == "" {
		return false
	}

	if quickBuildRequestCardTotal(maybeCardsIn) > 0 {
		return false
	}
	if quickBuildRequestCardTotal(sideboardCardsIn) > 0 {
		return false
	}

	visible := 0
	commanderKey := strings.ToLower(commanderName)
	for _, item := range cardsIn {
		name := strings.TrimSpace(item.Name)
		if name == "" || item.Qty <= 0 {
			continue
		}
		qty := item.Qty
		if strings.ToLower(name) == commanderKey {
			qty--
		}
		if qty > 0 {
			visible += qty
		}
	}
	return visible == 0
}

func quickBuildRequestCardTotal(cardsIn []quickBuildRequestCard) int {
	total := 0
	for _, item := range cardsIn {
		if item.Qty <= 0 {
			continue
		}
		total += item.Qty
	}
	return total
}

func quickBuildResultToDeckCardInputs(result quickbuild.Result) []decks.DeckCardInput {
	out := make([]decks.DeckCardInput, 0, len(result.Cards)+1)
	if result.Commander.OracleID != "" {
		out = append(out, decks.DeckCardInput{
			OracleID: result.Commander.OracleID,
			Qty:      1,
		})
	}
	for _, built := range result.Cards {
		if built.Qty <= 0 || strings.TrimSpace(built.Card.OracleID) == "" {
			continue
		}
		out = append(out, decks.DeckCardInput{
			OracleID: built.Card.OracleID,
			Qty:      built.Qty,
		})
	}
	return out
}

func quickBuildResultToDeckCards(result quickbuild.Result) []decks.DeckCard {
	out := make([]decks.DeckCard, 0, len(result.Cards)+1)
	appendCard := func(card quickbuild.CandidateCard, qty int) {
		if qty <= 0 || strings.TrimSpace(card.Name) == "" {
			return
		}
		out = append(out, decks.DeckCard{
			CardID:        strings.TrimSpace(card.OracleID),
			CardName:      strings.TrimSpace(card.Name),
			ManaCost:      strings.TrimSpace(card.ManaCost),
			ImageURI:      strings.TrimSpace(card.ImageURI),
			TypeLine:      strings.TrimSpace(card.TypeLine),
			OracleText:    strings.TrimSpace(card.OracleText),
			AllPartsJSON:  strings.TrimSpace(card.AllPartsJSON),
			CMC:           card.CMC,
			PriceUSD:      strings.TrimSpace(card.PriceUSD),
			ColorIdentity: strings.Join(card.ColorIdentity, ","),
			Quantity:      qty,
		})
	}

	appendCard(result.Commander, 1)
	for _, built := range result.Cards {
		appendCard(built.Card, built.Qty)
	}

	sort.Slice(out, func(i, j int) bool {
		left := strings.TrimSpace(out[i].CardName)
		right := strings.TrimSpace(out[j].CardName)
		if left == right {
			return out[i].CardID < out[j].CardID
		}
		return left < right
	})
	return out
}
