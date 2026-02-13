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

		// Case 1: adding a new card by name (from the "Add card" form)
		if cardName != "" {
			resolved, err := cards.ResolveCardByNameFuzzy(r.Context(), a.DB, cardName)
			if err != nil {
				// If we can't find a suitable local card match, show a friendly error on the deck page.
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

					maybeDeckCards, derr := decks.ListDeckMaybeCards(r.Context(), a.DB, id)
					if derr != nil {
						a.RenderServerError(w, r, derr)
						return
					}

					commanderCard := a.lookupCommanderCard(r.Context(), d.CommanderName)

					data := TemplateData{
						CurrentUser: user,
						Data: deckPageData{
							Deck:                d,
							DeckCards:           deckCards,
							MaybeDeckCards:      maybeDeckCards,
							VisibleCardCount:    visibleDeckCardCount(deckCards, d.CommanderName),
							MaybeCardCount:      deckCardQuantityTotal(maybeDeckCards),
							Analytics:           computeDeckAnalyticsFromDeckCards(d.CommanderName, deckCards),
							Commander:           commanderCard,
							CommanderCandidates: nil,
							GuestMode:           false,
						},
						Flash:      flash,
						Error:      fmt.Sprintf("No card found matching \"%s\". Please check the spelling.", cardName),
						WideLayout: true,
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

			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 2: move a mainboard card to the maybeboard.
		if action == "inc_main" && cardID != "" {
			if err := decks.AddCard(r.Context(), a.DB, id, cardID, 1); err != nil {
				a.RenderServerError(w, r, err)
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
			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 4: move a mainboard card to the maybeboard.
		if action == "to_maybe" && cardID != "" {
			if err := decks.MoveCardToMaybe(r.Context(), a.DB, id, cardID); err != nil {
				a.RenderServerError(w, r, err)
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

			http.Redirect(w, r, "/decks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}

		// Case 7: decrement an existing maybeboard card by maybe_card_id.
		if maybeCardID != "" {
			if err := decks.AddMaybeCard(r.Context(), a.DB, id, maybeCardID, -1); err != nil {
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

	maybeDeckCards, err := decks.ListDeckMaybeCards(r.Context(), a.DB, id)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	commanderCandidates := make([]commanderCandidate, 0)
	deckCardNames := make([]string, 0, len(deckCards))
	for _, dc := range deckCards {
		name := strings.TrimSpace(dc.CardName)
		if name == "" {
			continue
		}
		deckCardNames = append(deckCardNames, name)
	}
	deckCardsByName, err := cards.LookupCardsByNames(r.Context(), a.DB, deckCardNames)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
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

	commanderCard := a.lookupCommanderCard(r.Context(), d.CommanderName)

	data := TemplateData{
		CurrentUser: user,
		Data: deckPageData{
			Deck:                d,
			DeckCards:           deckCards,
			MaybeDeckCards:      maybeDeckCards,
			VisibleCardCount:    visibleDeckCardCount(deckCards, d.CommanderName),
			MaybeCardCount:      deckCardQuantityTotal(maybeDeckCards),
			Analytics:           computeDeckAnalyticsFromDeckCards(d.CommanderName, deckCards),
			Commander:           commanderCard,
			CommanderCandidates: commanderCandidates,
			GuestMode:           false,
		},
		Flash:      flash,
		WideLayout: true,
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

// HandleDeckUpdateCommander updates a deck's commander to a card that already exists in the deck.
func (a *App) HandleDeckUpdateCommander(w http.ResponseWriter, r *http.Request) {
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
	if err := decks.UpdateDeck(r.Context(), a.DB, deckID, d.Name, d.Description, commander); err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Commander updated!")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
}
