package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

// deckListItem is a lightweight view model for rendering the "My Decks" page.
// It includes the core deck fields plus an optional CommanderImageURI for UI use.
type deckListItem struct {
	ID                  int64
	OwnerID             int64
	OwnerDisplayName    string
	Name                string
	Description         string
	Tags                string
	Format              string
	CommanderName       string
	CommanderImageURI   string
	CommanderArtCropURI string
	ColorPips           []manaPipView
	ColorIdentityName   string
	IsPublic            bool
	PublicSlug          string
	PowerBracket        string
}

type manaPipView struct {
	Symbol   string
	Label    string
	ImageURI string
}

type deckNewPageData struct {
	Mode            string
	Format          string
	CommanderName   string
	Name            string
	Description     string
	PowerBracket    string
	ImportText      string
	ImportUnmatched []string
}

type publicDeckPageData struct {
	Deck               *decks.Deck
	DeckCards          []decks.DeckCard
	SideboardDeckCards []decks.DeckCard
	MaybeDeckCards     []decks.DeckCard
	Analytics          deckAnalyticsData
	Commander          *cards.Card
	Owner              *account.PublicProfile
}

type publicDeckListPageData struct {
	CommanderName string
	Format        string
	PowerBracket  string
	ColorFilters  []string
	ColorSelected map[string]bool
	Items         []deckListItem
}

func normalizeDeckBuilderMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sandbox":
		return "sandbox"
	case "import":
		return "import"
	default:
		return ""
	}
}

func defaultDeckFormat(rawFormat, commanderName, mode string) string {
	if strings.TrimSpace(rawFormat) != "" {
		switch decks.NormalizeFormat(rawFormat) {
		case "Commander":
			return "Commander"
		case "Sandbox":
			return "Sandbox"
		}
		if strings.TrimSpace(commanderName) != "" {
			return "Commander"
		}
		return "Sandbox"
	}
	if strings.TrimSpace(commanderName) != "" {
		return "Commander"
	}
	if normalizeDeckBuilderMode(mode) == "sandbox" {
		return "Sandbox"
	}
	return "Sandbox"
}

func defaultDeckPowerBracket(rawPowerBracket, format string) string {
	if defaultDeckFormat(format, "", "") != "Commander" {
		return ""
	}
	return decks.NormalizePowerBracket(rawPowerBracket)
}

func applyCommanderCardMetaToDeckItem(item *deckListItem, card cards.DBCard) {
	if item == nil {
		return
	}
	if imageURI := strings.TrimSpace(card.ImageURI); imageURI != "" {
		item.CommanderImageURI = imageURI
	}
	if artCropURI := strings.TrimSpace(card.ArtCropURI); artCropURI != "" {
		item.CommanderArtCropURI = artCropURI
	}
	item.ColorPips = manaPipsForColorIdentity(card.ColorIdentity)
	item.ColorIdentityName = colorCombinationName(card.ColorIdentity)
}

func manaPipsForColorIdentity(raw string) []manaPipView {
	colors := normalizeColorIdentitySymbols(raw)
	if len(colors) == 0 {
		colors = []string{"C"}
	}

	out := make([]manaPipView, 0, len(colors))
	for _, symbol := range colors {
		out = append(out, manaPipView{
			Symbol:   symbol,
			Label:    manaColorLabel(symbol),
			ImageURI: "https://svgs.scryfall.io/card-symbols/" + symbol + ".svg",
		})
	}
	return out
}

func normalizeColorIdentitySymbols(raw string) []string {
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(strings.ToUpper(strings.TrimSpace(raw)), func(r rune) bool {
		return r == ',' || r == ' ' || r == '/' || r == ';'
	}) {
		switch part {
		case "W", "U", "B", "R", "G", "C":
			seen[part] = true
		}
	}

	order := []string{"W", "U", "B", "R", "G"}
	out := make([]string, 0, len(order))
	for _, symbol := range order {
		if seen[symbol] {
			out = append(out, symbol)
		}
	}
	if len(out) == 0 && seen["C"] {
		return []string{"C"}
	}
	return out
}

func manaColorLabel(symbol string) string {
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "W":
		return "White"
	case "U":
		return "Blue"
	case "B":
		return "Black"
	case "R":
		return "Red"
	case "G":
		return "Green"
	default:
		return "Colorless"
	}
}

func colorCombinationName(raw string) string {
	colors := normalizeColorIdentitySymbols(raw)
	if len(colors) == 0 || (len(colors) == 1 && colors[0] == "C") {
		return "Colorless"
	}

	key := strings.Join(colors, "")
	switch key {
	case "W":
		return "Mono-white"
	case "U":
		return "Mono-blue"
	case "B":
		return "Mono-black"
	case "R":
		return "Mono-red"
	case "G":
		return "Mono-green"
	case "WU":
		return "Azorius"
	case "UB":
		return "Dimir"
	case "BR":
		return "Rakdos"
	case "RG":
		return "Gruul"
	case "WG":
		return "Selesnya"
	case "WB":
		return "Orzhov"
	case "UR":
		return "Izzet"
	case "BG":
		return "Golgari"
	case "WR":
		return "Boros"
	case "UG":
		return "Simic"
	case "WUB":
		return "Esper"
	case "UBR":
		return "Grixis"
	case "BRG":
		return "Jund"
	case "WRG":
		return "Naya"
	case "WUG":
		return "Bant"
	case "WBG":
		return "Abzan"
	case "WUR":
		return "Jeskai"
	case "UBG":
		return "Sultai"
	case "WBR":
		return "Mardu"
	case "URG":
		return "Temur"
	case "WUBR":
		return "Four-color (no green)"
	case "WUBG":
		return "Four-color (no red)"
	case "WURG":
		return "Four-color (no black)"
	case "WBRG":
		return "Four-color (no blue)"
	case "UBRG":
		return "Four-color (no white)"
	case "WUBRG":
		return "Five-color"
	default:
		return strings.Join(colors, "")
	}
}

func (a *App) renderDeckNew(w http.ResponseWriter, user *account.User, flash, errMsg string, page deckNewPageData) {
	data := TemplateData{
		CurrentUser: user,
		Data:        page,
		Flash:       flash,
		Error:       errMsg,
	}
	a.Renderer.Render(w, "decks_new", data)
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

func parsePublicDeckSlugFromPath(r *http.Request) string {
	const prefix = "/decks/public/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return ""
	}
	slug := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, prefix))
	if slug == "" || strings.Contains(slug, "/") {
		return ""
	}
	return slug
}

func parseUserProfileIDFromPath(r *http.Request) (int64, error) {
	const prefix = "/users/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return 0, fmt.Errorf("invalid user path")
	}
	segment := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, prefix))
	if segment == "" {
		return 0, fmt.Errorf("missing user id")
	}
	if idx := strings.IndexByte(segment, '/'); idx >= 0 {
		segment = segment[:idx]
	}
	id, err := strconv.ParseInt(segment, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid user id")
	}
	return id, nil
}

func parseJSONBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func visibleDeckCardCount(deckCards []decks.DeckCard, commanderName string) int {
	total := 0
	commanderName = strings.TrimSpace(commanderName)

	for _, dc := range deckCards {
		name := strings.TrimSpace(dc.CardName)
		if name == "" || dc.Quantity <= 0 {
			continue
		}
		if commanderName != "" && strings.EqualFold(name, commanderName) {
			continue
		}
		total += dc.Quantity
	}

	return total
}

func deckCardQuantityTotal(cards []decks.DeckCard) int {
	total := 0
	for _, dc := range cards {
		if dc.Quantity <= 0 {
			continue
		}
		total += dc.Quantity
	}
	return total
}

func (a *App) commanderForFormatChange(ctx context.Context, deckID int64, currentFormat, nextFormat, currentCommander string) (string, error) {
	currentCommander = strings.TrimSpace(currentCommander)
	currentFormat = defaultDeckFormat(currentFormat, currentCommander, "")
	nextFormat = defaultDeckFormat(nextFormat, currentCommander, "")

	if nextFormat == "Commander" {
		if currentFormat == "Commander" {
			return currentCommander, nil
		}
		return "", nil
	}

	if currentCommander == "" {
		return "", nil
	}

	if currentFormat == "Commander" {
		card, err := cards.EnsureCardByName(ctx, a.DB, currentCommander)
		if err == nil && card.OracleID != "" {
			if addErr := decks.AddCard(ctx, a.DB, deckID, card.OracleID, 1); addErr != nil {
				return "", addErr
			}
		}
	}

	return "", nil
}
