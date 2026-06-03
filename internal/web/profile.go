package web

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type profilePageData struct {
	Profile account.PublicProfile
	Items   []deckListItem
	Stats   profileStatsData
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
	AvatarChoices            []profileAvatarChoice
	CanEdit                  bool
}

type profileAvatarChoice struct {
	Name       string
	ImageURI   string
	IsSelected bool
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
			if c, ok := commanderCards[strings.ToLower(commanderName)]; ok {
				applyCommanderCardMetaToDeckItem(&item, c)
			}
		}
		items = append(items, item)
	}

	currentUser := CurrentUser(r)
	stats := buildProfileStats(*profile, publicDecks, items)
	stats.CanEdit = currentUser != nil && currentUser.ID == profile.ID

	a.Renderer.Render(w, "profile_show", TemplateData{
		CurrentUser: currentUser,
		Data: profilePageData{
			Profile: *profile,
			Items:   items,
			Stats:   stats,
		},
		Flash: readFlash(w, r),
	})
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
		setFlash(w, "Choose a commander from your public decks.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	if err := account.UpdateProfileAvatarCommander(r.Context(), a.DB, user.ID, commanderName); err != nil {
		setFlash(w, "Could not update profile avatar.")
		http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
		return
	}

	setFlash(w, "Profile avatar updated.")
	http.Redirect(w, r, userProfilePath(user.ID), http.StatusSeeOther)
}

func buildProfileStats(profile account.PublicProfile, publicDecks []decks.Deck, items []deckListItem) profileStatsData {
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
		imageURI := strings.TrimSpace(item.CommanderImageURI)
		name := strings.TrimSpace(item.CommanderName)
		if imageURI == "" || name == "" || seenAvatar[name] {
			continue
		}
		seenAvatar[name] = true
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
	if selectedApplied {
		hasSelectedChoice := false
		for i := range stats.AvatarChoices {
			stats.AvatarChoices[i].IsSelected = strings.EqualFold(stats.AvatarChoices[i].Name, selectedCommander)
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
	top := topProfileCountLabels(counts, order, 1)
	if len(top) == 0 {
		return "Not enough data"
	}
	return top[0]
}

func favoriteProfileColorPips(items []deckListItem) []manaPipView {
	favorite := topProfileColorCombination(items)
	if favorite == "Not enough data" {
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
