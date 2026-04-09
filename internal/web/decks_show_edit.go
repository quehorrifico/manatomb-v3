package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

func deckCardIDForName(deckCards []decks.DeckCard, cardName string) (string, bool) {
	targetName := strings.TrimSpace(cardName)
	if targetName == "" {
		return "", false
	}

	for _, dc := range deckCards {
		if strings.EqualFold(strings.TrimSpace(dc.CardName), targetName) {
			return dc.CardID, true
		}
	}
	return "", false
}

// Show a single deck, its cards, and commander details.
// Also handles POSTs to add/decrement cards.
func (a *App) HandleDeckShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}
	respondJSON := wantsJSONResponse(r)

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
		cardID := strings.TrimSpace(r.Form.Get("card_id"))
		maybeCardID := strings.TrimSpace(r.Form.Get("maybe_card_id"))
		action := strings.TrimSpace(r.Form.Get("action"))
		zone := strings.TrimSpace(r.Form.Get("zone"))

		if action == "save_overview" {
			current, err := decks.GetDeck(r.Context(), a.DB, id, user.ID)
			if err != nil {
				a.RenderNotFound(w, r)
				return
			}

			format := current.Format
			if _, ok := r.Form["format"]; ok {
				format = defaultDeckFormat(r.Form.Get("format"), current.CommanderName, "")
			} else {
				format = defaultDeckFormat(current.Format, current.CommanderName, "")
			}
			commander, err := a.commanderForFormatChange(r.Context(), id, current.Format, format, current.CommanderName)
			if err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			powerBracket := defaultDeckPowerBracket(current.PowerBracket, format)

			if err := decks.UpdateDeckWithOptions(r.Context(), a.DB, id, decks.DeckInput{
				Name:          current.Name,
				Description:   strings.TrimSpace(r.Form.Get("description")),
				Tags:          strings.TrimSpace(r.Form.Get("tags")),
				Format:        format,
				CommanderName: commander,
				IsPublic:      current.IsPublic,
				PublicSlug:    current.PublicSlug,
				PowerBracket:  powerBracket,
			}); err != nil {
				a.RenderServerError(w, r, err)
				return
			}

			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
			setFlash(w, "Overview updated.")
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 1: adding a new card by name (from the "Add card" form)
		if cardName != "" {
			resolved, err := cards.ResolveCardByNameFuzzy(r.Context(), a.DB, cardName)
			if err != nil {
				// If we can't find a suitable local card match, show a friendly error on the deck page.
				if errors.Is(err, cards.ErrCardNotFound) {
					if respondJSON {
						http.Error(w, fmt.Sprintf("No card found matching %q. Please check the spelling.", cardName), http.StatusNotFound)
						return
					}
					pageData, derr := a.loadSavedDeckWorkspace(r.Context(), user.ID, id)
					if derr != nil {
						a.RenderServerError(w, r, derr)
						return
					}

					data := TemplateData{
						CurrentUser: user,
						Data:        pageData,
						Flash:       flash,
						Error:       fmt.Sprintf("No card found matching \"%s\". Please check the spelling.", cardName),
						WideLayout:  true,
					}

					a.Renderer.Render(w, "deck_show", data)
					return
				}

				// Unexpected error: treat as real 500.
				a.RenderServerError(w, r, err)
				return
			}

			// Valid card, add +1 copy to the chosen zone.
			var addErr error
			targetCardID := resolved.OracleID
			resolvedName := strings.TrimSpace(resolved.Name)
			if zone == "maybe" {
				maybeDeckCards, err := decks.ListDeckMaybeCards(r.Context(), a.DB, id)
				if err != nil {
					a.RenderServerError(w, r, err)
					return
				}
				if existingID, ok := deckCardIDForName(maybeDeckCards, resolvedName); ok {
					targetCardID = existingID
				}
				addErr = decks.AddMaybeCard(r.Context(), a.DB, id, targetCardID, 1)
			} else {
				deckCards, err := decks.ListDeckCards(r.Context(), a.DB, id)
				if err != nil {
					a.RenderServerError(w, r, err)
					return
				}
				if existingID, ok := deckCardIDForName(deckCards, resolvedName); ok {
					targetCardID = existingID
				}
				addErr = decks.AddCard(r.Context(), a.DB, id, targetCardID, 1)
			}
			if addErr != nil {
				a.RenderServerError(w, r, addErr)
				return
			}

			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 2: move a mainboard card to the maybeboard.
		if action == "inc_main" && cardID != "" {
			if err := decks.AddCard(r.Context(), a.DB, id, cardID, 1); err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 3: increment an existing maybeboard card by maybe_card_id.
		if action == "inc_maybe" && maybeCardID != "" {
			if err := decks.AddMaybeCard(r.Context(), a.DB, id, maybeCardID, 1); err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 4: move a mainboard card to the maybeboard.
		if action == "to_maybe" && cardID != "" {
			if err := decks.MoveCardToMaybe(r.Context(), a.DB, id, cardID); err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 5: move a maybeboard card to the mainboard.
		if action == "to_main" && maybeCardID != "" {
			if err := decks.MoveMaybeToDeck(r.Context(), a.DB, id, maybeCardID); err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 6: decrement an existing mainboard card by card_id.
		if cardID != "" {
			// Use delta = -1 to decrement; AddCard will delete row if quantity goes to 0
			if err := decks.AddCard(r.Context(), a.DB, id, cardID, -1); err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 7: decrement an existing maybeboard card by maybe_card_id.
		if maybeCardID != "" {
			if err := decks.AddMaybeCard(r.Context(), a.DB, id, maybeCardID, -1); err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		http.Error(w, "missing card information", http.StatusBadRequest)
		return
	}

	// GET: load deck, cards, and commander details
	pageData, err := a.loadSavedDeckWorkspace(r.Context(), user.ID, id)
	if err != nil {
		a.RenderNotFound(w, r)
		return
	}
	if respondJSON {
		writeJSON(w, http.StatusOK, pageData.WorkspaceState)
		return
	}
	data := TemplateData{
		CurrentUser: user,
		Data:        pageData,
		Flash:       flash,
		WideLayout:  true,
	}

	a.Renderer.Render(w, "deck_show", data)
}

func (a *App) HandleDeckEditShow(w http.ResponseWriter, r *http.Request) {
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

	if _, err := decks.GetDeck(r.Context(), a.DB, id, user.ID); err != nil {
		a.RenderNotFound(w, r)
		return
	}

	http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
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

	current, err := decks.GetDeck(r.Context(), a.DB, id, user.ID)
	if err != nil {
		a.RenderNotFound(w, r)
		return
	}

	name := strings.TrimSpace(r.Form.Get("name"))
	if name == "" {
		name = current.Name
	}

	desc := current.Description
	if _, ok := r.Form["description"]; ok {
		desc = strings.TrimSpace(r.Form.Get("description"))
	}

	tags := current.Tags
	if _, ok := r.Form["tags"]; ok {
		tags = strings.TrimSpace(r.Form.Get("tags"))
	}

	format := current.Format
	if _, ok := r.Form["format"]; ok {
		format = defaultDeckFormat(r.Form.Get("format"), current.CommanderName, "")
	} else {
		format = defaultDeckFormat(current.Format, current.CommanderName, "")
	}
	commander, err := a.commanderForFormatChange(r.Context(), id, current.Format, format, current.CommanderName)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	isPublic := current.IsPublic
	if _, ok := r.Form["sharing_submitted"]; ok {
		isPublic = strings.TrimSpace(r.Form.Get("is_public")) != ""
	}

	publicSlug := current.PublicSlug
	if _, ok := r.Form["public_slug"]; ok {
		publicSlug = strings.TrimSpace(r.Form.Get("public_slug"))
	}

	powerBracket := current.PowerBracket
	if _, ok := r.Form["power_bracket"]; ok {
		powerBracket = defaultDeckPowerBracket(r.Form.Get("power_bracket"), format)
	} else {
		powerBracket = defaultDeckPowerBracket(current.PowerBracket, format)
	}

	if err := decks.UpdateDeckWithOptions(r.Context(), a.DB, id, decks.DeckInput{
		Name:          name,
		Description:   desc,
		Tags:          tags,
		Format:        format,
		CommanderName: commander,
		IsPublic:      isPublic,
		PublicSlug:    publicSlug,
		PowerBracket:  powerBracket,
	}); err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	if wantsJSONResponse(r) {
		a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
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

// HandleDeckCommanderUpdate updates a deck's commander to a card already in the deck.
func (a *App) HandleDeckCommanderUpdate(w http.ResponseWriter, r *http.Request) {
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

	idStr := strings.TrimSpace(r.Form.Get("id"))
	commander := strings.TrimSpace(r.Form.Get("commander_name"))

	deckID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || deckID <= 0 {
		a.RenderNotFound(w, r)
		return
	}

	// Load deck and verify ownership.
	d, err := decks.GetDeck(r.Context(), a.DB, deckID, user.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	if d.UserID != user.ID {
		a.RenderNotFound(w, r)
		return
	}
	if d.Format != "Commander" {
		if wantsJSONResponse(r) {
			http.Error(w, "Only Commander decks can set a commander.", http.StatusBadRequest)
			return
		}
		setFlash(w, "Only Commander decks can set a commander.")
		http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
		return
	}

	// Commander must be chosen from cards already in the deck.
	cardsInDeck, err := decks.ListDeckCards(r.Context(), a.DB, deckID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	byName := make(map[string]decks.DeckCard, len(cardsInDeck))
	for _, dc := range cardsInDeck {
		name := strings.TrimSpace(dc.CardName)
		if name != "" {
			byName[name] = dc
		}
	}

	picked, ok := byName[commander]
	if commander == "" || !ok {
		if wantsJSONResponse(r) {
			http.Error(w, "Could not update commander: card not found in this deck.", http.StatusBadRequest)
			return
		}
		setFlash(w, "Could not update commander: card not found in this deck.")
		http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
		return
	}

	resolvedCommander, err := cards.LookupCardsByNames(r.Context(), a.DB, []string{picked.CardName})
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	matched, ok := resolvedCommander[strings.ToLower(strings.TrimSpace(picked.CardName))]
	if !ok || !matched.IsCommanderCandidate {
		if wantsJSONResponse(r) {
			http.Error(w, "Could not update commander: that card is not a valid commander.", http.StatusBadRequest)
			return
		}
		setFlash(w, "Could not update commander: that card is not a valid commander.")
		http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
		return
	}

	oldCommander := strings.TrimSpace(d.CommanderName)
	newCommander := commander

	// Remove 1 copy of the new commander from the 99 (it becomes the commander slot).
	if picked.Quantity > 0 {
		_ = decks.AddCard(r.Context(), a.DB, deckID, picked.CardID, -1)
	}

	// Add the old commander back into the 99 if it existed and is changing.
	if oldCommander != "" && oldCommander != newCommander {
		oldCard, err := cards.EnsureCardByName(r.Context(), a.DB, oldCommander)
		if err == nil {
			_ = decks.AddCard(r.Context(), a.DB, deckID, oldCard.OracleID, 1)
		}
	}

	// Update commander while preserving name/description.
	if err := decks.UpdateDeckWithOptions(r.Context(), a.DB, deckID, decks.DeckInput{
		Name:          d.Name,
		Description:   d.Description,
		Tags:          d.Tags,
		Format:        d.Format,
		CommanderName: commander,
		IsPublic:      d.IsPublic,
		PublicSlug:    d.PublicSlug,
		PowerBracket:  d.PowerBracket,
	}); err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	if wantsJSONResponse(r) {
		a.renderSavedDeckWorkspaceJSON(w, r, user.ID, deckID)
		return
	}
	setFlash(w, "Commander updated!")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
}
