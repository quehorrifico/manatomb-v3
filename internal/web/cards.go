package web

import (
	"encoding/json"
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
	Name       string
	ManaCost   string
	TypeLine   string
	OracleText string
	ImageURI   string
	PriceUSD   string
	Artist     string

	// FacesJSON is a JSON-encoded []cards.CardFace (from cards.Card.Faces).
	// It is used by the frontend to support MDFC "flip" behavior in the detail modals.
	FacesJSON string
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
			Name:       c.Name,
			ManaCost:   c.ManaCost,
			TypeLine:   c.TypeLine,
			OracleText: c.OracleText,
			ImageURI:   c.ImageURI,
			PriceUSD:   c.PriceUSD,
			Artist:     c.Artist,
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

	// Ensure card exists in cards table
	dbCard, err := cards.EnsureCardByName(r.Context(), a.DB, cardName)
	if err != nil {
		http.Error(w, "could not add card", http.StatusInternalServerError)
		return
	}

	if err := decks.AddCard(r.Context(), a.DB, deckID, dbCard.ID, 1); err != nil {
		http.Error(w, "could not add card", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
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

	fallbackReturn := "/decks/new"
	if user != nil {
		fallbackReturn = "/decks/new?mode=commander"
	}
	returnTo := normalizeLocalReturnPath(r.URL.Query().Get("return_to"), fallbackReturn)

	data := TemplateData{
		CurrentUser: user,
		Data: struct {
			Query    string
			Results  []searchResult
			ReturnTo string
		}{
			Query:    query,
			Results:  viewResults,
			ReturnTo: returnTo,
		},
		Flash: flash,
		Error: errMsg,
	}

	a.Renderer.Render(w, "commanders_search", data)
}
