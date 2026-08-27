package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"

	"github.com/google/uuid"
)

const (
	commanderRecommendationPageSize      = 8
	commanderRecommendationMaxEDHRecRank = 1500
)

type commanderDeckBuilderPageData struct {
	Query                  string
	Results                []searchResult
	Recommended            []searchResult
	RecommendationSeed     string
	RecommendationCursor   string
	HasMoreRecommendations bool
}

type commanderDeckBuilderState struct {
	Query string
}

type commanderRecommendationChoice struct {
	OracleID   string `json:"oracle_id"`
	ScryfallID string `json:"scryfall_id"`
	Name       string `json:"name"`
	ImageURI   string `json:"image_uri"`
}

type commanderRecommendationsResponse struct {
	Items      []commanderRecommendationChoice `json:"items"`
	NextCursor string                          `json:"next_cursor"`
	HasMore    bool                            `json:"has_more"`
}

func commanderDeckBuilderPath(state commanderDeckBuilderState) string {
	values := url.Values{}

	if query := strings.TrimSpace(state.Query); query != "" {
		values.Set("q", query)
	}

	path := "/decks/new/commander/"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

func commanderDeckBuilderStateFromValues(values url.Values) commanderDeckBuilderState {
	return commanderDeckBuilderState{
		Query: strings.TrimSpace(values.Get("q")),
	}
}

func (a *App) startCommanderDeck(w http.ResponseWriter, r *http.Request, commanderName, commanderPrintID string) {
	commanderName = strings.TrimSpace(commanderName)
	commanderPrintID = strings.TrimSpace(commanderPrintID)
	if commanderName == "" {
		a.renderDeckCommanderBuilder(w, r, CurrentUser(r), "", "Choose a commander first.", commanderDeckBuilderState{})
		return
	}

	http.Redirect(w, r, deckWorkbenchPath(deckWorkbenchOptions{
		Format:           "Commander",
		CommanderName:    commanderName,
		CommanderPrintID: commanderPrintID,
		Reset:            true,
	}), http.StatusSeeOther)
}

func (a *App) loadCommanderRecommendations(
	ctx context.Context,
	seed string,
	afterOracleID string,
) ([]searchResult, string, bool, error) {
	found, err := cards.RandomizedTopCommanderBatch(
		ctx,
		a.DB,
		seed,
		afterOracleID,
		commanderRecommendationPageSize+1,
		commanderRecommendationMaxEDHRecRank,
	)
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(found) > commanderRecommendationPageSize
	if hasMore {
		found = found[:commanderRecommendationPageSize]
	}

	nextCursor := strings.TrimSpace(afterOracleID)
	if len(found) > 0 {
		nextCursor = strings.TrimSpace(found[len(found)-1].OracleID)
	}

	return buildSearchResults(found), nextCursor, hasMore, nil
}

func (a *App) renderDeckCommanderBuilder(
	w http.ResponseWriter,
	r *http.Request,
	user *account.User,
	flash string,
	errMsg string,
	state commanderDeckBuilderState,
) {
	var (
		searchResultsRaw       []cards.Card
		recommended            []searchResult
		recommendationSeed     string
		recommendationCursor   string
		hasMoreRecommendations bool
	)

	if state.Query != "" {
		found, err := cards.SearchCards(r.Context(), a.DB, cards.CardSearchParams{
			Query:         state.Query,
			CommanderOnly: true,
			Limit:         18,
		})
		if err != nil {
			if errMsg == "" {
				errMsg = "There was a problem searching for commanders. Please try again."
			}
		} else {
			searchResultsRaw = found
		}
	}

	if state.Query == "" {
		recommendationSeed = uuid.NewString()
		if choices, nextCursor, hasMore, err := a.loadCommanderRecommendations(r.Context(), recommendationSeed, ""); err == nil {
			recommended = choices
			recommendationCursor = nextCursor
			hasMoreRecommendations = hasMore
		}
	}

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		Error:       errMsg,
		Data: commanderDeckBuilderPageData{
			Query:                  state.Query,
			Results:                buildSearchResults(searchResultsRaw),
			Recommended:            recommended,
			RecommendationSeed:     recommendationSeed,
			RecommendationCursor:   recommendationCursor,
			HasMoreRecommendations: hasMoreRecommendations,
		},
	}

	a.Renderer.Render(w, "decks_new_commander", data)
}

func (a *App) HandleDeckNewCommanderMore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	seed := strings.TrimSpace(r.URL.Query().Get("seed"))
	if _, err := uuid.Parse(seed); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid commander shuffle seed"})
		return
	}

	afterOracleID := strings.TrimSpace(r.URL.Query().Get("after"))
	if afterOracleID != "" {
		if _, err := uuid.Parse(afterOracleID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid commander cursor"})
			return
		}
	}

	items, nextCursor, hasMore, err := a.loadCommanderRecommendations(r.Context(), seed, afterOracleID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "commanders are unavailable right now"})
		return
	}

	choices := make([]commanderRecommendationChoice, 0, len(items))
	for _, item := range items {
		choices = append(choices, commanderRecommendationChoice{
			OracleID:   item.OracleID,
			ScryfallID: item.ScryfallID,
			Name:       item.Name,
			ImageURI:   item.ImageURI,
		})
	}

	writeJSON(w, http.StatusOK, commanderRecommendationsResponse{
		Items:      choices,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}

func (a *App) HandleDeckNewCommanderRedirect(w http.ResponseWriter, r *http.Request) {
	target := "/decks/new/commander/"
	if raw := r.URL.RawQuery; raw != "" {
		target += "?" + raw
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (a *App) HandleDeckNewCommanderShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.renderDeckCommanderBuilder(
		w,
		r,
		CurrentUser(r),
		readFlash(w, r),
		"",
		commanderDeckBuilderStateFromValues(r.URL.Query()),
	)
}
