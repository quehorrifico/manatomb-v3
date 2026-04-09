package web

import (
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

func (a *App) HandlePublicDecks(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	query := r.URL.Query()
	format := strings.TrimSpace(query.Get("format"))
	commanderName := strings.TrimSpace(query.Get("commander"))
	powerBracket := strings.TrimSpace(query.Get("power_bracket"))
	colorFilters := query["color"]
	selectedFormat := ""
	if format != "" {
		selectedFormat = decks.NormalizeFormat(format)
	}
	selectedPowerBracket := ""
	if powerBracket != "" {
		selectedPowerBracket = decks.NormalizePowerBracket(powerBracket)
	}
	colorSelected := make(map[string]bool, len(colorFilters))
	for _, color := range colorFilters {
		color = strings.ToUpper(strings.TrimSpace(color))
		if color == "" {
			continue
		}
		colorSelected[color] = true
	}

	publicDecks, err := decks.ListPublicDecks(r.Context(), a.DB, decks.PublicDeckFilters{
		CommanderName: commanderName,
		Format:        format,
		PowerBracket:  powerBracket,
		ColorIdentity: colorFilters,
		Limit:         60,
	})
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	items := make([]deckListItem, 0, len(publicDecks))
	commanderNames := make([]string, 0, len(publicDecks))
	userIDs := make([]int64, 0, len(publicDecks))
	for _, d := range publicDecks {
		if name := strings.TrimSpace(d.CommanderName); name != "" {
			commanderNames = append(commanderNames, name)
		}
		if d.UserID > 0 {
			userIDs = append(userIDs, d.UserID)
		}
	}
	commanderCards, err := cards.LookupCardsByNames(r.Context(), a.DB, commanderNames)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	ownersByID, err := account.ListPublicProfilesByIDs(r.Context(), a.DB, userIDs)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	for _, d := range publicDecks {
		item := deckListItem{
			ID:            d.ID,
			OwnerID:       d.UserID,
			Name:          d.Name,
			Description:   d.Description,
			Tags:          d.Tags,
			Format:        d.Format,
			CommanderName: d.CommanderName,
			IsPublic:      d.IsPublic,
			PublicSlug:    d.PublicSlug,
			PowerBracket:  d.PowerBracket,
		}
		if owner, ok := ownersByID[d.UserID]; ok {
			item.OwnerDisplayName = owner.DisplayName
		}
		if commanderName := strings.TrimSpace(d.CommanderName); commanderName != "" {
			if c, ok := commanderCards[strings.ToLower(commanderName)]; ok && c.ImageURI != "" {
				item.CommanderImageURI = c.ImageURI
			}
		}
		items = append(items, item)
	}

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		Data: publicDeckListPageData{
			CommanderName: commanderName,
			Format:        selectedFormat,
			PowerBracket:  selectedPowerBracket,
			ColorFilters:  colorFilters,
			ColorSelected: colorSelected,
			Items:         items,
		},
	}

	a.Renderer.Render(w, "decks_public", data)
}

func (a *App) HandlePublicDeckShow(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)
	slug := parsePublicDeckSlugFromPath(r)
	if slug == "" {
		a.RenderNotFound(w, r)
		return
	}

	d, err := decks.GetPublicDeckBySlug(r.Context(), a.DB, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			a.RenderNotFound(w, r)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	deckCards, err := decks.ListDeckCards(r.Context(), a.DB, d.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	maybeDeckCards, err := decks.ListDeckMaybeCards(r.Context(), a.DB, d.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	var commanderCard *cards.Card
	if decks.FormatRequiresCommander(d.Format) {
		commanderCard = a.lookupCommanderCard(r.Context(), d.CommanderName)
	}
	owner, err := account.GetPublicProfileByID(r.Context(), a.DB, d.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			a.RenderNotFound(w, r)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		Data: publicDeckPageData{
			Deck:           d,
			DeckCards:      deckCards,
			MaybeDeckCards: maybeDeckCards,
			Analytics:      computeDeckAnalyticsFromDeckCards(d.Format, d.CommanderName, deckCards),
			Commander:      commanderCard,
			Owner:          owner,
		},
	}
	a.Renderer.Render(w, "decks_public_show", data)
}

func (a *App) HandlePublicDeckForkPost(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		next := "/decks/public"
		if slug := strings.TrimSpace(r.FormValue("slug")); slug != "" {
			next = "/decks/public/" + url.PathEscape(slug)
		}
		http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
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

	slug := strings.TrimSpace(r.Form.Get("slug"))
	if slug == "" {
		a.RenderNotFound(w, r)
		return
	}

	forked, err := decks.ForkDeckToUser(r.Context(), a.DB, slug, user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			a.RenderNotFound(w, r)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Deck copied to your account.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(forked.ID, 10), http.StatusSeeOther)
}
