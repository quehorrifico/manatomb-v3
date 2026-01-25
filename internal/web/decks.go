package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

// deckListItem is a lightweight view model for rendering the "My Decks" page.
// It includes the core deck fields plus an optional CommanderImageURI for UI use.
type deckListItem struct {
	ID                int64
	Name              string
	Description       string
	CommanderName     string
	CommanderImageURI string
}

type importDraftRequest struct {
	CommanderName string `json:"commander_name"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Cards         []struct {
		Name string `json:"name"`
		Qty  int    `json:"qty"`
	} `json:"cards"`
}

type importDraftResponse struct {
	DeckID int64 `json:"deck_id"`
}

func (a *App) HandleDeckImportDraft(w http.ResponseWriter, r *http.Request) {
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req importDraftRequest
	if err := parseJSONBody(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	commander := strings.TrimSpace(req.CommanderName)
	name := strings.TrimSpace(req.Name)
	desc := strings.TrimSpace(req.Description)

	if commander == "" {
		http.Error(w, "missing commander_name", http.StatusBadRequest)
		return
	}
	if name == "" {
		name = commander
	}

	// Create the saved deck
	d, err := decks.CreateDeck(r.Context(), a.DB, user.ID, name, desc, commander)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	// Add cards; skip unknown cards instead of failing the whole import
	for _, item := range req.Cards {
		cardName := strings.TrimSpace(item.Name)
		qty := item.Qty
		if cardName == "" || qty <= 0 {
			continue
		}

		card, err := cards.EnsureCardByName(r.Context(), a.DB, cardName)
		if err != nil {
			continue
		}

		_ = decks.AddCard(r.Context(), a.DB, d.ID, card.ID, qty) // best-effort
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(importDraftResponse{DeckID: d.ID})
}

// currentUserOrRedirect ensures there is a logged-in user, otherwise redirects
// to the login page and returns nil.
func (a *App) currentUserOrRedirect(w http.ResponseWriter, r *http.Request) *account.User {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}
	return user
}

// parseDeckIDFromPath extracts the deck ID from a path like /decks/{id}.
func parseDeckIDFromPath(r *http.Request) (int64, error) {
	const prefix = "/decks/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return 0, fmt.Errorf("invalid deck path")
	}
	idStr := strings.TrimPrefix(r.URL.Path, prefix)
	if idStr == "" {
		return 0, fmt.Errorf("missing deck id in path")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid deck id")
	}
	return id, nil
}

// parseDeckIDFromForm extracts the deck ID from a submitted form (field "id").
func parseDeckIDFromForm(r *http.Request) (int64, error) {
	idStr := strings.TrimSpace(r.Form.Get("id"))
	if idStr == "" {
		return 0, errors.New("missing deck id")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid deck id")
	}
	return id, nil
}

// List all decks for the current user.
func (a *App) HandleDecksList(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}

	userDecks, err := decks.ListDecksByUser(r.Context(), a.DB, user.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	// Build a view model with optional commander art URIs, fetched via EnsureCardByName.
	items := make([]deckListItem, 0, len(userDecks))

	for _, d := range userDecks {
		item := deckListItem{
			ID:            d.ID,
			Name:          d.Name,
			Description:   d.Description,
			CommanderName: d.CommanderName,
		}

		commanderName := strings.TrimSpace(d.CommanderName)
		if commanderName != "" {
			if c, err := cards.EnsureCardByName(r.Context(), a.DB, commanderName); err == nil {
				if c.ImageURI != "" {
					item.CommanderImageURI = c.ImageURI
				}
			}
			// If EnsureCardByName fails or has no image, we just show the placeholder in the UI.
		}

		items = append(items, item)
	}

	data := TemplateData{
		CurrentUser: user,
		Data:        items,
		Flash:       flash,
	}

	a.Renderer.Render(w, "decks_list", data)
}

func (a *App) HandleDeckNewShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	user := CurrentUser(r)

	commanderName := strings.TrimSpace(r.URL.Query().Get("commander_name"))
	if commanderName == "" {
		// No commander yet: push the user to the commander search flow first.
		http.Redirect(w, r, "/commanders/search", http.StatusSeeOther)
		return
	}

	data := TemplateData{
		CurrentUser: user,
		Data: struct {
			CommanderName string
			Name          string
			Description   string
		}{
			CommanderName: commanderName,
			Name:          "",
			Description:   "",
		},
		Flash: flash,
		Error: "",
	}

	a.Renderer.Render(w, "decks_new", data)
}

// Handle POST from "new deck" form.
func (a *App) HandleDeckNewPost(w http.ResponseWriter, r *http.Request) {
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.Form.Get("name"))
	desc := strings.TrimSpace(r.Form.Get("description"))
	commander := strings.TrimSpace(r.Form.Get("commander_name"))

	// Require a commander
	if commander == "" {
		data := TemplateData{
			CurrentUser: user,
			Data: struct {
				CommanderName string
				Name          string
				Description   string
			}{
				CommanderName: commander,
				Name:          name,
				Description:   desc,
			},
			Error: "Please choose a commander first.",
		}
		a.Renderer.Render(w, "decks_new", data)
		return
	}

	// Basic validation: require a name
	if name == "" {
		data := TemplateData{
			CurrentUser: user,
			Data: struct {
				CommanderName string
				Name          string
				Description   string
			}{
				CommanderName: commander,
				Name:          name,
				Description:   desc,
			},
			Error: "Deck name is required.",
		}
		a.Renderer.Render(w, "decks_new", data)
		return
	}

	d, err := decks.CreateDeck(r.Context(), a.DB, user.ID, name, desc, commander)
	if err != nil {
		// Use our pretty 500 page + logging
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Deck created.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(d.ID, 10), http.StatusSeeOther)
}

// Guest deck builder (no DB persistence).
// Renders the same deck page UI but uses client-side localStorage for the deck contents.
func (a *App) HandleGuestDeckShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	user := CurrentUser(r)

	commanderName := strings.TrimSpace(r.URL.Query().Get("commander_name"))
	if commanderName == "" {
		http.Redirect(w, r, "/commanders/search", http.StatusSeeOther)
		return
	}

	// Fake deck object (ID=0) so the template can render.
	fakeDeck := &decks.Deck{
		ID:            0,
		UserID:        0,
		Name:          "New Deck",
		Description:   "",
		CommanderName: commanderName,
	}

	// Try to fetch commander details from Scryfall for nicer UI.
	var commanderCard *cards.Card
	if commanderName != "" {
		scry := cards.NewScryfallClient()
		results, err := scry.SearchByName(r.Context(), commanderName+" is:commander")
		if err == nil && len(results) > 0 {
			commanderCard = &results[0]
		}
	}

	type deckPageData struct {
		Deck      *decks.Deck
		DeckCards []decks.DeckCard
		Commander *cards.Card
		GuestMode bool
	}

	data := TemplateData{
		CurrentUser: user, // will be nil for guests; template handles guest mode
		Data: deckPageData{
			Deck:      fakeDeck,
			DeckCards: nil,
			Commander: commanderCard,
			GuestMode: true,
		},
		Flash: flash,
	}

	a.Renderer.Render(w, "deck_show", data)
}

// Handle creating a new deck directly from a chosen commander.
//
// This is intended to be called from the commander search page when the user
// clicks "Use as commander". It will:
//   - default the deck name to the commander name (if no explicit name is given)
//   - create the deck with an empty description
//   - redirect straight to the deck show page.
func (a *App) HandleDeckCreateFromCommander(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	commander := strings.TrimSpace(r.FormValue("commander_name"))
	if commander == "" {
		// No commander provided – send user back to commander search with a friendly message.
		setFlash(w, "Please choose a commander first.")
		http.Redirect(w, r, "/commanders/search", http.StatusSeeOther)
		return
	}

	// If the user is not logged in, send them to the guest deck builder.
	if user == nil {
		http.Redirect(w, r, "/decks/guest?commander_name="+url.QueryEscape(commander), http.StatusSeeOther)
		return
	}

	// Optional deck name (for future flexibility); default to commander name.
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = commander
	}

	// For this flow we start with an empty description.
	desc := ""

	d, err := decks.CreateDeck(r.Context(), a.DB, user.ID, name, desc, commander)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Deck created.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(d.ID, 10), http.StatusSeeOther)
}

// Show a single deck, its cards, and commander details.
// Also handles POSTs to add/decrement cards.
func (a *App) HandleDeckShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}

	id, err := parseDeckIDFromPath(r)
	if err != nil {
		a.RenderNotFound(w, r)
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
				// If the card doesn't exist on Scryfall, show a friendly error on the deck page.
				if errors.Is(err, cards.ErrCardNotFound) {
					d, derr := decks.GetDeck(r.Context(), a.DB, id, user.ID)
					if derr != nil {
						a.RenderNotFound(w, r)
						return
					}

					deckCards, derr := decks.ListDeckCards(r.Context(), a.DB, id)
					if derr != nil {
						a.RenderServerError(w, r, derr)
						return
					}

					// Try to fetch commander details again (optional).
					var commanderCard *cards.Card
					if d.CommanderName != "" {
						scry := cards.NewScryfallClient()
						results, serr := scry.SearchByName(r.Context(), d.CommanderName+" is:commander")
						if serr == nil && len(results) > 0 {
							commanderCard = &results[0]
						}
					}

					type deckPageData struct {
						Deck      *decks.Deck
						DeckCards []decks.DeckCard
						Commander *cards.Card
						GuestMode bool
					}

					data := TemplateData{
						CurrentUser: user,
						Data: deckPageData{
							Deck:      d,
							DeckCards: deckCards,
							Commander: commanderCard,
							GuestMode: false,
						},
						Flash: flash,
						Error: fmt.Sprintf("No card found named “%s”. Please check the spelling.", cardName),
					}

					a.Renderer.Render(w, "deck_show", data)
					return
				}

				// Unexpected error: treat as real 500.
				a.RenderServerError(w, r, err)
				return
			}

			// Valid card, add +1 copy
			if err := decks.AddCard(r.Context(), a.DB, id, c.ID, 1); err != nil {
				a.RenderServerError(w, r, err)
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
				a.RenderServerError(w, r, err)
				return
			}

			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		http.Error(w, "missing card information", http.StatusBadRequest)
		return
	}

	// GET: load deck, cards, and commander details
	d, err := decks.GetDeck(r.Context(), a.DB, id, user.ID)
	if err != nil {
		a.RenderNotFound(w, r)
		return
	}

	deckCards, err := decks.ListDeckCards(r.Context(), a.DB, id)
	if err != nil {
		a.RenderServerError(w, r, err)
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
		GuestMode bool
	}

	data := TemplateData{
		CurrentUser: user,
		Data: deckPageData{
			Deck:      d,
			DeckCards: deckCards,
			Commander: commanderCard,
			GuestMode: false,
		},
		Flash: flash,
	}

	a.Renderer.Render(w, "deck_show", data)
}

func (a *App) HandleDeckEditShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		a.RenderNotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		a.RenderNotFound(w, r)
		return
	}

	d, err := decks.GetDeck(r.Context(), a.DB, id, user.ID)
	if err != nil {
		a.RenderNotFound(w, r)
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
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	id, err := parseDeckIDFromForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := decks.GetDeck(r.Context(), a.DB, id, user.ID); err != nil {
		a.RenderNotFound(w, r)
		return
	}

	name := r.Form.Get("name")
	desc := r.Form.Get("description")
	commander := r.Form.Get("commander_name")

	if err := decks.UpdateDeck(r.Context(), a.DB, id, name, desc, commander); err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Deck updated.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) HandleDeckDeletePost(w http.ResponseWriter, r *http.Request) {
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
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

	id, err := parseDeckIDFromForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := decks.GetDeck(r.Context(), a.DB, id, user.ID); err != nil {
		a.RenderNotFound(w, r)
		return
	}

	if err := decks.DeleteDeck(r.Context(), a.DB, id); err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Deck deleted.")
	http.Redirect(w, r, "/decks", http.StatusSeeOther)
}

func parseJSONBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
