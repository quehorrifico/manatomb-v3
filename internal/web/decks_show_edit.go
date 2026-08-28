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
		current, err := decks.GetDeck(r.Context(), a.DB, id, user.ID)
		if err != nil {
			a.RenderNotFound(w, r)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		cardName := r.Form.Get("card_name")
		cardID := strings.TrimSpace(r.Form.Get("card_id"))
		sideCardID := strings.TrimSpace(r.Form.Get("side_card_id"))
		maybeCardID := strings.TrimSpace(r.Form.Get("maybe_card_id"))
		action := strings.TrimSpace(r.Form.Get("action"))
		zone := strings.TrimSpace(r.Form.Get("zone"))
		quantityRaw := strings.TrimSpace(r.Form.Get("quantity"))

		addCardToBoard := func(board, oracleID string, delta int) error {
			switch strings.ToLower(strings.TrimSpace(board)) {
			case "maybe", "maybeboard":
				return decks.AddMaybeCard(r.Context(), a.DB, id, oracleID, delta)
			case "side", "sideboard":
				return decks.AddSideboardCard(r.Context(), a.DB, id, oracleID, delta)
			default:
				return decks.AddCard(r.Context(), a.DB, id, oracleID, delta)
			}
		}

		listCardsForBoard := func(board string) ([]decks.DeckCard, error) {
			switch strings.ToLower(strings.TrimSpace(board)) {
			case "maybe", "maybeboard":
				return decks.ListDeckMaybeCards(r.Context(), a.DB, id)
			case "side", "sideboard":
				return decks.ListDeckSideboardCards(r.Context(), a.DB, id)
			default:
				return decks.ListDeckCards(r.Context(), a.DB, id)
			}
		}

		boardCardIDFromRequest := func() (string, string) {
			if cardID != "" {
				return "main", cardID
			}
			if sideCardID != "" {
				return "side", sideCardID
			}
			if maybeCardID != "" {
				return "maybe", maybeCardID
			}
			return "", ""
		}

		if action == "save_overview" {
			format := current.Format
			if _, ok := r.Form["format"]; ok {
				format = defaultDeckFormat(r.Form.Get("format"), current.CommanderName, "")
			} else {
				format = defaultDeckFormat(current.Format, current.CommanderName, "")
			}
			commander, err := a.commanderForFormatChange(
				r.Context(),
				id,
				current.Format,
				format,
				current.CommanderName,
				current.CommanderPrintID,
			)
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

		if action == "undo_card_change" {
			_, sourceCardID := boardCardIDFromRequest()
			if sourceCardID == "" {
				http.Error(w, "missing card information", http.StatusBadRequest)
				return
			}

			var expected []decks.CardBoardState
			var restore []decks.CardBoardState
			if err := json.Unmarshal([]byte(r.Form.Get("expected_states")), &expected); err != nil {
				http.Error(w, "invalid expected card state", http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal([]byte(r.Form.Get("restore_states")), &restore); err != nil {
				http.Error(w, "invalid restore card state", http.StatusBadRequest)
				return
			}

			err := decks.RestoreCardBoardStatesIfCurrent(
				r.Context(),
				a.DB,
				id,
				sourceCardID,
				expected,
				restore,
			)
			if errors.Is(err, decks.ErrCardBoardStateConflict) {
				http.Error(w, "Deck changed; undo is no longer available.", http.StatusConflict)
				return
			}
			if errors.Is(err, decks.ErrInvalidCardBoardState) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err != nil {
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

		if action == "set_qty" {
			sourceBoard, sourceCardID := boardCardIDFromRequest()
			if sourceBoard == "" || sourceCardID == "" {
				http.Error(w, "missing card information", http.StatusBadRequest)
				return
			}
			nextQty, err := strconv.Atoi(quantityRaw)
			if err != nil || nextQty < 0 {
				http.Error(w, "invalid quantity", http.StatusBadRequest)
				return
			}
			if err := decks.SetCardQuantity(r.Context(), a.DB, id, sourceCardID, sourceBoard, nextQty); err != nil {
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

		if action == "set_board" {
			sourceBoard, sourceCardID := boardCardIDFromRequest()
			if sourceBoard == "" || sourceCardID == "" {
				http.Error(w, "missing card information", http.StatusBadRequest)
				return
			}
			targetBoard := "main"
			switch strings.ToLower(strings.TrimSpace(r.Form.Get("to_board"))) {
			case "maybe", "maybeboard":
				targetBoard = "maybe"
			case "side", "sideboard":
				targetBoard = "side"
			}
			moveQty := 0
			if quantityRaw != "" {
				if parsedQty, err := strconv.Atoi(quantityRaw); err == nil {
					moveQty = parsedQty
				}
			}
			if err := decks.MoveCardQuantityBetweenBoards(r.Context(), a.DB, id, sourceCardID, sourceBoard, targetBoard, moveQty); err != nil {
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

		if action == "set_print" {
			sourceBoard, sourceCardID := boardCardIDFromRequest()
			if sourceBoard == "" || sourceCardID == "" {
				http.Error(w, "missing card information", http.StatusBadRequest)
				return
			}
			printID := strings.TrimSpace(r.Form.Get("print_id"))
			if err := decks.SetCardPreferredPrint(r.Context(), a.DB, id, sourceCardID, sourceBoard, printID); err != nil {
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

		if action == "set_commander_print" {
			_, sourceCardID := boardCardIDFromRequest()
			if sourceCardID == "" {
				http.Error(w, "missing commander information", http.StatusBadRequest)
				return
			}
			printID := strings.TrimSpace(r.Form.Get("print_id"))
			if err := decks.SetCommanderPreferredPrint(r.Context(), a.DB, id, sourceCardID, printID); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if respondJSON {
				a.renderSavedDeckWorkspaceJSON(w, r, user.ID, id)
				return
			}
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
						Meta:        deckWorkspacePageMeta(pageData.Deck),
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
			targetBoard := "main"
			switch strings.ToLower(strings.TrimSpace(zone)) {
			case "maybe", "maybeboard":
				targetBoard = "maybe"
			case "side", "sideboard":
				targetBoard = "side"
			}

			targetCardID := resolved.OracleID
			resolvedName := strings.TrimSpace(resolved.Name)
			existingDeckCards, err := listCardsForBoard(targetBoard)
			if err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			if existingID, ok := deckCardIDForName(existingDeckCards, resolvedName); ok {
				targetCardID = existingID
			}
			if err := addCardToBoard(targetBoard, targetCardID, 1); err != nil {
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

		// Case 2: increment an existing mainboard card by card_id.
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

		// Case 3: increment an existing sideboard card by side_card_id.
		if action == "inc_side" && sideCardID != "" {
			if err := decks.AddSideboardCard(r.Context(), a.DB, id, sideCardID, 1); err != nil {
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

		// Case 4: increment an existing maybeboard card by maybe_card_id.
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

		// Case 5: move a mainboard card to the sideboard.
		if action == "to_side" && cardID != "" {
			if err := decks.MoveCardBetweenBoards(r.Context(), a.DB, id, cardID, "main", "side"); err != nil {
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

		// Case 6: move a mainboard card to the maybeboard.
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

		// Case 7: move a sideboard card to the mainboard.
		if action == "to_main" && sideCardID != "" {
			if err := decks.MoveCardBetweenBoards(r.Context(), a.DB, id, sideCardID, "side", "main"); err != nil {
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

		// Case 8: move a sideboard card to the maybeboard.
		if action == "side_to_maybe" && sideCardID != "" {
			if err := decks.MoveCardBetweenBoards(r.Context(), a.DB, id, sideCardID, "side", "maybe"); err != nil {
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

		// Case 9: move a maybeboard card to the sideboard.
		if action == "to_side" && maybeCardID != "" {
			if err := decks.MoveCardBetweenBoards(r.Context(), a.DB, id, maybeCardID, "maybe", "side"); err != nil {
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

		// Case 10: move a maybeboard card to the mainboard.
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

		// Case 11: decrement an existing mainboard card by card_id.
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

		// Case 12: decrement an existing sideboard card by side_card_id.
		if sideCardID != "" {
			if err := decks.AddSideboardCard(r.Context(), a.DB, id, sideCardID, -1); err != nil {
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

		// Case 13: decrement an existing maybeboard card by maybe_card_id.
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
		Meta:        deckWorkspacePageMeta(pageData.Deck),
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
	commander, err := a.commanderForFormatChange(
		r.Context(),
		id,
		current.Format,
		format,
		current.CommanderName,
		current.CommanderPrintID,
	)
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
	if !ok || !isCommanderCandidateAllowed(matched.IsCommanderCandidate, matched.TypeLine) {
		if wantsJSONResponse(r) {
			http.Error(w, "Could not update commander: that card is not a valid commander.", http.StatusBadRequest)
			return
		}
		setFlash(w, "Could not update commander: that card is not a valid commander.")
		http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
		return
	}

	if err := decks.ChangeCommander(r.Context(), a.DB, deckID, decks.CommanderChange{
		NewCommanderName:     commander,
		NewCommanderOracleID: picked.CardID,
		NewCommanderPrintID:  strings.TrimSpace(picked.PrintID),
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
