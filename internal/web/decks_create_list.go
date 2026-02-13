package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

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
	commanderNames := make([]string, 0, len(userDecks))
	for _, d := range userDecks {
		name := strings.TrimSpace(d.CommanderName)
		if name != "" {
			commanderNames = append(commanderNames, name)
		}
	}
	commanderCards, err := cards.LookupCardsByNames(r.Context(), a.DB, commanderNames)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	for _, d := range userDecks {
		item := deckListItem{
			ID:            d.ID,
			Name:          d.Name,
			Description:   d.Description,
			CommanderName: d.CommanderName,
		}

		commanderName := strings.TrimSpace(d.CommanderName)
		if commanderName != "" {
			if c, ok := commanderCards[strings.ToLower(commanderName)]; ok && c.ImageURI != "" {
				item.CommanderImageURI = c.ImageURI
			}
			// If lookup misses or has no image, we just show the placeholder in the UI.
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

	mode := normalizeDeckBuilderMode(r.URL.Query().Get("mode"))
	commanderName := strings.TrimSpace(r.URL.Query().Get("commander_name"))

	// Guests skip the naming step and go straight to the local draft builder.
	if user == nil && commanderName != "" {
		http.Redirect(w, r, "/decks/guest?commander_name="+url.QueryEscape(commanderName), http.StatusSeeOther)
		return
	}

	a.renderDeckNew(w, user, flash, "", deckNewPageData{
		Mode:          mode,
		CommanderName: commanderName,
	})
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
		a.renderDeckNew(w, user, "", "Please choose a commander first.", deckNewPageData{
			Mode:          "",
			CommanderName: commander,
			Name:          name,
			Description:   desc,
		})
		return
	}

	// Basic validation: require a name
	if name == "" {
		a.renderDeckNew(w, user, "", "Deck name is required.", deckNewPageData{
			Mode:          "",
			CommanderName: commander,
			Name:          name,
			Description:   desc,
		})
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
	isSandbox := strings.TrimSpace(r.URL.Query().Get("sandbox")) == "1"
	deckName := "New Deck"
	if isSandbox {
		deckName = ""
	}

	// Fake deck object (ID=0) so the template can render.
	fakeDeck := &decks.Deck{
		ID:            0,
		UserID:        0,
		Name:          deckName,
		Description:   "",
		CommanderName: commanderName,
	}

	commanderCard := a.lookupCommanderCard(r.Context(), commanderName)

	data := TemplateData{
		CurrentUser: user, // will be nil for guests; template handles guest mode
		Data: deckPageData{
			Deck:             fakeDeck,
			DeckCards:        nil,
			MaybeDeckCards:   nil,
			VisibleCardCount: 0,
			MaybeCardCount:   0,
			Analytics:        emptyDeckAnalytics(commanderName),
			Commander:        commanderCard,
			// CommanderCandidates is only used for saved (non-guest) decks.
			CommanderCandidates: nil,
			GuestMode:           true,
			GuestSandbox:        isSandbox,
		},
		Flash:      flash,
		WideLayout: true,
	}

	a.Renderer.Render(w, "deck_show", data)
}

func (a *App) HandleDeckSandboxWIP(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/decks/guest?sandbox=1&reset=1", http.StatusSeeOther)
}

// HandleDeckCreateFromCommander starts the next builder step after commander pick.
//
// - Guests are sent to the guest/local deck builder.
// - Logged-in users are sent to the saved-deck naming step.
func (a *App) HandleDeckCreateFromCommander(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	commander := strings.TrimSpace(r.FormValue("commander_name"))
	if commander == "" {
		// No commander provided - send user back to commander search with a friendly message.
		setFlash(w, "Please choose a commander first.")
		http.Redirect(w, r, "/commanders/search", http.StatusSeeOther)
		return
	}

	// If the user is not logged in, send them to the guest deck builder.
	if user == nil {
		http.Redirect(w, r, "/decks/guest?commander_name="+url.QueryEscape(commander), http.StatusSeeOther)
		return
	}

	// Logged-in users: go to Step 2 (name/description) instead of auto-creating.
	http.Redirect(w, r, "/decks/new?commander_name="+url.QueryEscape(commander), http.StatusSeeOther)
	return
}
