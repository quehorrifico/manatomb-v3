package web

import (
	"database/sql"
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
	OracleID             string
	DetailPath           string
	Name                 string
	ManaCost             string
	TypeLine             string
	OracleText           string
	ImageURI             string
	PriceUSD             string
	Artist               string
	SetCode              string
	SetName              string
	Rarity               string
	ReleasedAt           string
	ColorIdentity        string
	ManaValue            string
	IsCommanderCandidate bool

	// FacesJSON is a JSON-encoded []cards.CardFace (from cards.Card.Faces).
	// It is used by the frontend to support MDFC "flip" behavior in the detail modals.
	FacesJSON string
}

type cardSearchTypeFilter struct {
	Value string `json:"value"`
	Mode  string `json:"mode"`
}

type cardSearchFilterChip struct {
	Label      string
	RemovePath string
}

type cardSearchSelectOption struct {
	Value string
	Label string
}

type cardSearchPageData struct {
	NameQuery             string
	NameExact             bool
	ManaCostQuery         string
	TextQuery             string
	TextMode              string
	TypeOptions           []string
	TypeOptionsJSON       string
	TypeFilters           []cardSearchTypeFilter
	TypeFiltersJSON       string
	TypePartial           bool
	SetSuggestions        []string
	LayoutOptions         []cardSearchSelectOption
	SelectedLayout        string
	StatOptions           []cardSearchSelectOption
	StatOperatorOptions   []cardSearchSelectOption
	SelectedStat          string
	SelectedStatOperator  string
	StatValue             string
	PriceOperatorOptions  []cardSearchSelectOption
	SelectedPriceOperator string
	PriceValue            string
	ColorsSelected        map[string]bool
	ColorMode             string
	ManaValueMin          string
	ManaValueMax          string
	Rarity                string
	SetQuery              string
	ArtistQuery           string
	CommanderOnly         bool
	CommanderLegal        bool
	IncludeTokens         bool
	SearchActionPath      string
	ClearPath             string
}

type cardListPageData struct {
	Results         []searchResult
	HasSearched     bool
	SavedDecks      []deckListItem
	AppliedFilters  []cardSearchFilterChip
	CurrentPath     string
	ClearPath       string
	EditFiltersPath string
}

type cardSearchRequest struct {
	NameQuery       string
	NameExact       bool
	ManaCostQuery   string
	TextQuery       string
	TextMode        string
	TypeFilters     []cardSearchTypeFilter
	TypePartial     bool
	Layout          string
	Stat            string
	StatOperator    string
	StatValue       *int
	StatValueRaw    string
	StatMin         *float64
	StatMax         *float64
	StatMinRaw      string
	StatMaxRaw      string
	ColorParams     []string
	ColorMode       string
	ManaValueMin    *float64
	ManaValueMax    *float64
	ManaValueMinRaw string
	ManaValueMaxRaw string
	PriceOperator   string
	PriceValue      *float64
	PriceValueRaw   string
	PriceMin        *float64
	PriceMax        *float64
	PriceMinRaw     string
	PriceMaxRaw     string
	Rarity          string
	SetQuery        string
	ArtistQuery     string
	CommanderOnly   bool
	CommanderLegal  bool
	IncludeTokens   bool
	HasSearched     bool
}

type cardDetailFaceData struct {
	Name          string
	ManaCost      string
	TypeLine      string
	OracleText    string
	FlavorText    string
	ImageURI      string
	Artist        string
	Colors        string
	ColorIdentity string
}

type cardDetailPrintingData struct {
	Name            string
	ManaCost        string
	TypeLine        string
	OracleText      string
	FlavorText      string
	ImageURI        string
	SetName         string
	SetCode         string
	CollectorNumber string
	Rarity          string
	ReleasedAt      string
	Artist          string
	PriceUSD        string
	PriceSort       float64
	Lang            string
	ScryfallURI     string
	Faces           []cardDetailFaceData
}

type cardDetailData struct {
	OracleID        string
	Name            string
	ManaCost        string
	TypeLine        string
	OracleText      string
	FlavorText      string
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
	Card        cardDetailData
	Printings   []cardDetailPrintingData
	SavedDecks  []deckListItem
	CurrentPath string
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

var advancedCardLayoutOptions = []cardSearchSelectOption{
	{Value: "", Label: "Any layout"},
	{Value: "normal", Label: "Normal"},
	{Value: "transform", Label: "Transform"},
	{Value: "modal_dfc", Label: "Modal DFC"},
	{Value: "split", Label: "Split"},
	{Value: "flip", Label: "Flip"},
	{Value: "adventure", Label: "Adventure"},
	{Value: "meld", Label: "Meld"},
	{Value: "leveler", Label: "Leveler"},
	{Value: "class", Label: "Class"},
	{Value: "saga", Label: "Saga"},
	{Value: "token", Label: "Token"},
	{Value: "double_faced_token", Label: "Double-Faced Token"},
}

var advancedCardStatOptions = []cardSearchSelectOption{
	{Value: "mana_value", Label: "Mana Value"},
	{Value: "power", Label: "Power"},
	{Value: "toughness", Label: "Toughness"},
	{Value: "loyalty", Label: "Loyalty"},
}

var advancedCardStatOperatorOptions = []cardSearchSelectOption{
	{Value: "eq", Label: "Equal to"},
	{Value: "lt", Label: "Less than"},
	{Value: "gt", Label: "Greater than"},
	{Value: "lte", Label: "Less than or equal to"},
	{Value: "gte", Label: "Greater than or equal to"},
	{Value: "neq", Label: "Not equal to"},
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
			OracleID:             c.OracleID,
			DetailPath:           cardDetailPath(c.OracleID),
			Name:                 c.Name,
			ManaCost:             c.ManaCost,
			TypeLine:             c.TypeLine,
			OracleText:           c.OracleText,
			ImageURI:             c.ImageURI,
			PriceUSD:             c.PriceUSD,
			Artist:               c.Artist,
			SetCode:              c.SetCode,
			SetName:              c.SetName,
			Rarity:               c.Rarity,
			ReleasedAt:           c.ReleasedAt,
			ColorIdentity:        formatCardColorNames(c.ColorIdentity),
			ManaValue:            formatCardManaValue(c.CMC),
			IsCommanderCandidate: c.IsCommanderCandidate,
			FacesJSON:            facesJSON,
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

func cardPriceSortValue(value string) float64 {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "$"))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
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

func firstNonEmptyCardFlavor(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstFaceFlavor(faces []cards.CardFace) string {
	for _, face := range faces {
		if value := strings.TrimSpace(face.FlavorText); value != "" {
			return value
		}
	}
	return ""
}

func firstPrintingFlavor(printings []cards.Card) string {
	for _, printing := range printings {
		if value := firstNonEmptyCardFlavor(
			printing.FlavorText,
			firstFaceFlavor(printing.Faces),
		); value != "" {
			return value
		}
	}
	return ""
}

func buildCardDetailPageData(card cards.Card, printings []cards.Card) cardDetailPageData {
	if len(printings) == 0 {
		printings = []cards.Card{card}
	}

	cardImageURI := strings.TrimSpace(card.ImageURI)
	cardArtist := strings.TrimSpace(card.Artist)
	cardFlavorText := strings.TrimSpace(card.FlavorText)
	if cardFlavorText == "" {
		cardFlavorText = firstPrintingFlavor(printings)
	}

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
			FlavorText:    strings.TrimSpace(face.FlavorText),
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

		printingFaces := make([]cardDetailFaceData, 0, len(printing.Faces))
		for _, face := range printing.Faces {
			printingFaces = append(printingFaces, cardDetailFaceData{
				Name:          cardMetaValue(face.Name, printing.Name),
				ManaCost:      strings.TrimSpace(face.ManaCost),
				TypeLine:      cardMetaValue(face.TypeLine, printing.TypeLine),
				OracleText:    cardMetaValue(face.OracleText, "No oracle text."),
				FlavorText:    strings.TrimSpace(face.FlavorText),
				ImageURI:      strings.TrimSpace(face.ImageURI),
				Artist:        cardMetaValue(face.Artist, printing.Artist),
				Colors:        formatCardColorNames(face.Colors),
				ColorIdentity: formatCardColorNames(face.ColorID),
			})
		}
		if len(printingFaces) == 0 && len(faces) > 0 {
			printingFaces = append(printingFaces, faces...)
		}
		printingFlavorText := firstNonEmptyCardFlavor(
			printing.FlavorText,
			firstFaceFlavor(printing.Faces),
		)

		printingItems = append(printingItems, cardDetailPrintingData{
			Name:            cardMetaValue(printing.Name, card.Name),
			ManaCost:        strings.TrimSpace(printing.ManaCost),
			TypeLine:        cardMetaValue(printing.TypeLine, card.TypeLine),
			OracleText:      cardMetaValue(printing.OracleText, "No oracle text."),
			FlavorText:      printingFlavorText,
			ImageURI:        imageURI,
			SetName:         cardMetaValue(printing.SetName, "Unknown set"),
			SetCode:         strings.ToUpper(strings.TrimSpace(printing.SetCode)),
			CollectorNumber: cardMetaValue(printing.CollectorNumber, "N/A"),
			Rarity:          cardMetaValue(printing.Rarity, "N/A"),
			ReleasedAt:      cardMetaValue(printing.ReleasedAt, "Unknown"),
			Artist:          cardMetaValue(printing.Artist, "Unknown"),
			PriceUSD:        formatCardPrice(printing.PriceUSD),
			PriceSort:       cardPriceSortValue(printing.PriceUSD),
			Lang:            cardMetaValue(strings.ToUpper(strings.TrimSpace(printing.Lang)), "EN"),
			ScryfallURI:     strings.TrimSpace(printing.ScryfallURI),
			Faces:           printingFaces,
		})
	}

	return cardDetailPageData{
		Card: cardDetailData{
			OracleID:        card.OracleID,
			Name:            cardMetaValue(card.Name, "Unknown card"),
			ManaCost:        strings.TrimSpace(card.ManaCost),
			TypeLine:        cardMetaValue(card.TypeLine, "Type unknown"),
			OracleText:      cardMetaValue(card.OracleText, "No oracle text."),
			FlavorText:      cardFlavorText,
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
	raw = strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if raw == "" {
		return ""
	}
	return raw
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

func copyCardSearchTypeFilters(filters []cardSearchTypeFilter) []cardSearchTypeFilter {
	if len(filters) == 0 {
		return nil
	}
	out := make([]cardSearchTypeFilter, len(filters))
	copy(out, filters)
	return out
}

func copyStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func cloneCardSearchRequest(req cardSearchRequest) cardSearchRequest {
	req.TypeFilters = copyCardSearchTypeFilters(req.TypeFilters)
	req.ColorParams = copyStringSlice(req.ColorParams)
	return req
}

func cardSearchTypeFiltersJSON(filters []cardSearchTypeFilter) string {
	encoded, err := json.Marshal(filters)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func cardSearchStringSliceJSON(values []string) string {
	encoded, err := json.Marshal(values)
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

func normalizeCardSearchTextMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "not":
		return "not"
	default:
		return "contains"
	}
}

func normalizeCardSearchLayout(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	for _, option := range advancedCardLayoutOptions {
		if option.Value == value {
			return value
		}
	}
	return ""
}

func normalizeCardSearchStat(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "power":
		return "power"
	case "toughness":
		return "toughness"
	case "loyalty":
		return "loyalty"
	default:
		return "mana_value"
	}
}

func normalizeCardSearchStatOperator(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "lt", "<":
		return "lt"
	case "gt", ">":
		return "gt"
	case "lte", "<=":
		return "lte"
	case "gte", ">=":
		return "gte"
	case "neq", "!=", "<>":
		return "neq"
	default:
		return "eq"
	}
}

func formatCardSearchStatLabel(raw string) string {
	switch normalizeCardSearchStat(raw) {
	case "power":
		return "Power"
	case "toughness":
		return "Toughness"
	case "loyalty":
		return "Loyalty"
	default:
		return "Mana Value"
	}
}

func formatCardSearchStatOperatorLabel(raw string) string {
	switch normalizeCardSearchStatOperator(raw) {
	case "lt":
		return "<"
	case "gt":
		return ">"
	case "lte":
		return "<="
	case "gte":
		return ">="
	case "neq":
		return "!="
	default:
		return "="
	}
}

func formatCardSearchPriceOperatorLabel(raw string) string {
	return formatCardSearchStatOperatorLabel(raw)
}

func fallbackCardTypeSuggestions() []string {
	out := make([]string, len(advancedCardTypeOptions))
	copy(out, advancedCardTypeOptions)
	return out
}

func loadCardSearchTypeSuggestions(r *http.Request, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(r.Context(), `
		WITH tokens AS (
			SELECT DISTINCT lower(token) AS token
			FROM oracle_cards oc,
			LATERAL regexp_split_to_table(
				regexp_replace(COALESCE(oc.type_line, ''), '[^[:alnum:]]+', ' ', 'g'),
				'\s+'
			) AS token
		)
		SELECT token
		FROM tokens
		WHERE token <> ''
		  AND length(token) >= 3
		ORDER BY token ASC
		LIMIT 1200
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 256)
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		out = append(out, strings.ToUpper(token[:1])+token[1:])
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return fallbackCardTypeSuggestions(), nil
	}
	return out, nil
}

func loadCardSearchSetSuggestions(r *http.Request, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(r.Context(), `
		WITH suggestions AS (
			SELECT DISTINCT upper(btrim(set_code)) AS suggestion
			FROM card_prints
			WHERE btrim(COALESCE(set_code, '')) <> ''
			UNION
			SELECT DISTINCT btrim(set_name) AS suggestion
			FROM card_prints
			WHERE btrim(COALESCE(set_name, '')) <> ''
		)
		SELECT suggestion
		FROM suggestions
		ORDER BY suggestion ASC
		LIMIT 1200
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 256)
	for rows.Next() {
		var suggestion string
		if err := rows.Scan(&suggestion); err != nil {
			return nil, err
		}
		suggestion = strings.TrimSpace(suggestion)
		if suggestion == "" {
			continue
		}
		out = append(out, suggestion)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
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

func parseOptionalIntFilter(raw string) (*int, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, "", nil
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, trimmed, err
	}
	if value < 0 {
		return nil, trimmed, fmt.Errorf("must be zero or greater")
	}
	return &value, trimmed, nil
}

func parseCardSearchRequest(q url.Values) (cardSearchRequest, string) {
	statOperatorRaw := strings.TrimSpace(q.Get("stat_op"))
	statValueRaw := strings.TrimSpace(q.Get("stat_value"))
	priceOperatorRaw := strings.TrimSpace(q.Get("price_op"))
	priceValueRaw := strings.TrimSpace(q.Get("price_value"))
	statMinRaw := strings.TrimSpace(q.Get("stat_min"))
	statMaxRaw := strings.TrimSpace(q.Get("stat_max"))
	statRaw := strings.TrimSpace(q.Get("stat"))
	if statRaw == "" && (strings.TrimSpace(q.Get("mana_value_min")) != "" || strings.TrimSpace(q.Get("mana_value_max")) != "") {
		statRaw = "mana_value"
		if statMinRaw == "" {
			statMinRaw = strings.TrimSpace(q.Get("mana_value_min"))
		}
		if statMaxRaw == "" {
			statMaxRaw = strings.TrimSpace(q.Get("mana_value_max"))
		}
	}

	req := cardSearchRequest{
		NameQuery:      strings.TrimSpace(q.Get("q")),
		NameExact:      strings.TrimSpace(q.Get("name_exact")) == "1",
		ManaCostQuery:  strings.TrimSpace(q.Get("mana_cost")),
		TextQuery:      strings.TrimSpace(q.Get("text")),
		TextMode:       normalizeCardSearchTextMode(q.Get("text_mode")),
		TypePartial:    strings.TrimSpace(q.Get("type_partial")) == "1",
		Layout:         normalizeCardSearchLayout(q.Get("layout")),
		Stat:           normalizeCardSearchStat(statRaw),
		StatOperator:   normalizeCardSearchStatOperator(statOperatorRaw),
		StatValueRaw:   statValueRaw,
		StatMinRaw:     statMinRaw,
		StatMaxRaw:     statMaxRaw,
		Rarity:         strings.ToLower(strings.TrimSpace(q.Get("rarity"))),
		SetQuery:       strings.TrimSpace(q.Get("set")),
		ArtistQuery:    strings.TrimSpace(q.Get("artist")),
		CommanderOnly:  strings.TrimSpace(q.Get("commander")) == "1",
		CommanderLegal: strings.TrimSpace(q.Get("commander_legal")) == "1",
		IncludeTokens:  strings.TrimSpace(q.Get("include_tokens")) == "1",
		PriceOperator:  normalizeCardSearchStatOperator(priceOperatorRaw),
		PriceValueRaw:  priceValueRaw,
		PriceMinRaw:    q.Get("price_min"),
		PriceMaxRaw:    q.Get("price_max"),
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

	hasFilters := req.ManaCostQuery != "" ||
		req.TextQuery != "" ||
		len(req.TypeFilters) > 0 ||
		req.Layout != "" ||
		strings.TrimSpace(req.StatValueRaw) != "" ||
		strings.TrimSpace(req.StatMinRaw) != "" ||
		strings.TrimSpace(req.StatMaxRaw) != "" ||
		len(req.ColorParams) > 0 ||
		strings.TrimSpace(req.PriceValueRaw) != "" ||
		strings.TrimSpace(req.PriceMinRaw) != "" ||
		strings.TrimSpace(req.PriceMaxRaw) != "" ||
		req.Rarity != "" ||
		req.SetQuery != "" ||
		req.ArtistQuery != "" ||
		req.CommanderOnly ||
		req.CommanderLegal ||
		req.IncludeTokens
	req.HasSearched = req.NameQuery != "" || hasFilters

	var errMsg string
	statValue, statValueText, statValueErr := parseOptionalIntFilter(req.StatValueRaw)
	req.StatValue = statValue
	req.StatValueRaw = statValueText

	if statValueErr != nil {
		errMsg = "Stat value must be a valid whole number."
	}

	statMin, statMinText, statMinErr := parseOptionalFloatFilter(req.StatMinRaw)
	statMax, statMaxText, statMaxErr := parseOptionalFloatFilter(req.StatMaxRaw)
	req.StatMin = statMin
	req.StatMax = statMax
	req.StatMinRaw = statMinText
	req.StatMaxRaw = statMaxText

	if errMsg == "" {
		if statMinErr != nil || statMaxErr != nil {
			errMsg = "Stat filters must be valid numbers."
		} else if req.StatMin != nil && req.StatMax != nil && *req.StatMin > *req.StatMax {
			errMsg = formatCardSearchStatLabel(req.Stat) + " minimum cannot be greater than the maximum."
		}
	}

	priceValue, priceValueText, priceValueErr := parseOptionalFloatFilter(req.PriceValueRaw)
	req.PriceValue = priceValue
	req.PriceValueRaw = priceValueText

	if errMsg == "" && priceValueErr != nil {
		errMsg = "Price value must be a valid number."
	}

	priceMin, priceMinText, priceMinErr := parseOptionalFloatFilter(req.PriceMinRaw)
	priceMax, priceMaxText, priceMaxErr := parseOptionalFloatFilter(req.PriceMaxRaw)
	req.PriceMin = priceMin
	req.PriceMax = priceMax
	req.PriceMinRaw = priceMinText
	req.PriceMaxRaw = priceMaxText

	if errMsg == "" {
		if priceMinErr != nil || priceMaxErr != nil {
			errMsg = "Price filters must be valid numbers."
		} else if req.PriceMin != nil && req.PriceMax != nil && *req.PriceMin > *req.PriceMax {
			errMsg = "Price minimum cannot be greater than the maximum."
		}
	}

	return req, errMsg
}

func cardSearchQueryValues(req cardSearchRequest) url.Values {
	values := url.Values{}

	if value := strings.TrimSpace(req.NameQuery); value != "" {
		values.Set("q", value)
		if req.NameExact {
			values.Set("name_exact", "1")
		}
	}
	if value := strings.TrimSpace(req.ManaCostQuery); value != "" {
		values.Set("mana_cost", value)
	}
	if value := strings.TrimSpace(req.TextQuery); value != "" {
		values.Set("text", value)
		if normalizeCardSearchTextMode(req.TextMode) != "contains" {
			values.Set("text_mode", normalizeCardSearchTextMode(req.TextMode))
		}
	}
	for _, filter := range req.TypeFilters {
		value := strings.TrimSpace(filter.Value)
		if value == "" {
			continue
		}
		values.Add("type_value", value)
		values.Add("type_mode", normalizeCardSearchTypeMode(filter.Mode))
	}
	if req.TypePartial && len(req.TypeFilters) > 0 {
		values.Set("type_partial", "1")
	}
	for _, color := range req.ColorParams {
		color = strings.ToUpper(strings.TrimSpace(color))
		if color == "" {
			continue
		}
		values.Add("color", color)
	}
	if len(req.ColorParams) > 0 && normalizeAdvancedColorMode(req.ColorMode) != "includes" {
		values.Set("color_mode", normalizeAdvancedColorMode(req.ColorMode))
	}
	if value := normalizeCardSearchLayout(req.Layout); value != "" {
		values.Set("layout", value)
	}
	if value := strings.TrimSpace(req.StatValueRaw); value != "" {
		values.Set("stat", normalizeCardSearchStat(req.Stat))
		values.Set("stat_op", normalizeCardSearchStatOperator(req.StatOperator))
		values.Set("stat_value", value)
	}
	if value := strings.TrimSpace(req.StatMinRaw); value != "" {
		values.Set("stat", normalizeCardSearchStat(req.Stat))
		values.Set("stat_min", value)
	}
	if value := strings.TrimSpace(req.StatMaxRaw); value != "" {
		values.Set("stat", normalizeCardSearchStat(req.Stat))
		values.Set("stat_max", value)
	}
	if value := strings.TrimSpace(req.PriceValueRaw); value != "" {
		values.Set("price_op", normalizeCardSearchStatOperator(req.PriceOperator))
		values.Set("price_value", value)
	}
	if value := strings.TrimSpace(req.PriceMinRaw); value != "" {
		values.Set("price_min", value)
	}
	if value := strings.TrimSpace(req.PriceMaxRaw); value != "" {
		values.Set("price_max", value)
	}
	if value := strings.ToLower(strings.TrimSpace(req.Rarity)); value != "" {
		values.Set("rarity", value)
	}
	if value := strings.TrimSpace(req.SetQuery); value != "" {
		values.Set("set", value)
	}
	if value := strings.TrimSpace(req.ArtistQuery); value != "" {
		values.Set("artist", value)
	}
	if req.CommanderOnly {
		values.Set("commander", "1")
	}
	if req.CommanderLegal {
		values.Set("commander_legal", "1")
	}
	if req.IncludeTokens {
		values.Set("include_tokens", "1")
	}

	return values
}

func cardSearchPath(req cardSearchRequest) string {
	values := cardSearchQueryValues(req)
	if encoded := values.Encode(); encoded != "" {
		return "/cards?" + encoded
	}
	return "/cards"
}

func cardSearchEditPath(req cardSearchRequest) string {
	values := cardSearchQueryValues(req)
	if encoded := values.Encode(); encoded != "" {
		values.Set("edit", "1")
		return "/cards/search?" + values.Encode()
	}
	return "/cards/search"
}

func buildCardSearchFilterChips(req cardSearchRequest) []cardSearchFilterChip {
	chips := make([]cardSearchFilterChip, 0)

	if value := strings.TrimSpace(req.NameQuery); value != "" {
		next := cloneCardSearchRequest(req)
		next.NameQuery = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      "Name: " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if req.NameExact && strings.TrimSpace(req.NameQuery) != "" {
		next := cloneCardSearchRequest(req)
		next.NameExact = false
		chips = append(chips, cardSearchFilterChip{
			Label:      "Exact name match",
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.ManaCostQuery); value != "" {
		next := cloneCardSearchRequest(req)
		next.ManaCostQuery = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      "Mana Cost: " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.TextQuery); value != "" {
		next := cloneCardSearchRequest(req)
		next.TextQuery = ""
		next.TextMode = "contains"
		chips = append(chips, cardSearchFilterChip{
			Label:      "Text: " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if strings.TrimSpace(req.TextQuery) != "" && normalizeCardSearchTextMode(req.TextMode) == "not" {
		next := cloneCardSearchRequest(req)
		next.TextMode = "contains"
		chips = append(chips, cardSearchFilterChip{
			Label:      "Text: Does not contain",
			RemovePath: cardSearchPath(next),
		})
	}

	for idx, filter := range req.TypeFilters {
		next := cloneCardSearchRequest(req)
		next.TypeFilters = append(copyCardSearchTypeFilters(req.TypeFilters[:idx]), req.TypeFilters[idx+1:]...)
		label := "Type: " + strings.TrimSpace(filter.Value)
		if normalizeCardSearchTypeMode(filter.Mode) == "not" {
			label = "Type: NOT " + strings.TrimSpace(filter.Value)
		}
		chips = append(chips, cardSearchFilterChip{
			Label:      label,
			RemovePath: cardSearchPath(next),
		})
	}

	if req.TypePartial && len(req.TypeFilters) > 0 {
		next := cloneCardSearchRequest(req)
		next.TypePartial = false
		chips = append(chips, cardSearchFilterChip{
			Label:      "Allow partial type matches",
			RemovePath: cardSearchPath(next),
		})
	}

	if len(req.ColorParams) > 0 {
		mode := normalizeAdvancedColorMode(req.ColorMode)
		if mode == "exact" || mode == "at_most" {
			next := cloneCardSearchRequest(req)
			next.ColorMode = "includes"
			label := "Color match: Exactly these"
			if mode == "at_most" {
				label = "Color match: At most these"
			}
			chips = append(chips, cardSearchFilterChip{
				Label:      label,
				RemovePath: cardSearchPath(next),
			})
		}

		for idx, color := range req.ColorParams {
			next := cloneCardSearchRequest(req)
			next.ColorParams = append(copyStringSlice(req.ColorParams[:idx]), req.ColorParams[idx+1:]...)
			if len(next.ColorParams) == 0 {
				next.ColorMode = "includes"
			}
			chips = append(chips, cardSearchFilterChip{
				Label:      "Color: " + formatCardColorNames([]string{color}),
				RemovePath: cardSearchPath(next),
			})
		}
	}

	if value := normalizeCardSearchLayout(req.Layout); value != "" {
		next := cloneCardSearchRequest(req)
		next.Layout = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      "Layout: " + formatCardSearchLayoutLabel(value),
			RemovePath: cardSearchPath(next),
		})
	}

	statLabel := formatCardSearchStatLabel(req.Stat)
	if value := strings.TrimSpace(req.StatValueRaw); value != "" {
		next := cloneCardSearchRequest(req)
		next.StatValue = nil
		next.StatValueRaw = ""
		next.StatOperator = "eq"
		chips = append(chips, cardSearchFilterChip{
			Label:      statLabel + " " + formatCardSearchStatOperatorLabel(req.StatOperator) + " " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.StatMinRaw); value != "" {
		next := cloneCardSearchRequest(req)
		next.StatMin = nil
		next.StatMinRaw = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      statLabel + " >= " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.StatMaxRaw); value != "" {
		next := cloneCardSearchRequest(req)
		next.StatMax = nil
		next.StatMaxRaw = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      statLabel + " <= " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.PriceMinRaw); value != "" {
		next := cloneCardSearchRequest(req)
		next.PriceMin = nil
		next.PriceMinRaw = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      "Price >= $" + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.PriceValueRaw); value != "" {
		next := cloneCardSearchRequest(req)
		next.PriceValue = nil
		next.PriceValueRaw = ""
		next.PriceOperator = "eq"
		chips = append(chips, cardSearchFilterChip{
			Label:      "Price " + formatCardSearchPriceOperatorLabel(req.PriceOperator) + " $" + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.PriceMaxRaw); value != "" {
		next := cloneCardSearchRequest(req)
		next.PriceMax = nil
		next.PriceMaxRaw = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      "Price <= $" + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.Rarity); value != "" {
		next := cloneCardSearchRequest(req)
		next.Rarity = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      "Rarity: " + formatCardSearchRarityLabel(value),
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.SetQuery); value != "" {
		next := cloneCardSearchRequest(req)
		next.SetQuery = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      "Set: " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if value := strings.TrimSpace(req.ArtistQuery); value != "" {
		next := cloneCardSearchRequest(req)
		next.ArtistQuery = ""
		chips = append(chips, cardSearchFilterChip{
			Label:      "Artist: " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	if req.CommanderOnly {
		next := cloneCardSearchRequest(req)
		next.CommanderOnly = false
		chips = append(chips, cardSearchFilterChip{
			Label:      "Commander candidates",
			RemovePath: cardSearchPath(next),
		})
	}

	if req.CommanderLegal {
		next := cloneCardSearchRequest(req)
		next.CommanderLegal = false
		chips = append(chips, cardSearchFilterChip{
			Label:      "Commander legal",
			RemovePath: cardSearchPath(next),
		})
	}

	if req.IncludeTokens {
		next := cloneCardSearchRequest(req)
		next.IncludeTokens = false
		chips = append(chips, cardSearchFilterChip{
			Label:      "Include tokens",
			RemovePath: cardSearchPath(next),
		})
	}

	return chips
}

func buildSavedDeckPickerItems(userDecks []decks.Deck) []deckListItem {
	items := make([]deckListItem, 0, len(userDecks))
	for _, d := range userDecks {
		items = append(items, deckListItem{
			ID:            d.ID,
			Name:          d.Name,
			Description:   d.Description,
			Format:        d.Format,
			CommanderName: d.CommanderName,
			IsPublic:      d.IsPublic,
			PublicSlug:    d.PublicSlug,
			PowerBracket:  d.PowerBracket,
		})
	}
	return items
}

func formatCardSearchRarityLabel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "common":
		return "Common"
	case "uncommon":
		return "Uncommon"
	case "rare":
		return "Rare"
	case "mythic":
		return "Mythic"
	default:
		return strings.TrimSpace(raw)
	}
}

func formatCardSearchLayoutLabel(raw string) string {
	value := normalizeCardSearchLayout(raw)
	for _, option := range advancedCardLayoutOptions {
		if option.Value == value {
			return option.Label
		}
	}
	return strings.TrimSpace(raw)
}

func (req cardSearchRequest) searchParams(limit int) cards.CardSearchParams {
	var statValue *float64
	if req.StatValue != nil {
		value := float64(*req.StatValue)
		statValue = &value
	}

	return cards.CardSearchParams{
		Query:          req.NameQuery,
		NameExact:      req.NameExact,
		ManaCost:       req.ManaCostQuery,
		OracleText:     req.TextQuery,
		OracleTextNot:  normalizeCardSearchTextMode(req.TextMode) == "not",
		TypeFilters:    toCardTypeFilters(req.TypeFilters),
		TypePartial:    req.TypePartial,
		Colors:         req.ColorParams,
		ColorMode:      req.ColorMode,
		Stat:           req.Stat,
		StatOperator:   req.StatOperator,
		StatValue:      statValue,
		PriceOperator:  req.PriceOperator,
		PriceValue:     req.PriceValue,
		StatMin:        req.StatMin,
		StatMax:        req.StatMax,
		PriceUSDMin:    req.PriceMin,
		PriceUSDMax:    req.PriceMax,
		Rarity:         req.Rarity,
		SetQuery:       req.SetQuery,
		ArtistQuery:    req.ArtistQuery,
		Layout:         req.Layout,
		CommanderLegal: req.CommanderLegal,
		CommanderOnly:  req.CommanderOnly,
		IncludeTokens:  req.IncludeTokens,
		Limit:          limit,
	}
}

func (a *App) HandleCardList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, errMsg := parseCardSearchRequest(r.URL.Query())
	if errMsg != "" {
		http.Redirect(w, r, cardSearchEditPath(req), http.StatusSeeOther)
		return
	}

	user := CurrentUser(r)
	currentPath := cardSearchPath(req)

	savedDecks := []deckListItem(nil)
	if user != nil {
		userDecks, err := decks.ListDecksByUser(r.Context(), a.DB, user.ID)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		savedDecks = buildSavedDeckPickerItems(userDecks)
	}

	var results []cards.Card
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

	flash := readFlash(w, r)

	data := TemplateData{
		CurrentUser: user,
		Data: cardListPageData{
			Results:         buildSearchResults(results),
			HasSearched:     true,
			SavedDecks:      savedDecks,
			AppliedFilters:  buildCardSearchFilterChips(req),
			CurrentPath:     currentPath,
			ClearPath:       "/cards/search",
			EditFiltersPath: cardSearchEditPath(req),
		},
		Flash: flash,
		Error: errMsg,
	}

	a.Renderer.Render(w, "cards_list", data)
}

func (a *App) HandleCardSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, errMsg := parseCardSearchRequest(r.URL.Query())
	viewRequested := strings.TrimSpace(r.URL.Query().Get("view")) == "1"
	if (req.HasSearched || viewRequested) && errMsg == "" && strings.TrimSpace(r.URL.Query().Get("edit")) != "1" {
		http.Redirect(w, r, cardSearchPath(req), http.StatusSeeOther)
		return
	}

	user := CurrentUser(r)
	flash := readFlash(w, r)
	typeOptions, typeOptionsErr := loadCardSearchTypeSuggestions(r, a.DB)
	if typeOptionsErr != nil || len(typeOptions) == 0 {
		typeOptions = fallbackCardTypeSuggestions()
	}
	setSuggestions, _ := loadCardSearchSetSuggestions(r, a.DB)

	data := TemplateData{
		CurrentUser: user,
		Data: cardSearchPageData{
			NameQuery:             req.NameQuery,
			NameExact:             req.NameExact,
			ManaCostQuery:         req.ManaCostQuery,
			TextQuery:             req.TextQuery,
			TextMode:              req.TextMode,
			TypeOptions:           typeOptions,
			TypeOptionsJSON:       cardSearchStringSliceJSON(typeOptions),
			TypeFilters:           req.TypeFilters,
			TypeFiltersJSON:       cardSearchTypeFiltersJSON(req.TypeFilters),
			TypePartial:           req.TypePartial,
			SetSuggestions:        setSuggestions,
			LayoutOptions:         advancedCardLayoutOptions,
			SelectedLayout:        req.Layout,
			StatOptions:           advancedCardStatOptions,
			StatOperatorOptions:   advancedCardStatOperatorOptions,
			SelectedStat:          normalizeCardSearchStat(req.Stat),
			SelectedStatOperator:  normalizeCardSearchStatOperator(req.StatOperator),
			StatValue:             req.StatValueRaw,
			PriceOperatorOptions:  advancedCardStatOperatorOptions,
			SelectedPriceOperator: normalizeCardSearchStatOperator(req.PriceOperator),
			PriceValue:            req.PriceValueRaw,
			ColorsSelected:        selectedColorMap(req.ColorParams),
			ColorMode:             req.ColorMode,
			Rarity:                req.Rarity,
			SetQuery:              req.SetQuery,
			ArtistQuery:           req.ArtistQuery,
			CommanderOnly:         req.CommanderOnly,
			CommanderLegal:        req.CommanderLegal,
			IncludeTokens:         req.IncludeTokens,
			SearchActionPath:      "/cards/search",
			ClearPath:             "/cards/search",
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

	savedDecks := []deckListItem(nil)
	if user != nil {
		userDecks, err := decks.ListDecksByUser(r.Context(), a.DB, user.ID)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		savedDecks = buildSavedDeckPickerItems(userDecks)
	}

	page := buildCardDetailPageData(*card, printings)
	page.SavedDecks = savedDecks
	page.CurrentPath = r.URL.RequestURI()

	data := TemplateData{
		CurrentUser: user,
		Data:        page,
		Flash:       flash,
		WideLayout:  true,
	}

	a.Renderer.Render(w, "card_show", data)
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
