package web

import (
	"net/http"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type profilePageData struct {
	Profile account.PublicProfile
	Items   []deckListItem
}

func (a *App) HandleProfileShow(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserProfileIDFromPath(r)
	if err != nil {
		a.RenderNotFound(w, r)
		return
	}

	profile, err := account.GetPublicProfileByID(r.Context(), a.DB, userID)
	if err != nil {
		a.RenderNotFound(w, r)
		return
	}

	publicDecks, err := decks.ListPublicDecksByUser(r.Context(), a.DB, userID, 60)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	commanderNames := make([]string, 0, len(publicDecks))
	for _, d := range publicDecks {
		if name := strings.TrimSpace(d.CommanderName); name != "" {
			commanderNames = append(commanderNames, name)
		}
	}
	commanderCards, err := cards.LookupCardsByNames(r.Context(), a.DB, commanderNames)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	items := make([]deckListItem, 0, len(publicDecks))
	for _, d := range publicDecks {
		item := deckListItem{
			ID:               d.ID,
			OwnerID:          profile.ID,
			OwnerDisplayName: profile.DisplayName,
			Name:             d.Name,
			Description:      d.Description,
			Tags:             d.Tags,
			Format:           d.Format,
			CommanderName:    d.CommanderName,
			IsPublic:         d.IsPublic,
			PublicSlug:       d.PublicSlug,
			PowerBracket:     d.PowerBracket,
		}
		if commanderName := strings.TrimSpace(d.CommanderName); commanderName != "" {
			if c, ok := commanderCards[strings.ToLower(commanderName)]; ok && c.ImageURI != "" {
				item.CommanderImageURI = c.ImageURI
			}
		}
		items = append(items, item)
	}

	a.Renderer.Render(w, "profile_show", TemplateData{
		CurrentUser: CurrentUser(r),
		Data: profilePageData{
			Profile: *profile,
			Items:   items,
		},
		Flash: readFlash(w, r),
	})
}
