package web

import (
	"net/http"
	"strconv"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

// List all decks for the current user.
func (a *App) HandleDecksList(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userDecks, err := decks.ListDecksByUser(r.Context(), a.DB, user.ID)
	if err != nil {
		http.Error(w, "could not load decks", http.StatusInternalServerError)
		return
	}

	data := TemplateData{
		CurrentUser: user,
		Data:        userDecks,
		Flash:       flash,
	}

	a.Renderer.Render(w, "decks_list", data)
}

// Show "new deck" form. Supports optional ?commander_name=... from commander search.
func (a *App) HandleDeckNewShow(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Optional commander_name from query string (e.g., coming from commander search)
	commanderName := r.URL.Query().Get("commander_name")

	data := TemplateData{
		CurrentUser: user,
		Data: struct {
			CommanderName string
		}{
			CommanderName: commanderName,
		},
		Flash: flash,
	}

	a.Renderer.Render(w, "decks_new", data)
}

// Handle POST from "new deck" form.
func (a *App) HandleDeckNewPost(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	name := r.Form.Get("name")
	desc := r.Form.Get("description")
	commander := r.Form.Get("commander_name")

	d, err := decks.CreateDeck(r.Context(), a.DB, user.ID, name, desc, commander)
	if err != nil {
		http.Error(w, "could not create deck", http.StatusInternalServerError)
		return
	}

	setFlash(w, "Deck created.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(d.ID, 10), http.StatusSeeOther)
}

// Show a single deck, its cards, and commander details.
// Also handles POSTs to add/decrement cards.
func (a *App) HandleDeckShow(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Very simple param parsing: /decks/{id}
	idStr := r.URL.Path[len("/decks/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Handle add / decrement operations
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		cardName := r.Form.Get("card_name")
		cardIDStr := r.Form.Get("card_id")

		// Case 1: adding a new card by name (from the "Add card" form)
		if cardName != "" {
			c, err := cards.EnsureCardByName(r.Context(), a.DB, cardName)
			if err != nil {
				http.Error(w, "could not add card", http.StatusInternalServerError)
				return
			}
			// Add +1 copy
			if err := decks.AddCard(r.Context(), a.DB, id, c.ID, 1); err != nil {
				http.Error(w, "could not add card", http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 2: decrement an existing card by card_id
		if cardIDStr != "" {
			cardID, err := strconv.ParseInt(cardIDStr, 10, 64)
			if err != nil {
				http.Error(w, "invalid card id", http.StatusBadRequest)
				return
			}

			// Use delta = -1 to decrement; AddCard will delete row if quantity goes to 0
			if err := decks.AddCard(r.Context(), a.DB, id, cardID, -1); err != nil {
				http.Error(w, "could not update card", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		http.Error(w, "missing card information", http.StatusBadRequest)
		return
	}

	// GET: load deck, cards, and commander card details
	d, err := decks.GetDeck(r.Context(), a.DB, id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	deckCards, err := decks.ListDeckCards(r.Context(), a.DB, id)
	if err != nil {
		http.Error(w, "could not load deck cards", http.StatusInternalServerError)
		return
	}

	// Try to fetch commander details from Scryfall, if we have a commander name
	var commanderCard *cards.Card
	if d.CommanderName != "" {
		scry := cards.NewScryfallClient()
		results, err := scry.SearchByName(r.Context(), d.CommanderName+" is:commander")
		if err == nil && len(results) > 0 {
			commanderCard = &results[0]
		}
		// If there is an error or no results, we just leave commanderCard nil
	}

	type deckPageData struct {
		Deck      *decks.Deck
		DeckCards []decks.DeckCard
		Commander *cards.Card
	}

	data := TemplateData{
		CurrentUser: user,
		Data: deckPageData{
			Deck:      d,
			DeckCards: deckCards,
			Commander: commanderCard,
		},
		Flash: flash,
	}

	a.Renderer.Render(w, "deck_show", data)
}

func (a *App) HandleDeckEditShow(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	d, err := decks.GetDeck(r.Context(), a.DB, id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := TemplateData{
		CurrentUser: user,
		Data:        d,
		Flash:       flash,
	}

	a.Renderer.Render(w, "decks_edit", data)
}

func (a *App) HandleDeckEditPost(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	idStr := r.Form.Get("id")
	if idStr == "" {
		http.Error(w, "missing deck id", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid deck id", http.StatusBadRequest)
		return
	}

	// Ensure the deck belongs to this user
	if _, err := decks.GetDeck(r.Context(), a.DB, id, user.ID); err != nil {
		http.NotFound(w, r)
		return
	}

	name := r.Form.Get("name")
	desc := r.Form.Get("description")
	commander := r.Form.Get("commander_name")

	if err := decks.UpdateDeck(r.Context(), a.DB, id, name, desc, commander); err != nil {
		http.Error(w, "could not update deck", http.StatusInternalServerError)
		return
	}

	setFlash(w, "Deck updated.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) HandleDeckDeletePost(w http.ResponseWriter, r *http.Request) {
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

	idStr := r.Form.Get("id")
	if idStr == "" {
		http.Error(w, "missing deck id", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid deck id", http.StatusBadRequest)
		return
	}

	if _, err := decks.GetDeck(r.Context(), a.DB, id, user.ID); err != nil {
		http.NotFound(w, r)
		return
	}

	if err := decks.DeleteDeck(r.Context(), a.DB, id); err != nil {
		http.Error(w, "could not delete deck", http.StatusInternalServerError)
		return
	}

	setFlash(w, "Deck deleted.")
	http.Redirect(w, r, "/decks", http.StatusSeeOther)
}
