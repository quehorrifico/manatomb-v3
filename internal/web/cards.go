package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

// searchResult is a shared view model for both general card search and
// commander search. It flattens the core fields we display in the grid plus
// a JSON-encoded faces slice for MDFC / multi-faced cards.
type searchResult struct {
	OracleID   string
	Name       string
	ManaCost   string
	TypeLine   string
	OracleText string
	ImageURI   string
	PriceUSD   string
	Artist     string
	SetCode    string
	SetName    string
	ReleasedAt string

	// FacesJSON is a JSON-encoded []cards.CardFace (from cards.Card.Faces).
	// It is used by the frontend to support MDFC "flip" behavior in the detail modals.
	FacesJSON string
}

type cardResolveResponse struct {
	OracleID   string            `json:"oracle_id,omitempty"`
	Name       string            `json:"name"`
	ManaCost   string            `json:"mana_cost,omitempty"`
	TypeLine   string            `json:"type_line,omitempty"`
	OracleText string            `json:"oracle_text,omitempty"`
	CMC        float64           `json:"cmc"`
	ImageURIs  map[string]string `json:"image_uris,omitempty"`
	Prices     struct {
		USD string `json:"usd,omitempty"`
	} `json:"prices,omitempty"`
}

type cardVersionResponse struct {
	ScryfallID      string           `json:"scryfall_id,omitempty"`
	Lang            string           `json:"lang,omitempty"`
	Name            string           `json:"name"`
	ManaCost        string           `json:"mana_cost,omitempty"`
	TypeLine        string           `json:"type_line,omitempty"`
	OracleText      string           `json:"oracle_text,omitempty"`
	ImageURI        string           `json:"image_uri,omitempty"`
	PriceUSD        string           `json:"price_usd,omitempty"`
	Artist          string           `json:"artist,omitempty"`
	SetCode         string           `json:"set_code,omitempty"`
	SetName         string           `json:"set_name,omitempty"`
	CollectorNumber string           `json:"collector_number,omitempty"`
	Rarity          string           `json:"rarity,omitempty"`
	ReleasedAt      string           `json:"released_at,omitempty"`
	Faces           []cards.CardFace `json:"faces,omitempty"`
}

type cardVersionsPayload struct {
	Versions []cardVersionResponse `json:"versions"`
}

type deckCardSearchResult struct {
	OracleID string `json:"oracle_id"`
	Name     string `json:"name"`
}

type deckCardSearchPayload struct {
	Exact   bool                   `json:"exact"`
	Results []deckCardSearchResult `json:"results"`
}

// buildSearchResults converts a slice of cards.Card into a slice of searchResult,
// pre-encoding the Faces slice into JSON for MDFC support.
func buildSearchResults(cardsIn []cards.Card) []searchResult {
	viewResults := make([]searchResult, 0, len(cardsIn))

	for _, c := range cardsIn {
		facesJSON := ""
		if len(c.Faces) > 0 {
			if b, err := json.Marshal(c.Faces); err == nil {
				facesJSON = string(b)
			}
		}

		viewResults = append(viewResults, searchResult{
			OracleID:   c.OracleID,
			Name:       c.Name,
			ManaCost:   c.ManaCost,
			TypeLine:   c.TypeLine,
			OracleText: c.OracleText,
			ImageURI:   c.ImageURI,
			PriceUSD:   c.PriceUSD,
			Artist:     c.Artist,
			SetCode:    c.SetCode,
			SetName:    c.SetName,
			ReleasedAt: c.ReleasedAt,
			FacesJSON:  facesJSON,
		})
	}

	return viewResults
}

func normalizeLocalReturnPath(raw, fallback string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return fallback
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return fallback
	}
	return path
}

func (a *App) HandleCardSearch(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	q := r.URL.Query()
	query := strings.TrimSpace(q.Get("q"))
	colorParams := q["color"]                      // e.g. ["W", "U"]
	typeFilter := strings.TrimSpace(q.Get("type")) // e.g. "creature"

	hasFilters := len(colorParams) > 0 || typeFilter != ""
	hasSearched := query != "" || hasFilters

	var results []cards.Card
	var errMsg string

	if hasSearched {
		found, err := cards.SearchCards(r.Context(), a.DB, cards.CardSearchParams{
			Query:         query,
			TypeFilter:    typeFilter,
			ColorIdentity: colorParams,
			CommanderOnly: false,
			Limit:         120,
		})
		if err != nil {
			errMsg = "We couldn't search for cards right now. Please try again."
		} else if len(found) == 0 {
			if query == "" && hasFilters {
				errMsg = "No cards matched your filters."
			} else {
				errMsg = fmt.Sprintf("No cards found for “%s”. Please check the spelling or filters.", query)
			}
		} else {
			results = found
		}
	}

	// Build view-model results with pre-encoded faces JSON for MDFC support.
	viewResults := buildSearchResults(results)

	var userDecks []decks.Deck
	if user != nil {
		var err error
		userDecks, err = decks.ListDecksByUser(r.Context(), a.DB, user.ID)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
	}

	data := TemplateData{
		CurrentUser: user,
		Data: struct {
			Query       string
			Results     []searchResult
			Decks       []decks.Deck
			HasSearched bool
		}{
			Query:       query,
			Results:     viewResults,
			Decks:       userDecks,
			HasSearched: hasSearched,
		},
		Flash: flash,
		Error: errMsg, // 🔴 shows in red banner via layout_header
	}

	a.Renderer.Render(w, "cards_search", data)
}

func (a *App) HandleCardAddToDeck(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	deckIDStr := r.Form.Get("deck_id")
	cardName := r.Form.Get("card_name")

	if deckIDStr == "" || cardName == "" {
		http.Error(w, "missing deck or card", http.StatusBadRequest)
		return
	}

	deckID, err := strconv.ParseInt(deckIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid deck id", http.StatusBadRequest)
		return
	}

	// Ensure deck belongs to current user
	if _, err := decks.GetDeck(r.Context(), a.DB, deckID, user.ID); err != nil {
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	// Resolve best local card match so card add uses the same fuzzy logic as search.
	resolved, err := cards.ResolveCardByNameFuzzy(r.Context(), a.DB, cardName)
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			http.Error(w, "card not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not add card", http.StatusInternalServerError)
		return
	}
	if err := decks.AddCard(r.Context(), a.DB, deckID, resolved.OracleID, 1); err != nil {
		http.Error(w, "could not add card", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
}

func (a *App) HandleCardResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "missing query", http.StatusBadRequest)
		return
	}

	card, err := cards.ResolveCardByNameFuzzy(r.Context(), a.DB, query)
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			http.Error(w, "card not found", http.StatusNotFound)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	resp := cardResolveResponse{
		OracleID:   card.OracleID,
		Name:       card.Name,
		ManaCost:   card.ManaCost,
		TypeLine:   card.TypeLine,
		OracleText: card.OracleText,
		CMC:        card.CMC,
	}
	if card.ImageURI != "" {
		resp.ImageURIs = map[string]string{
			"normal": card.ImageURI,
		}
	}
	resp.Prices.USD = card.PriceUSD

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleDeckCardSearch powers deckbuilder search UX:
// return one exact match when present; otherwise return fuzzy candidates.
func (a *App) HandleDeckCardSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "missing query", http.StatusBadRequest)
		return
	}

	limit := 20
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil {
			limit = parsed
		}
	}
	commanderOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("commander_only")), "true")

	results, exact, err := cards.SearchCardNamesExactThenFuzzy(r.Context(), a.DB, query, limit, commanderOnly)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	payload := deckCardSearchPayload{
		Exact:   exact,
		Results: make([]deckCardSearchResult, 0, len(results)),
	}
	for _, card := range results {
		payload.Results = append(payload.Results, deckCardSearchResult{
			OracleID: card.OracleID,
			Name:     card.Name,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *App) HandleCardVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	oracleID := strings.TrimSpace(r.URL.Query().Get("oracle_id"))
	if name == "" && oracleID == "" {
		http.Error(w, "missing name or oracle_id", http.StatusBadRequest)
		return
	}

	var (
		versions []cards.Card
		err      error
	)
	if oracleID != "" {
		versions, err = cards.ListCardVersionsByOracleID(r.Context(), a.DB, oracleID, 500)
		if err != nil && errors.Is(err, cards.ErrCardNotFound) && name != "" {
			versions, err = cards.ListCardVersionsByName(r.Context(), a.DB, name, 500)
		}
	} else {
		versions, err = cards.ListCardVersionsByName(r.Context(), a.DB, name, 500)
	}
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			http.Error(w, "card not found", http.StatusNotFound)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	payload := cardVersionsPayload{
		Versions: make([]cardVersionResponse, 0, len(versions)),
	}
	for _, c := range versions {
		payload.Versions = append(payload.Versions, cardVersionResponse{
			ScryfallID:      c.ID,
			Lang:            c.Lang,
			Name:            c.Name,
			ManaCost:        c.ManaCost,
			TypeLine:        c.TypeLine,
			OracleText:      c.OracleText,
			ImageURI:        c.ImageURI,
			PriceUSD:        c.PriceUSD,
			Artist:          c.Artist,
			SetCode:         c.SetCode,
			SetName:         c.SetName,
			CollectorNumber: c.CollectorNumber,
			Rarity:          c.Rarity,
			ReleasedAt:      c.ReleasedAt,
			Faces:           c.Faces,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *App) HandleCommanderSearch(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	hasSearched := query != ""

	var rawResults []cards.Card
	var errMsg string

	if hasSearched {
		found, err := cards.SearchCards(r.Context(), a.DB, cards.CardSearchParams{
			Query:         query,
			CommanderOnly: true,
			Limit:         120,
		})
		if err != nil {
			errMsg = "There was a problem searching for commanders. Please try again."
		} else {
			rawResults = found
		}
	}

	// Build view-model results with pre-encoded faces JSON for MDFC commanders.
	viewResults := buildSearchResults(rawResults)

	recommendedResults := make([]searchResult, 0)
	if recommended, err := cards.RandomTopCommanders(r.Context(), a.DB, 6, 1500); err == nil {
		recommendedResults = buildSearchResults(recommended)
	}

	fallbackReturn := "/decks/new"
	returnTo := normalizeLocalReturnPath(r.URL.Query().Get("return_to"), fallbackReturn)
	if strings.HasPrefix(returnTo, "/decks/new") && strings.Contains(returnTo, "mode=commander") {
		returnTo = "/decks/new"
	}

	data := TemplateData{
		CurrentUser: user,
		Data: struct {
			Query       string
			Results     []searchResult
			Recommended []searchResult
			ReturnTo    string
		}{
			Query:       query,
			Results:     viewResults,
			Recommended: recommendedResults,
			ReturnTo:    returnTo,
		},
		Flash: flash,
		Error: errMsg,
	}

	a.Renderer.Render(w, "commanders_search", data)
}
