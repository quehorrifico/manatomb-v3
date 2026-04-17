package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

// HandleDeckList renders the current user's saved decks.
func (a *App) HandleDeckList(w http.ResponseWriter, r *http.Request) {
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
			Format:        d.Format,
			CommanderName: d.CommanderName,
			IsPublic:      d.IsPublic,
			PublicSlug:    d.PublicSlug,
			PowerBracket:  d.PowerBracket,
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
	switch mode {
	case "import":
		http.Redirect(w, r, "/decks/import", http.StatusSeeOther)
		return
	case "sandbox":
		http.Redirect(w, r, "/decks/new/sandbox", http.StatusSeeOther)
		return
	}

	commanderName := strings.TrimSpace(r.URL.Query().Get("commander_name"))
	if commanderName != "" {
		http.Redirect(w, r, commanderDeckBuilderPath(commanderDeckBuilderState{
			Query: commanderName,
		}), http.StatusSeeOther)
		return
	}
	format := defaultDeckFormat(r.URL.Query().Get("format"), commanderName, mode)

	a.renderDeckNew(w, user, flash, "", deckNewPageData{
		Mode:          mode,
		Format:        format,
		CommanderName: commanderName,
	})
}

func (a *App) HandleDeckImportShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	user := CurrentUser(r)

	a.renderDeckNew(w, user, flash, "", deckNewPageData{
		Mode: "import",
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
	format := defaultDeckFormat(r.Form.Get("format"), commander, normalizeDeckBuilderMode(r.Form.Get("mode")))
	powerBracket := defaultDeckPowerBracket(r.Form.Get("power_bracket"), format)

	if decks.FormatRequiresCommander(format) && commander == "" {
		a.renderDeckNew(w, user, "", "Please choose a commander first.", deckNewPageData{
			Mode:          "",
			Format:        format,
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
			Format:        format,
			CommanderName: commander,
			Name:          name,
			Description:   desc,
		})
		return
	}

	if !decks.FormatRequiresCommander(format) {
		commander = ""
	}

	d, err := decks.CreateDeckWithOptions(r.Context(), a.DB, user.ID, decks.DeckInput{
		Name:          name,
		Description:   desc,
		Format:        format,
		CommanderName: commander,
		PowerBracket:  powerBracket,
	})
	if err != nil {
		// Use our pretty 500 page + logging
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Deck created.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(d.ID, 10), http.StatusSeeOther)
}

// HandleDeckWorkbench renders the local unsaved deck workspace.
func (a *App) HandleDeckWorkbench(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	user := CurrentUser(r)

	commanderName := strings.TrimSpace(r.URL.Query().Get("commander_name"))
	format := defaultDeckFormat(r.URL.Query().Get("format"), commanderName, normalizeDeckBuilderMode(r.URL.Query().Get("mode")))
	isSandbox := strings.TrimSpace(r.URL.Query().Get("sandbox")) == "1"
	deckName := "New Guest Deck"

	// Fake deck object (ID=0) so the template can render.
	fakeDeck := &decks.Deck{
		ID:            0,
		UserID:        0,
		Name:          deckName,
		Description:   "",
		Format:        format,
		CommanderName: commanderName,
	}

	var commanderCard *cards.Card
	if decks.FormatRequiresCommander(format) {
		commanderCard = a.lookupCommanderCard(r.Context(), commanderName)
	}

	data := TemplateData{
		CurrentUser: user, // may be nil; template handles local workbench mode
		Data: deckPageData{
			Deck:                  fakeDeck,
			DeckCards:             nil,
			MaybeDeckCards:        nil,
			VisibleCardCount:      0,
			MaybeCardCount:        0,
			Analytics:             emptyDeckAnalytics(format, commanderName),
			Commander:             commanderCard,
			CommanderCandidateSet: nil,
			// CommanderCandidates is only used for saved decks, not the local workbench.
			CommanderCandidates: nil,
			WorkspaceState: buildWorkspaceStateFromDeck(
				fakeDeck,
				nil,
				nil,
				nil,
				commanderCard,
			),
			WorkbenchMode:       true,
			WorkbenchSandbox:    isSandbox,
		},
		Flash:      flash,
		WideLayout: true,
	}

	a.Renderer.Render(w, "deck_show", data)
}

func (a *App) createSavedBuilderDeck(w http.ResponseWriter, r *http.Request, user *account.User, format, commanderName string) {
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	format = defaultDeckFormat(format, commanderName, "")
	commanderName = strings.TrimSpace(commanderName)
	if !decks.FormatRequiresCommander(format) {
		commanderName = ""
	}

	created, err := decks.CreateDeckWithOptions(r.Context(), a.DB, user.ID, decks.DeckInput{
		Name:          "New Deck",
		Description:   "",
		Format:        format,
		CommanderName: commanderName,
	})
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Deck created.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
}

// HandleDeckWorkbenchAliasRedirect keeps older local-builder paths working while
// the canonical route is /decks/new/workbench.
func (a *App) HandleDeckWorkbenchAliasRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, deckWorkbenchPath(deckWorkbenchOptions{
		Format:        r.URL.Query().Get("format"),
		CommanderName: r.URL.Query().Get("commander_name"),
		Sandbox:       strings.TrimSpace(r.URL.Query().Get("sandbox")) == "1",
		SaveWorkbench: strings.TrimSpace(r.URL.Query().Get("save_guest")) == "1",
		Reset:         strings.TrimSpace(r.URL.Query().Get("reset")) == "1",
	}), http.StatusSeeOther)
}

func (a *App) HandleDeckSandboxRedirect(w http.ResponseWriter, r *http.Request) {
	if user := CurrentUser(r); user != nil {
		a.createSavedBuilderDeck(w, r, user, "Sandbox", "")
		return
	}

	http.Redirect(w, r, deckWorkbenchPath(deckWorkbenchOptions{
		Sandbox: true,
		Reset:   true,
	}), http.StatusSeeOther)
}

// HandleDeckCommanderSelect starts the next builder step after commander pick.
//
// - Local workbench users are sent to the unsaved deck workspace.
// - Logged-in users are sent to the right next screen based on return_to.
func (a *App) HandleDeckCommanderSelect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	commander := strings.TrimSpace(r.FormValue("commander_name"))
	returnTo := canonicalizeLocalReturnPath(r.FormValue("return_to"), "/decks/new/commander/")
	if commander == "" {
		// No commander provided - send user back to search with a friendly message.
		setFlash(w, "Please choose a commander card first.")
		http.Redirect(w, r, "/commanders/search?return_to="+url.QueryEscape(returnTo), http.StatusSeeOther)
		return
	}

	if isDeckSettingsPath(returnTo) {
		next := mergeLocalReturnPath(returnTo, "/decks/settings", map[string]string{
			"commander_name": commander,
		})
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}

	a.startCommanderDeck(w, r, commander)
}
