package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"

	"github.com/google/uuid"
)

// searchResult is a shared view model for both general card search and
// commander search. It flattens the core fields we display in the grid plus
// a JSON-encoded faces slice for MDFC / multi-faced cards.
type searchResult struct {
	OracleID   string
	DetailPath string
	Name       string
	ManaCost   string
	TypeLine   string
	OracleText string
	ImageURI   string
	PriceUSD   string
	Artist     string
	SetCode    string
	SetName    string
	Rarity     string
	ReleasedAt string

	// FacesJSON is a JSON-encoded []cards.CardFace (from cards.Card.Faces).
	// It is used by the frontend to support MDFC "flip" behavior in the detail modals.
	FacesJSON string
}

type cardSearchTypeFilter struct {
	Value string `json:"value"`
	Mode  string `json:"mode"`
}

type cardSearchPageData struct {
	NameQuery       string
	TextQuery       string
	TypeOptions     []string
	TypeFilters     []cardSearchTypeFilter
	TypeFiltersJSON string
	ColorsSelected  map[string]bool
	ColorMode       string
	ManaValueMin    string
	ManaValueMax    string
	Rarity          string
	SetQuery        string
	ArtistQuery     string
	CommanderOnly   bool
}

type cardListPageData struct {
	Query       string
	Results     []searchResult
	HasSearched bool
}

type cardSearchRequest struct {
	NameQuery       string
	TextQuery       string
	TypeFilters     []cardSearchTypeFilter
	ColorParams     []string
	ColorMode       string
	ManaValueMin    *float64
	ManaValueMax    *float64
	ManaValueMinRaw string
	ManaValueMaxRaw string
	Rarity          string
	SetQuery        string
	ArtistQuery     string
	CommanderOnly   bool
	HasSearched     bool
}

type cardDetailFaceData struct {
	Name          string
	ManaCost      string
	TypeLine      string
	OracleText    string
	ImageURI      string
	Artist        string
	Colors        string
	ColorIdentity string
}

type cardDetailPrintingData struct {
	ImageURI        string
	SetName         string
	SetCode         string
	CollectorNumber string
	Rarity          string
	ReleasedAt      string
	Artist          string
	PriceUSD        string
	Lang            string
	ScryfallURI     string
}

type cardDetailData struct {
	OracleID        string
	Name            string
	ManaCost        string
	TypeLine        string
	OracleText      string
	ImageURI        string
	PriceUSD        string
	Artist          string
	SetCode         string
	SetName         string
	Rarity          string
	ReleasedAt      string
	Layout          string
	Colors          string
	ColorIdentity   string
	ManaValue       string
	EDHRecRank      string
	CommanderStatus string
	ScryfallURI     string
	Faces           []cardDetailFaceData
}

type cardDetailPageData struct {
	Card      cardDetailData
	Printings []cardDetailPrintingData
}

var advancedCardTypeOptions = []string{
	"Creature",
	"Artifact",
	"Enchantment",
	"Land",
	"Planeswalker",
	"Battle",
	"Instant",
	"Sorcery",
}

type cardResolveResponse struct {
	OracleID             string            `json:"oracle_id,omitempty"`
	Name                 string            `json:"name"`
	ManaCost             string            `json:"mana_cost,omitempty"`
	TypeLine             string            `json:"type_line,omitempty"`
	OracleText           string            `json:"oracle_text,omitempty"`
	CMC                  float64           `json:"cmc"`
	IsCommanderCandidate bool              `json:"is_commander_candidate"`
	Faces                []cards.CardFace  `json:"faces,omitempty"`
	ImageURIs            map[string]string `json:"image_uris,omitempty"`
	Prices               struct {
		USD string `json:"usd,omitempty"`
	} `json:"prices,omitempty"`
}

type cardVersionResponse struct {
	ScryfallID      string           `json:"scryfall_id,omitempty"`
	Lang            string           `json:"lang,omitempty"`
	Name            string           `json:"name"`
	ManaCost        string           `json:"mana_cost,omitempty"`
	TypeLine        string           `json:"type_line,omitempty"`
	OracleText      string           `json:"oracle_text,omitempty"`
	ImageURI        string           `json:"image_uri,omitempty"`
	PriceUSD        string           `json:"price_usd,omitempty"`
	Artist          string           `json:"artist,omitempty"`
	SetCode         string           `json:"set_code,omitempty"`
	SetName         string           `json:"set_name,omitempty"`
	CollectorNumber string           `json:"collector_number,omitempty"`
	Rarity          string           `json:"rarity,omitempty"`
	ReleasedAt      string           `json:"released_at,omitempty"`
	Faces           []cards.CardFace `json:"faces,omitempty"`
}

type cardVersionsPayload struct {
	Versions []cardVersionResponse `json:"versions"`
}

type deckCardSearchResult struct {
	OracleID string `json:"oracle_id"`
	Name     string `json:"name"`
}

type deckCardSearchPayload struct {
	Exact   bool                   `json:"exact"`
	Results []deckCardSearchResult `json:"results"`
}

// buildSearchResults converts a slice of cards.Card into a slice of searchResult,
// pre-encoding the Faces slice into JSON for MDFC support.
func buildSearchResults(cardsIn []cards.Card) []searchResult {
	viewResults := make([]searchResult, 0, len(cardsIn))

	for _, c := range cardsIn {
		facesJSON := ""
		if len(c.Faces) > 0 {
			if b, err := json.Marshal(c.Faces); err == nil {
				facesJSON = string(b)
			}
		}

		viewResults = append(viewResults, searchResult{
			OracleID:   c.OracleID,
			DetailPath: cardDetailPath(c.OracleID),
			Name:       c.Name,
			ManaCost:   c.ManaCost,
			TypeLine:   c.TypeLine,
			OracleText: c.OracleText,
			ImageURI:   c.ImageURI,
			PriceUSD:   c.PriceUSD,
			Artist:     c.Artist,
			SetCode:    c.SetCode,
			SetName:    c.SetName,
			Rarity:     c.Rarity,
			ReleasedAt: c.ReleasedAt,
			FacesJSON:  facesJSON,
		})
	}

	return viewResults
}

func cardDetailPath(oracleID string) string {
	oracleID = strings.TrimSpace(oracleID)
	if oracleID == "" {
		return "/cards"
	}
	return "/cards/view/" + url.PathEscape(oracleID)
}

func singleCardResultPath(results []cards.Card) string {
	if len(results) != 1 {
		return ""
	}
	return cardDetailPath(results[0].OracleID)
}

func parseCardOracleIDFromPath(r *http.Request) string {
	const prefix = "/cards/view/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return ""
	}

	oracleID, err := url.PathUnescape(strings.TrimSpace(strings.TrimPrefix(r.URL.Path, prefix)))
	if err != nil || oracleID == "" || strings.Contains(oracleID, "/") {
		return ""
	}
	return oracleID
}

func formatCardColorNames(values []string) string {
	normalized := normalizeSearchColors(values)
	if len(normalized) == 0 {
		return "Colorless"
	}

	colorNames := map[string]string{
		"W": "White",
		"U": "Blue",
		"B": "Black",
		"R": "Red",
		"G": "Green",
		"C": "Colorless",
	}

	out := make([]string, 0, len(normalized))
	for _, value := range normalized {
		if label, ok := colorNames[value]; ok {
			out = append(out, label)
		}
	}
	if len(out) == 0 {
		return "Colorless"
	}
	return strings.Join(out, ", ")
}

func formatCardManaValue(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func formatCardPrice(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Not listed"
	}
	if strings.HasPrefix(value, "$") {
		return value
	}
	return "$" + value
}

func cardMetaValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func formatCardEDHRecRank(rank int) string {
	if rank <= 0 {
		return "Unranked"
	}
	return strconv.Itoa(rank)
}

func formatCommanderStatus(card cards.Card) string {
	switch {
	case card.IsCommanderCandidate && card.CommanderLegal:
		return "Commander-legal commander"
	case card.CommanderLegal:
		return "Commander-legal"
	case card.IsCommanderCandidate:
		return "Commander candidate"
	default:
		return "Not commander-legal"
	}
}

func buildCardDetailPageData(card cards.Card, printings []cards.Card) cardDetailPageData {
	if len(printings) == 0 {
		printings = []cards.Card{card}
	}

	cardImageURI := strings.TrimSpace(card.ImageURI)
	cardArtist := strings.TrimSpace(card.Artist)

	faces := make([]cardDetailFaceData, 0, len(card.Faces))
	for _, face := range card.Faces {
		faceImageURI := strings.TrimSpace(face.ImageURI)
		if cardImageURI == "" && faceImageURI != "" {
			cardImageURI = faceImageURI
		}
		if cardArtist == "" && strings.TrimSpace(face.Artist) != "" {
			cardArtist = strings.TrimSpace(face.Artist)
		}

		faces = append(faces, cardDetailFaceData{
			Name:          cardMetaValue(face.Name, card.Name),
			ManaCost:      strings.TrimSpace(face.ManaCost),
			TypeLine:      strings.TrimSpace(face.TypeLine),
			OracleText:    cardMetaValue(face.OracleText, "No oracle text."),
			ImageURI:      faceImageURI,
			Artist:        cardMetaValue(face.Artist, "Unknown"),
			Colors:        formatCardColorNames(face.Colors),
			ColorIdentity: formatCardColorNames(face.ColorID),
		})
	}

	printingItems := make([]cardDetailPrintingData, 0, len(printings))
	for _, printing := range printings {
		imageURI := strings.TrimSpace(printing.ImageURI)
		if imageURI == "" && len(printing.Faces) > 0 {
			imageURI = strings.TrimSpace(printing.Faces[0].ImageURI)
		}

		printingItems = append(printingItems, cardDetailPrintingData{
			ImageURI:        imageURI,
			SetName:         cardMetaValue(printing.SetName, "Unknown set"),
			SetCode:         strings.ToUpper(strings.TrimSpace(printing.SetCode)),
			CollectorNumber: cardMetaValue(printing.CollectorNumber, "N/A"),
			Rarity:          cardMetaValue(printing.Rarity, "N/A"),
			ReleasedAt:      cardMetaValue(printing.ReleasedAt, "Unknown"),
			Artist:          cardMetaValue(printing.Artist, "Unknown"),
			PriceUSD:        formatCardPrice(printing.PriceUSD),
			Lang:            cardMetaValue(strings.ToUpper(strings.TrimSpace(printing.Lang)), "EN"),
			ScryfallURI:     strings.TrimSpace(printing.ScryfallURI),
		})
	}

	return cardDetailPageData{
		Card: cardDetailData{
			OracleID:        card.OracleID,
			Name:            cardMetaValue(card.Name, "Unknown card"),
			ManaCost:        strings.TrimSpace(card.ManaCost),
			TypeLine:        cardMetaValue(card.TypeLine, "Type unknown"),
			OracleText:      cardMetaValue(card.OracleText, "No oracle text."),
			ImageURI:        cardImageURI,
			PriceUSD:        formatCardPrice(card.PriceUSD),
			Artist:          cardMetaValue(cardArtist, "Unknown"),
			SetCode:         strings.ToUpper(strings.TrimSpace(card.SetCode)),
			SetName:         cardMetaValue(card.SetName, "Unknown set"),
			Rarity:          cardMetaValue(card.Rarity, "N/A"),
			ReleasedAt:      cardMetaValue(card.ReleasedAt, "Unknown"),
			Layout:          cardMetaValue(card.Layout, "Unknown"),
			Colors:          formatCardColorNames(card.Colors),
			ColorIdentity:   formatCardColorNames(card.ColorIdentity),
			ManaValue:       formatCardManaValue(card.CMC),
			EDHRecRank:      formatCardEDHRecRank(card.EDHRecRank),
			CommanderStatus: formatCommanderStatus(card),
			ScryfallURI:     strings.TrimSpace(card.ScryfallURI),
			Faces:           faces,
		},
		Printings: printingItems,
	}
}

func normalizeCardSearchTypeMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "not":
		return "not"
	default:
		return "is"
	}
}

func normalizeCardSearchTypeValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, option := range advancedCardTypeOptions {
		if strings.EqualFold(option, raw) {
			return option
		}
	}
	return ""
}

func normalizeCardSearchTypeFilters(values, modes []string) []cardSearchTypeFilter {
	if len(values) == 0 {
		return nil
	}

	seen := map[string]int{}
	out := make([]cardSearchTypeFilter, 0, len(values))
	for idx, value := range values {
		normalizedValue := normalizeCardSearchTypeValue(value)
		if normalizedValue == "" {
			continue
		}

		mode := "is"
		if idx < len(modes) {
			mode = normalizeCardSearchTypeMode(modes[idx])
		}

		key := strings.ToLower(normalizedValue)
		if existingIdx, ok := seen[key]; ok {
			out[existingIdx].Mode = mode
			continue
		}

		seen[key] = len(out)
		out = append(out, cardSearchTypeFilter{
			Value: normalizedValue,
			Mode:  mode,
		})
	}
	return out
}

func cardSearchTypeFiltersJSON(filters []cardSearchTypeFilter) string {
	encoded, err := json.Marshal(filters)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func toCardTypeFilters(filters []cardSearchTypeFilter) []cards.CardTypeFilter {
	out := make([]cards.CardTypeFilter, 0, len(filters))
	for _, filter := range filters {
		value := strings.TrimSpace(filter.Value)
		if value == "" {
			continue
		}
		out = append(out, cards.CardTypeFilter{
			Value:   value,
			Negated: normalizeCardSearchTypeMode(filter.Mode) == "not",
		})
	}
	return out
}

func normalizeSearchColors(values []string) []string {
	allowed := []string{"W", "U", "B", "R", "G", "C"}
	seen := map[string]bool{}
	for _, raw := range values {
		value := strings.ToUpper(strings.TrimSpace(raw))
		for _, allowedValue := range allowed {
			if value == allowedValue {
				seen[value] = true
				break
			}
		}
	}

	if seen["C"] && (seen["W"] || seen["U"] || seen["B"] || seen["R"] || seen["G"]) {
		delete(seen, "C")
	}

	out := make([]string, 0, len(allowed))
	for _, value := range allowed {
		if seen[value] {
			out = append(out, value)
		}
	}
	return out
}

func selectedColorMap(values []string) map[string]bool {
	selected := map[string]bool{
		"W": false,
		"U": false,
		"B": false,
		"R": false,
		"G": false,
		"C": false,
	}
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if _, ok := selected[value]; ok {
			selected[value] = true
		}
	}
	return selected
}

func normalizeAdvancedColorMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "exact":
		return "exact"
	case "at_most":
		return "at_most"
	default:
		return "includes"
	}
}

func parseOptionalFloatFilter(raw string) (*float64, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, "", nil
	}

	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil, trimmed, err
	}
	if value < 0 {
		return nil, trimmed, fmt.Errorf("must be zero or greater")
	}
	return &value, trimmed, nil
}

func parseCardSearchRequest(q url.Values) (cardSearchRequest, string) {
	req := cardSearchRequest{
		NameQuery:     strings.TrimSpace(q.Get("q")),
		TextQuery:     strings.TrimSpace(q.Get("text")),
		Rarity:        strings.ToLower(strings.TrimSpace(q.Get("rarity"))),
		SetQuery:      strings.TrimSpace(q.Get("set")),
		ArtistQuery:   strings.TrimSpace(q.Get("artist")),
		CommanderOnly: strings.TrimSpace(q.Get("commander")) == "1",
	}

	req.TypeFilters = normalizeCardSearchTypeFilters(q["type_value"], q["type_mode"])
	if len(req.TypeFilters) == 0 {
		if legacyType := strings.TrimSpace(q.Get("type")); legacyType != "" {
			req.TypeFilters = normalizeCardSearchTypeFilters([]string{legacyType}, []string{"is"})
		}
	}

	req.ColorParams = normalizeSearchColors(q["color"])
	if len(req.ColorParams) == 0 {
		if legacyColors := normalizeSearchColors(q["color_identity"]); len(legacyColors) > 0 {
			req.ColorParams = legacyColors
		} else if legacyColors := normalizeSearchColors(q["card_color"]); len(legacyColors) > 0 {
			req.ColorParams = legacyColors
		}
	}

	colorModeRaw := strings.TrimSpace(q.Get("color_mode"))
	if colorModeRaw == "" {
		if legacyRaw := strings.TrimSpace(q.Get("color_identity_mode")); legacyRaw != "" {
			colorModeRaw = legacyRaw
		} else if legacyRaw := strings.TrimSpace(q.Get("card_color_mode")); legacyRaw != "" {
			colorModeRaw = legacyRaw
		}
	}
	req.ColorMode = normalizeAdvancedColorMode(colorModeRaw)
	req.ManaValueMinRaw = q.Get("mana_value_min")
	req.ManaValueMaxRaw = q.Get("mana_value_max")

	hasFilters := req.TextQuery != "" ||
		len(req.TypeFilters) > 0 ||
		len(req.ColorParams) > 0 ||
		strings.TrimSpace(req.ManaValueMinRaw) != "" ||
		strings.TrimSpace(req.ManaValueMaxRaw) != "" ||
		req.Rarity != "" ||
		req.SetQuery != "" ||
		req.ArtistQuery != "" ||
		req.CommanderOnly
	req.HasSearched = req.NameQuery != "" || hasFilters

	var errMsg string
	manaValueMin, manaValueMinText, minErr := parseOptionalFloatFilter(req.ManaValueMinRaw)
	manaValueMax, manaValueMaxText, maxErr := parseOptionalFloatFilter(req.ManaValueMaxRaw)
	req.ManaValueMin = manaValueMin
	req.ManaValueMax = manaValueMax
	req.ManaValueMinRaw = manaValueMinText
	req.ManaValueMaxRaw = manaValueMaxText

	if minErr != nil || maxErr != nil {
		errMsg = "Mana value filters must be valid numbers."
	} else if req.ManaValueMin != nil && req.ManaValueMax != nil && *req.ManaValueMin > *req.ManaValueMax {
		errMsg = "Mana value minimum cannot be greater than the maximum."
	}

	return req, errMsg
}

func (req cardSearchRequest) searchParams(limit int) cards.CardSearchParams {
	return cards.CardSearchParams{
		Query:         req.NameQuery,
		OracleText:    req.TextQuery,
		TypeFilters:   toCardTypeFilters(req.TypeFilters),
		Colors:        req.ColorParams,
		ColorMode:     req.ColorMode,
		ManaValueMin:  req.ManaValueMin,
		ManaValueMax:  req.ManaValueMax,
		Rarity:        req.Rarity,
		SetQuery:      req.SetQuery,
		ArtistQuery:   req.ArtistQuery,
		CommanderOnly: req.CommanderOnly,
		Limit:         limit,
	}
}

func (a *App) HandleCardList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := CurrentUser(r)
	flash := readFlash(w, r)
	req, errMsg := parseCardSearchRequest(r.URL.Query())

	var results []cards.Card
	if req.HasSearched && errMsg == "" {
		found, err := cards.SearchCards(r.Context(), a.DB, req.searchParams(120))
		if err != nil {
			errMsg = "We couldn't search for cards right now. Please try again."
		} else {
			if singlePath := singleCardResultPath(found); singlePath != "" {
				http.Redirect(w, r, singlePath, http.StatusSeeOther)
				return
			}
			results = found
		}
	}

	data := TemplateData{
		CurrentUser: user,
		Data: cardListPageData{
			Query:       req.NameQuery,
			Results:     buildSearchResults(results),
			HasSearched: req.HasSearched,
		},
		Flash: flash,
		Error: errMsg,
	}

	a.Renderer.Render(w, "cards_list", data)
}

func (a *App) HandleCardSearch(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)
	req, errMsg := parseCardSearchRequest(r.URL.Query())

	data := TemplateData{
		CurrentUser: user,
		Data: cardSearchPageData{
			NameQuery:       req.NameQuery,
			TextQuery:       req.TextQuery,
			TypeOptions:     advancedCardTypeOptions,
			TypeFilters:     req.TypeFilters,
			TypeFiltersJSON: cardSearchTypeFiltersJSON(req.TypeFilters),
			ColorsSelected:  selectedColorMap(req.ColorParams),
			ColorMode:       req.ColorMode,
			ManaValueMin:    req.ManaValueMinRaw,
			ManaValueMax:    req.ManaValueMaxRaw,
			Rarity:          req.Rarity,
			SetQuery:        req.SetQuery,
			ArtistQuery:     req.ArtistQuery,
			CommanderOnly:   req.CommanderOnly,
		},
		Flash: flash,
		Error: errMsg,
	}

	a.Renderer.Render(w, "cards_search", data)
}

func (a *App) HandleCardShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := CurrentUser(r)
	flash := readFlash(w, r)

	oracleID := parseCardOracleIDFromPath(r)
	if oracleID == "" {
		a.RenderNotFound(w, r)
		return
	}
	if _, err := uuid.Parse(oracleID); err != nil {
		a.RenderNotFound(w, r)
		return
	}

	card, err := cards.GetCardByOracleID(r.Context(), a.DB, oracleID)
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			a.RenderNotFound(w, r)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	printings, err := cards.ListCardVersionsByOracleID(r.Context(), a.DB, oracleID, 120)
	if err != nil {
		if !errors.Is(err, cards.ErrCardNotFound) {
			a.RenderServerError(w, r, err)
			return
		}
		printings = []cards.Card{*card}
	}

	data := TemplateData{
		CurrentUser: user,
		Data:        buildCardDetailPageData(*card, printings),
		Flash:       flash,
		WideLayout:  true,
	}

	a.Renderer.Render(w, "card_show", data)
}

func (a *App) HandleCardAddToDeck(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
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

	deckIDStr := r.Form.Get("deck_id")
	cardName := r.Form.Get("card_name")

	if deckIDStr == "" || cardName == "" {
		http.Error(w, "missing deck or card", http.StatusBadRequest)
		return
	}

	deckID, err := strconv.ParseInt(deckIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid deck id", http.StatusBadRequest)
		return
	}

	// Ensure deck belongs to current user
	if _, err := decks.GetDeck(r.Context(), a.DB, deckID, user.ID); err != nil {
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	// Resolve best local card match so card add uses the same fuzzy logic as search.
	resolved, err := cards.ResolveCardByNameFuzzy(r.Context(), a.DB, cardName)
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			http.Error(w, "card not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not add card", http.StatusInternalServerError)
		return
	}
	if err := decks.AddCard(r.Context(), a.DB, deckID, resolved.OracleID, 1); err != nil {
		http.Error(w, "could not add card", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/decks/"+strconv.FormatInt(deckID, 10), http.StatusSeeOther)
}

func (a *App) HandleCardResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "missing query", http.StatusBadRequest)
		return
	}

	exactRaw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("exact")))
	exactOnly := exactRaw == "1" || exactRaw == "true" || exactRaw == "yes"

	var (
		card *cards.Card
		err  error
	)
	if exactOnly {
		card, err = cards.GetCardByName(r.Context(), a.DB, query)
	} else {
		card, err = cards.ResolveCardByNameFuzzy(r.Context(), a.DB, query)
	}
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			http.Error(w, "card not found", http.StatusNotFound)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	resp := cardResolveResponse{
		OracleID:             card.OracleID,
		Name:                 card.Name,
		ManaCost:             card.ManaCost,
		TypeLine:             card.TypeLine,
		OracleText:           card.OracleText,
		CMC:                  card.CMC,
		IsCommanderCandidate: card.IsCommanderCandidate,
	}
	if len(card.Faces) > 0 {
		resp.Faces = card.Faces
	}
	if card.ImageURI != "" {
		resp.ImageURIs = map[string]string{
			"normal": card.ImageURI,
		}
	}
	resp.Prices.USD = card.PriceUSD

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleCardAutocomplete powers deckbuilder search UX:
// return one exact match when present; otherwise return fuzzy candidates.
func (a *App) HandleCardAutocomplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "missing query", http.StatusBadRequest)
		return
	}

	limit := 20
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil {
			limit = parsed
		}
	}
	commanderOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("commander_only")), "true")

	results, exact, err := cards.SearchCardNamesExactThenFuzzy(r.Context(), a.DB, query, limit, commanderOnly)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	payload := deckCardSearchPayload{
		Exact:   exact,
		Results: make([]deckCardSearchResult, 0, len(results)),
	}
	for _, card := range results {
		payload.Results = append(payload.Results, deckCardSearchResult{
			OracleID: card.OracleID,
			Name:     card.Name,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *App) HandleCardVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	oracleID := strings.TrimSpace(r.URL.Query().Get("oracle_id"))
	if name == "" && oracleID == "" {
		http.Error(w, "missing name or oracle_id", http.StatusBadRequest)
		return
	}

	var (
		versions []cards.Card
		err      error
	)
	if oracleID != "" {
		versions, err = cards.ListCardVersionsByOracleID(r.Context(), a.DB, oracleID, 500)
		if err != nil && errors.Is(err, cards.ErrCardNotFound) && name != "" {
			versions, err = cards.ListCardVersionsByName(r.Context(), a.DB, name, 500)
		}
	} else {
		versions, err = cards.ListCardVersionsByName(r.Context(), a.DB, name, 500)
	}
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			http.Error(w, "card not found", http.StatusNotFound)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	payload := cardVersionsPayload{
		Versions: make([]cardVersionResponse, 0, len(versions)),
	}
	for _, c := range versions {
		payload.Versions = append(payload.Versions, cardVersionResponse{
			ScryfallID:      c.ID,
			Lang:            c.Lang,
			Name:            c.Name,
			ManaCost:        c.ManaCost,
			TypeLine:        c.TypeLine,
			OracleText:      c.OracleText,
			ImageURI:        c.ImageURI,
			PriceUSD:        c.PriceUSD,
			Artist:          c.Artist,
			SetCode:         c.SetCode,
			SetName:         c.SetName,
			CollectorNumber: c.CollectorNumber,
			Rarity:          c.Rarity,
			ReleasedAt:      c.ReleasedAt,
			Faces:           c.Faces,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *App) HandleCommanderSearch(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	hasSearched := query != ""

	var rawResults []cards.Card
	var errMsg string

	if hasSearched {
		found, err := cards.SearchCards(r.Context(), a.DB, cards.CardSearchParams{
			Query:         query,
			CommanderOnly: true,
			Limit:         120,
		})
		if err != nil {
			errMsg = "There was a problem searching for commanders. Please try again."
		} else {
			rawResults = found
		}
	}

	// Build view-model results with pre-encoded faces JSON for MDFC commanders.
	viewResults := buildSearchResults(rawResults)

	recommendedResults := make([]searchResult, 0)
	if recommended, err := cards.RandomTopCommanders(r.Context(), a.DB, 6, 1500); err == nil {
		recommendedResults = buildSearchResults(recommended)
	}

	fallbackReturn := "/decks/new"
	returnTo := canonicalizeLocalReturnPath(r.URL.Query().Get("return_to"), fallbackReturn)
	returnLabel := commanderSearchReturnLabel(returnTo)

	data := TemplateData{
		CurrentUser: user,
		Data: struct {
			Query       string
			Results     []searchResult
			Recommended []searchResult
			ReturnTo    string
			ReturnLabel string
		}{
			Query:       query,
			Results:     viewResults,
			Recommended: recommendedResults,
			ReturnTo:    returnTo,
			ReturnLabel: returnLabel,
		},
		Flash: flash,
		Error: errMsg,
	}

	a.Renderer.Render(w, "commanders_search", data)
}
