package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type profilePageData struct {
	Profile            account.PublicProfile
	Items              []deckListItem
	Stats              profileStatsData
	ActiveTab          string
	AwardTotal         int
	FavoritePrintings  []profileFavoritePrintingView
	FavoriteView       string
	GuessWins          []profileGuessWinView
	SpellifyAwards     []profileSpellifyAwardView
	FavoritePagination profilePagination
	GuessPagination    profilePagination
	SpellifyPagination profilePagination
	FavoriteReturnTo   string
}

type profilePagination struct {
	Total      int
	Page       int
	TotalPages int
	PrevURL    string
	NextURL    string
}

type profileStatsData struct {
	JoinedLabel              string
	AveragePowerBracket      string
	Likes                    []string
	UsuallyPlays             string
	FavoriteColorCombination string
	FavoriteColorPips        []manaPipView
	AvatarImageURI           string
	AvatarAlt                string
	BackgroundImageURI       string
	BackgroundAlt            string
	ProfilePicturePrintID    string
	ProfileBackgroundPrintID string
	AvatarChoices            []profileAvatarChoice
	CanEdit                  bool
}

type profileAvatarChoice struct {
	ScryfallID string
	OracleID   string
	Name       string
	ImageURI   string
	SetLabel   string
	IsSelected bool
}

type profileFavoritePrintingView struct {
	ScryfallID      string
	OracleID        string
	Name            string
	ImageURI        string
	SetName         string
	SetCode         string
	SetLabel        string
	CollectorNumber string
	Rarity          string
	PriceUSD        string
	Artist          string
	ReleasedAt      string
	DetailPath      string
	FavoritedLabel  string
}

type profileGuessWinView struct {
	OracleID   string
	CardName   string
	ImageURI   string
	DetailPath string
	WonLabel   string
	GuessCount int
	ModeLabel  string
}

type profileSpellifyAwardView struct {
	OracleID   string
	CardName   string
	ImageURI   string
	DetailPath string
	WonLabel   string
	GuessCount int
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
	for index, d := range publicDecks {
		item := deckListItem{
			ID:            d.ID,
			DeckPath:      "/decks/public/" + d.PublicSlug,
			Name:          d.Name,
			Description:   d.Description,
			Tags:          d.Tags,
			Format:        d.Format,
			CommanderName: d.CommanderName,
			IsPublic:      d.IsPublic,
			PublicSlug:    d.PublicSlug,
			PowerBracket:  d.PowerBracket,
			ProfileTile:   true,
			TileOrder:     index,
		}
		if currentUser := CurrentUser(r); currentUser != nil && currentUser.ID == profile.ID {
			item.DeckPath = "/decks/" + strconv.FormatInt(d.ID, 10)
		}
		if d.PublishedAt != nil {
			item.PublishedLabel = d.PublishedAt.Format("Jan 2, 2006")
		}
		if commanderName := strings.TrimSpace(d.CommanderName); commanderName != "" {
			if c, ok := commanderCards[strings.ToLower(commanderName)]; ok {
				applyCommanderCardMetaToDeckItem(&item, c)
				if strings.TrimSpace(d.CommanderPrintID) != "" {
					if selected, selectedErr := cards.GetCardPrintingByID(
						r.Context(),
						a.DB,
						c.OracleID,
						d.CommanderPrintID,
					); selectedErr == nil {
						applyCommanderPrintingMetaToDeckItem(&item, selected)
					}
				}
			}
		}
		items = append(items, item)
	}

	currentUser := CurrentUser(r)
	legacyAvatar, err := a.profileCustomAvatarChoice(r, *profile, items)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	profilePicture, err := a.profileArtworkChoiceByPrintingID(r, profile.ProfilePicturePrintID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	profileBackground, err := a.profileArtworkChoiceByPrintingID(r, profile.ProfileBackgroundPrintID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	stats := buildProfileStats(*profile, publicDecks, items, legacyAvatar)
	applyProfileArtwork(&stats, *profile, profilePicture, profileBackground, legacyAvatar)
	stats.CanEdit = currentUser != nil && currentUser.ID == profile.ID
	const profilePageSize = 24
	activeTab := normalizeProfileTab(r.URL.Query())
	favoriteTotal, err := countFavoritePrintings(r.Context(), a.DB, profile.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	guessTotal, err := countGuessCardWins(r.Context(), a.DB, profile.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	spellifyTotal, err := countSpellifyAwards(r.Context(), a.DB, profile.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	favoritePagination := buildProfilePagination(r.URL.Query(), profile.ID, "favorites_page", queryPage(r, "favorites_page"), favoriteTotal, profilePageSize)
	guessPagination := buildProfilePagination(r.URL.Query(), profile.ID, "guess_page", queryPage(r, "guess_page"), guessTotal, profilePageSize)
	spellifyPagination := buildProfilePagination(r.URL.Query(), profile.ID, "tombscript_page", queryPage(r, "tombscript_page"), spellifyTotal, profilePageSize)

	favoritePrintings, err := listFavoritePrintingsPage(r.Context(), a.DB, profile.ID, profilePageSize, (favoritePagination.Page-1)*profilePageSize)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	guessWins, err := listGuessCardWinsPage(r.Context(), a.DB, profile.ID, profilePageSize, (guessPagination.Page-1)*profilePageSize)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	spellifyAwards, err := listSpellifyAwardsPage(r.Context(), a.DB, profile.ID, profilePageSize, (spellifyPagination.Page-1)*profilePageSize)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	a.Renderer.Render(w, "profile_show", TemplateData{
		CurrentUser: currentUser,
		Meta: &PageMeta{
			Title:        profile.DisplayName,
			Description:  fmt.Sprintf("View %s's public decks, favorite printings, and ManaTomb achievements.", profile.DisplayName),
			CanonicalURL: absoluteSiteURL(a.PublicBaseURL, userProfilePath(profile.ID)),
			ImageURL:     firstNonEmptyCardFlavor(stats.BackgroundImageURI, stats.AvatarImageURI),
			ImageAlt:     firstNonEmptyCardFlavor(stats.BackgroundAlt, stats.AvatarAlt),
			Type:         "profile",
		},
		Data: profilePageData{
			Profile:            *profile,
			Items:              items,
			Stats:              stats,
			ActiveTab:          activeTab,
			AwardTotal:         guessTotal + spellifyTotal,
			FavoritePrintings:  buildProfileFavoritePrintingViews(favoritePrintings),
			FavoriteView:       normalizeProfileFavoriteView(r.URL.Query().Get("favorites_view")),
			GuessWins:          buildProfileGuessWinViews(guessWins),
			SpellifyAwards:     buildProfileSpellifyAwardViews(spellifyAwards),
			FavoritePagination: favoritePagination,
			GuessPagination:    guessPagination,
			SpellifyPagination: spellifyPagination,
			FavoriteReturnTo:   r.URL.RequestURI(),
		},
		Flash: readFlash(w, r),
	})
}

func queryPage(r *http.Request, key string) int {
	page, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(key)))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func normalizeProfileTab(values url.Values) string {
	switch strings.ToLower(strings.TrimSpace(values.Get("tab"))) {
	case "favorites":
		return "favorites"
	case "achievements":
		return "achievements"
	case "decks":
		return "decks"
	}
	if values.Has("favorites_view") || values.Has("favorites_page") {
		return "favorites"
	}
	if values.Has("guess_page") || values.Has("tombscript_page") {
		return "achievements"
	}
	return "decks"
}

func buildProfilePagination(values url.Values, userID int64, key string, page, total, pageSize int) profilePagination {
	if pageSize < 1 {
		pageSize = 24
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	pageURL := func(target int) string {
		query := make(url.Values, len(values))
		for valueKey, valueItems := range values {
			query[valueKey] = append([]string(nil), valueItems...)
		}
		if target <= 1 {
			query.Del(key)
		} else {
			query.Set(key, strconv.Itoa(target))
		}
		path := userProfilePath(userID)
		if encoded := query.Encode(); encoded != "" {
			path += "?" + encoded
		}
		return path
	}

	pagination := profilePagination{Total: total, Page: page, TotalPages: totalPages}
	if page > 1 {
		pagination.PrevURL = pageURL(page - 1)
	}
	if page < totalPages {
		pagination.NextURL = pageURL(page + 1)
	}
	return pagination
}

func buildProfileSpellifyAwardViews(items []spellifyAward) []profileSpellifyAwardView {
	out := make([]profileSpellifyAwardView, 0, len(items))
	for _, item := range items {
		out = append(out, profileSpellifyAwardView{
			OracleID:   item.OracleID,
			CardName:   item.CardName,
			ImageURI:   item.ImageURI,
			DetailPath: cardPrintingDetailPath(item.OracleID, item.ScryfallID),
			WonLabel:   formatProfileJoined(item.WonAt),
			GuessCount: item.GuessCount,
		})
	}
	return out
}

func normalizeProfileFavoriteView(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "stacks", "text", "table":
		return strings.ToLower(strings.TrimSpace(raw))
	case "columns":
		return "table"
	default:
		return "grid"
	}
}

func buildProfileFavoritePrintingViews(items []favoritePrintingData) []profileFavoritePrintingView {
	out := make([]profileFavoritePrintingView, 0, len(items))
	for _, item := range items {
		imageURI := strings.TrimSpace(item.ImageURI)
		if imageURI == "" {
			imageURI = strings.TrimSpace(item.ArtCropURI)
		}
		out = append(out, profileFavoritePrintingView{
			ScryfallID:      item.ScryfallID,
			OracleID:        item.OracleID,
			Name:            item.Name,
			ImageURI:        imageURI,
			SetName:         item.SetName,
			SetCode:         strings.ToUpper(strings.TrimSpace(item.SetCode)),
			SetLabel:        profilePrintingSetLabel(item),
			CollectorNumber: item.CollectorNumber,
			Rarity:          cardMetaValue(item.Rarity, "Unknown"),
			PriceUSD:        formatCardPrice(item.PriceUSD),
			Artist:          cardMetaValue(item.Artist, "Unknown"),
			ReleasedAt:      item.ReleasedAt,
			DetailPath:      cardPrintingDetailPath(item.OracleID, item.ScryfallID),
			FavoritedLabel:  formatProfileJoined(item.CreatedAt),
		})
	}
	return out
}

func profilePrintingSetLabel(item favoritePrintingData) string {
	setName := strings.TrimSpace(item.SetName)
	setCode := strings.ToUpper(strings.TrimSpace(item.SetCode))
	switch {
	case setName != "" && setCode != "":
		return setName + " (" + setCode + ")"
	case setName != "":
		return setName
	case setCode != "":
		return setCode
	default:
		return "Unknown set"
	}
}

func buildProfileGuessWinViews(items []guessCardWin) []profileGuessWinView {
	out := make([]profileGuessWinView, 0, len(items))
	for _, item := range items {
		modeLabel := "Practice"
		if item.IsDaily {
			modeLabel = "Daily"
		}
		out = append(out, profileGuessWinView{
			OracleID:   item.OracleID,
			CardName:   item.CardName,
			ImageURI:   item.ImageURI,
			DetailPath: cardPrintingDetailPath(item.OracleID, item.ScryfallID),
			WonLabel:   formatProfileJoined(item.WonAt),
			GuessCount: item.GuessCount,
			ModeLabel:  modeLabel,
		})
	}
	return out
}

func (a *App) HandleProfileAvatarPost(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		setFlash(w, "Invalid profile avatar selection.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	commanderName := strings.TrimSpace(r.Form.Get("commander_name"))
	action := strings.ToLower(strings.TrimSpace(r.Form.Get("action")))
	if action == "random" {
		card, err := cards.RandomCard(r.Context(), a.DB)
		if err != nil || card == nil || strings.TrimSpace(profileAvatarCardImageURI(*card)) == "" {
			setFlash(w, "Could not choose random card art.")
			http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
			return
		}
		if err := account.UpdateProfileAvatarCommander(r.Context(), a.DB, user.ID, strings.TrimSpace(card.Name)); err != nil {
			setFlash(w, "Could not update profile avatar.")
			http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
			return
		}
		setFlash(w, "Profile avatar updated.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	selectionRequired := strings.TrimSpace(r.Form.Get("selection_required")) == "1"
	if selectionRequired && commanderName == "" {
		setFlash(w, "Choose a card from the suggestions first.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}
	if commanderName == "" {
		if err := account.UpdateProfileAvatarCommander(r.Context(), a.DB, user.ID, ""); err != nil {
			setFlash(w, "Could not update profile avatar.")
			http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
			return
		}
		setFlash(w, "Profile avatar updated.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	verifiedSuggestion := false
	if selectionRequired {
		oracleID := strings.TrimSpace(r.Form.Get("avatar_oracle_id"))
		if oracleID == "" {
			setFlash(w, "Choose a card from the suggestions first.")
			http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
			return
		}
		if _, err := uuid.Parse(oracleID); err != nil {
			setFlash(w, "Choose a valid card suggestion.")
			http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
			return
		}
		card, err := cards.GetCardByOracleID(r.Context(), a.DB, oracleID)
		if err != nil || card == nil || !strings.EqualFold(strings.TrimSpace(card.Name), commanderName) {
			if err != nil && !errors.Is(err, cards.ErrCardNotFound) {
				a.RenderServerError(w, r, err)
				return
			}
			setFlash(w, "Choose a valid card suggestion.")
			http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
			return
		}
		commanderName = strings.TrimSpace(card.Name)
		verifiedSuggestion = true
	}

	if !verifiedSuggestion {
		publicDecks, err := decks.ListPublicDecksByUser(r.Context(), a.DB, user.ID, 60)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}

		allowed := false
		for _, d := range publicDecks {
			if strings.EqualFold(strings.TrimSpace(d.CommanderName), commanderName) {
				allowed = true
				commanderName = strings.TrimSpace(d.CommanderName)
				break
			}
		}
		if !allowed {
			setFlash(w, "Choose one of your commanders or select a card suggestion.")
			http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
			return
		}
	}

	if err := account.UpdateProfileAvatarCommander(r.Context(), a.DB, user.ID, commanderName); err != nil {
		setFlash(w, "Could not update profile avatar.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	setFlash(w, "Profile avatar updated.")
	http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
}

type profileArtResponse struct {
	Target      string `json:"target"`
	ScryfallID  string `json:"scryfall_id"`
	OracleID    string `json:"oracle_id"`
	Name        string `json:"name"`
	ImageURI    string `json:"image_uri"`
	SetLabel    string `json:"set_label"`
	Message     string `json:"message"`
	Error       string `json:"error,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`
}

func normalizeProfileArtTarget(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "picture", "profile_picture", "avatar":
		return "picture"
	case "background", "profile_background", "banner":
		return "background"
	default:
		return ""
	}
}

func wantsProfileArtJSON(r *http.Request) bool {
	return r != nil && strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json")
}

func writeProfileArtJSON(w http.ResponseWriter, status int, payload profileArtResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *App) HandleProfileArtPost(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		if wantsProfileArtJSON(r) {
			writeProfileArtJSON(w, http.StatusUnauthorized, profileArtResponse{
				Error:       "Sign in to customize your profile.",
				RedirectURL: "/login",
			})
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		if wantsProfileArtJSON(r) {
			writeProfileArtJSON(w, http.StatusBadRequest, profileArtResponse{Error: "Invalid profile art request."})
			return
		}
		setFlash(w, "Invalid profile art request.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	target := normalizeProfileArtTarget(r.Form.Get("target"))
	if target == "" {
		if wantsProfileArtJSON(r) {
			writeProfileArtJSON(w, http.StatusBadRequest, profileArtResponse{Error: "Choose profile picture or background."})
			return
		}
		setFlash(w, "Choose profile picture or background.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	var selected *cards.Card
	var err error
	if strings.EqualFold(strings.TrimSpace(r.Form.Get("action")), "random") {
		for attempt := 0; attempt < 5; attempt++ {
			selected, err = cards.RandomCard(r.Context(), a.DB)
			if err == nil && selected != nil && strings.TrimSpace(selected.ID) != "" && profileAvatarCardImageURI(*selected) != "" {
				break
			}
			selected = nil
		}
	} else {
		scryfallID := strings.TrimSpace(r.Form.Get("scryfall_id"))
		if _, parseErr := uuid.Parse(scryfallID); parseErr == nil {
			selected, err = cards.GetCardPrintingByScryfallID(r.Context(), a.DB, scryfallID)
		} else {
			err = cards.ErrCardNotFound
		}
	}
	if err != nil || selected == nil || strings.TrimSpace(selected.ID) == "" || profileAvatarCardImageURI(*selected) == "" {
		if err != nil && !errors.Is(err, cards.ErrCardNotFound) {
			a.RenderServerError(w, r, err)
			return
		}
		if wantsProfileArtJSON(r) {
			writeProfileArtJSON(w, http.StatusBadRequest, profileArtResponse{Error: "Choose a printing with available artwork."})
			return
		}
		setFlash(w, "Choose a printing with available artwork.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	if target == "picture" {
		err = account.UpdateProfilePicturePrint(r.Context(), a.DB, user.ID, selected.ID)
	} else {
		err = account.UpdateProfileBackgroundPrint(r.Context(), a.DB, user.ID, selected.ID)
	}
	if err != nil {
		if wantsProfileArtJSON(r) {
			writeProfileArtJSON(w, http.StatusInternalServerError, profileArtResponse{Error: "Could not update profile art."})
			return
		}
		setFlash(w, "Could not update profile art.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	message := "Profile picture updated."
	if target == "background" {
		message = "Profile background updated."
	}
	choice := profileAvatarChoiceFromCard(*selected)
	if wantsProfileArtJSON(r) {
		writeProfileArtJSON(w, http.StatusOK, profileArtResponse{
			Target:     target,
			ScryfallID: choice.ScryfallID,
			OracleID:   choice.OracleID,
			Name:       choice.Name,
			ImageURI:   choice.ImageURI,
			SetLabel:   choice.SetLabel,
			Message:    message,
		})
		return
	}

	setFlash(w, message)
	returnTo := normalizeLocalReturnPath(r.Form.Get("return_to"), userProfilePath(user.ID))
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func profileAvatarChoiceFromCard(card cards.Card) profileAvatarChoice {
	setLabel := strings.TrimSpace(card.SetName)
	setCode := strings.ToUpper(strings.TrimSpace(card.SetCode))
	if setLabel == "" {
		setLabel = setCode
	} else if setCode != "" {
		setLabel += " (" + setCode + ")"
	}
	if collector := strings.TrimSpace(card.CollectorNumber); collector != "" {
		setLabel += " #" + collector
	}
	return profileAvatarChoice{
		ScryfallID: strings.TrimSpace(card.ID),
		OracleID:   strings.TrimSpace(card.OracleID),
		Name:       strings.TrimSpace(card.Name),
		ImageURI:   profileAvatarCardImageURI(card),
		SetLabel:   strings.TrimSpace(setLabel),
	}
}

func (a *App) profileArtworkChoiceByPrintingID(r *http.Request, scryfallID string) (*profileAvatarChoice, error) {
	scryfallID = strings.TrimSpace(scryfallID)
	if scryfallID == "" {
		return nil, nil
	}
	card, err := cards.GetCardPrintingByScryfallID(r.Context(), a.DB, scryfallID)
	if errors.Is(err, cards.ErrCardNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	choice := profileAvatarChoiceFromCard(*card)
	if choice.ImageURI == "" {
		return nil, nil
	}
	return &choice, nil
}

func applyProfileArtwork(
	stats *profileStatsData,
	profile account.PublicProfile,
	profilePicture *profileAvatarChoice,
	profileBackground *profileAvatarChoice,
	legacyArtwork *profileAvatarChoice,
) {
	if stats == nil {
		return
	}
	stats.ProfilePicturePrintID = strings.TrimSpace(profile.ProfilePicturePrintID)
	stats.ProfileBackgroundPrintID = strings.TrimSpace(profile.ProfileBackgroundPrintID)
	if profilePicture != nil && strings.TrimSpace(profilePicture.ImageURI) != "" {
		stats.AvatarImageURI = strings.TrimSpace(profilePicture.ImageURI)
		stats.AvatarAlt = cardMetaValue(profilePicture.Name, profile.DisplayName)
	}
	if profileBackground != nil && strings.TrimSpace(profileBackground.ImageURI) != "" {
		stats.BackgroundImageURI = strings.TrimSpace(profileBackground.ImageURI)
		stats.BackgroundAlt = cardMetaValue(profileBackground.Name, profile.DisplayName)
	} else if legacyArtwork != nil && strings.TrimSpace(legacyArtwork.ImageURI) != "" {
		stats.BackgroundImageURI = strings.TrimSpace(legacyArtwork.ImageURI)
		stats.BackgroundAlt = cardMetaValue(legacyArtwork.Name, profile.DisplayName)
	} else {
		stats.BackgroundImageURI = stats.AvatarImageURI
		stats.BackgroundAlt = stats.AvatarAlt
	}
	if stats.AvatarAlt == "" {
		stats.AvatarAlt = profile.DisplayName
	}
	if stats.BackgroundAlt == "" {
		stats.BackgroundAlt = profile.DisplayName
	}
}

func (a *App) profileCustomAvatarChoice(r *http.Request, profile account.PublicProfile, items []deckListItem) (*profileAvatarChoice, error) {
	selectedName := strings.TrimSpace(profile.ProfileAvatarCommander)
	if selectedName == "" {
		return nil, nil
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.CommanderName), selectedName) {
			return nil, nil
		}
	}

	card, err := resolveProfileAvatarCard(r, a, selectedName)
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			return nil, nil
		}
		return nil, err
	}
	imageURI := profileAvatarCardImageURI(*card)
	if imageURI == "" {
		return nil, nil
	}
	return &profileAvatarChoice{
		Name:       card.Name,
		ImageURI:   imageURI,
		IsSelected: true,
	}, nil
}

func resolveProfileAvatarCard(r *http.Request, a *App, name string) (*cards.Card, error) {
	return cards.GetCardByName(r.Context(), a.DB, strings.TrimSpace(name))
}

func profileAvatarCardImageURI(card cards.Card) string {
	if imageURI := strings.TrimSpace(card.ArtCropURI); imageURI != "" {
		return imageURI
	}
	return strings.TrimSpace(card.ImageURI)
}

func profileAvatarDeckImageURI(item deckListItem) string {
	if imageURI := strings.TrimSpace(item.CommanderArtCropURI); imageURI != "" {
		return imageURI
	}
	return strings.TrimSpace(item.CommanderImageURI)
}

func buildProfileStats(profile account.PublicProfile, publicDecks []decks.Deck, items []deckListItem, customAvatar *profileAvatarChoice) profileStatsData {
	stats := profileStatsData{
		JoinedLabel:              formatProfileJoined(profile.CreatedAt),
		AveragePowerBracket:      averageProfilePowerBracket(publicDecks),
		Likes:                    topProfileTags(publicDecks, 3),
		UsuallyPlays:             topProfileFormat(publicDecks),
		FavoriteColorCombination: topProfileColorCombination(items),
		FavoriteColorPips:        favoriteProfileColorPips(items),
	}

	seenAvatar := map[string]bool{}
	selectedCommander := strings.TrimSpace(profile.ProfileAvatarCommander)
	selectedApplied := false
	var selectedChoice profileAvatarChoice
	for _, item := range items {
		imageURI := profileAvatarDeckImageURI(item)
		name := strings.TrimSpace(item.CommanderName)
		avatarKey := strings.ToLower(name)
		if imageURI == "" || name == "" || seenAvatar[avatarKey] {
			continue
		}
		seenAvatar[avatarKey] = true
		isSelected := selectedCommander != "" && strings.EqualFold(name, selectedCommander)
		if isSelected {
			stats.AvatarImageURI = imageURI
			stats.AvatarAlt = name
			selectedApplied = true
			selectedChoice = profileAvatarChoice{
				Name:       name,
				ImageURI:   imageURI,
				IsSelected: true,
			}
		}
		if stats.AvatarImageURI == "" {
			stats.AvatarImageURI = imageURI
			stats.AvatarAlt = name
		}
		if len(stats.AvatarChoices) < 4 {
			stats.AvatarChoices = append(stats.AvatarChoices, profileAvatarChoice{
				Name:       name,
				ImageURI:   imageURI,
				IsSelected: isSelected,
			})
		}
	}
	if !selectedApplied && customAvatar != nil && strings.TrimSpace(customAvatar.ImageURI) != "" {
		stats.AvatarImageURI = strings.TrimSpace(customAvatar.ImageURI)
		stats.AvatarAlt = cardMetaValue(customAvatar.Name, profile.DisplayName)
		selectedApplied = true
		selectedChoice = *customAvatar
		selectedChoice.IsSelected = true
	}
	if selectedApplied {
		hasSelectedChoice := false
		for i := range stats.AvatarChoices {
			stats.AvatarChoices[i].IsSelected = strings.EqualFold(stats.AvatarChoices[i].Name, selectedChoice.Name)
			if stats.AvatarChoices[i].IsSelected {
				hasSelectedChoice = true
			}
		}
		if !hasSelectedChoice {
			stats.AvatarChoices = append([]profileAvatarChoice{selectedChoice}, stats.AvatarChoices...)
			if len(stats.AvatarChoices) > 4 {
				stats.AvatarChoices = stats.AvatarChoices[:4]
			}
		}
	} else if len(stats.AvatarChoices) > 0 {
		stats.AvatarChoices[0].IsSelected = true
	}
	if stats.AvatarAlt == "" {
		stats.AvatarAlt = profile.DisplayName
	}
	return stats
}

func formatProfileJoined(joined time.Time) string {
	if joined.IsZero() {
		return "Unknown"
	}
	return joined.Format("Jan 2006")
}

func averageProfilePowerBracket(publicDecks []decks.Deck) string {
	var sum float64
	var count int
	for _, d := range publicDecks {
		value := profilePowerBracketValue(d.PowerBracket)
		if value <= 0 {
			continue
		}
		sum += float64(value)
		count++
	}
	if count == 0 {
		return "Not enough data"
	}
	avg := sum / float64(count)
	if avg == float64(int(avg)) {
		return fmt.Sprintf("%.0f / 5", avg)
	}
	return fmt.Sprintf("%.1f / 5", avg)
}

func profilePowerBracketValue(raw string) int {
	normalized := decks.NormalizePowerBracket(raw)
	if normalized == "" {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(strings.Split(normalized, "-")[0]))
	if err != nil {
		return 0
	}
	return value
}

func topProfileTags(publicDecks []decks.Deck, limit int) []string {
	if limit <= 0 {
		return nil
	}
	counts := map[string]int{}
	order := []string{}
	for _, d := range publicDecks {
		for _, tag := range decks.SplitTags(d.Tags) {
			if tag == "" {
				continue
			}
			if _, ok := counts[tag]; !ok {
				order = append(order, tag)
			}
			counts[tag]++
		}
	}
	return topProfileCountLabels(counts, order, limit)
}

func topProfileFormat(publicDecks []decks.Deck) string {
	counts := map[string]int{}
	order := []string{}
	for _, d := range publicDecks {
		format := decks.NormalizeFormat(d.Format)
		if format == "" {
			format = "Commander"
		}
		if _, ok := counts[format]; !ok {
			order = append(order, format)
		}
		counts[format]++
	}
	top := topProfileCountLabels(counts, order, 1)
	if len(top) == 0 {
		return "Not enough data"
	}
	return top[0]
}

func topProfileColorCombination(items []deckListItem) string {
	counts := map[string]int{}
	order := []string{}
	for _, item := range items {
		name := strings.TrimSpace(item.ColorIdentityName)
		if name == "" {
			continue
		}
		if _, ok := counts[name]; !ok {
			order = append(order, name)
		}
		counts[name]++
	}
	if len(counts) == 0 {
		return "Not enough data"
	}

	maxCount := 0
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}
	// A tie is not a preference. The deck list is ordered by publication time,
	// so choosing the first tied item would incorrectly make the latest public
	// deck look like the user's favorite color pairing.
	winners := 0
	for _, count := range counts {
		if count == maxCount {
			winners++
		}
	}
	if winners != 1 {
		return "No clear favorite yet"
	}

	top := topProfileCountLabels(counts, order, 1)
	return top[0]
}

func favoriteProfileColorPips(items []deckListItem) []manaPipView {
	favorite := topProfileColorCombination(items)
	if favorite == "Not enough data" || favorite == "No clear favorite yet" {
		return nil
	}
	for _, item := range items {
		if item.ColorIdentityName == favorite {
			return item.ColorPips
		}
	}
	return nil
}

func topProfileCountLabels(counts map[string]int, order []string, limit int) []string {
	if limit <= 0 || len(counts) == 0 {
		return nil
	}

	index := map[string]int{}
	for i, label := range order {
		index[label] = i
	}
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.SliceStable(labels, func(i, j int) bool {
		if counts[labels[i]] == counts[labels[j]] {
			return index[labels[i]] < index[labels[j]]
		}
		return counts[labels[i]] > counts[labels[j]]
	})
	if len(labels) > limit {
		labels = labels[:limit]
	}
	return labels
}
