package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
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
	ScryfallID           string
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
	Layout               string
	ColorIdentity        string
	ManaValue            string
	IsCommanderCandidate bool
	IsSplitLayout        bool

	// FacesJSON is a JSON-encoded []cards.CardFace (from cards.Card.Faces).
	// It supports MDFC "turn over" behavior in result tiles and detail modals.
	FacesJSON string
}

type cardSearchTypeFilter struct {
	Value string `json:"value"`
	Mode  string `json:"mode"`
}

type cardSearchStatFilter struct {
	Stat     string `json:"stat"`
	Operator string `json:"operator"`
	Value    *int   `json:"-"`
	ValueRaw string `json:"value"`
}

type cardSearchPriceFilter struct {
	Operator string   `json:"operator"`
	Value    *float64 `json:"-"`
	ValueRaw string   `json:"value"`
}

type cardSearchFilterChip struct {
	Label      string
	RemovePath string
}

type cardSearchSelectOption struct {
	Value string
	Label string
}

type cardSearchQueryField struct {
	Name  string
	Value string
}

type cardResultsMode string

const (
	cardResultsModeStandard cardResultsMode = "standard"
	cardResultsModeAdvanced cardResultsMode = "advanced"
)

func normalizeCardResultsMode(raw string) cardResultsMode {
	if strings.EqualFold(strings.TrimSpace(raw), string(cardResultsModeAdvanced)) {
		return cardResultsModeAdvanced
	}
	return cardResultsModeStandard
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
	StatFilters           []cardSearchStatFilter
	PriceOperatorOptions  []cardSearchSelectOption
	SelectedPriceOperator string
	PriceValue            string
	PriceFilters          []cardSearchPriceFilter
	ColorsSelected        map[string]bool
	ColorMode             string
	ManaValueMin          string
	ManaValueMax          string
	Rarity                string
	RaritiesSelected      map[string]bool
	SetQuery              string
	ArtistQuery           string
	CommanderOnly         bool
	CommanderLegal        bool
	IncludeTokens         bool
	Sort                  string
	SortDirection         string
	SortDirectionExplicit bool
	CurrentSortLabel      string
	CurrentDirectionLabel string
	ShowCurrentSort       bool
	SearchActionPath      string
	ClearPath             string
	ResultsMode           cardResultsMode
	InlineResults         bool
	SubmitLabel           string
}

type cardListPageData struct {
	Results                []searchResult
	HasSearched            bool
	NameQuery              string
	SavedDecks             []deckListItem
	AppliedFilters         []cardSearchFilterChip
	CurrentPath            string
	ClearPath              string
	EditFiltersPath        string
	FilterForm             cardSearchPageData
	FiltersOpen            bool
	SortOptions            []cardSearchSelectOption
	DirectionOptions       []cardSearchSelectOption
	SelectedSort           string
	SelectedSortDirection  string
	CurrentSortLabel       string
	CurrentDirectionLabel  string
	SortFields             []cardSearchQueryField
	ShowOldestPrintingNote bool
	ResultsMode            cardResultsMode
	TotalResults           int
	Page                   int
	TotalPages             int
	ResultStart            int
	ResultEnd              int
	PreviousPath           string
	NextPath               string
}

type cardSearchRequest struct {
	NameQuery             string
	NameExact             bool
	ManaCostQuery         string
	TextQuery             string
	TextMode              string
	TypeFilters           []cardSearchTypeFilter
	TypePartial           bool
	Layout                string
	Stat                  string
	StatOperator          string
	StatValue             *int
	StatValueRaw          string
	StatFilters           []cardSearchStatFilter
	StatMin               *float64
	StatMax               *float64
	StatMinRaw            string
	StatMaxRaw            string
	ColorParams           []string
	ColorMode             string
	ManaValueMin          *float64
	ManaValueMax          *float64
	ManaValueMinRaw       string
	ManaValueMaxRaw       string
	PriceOperator         string
	PriceValue            *float64
	PriceValueRaw         string
	PriceFilters          []cardSearchPriceFilter
	PriceMin              *float64
	PriceMax              *float64
	PriceMinRaw           string
	PriceMaxRaw           string
	Rarity                string
	Rarities              []string
	SetQuery              string
	ArtistQuery           string
	CommanderOnly         bool
	CommanderLegal        bool
	IncludeTokens         bool
	Sort                  string
	SortDirection         string
	SortDirectionExplicit bool
	HasSearched           bool
	ResultsMode           cardResultsMode
	Page                  int
}

type cardDetailFaceData struct {
	Name              string
	ManaCost          string
	TypeLine          string
	OracleText        string
	FlavorText        string
	ImageURI          string
	ArtCropURI        string
	Artist            string
	Power             string
	Toughness         string
	HasPowerToughness bool
	Loyalty           string
	Colors            string
	ColorIdentity     string
}

type cardFormatLegalityData struct {
	Format      string
	Status      string
	StatusLabel string
}

type cardDetailPrintingData struct {
	ScryfallID        string
	OracleID          string
	Name              string
	ManaCost          string
	TypeLine          string
	OracleText        string
	FlavorText        string
	ImageURI          string
	ArtCropURI        string
	SetName           string
	SetCode           string
	CollectorNumber   string
	Rarity            string
	ReleasedAt        string
	Artist            string
	PriceUSD          string
	PriceSort         float64
	Lang              string
	ScryfallURI       string
	Layout            string
	Colors            string
	ColorIdentity     string
	ManaValue         string
	Power             string
	Toughness         string
	HasPowerToughness bool
	Loyalty           string
	EDHRecRank        string
	Legalities        []cardFormatLegalityData
	IsFavorited       bool
	Faces             []cardDetailFaceData
}

type cardDetailData struct {
	ScryfallID        string
	OracleID          string
	Name              string
	ManaCost          string
	TypeLine          string
	OracleText        string
	FlavorText        string
	ImageURI          string
	ArtCropURI        string
	PriceUSD          string
	Artist            string
	SetCode           string
	SetName           string
	CollectorNumber   string
	Rarity            string
	ReleasedAt        string
	Lang              string
	Layout            string
	Colors            string
	ColorIdentity     string
	ManaValue         string
	Power             string
	Toughness         string
	HasPowerToughness bool
	Loyalty           string
	EDHRecRank        string
	Legalities        []cardFormatLegalityData
	ScryfallURI       string
	Faces             []cardDetailFaceData
}

type cardDetailPageData struct {
	Card                        cardDetailData
	Printings                   []cardDetailPrintingData
	SavedDecks                  []deckListItem
	CurrentPath                 string
	FavoritePrintingCount       int
	OtherFavoritePrintingCount  int
	SelectedPrintingIsFavorited bool
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

var advancedCardSortOptions = []cardSearchSelectOption{
	{Value: "relevance", Label: "Relevance"},
	{Value: "alphabetical", Label: "Alphabetical"},
	{Value: "mana_value", Label: "Mana Value"},
	{Value: "newest_printing", Label: "Newest Printing"},
	{Value: "oldest_printing", Label: "Oldest Printing"},
	{Value: "rarity", Label: "Rarity"},
}

var advancedCardSortDirectionOptions = []cardSearchSelectOption{
	{Value: "asc", Label: "Ascending"},
	{Value: "desc", Label: "Descending"},
}

type cardResolveResponse struct {
	OracleID             string            `json:"oracle_id,omitempty"`
	ScryfallID           string            `json:"scryfall_id,omitempty"`
	Name                 string            `json:"name"`
	ManaCost             string            `json:"mana_cost,omitempty"`
	TypeLine             string            `json:"type_line,omitempty"`
	OracleText           string            `json:"oracle_text,omitempty"`
	CMC                  float64           `json:"cmc"`
	IsCommanderCandidate bool              `json:"is_commander_candidate"`
	Faces                []cards.CardFace  `json:"faces,omitempty"`
	ImageURIs            map[string]string `json:"image_uris,omitempty"`
	ArtCropURI           string            `json:"art_crop_uri,omitempty"`
	Prices               struct {
		USD string `json:"usd,omitempty"`
	} `json:"prices,omitempty"`
}

type cardVersionResponse struct {
	ScryfallID      string           `json:"scryfall_id,omitempty"`
	OracleID        string           `json:"oracle_id,omitempty"`
	Lang            string           `json:"lang,omitempty"`
	Name            string           `json:"name"`
	ManaCost        string           `json:"mana_cost,omitempty"`
	TypeLine        string           `json:"type_line,omitempty"`
	OracleText      string           `json:"oracle_text,omitempty"`
	ImageURI        string           `json:"image_uri,omitempty"`
	ArtCropURI      string           `json:"art_crop_uri,omitempty"`
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
			ScryfallID:           c.ID,
			DetailPath:           cardPrintingDetailPath(c.OracleID, c.ID),
			Name:                 c.Name,
			ManaCost:             c.ManaCost,
			TypeLine:             c.TypeLine,
			OracleText:           c.OracleText,
			ImageURI:             c.ImageURI,
			PriceUSD:             formatCardPrice(c.PriceUSD),
			Artist:               c.Artist,
			SetCode:              strings.ToUpper(strings.TrimSpace(c.SetCode)),
			SetName:              c.SetName,
			Rarity:               c.Rarity,
			ReleasedAt:           c.ReleasedAt,
			Layout:               strings.TrimSpace(c.Layout),
			ColorIdentity:        formatCardColorNames(c.ColorIdentity),
			ManaValue:            formatCardManaValue(c.CMC),
			IsCommanderCandidate: c.IsCommanderCandidate,
			IsSplitLayout:        isSplitCardLayout(c.Layout),
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

func cardPrintingDetailPath(oracleID string, scryfallID string) string {
	path := cardDetailPath(oracleID)
	scryfallID = strings.TrimSpace(scryfallID)
	if scryfallID == "" || path == "/cards" {
		return path
	}
	return path + "?printing=" + url.QueryEscape(scryfallID)
}

func ensureCardDetailPrinting(printings []cards.Card, selected cards.Card) []cards.Card {
	selectedID := strings.TrimSpace(selected.ID)
	if selectedID == "" {
		return printings
	}
	for index := range printings {
		if strings.EqualFold(strings.TrimSpace(printings[index].ID), selectedID) {
			printings[index] = selected
			return printings
		}
	}
	return append([]cards.Card{selected}, printings...)
}

func selectCardDetailPrinting(selected cards.Card, printings []cards.Card, requestedPrintingID string) cards.Card {
	if strings.TrimSpace(requestedPrintingID) == "" && len(printings) > 0 {
		return printings[0]
	}
	return selected
}

func isSplitCardLayout(layout string) bool {
	return strings.EqualFold(strings.TrimSpace(layout), "split")
}

func singleCardResultPath(results []cards.Card) string {
	if len(results) != 1 {
		return ""
	}
	return cardDetailPath(results[0].OracleID)
}

func cardSearchRedirectPath(
	mode cardResultsMode,
	exactNameMatch bool,
	matchingPrintings bool,
	results []cards.Card,
) string {
	if normalizeCardResultsMode(string(mode)) != cardResultsModeStandard || !exactNameMatch {
		return ""
	}
	if matchingPrintings && len(results) == 1 {
		return cardPrintingDetailPath(results[0].OracleID, results[0].ID)
	}
	return singleCardResultPath(results)
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

var cardFormatLabels = []struct {
	Key   string
	Label string
}{
	{Key: "standard", Label: "Standard"},
	{Key: "alchemy", Label: "Alchemy"},
	{Key: "pioneer", Label: "Pioneer"},
	{Key: "historic", Label: "Historic"},
	{Key: "modern", Label: "Modern"},
	{Key: "brawl", Label: "Brawl"},
	{Key: "legacy", Label: "Legacy"},
	{Key: "competitivebrawl", Label: "Competitive Brawl"},
	{Key: "vintage", Label: "Vintage"},
	{Key: "timeless", Label: "Timeless"},
	{Key: "commander", Label: "Commander"},
	{Key: "pauper", Label: "Pauper"},
	{Key: "oathbreaker", Label: "Oathbreaker"},
	{Key: "penny", Label: "Penny"},
}

func formatCardLegalityStatus(raw string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "legal":
		return "legal", "Legal"
	case "not_legal":
		return "not_legal", "Not legal"
	case "restricted":
		return "restricted", "Restricted"
	case "banned":
		return "banned", "Banned"
	default:
		return "unknown", "Unknown"
	}
}

func formatCardLegalities(legalities map[string]string) []cardFormatLegalityData {
	normalized := make(map[string]string, len(legalities))
	for key, status := range legalities {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		normalized[key] = status
	}

	out := make([]cardFormatLegalityData, 0, len(cardFormatLabels))
	appendLegality := func(label, rawStatus string) {
		status, statusLabel := formatCardLegalityStatus(rawStatus)
		out = append(out, cardFormatLegalityData{
			Format:      label,
			Status:      status,
			StatusLabel: statusLabel,
		})
	}

	for _, format := range cardFormatLabels {
		if status, ok := normalized[format.Key]; ok {
			appendLegality(format.Label, status)
		}
	}

	return out
}

func formatCardStatText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "N/A"
	}
	return value
}

func hasCardPowerToughness(power, toughness string) bool {
	return strings.TrimSpace(power) != "" || strings.TrimSpace(toughness) != ""
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

func buildCardDetailPageData(card cards.Card, printings []cards.Card) cardDetailPageData {
	if len(printings) == 0 {
		printings = []cards.Card{card}
	}

	cardImageURI := strings.TrimSpace(card.ImageURI)
	cardArtCropURI := strings.TrimSpace(card.ArtCropURI)
	cardArtist := strings.TrimSpace(card.Artist)
	cardFlavorText := firstNonEmptyCardFlavor(
		card.FlavorText,
		firstFaceFlavor(card.Faces),
	)

	faces := make([]cardDetailFaceData, 0, len(card.Faces))
	for _, face := range card.Faces {
		faceImageURI := strings.TrimSpace(face.ImageURI)
		faceArtCropURI := strings.TrimSpace(face.ArtCropURI)
		if cardImageURI == "" && faceImageURI != "" {
			cardImageURI = faceImageURI
		}
		if cardArtCropURI == "" && faceArtCropURI != "" {
			cardArtCropURI = faceArtCropURI
		}
		if cardArtist == "" && strings.TrimSpace(face.Artist) != "" {
			cardArtist = strings.TrimSpace(face.Artist)
		}

		faces = append(faces, cardDetailFaceData{
			Name:              cardMetaValue(face.Name, card.Name),
			ManaCost:          strings.TrimSpace(face.ManaCost),
			TypeLine:          strings.TrimSpace(face.TypeLine),
			OracleText:        cardMetaValue(face.OracleText, "No oracle text."),
			FlavorText:        strings.TrimSpace(face.FlavorText),
			ImageURI:          faceImageURI,
			ArtCropURI:        faceArtCropURI,
			Artist:            cardMetaValue(face.Artist, "Unknown"),
			Power:             formatCardStatText(face.Power),
			Toughness:         formatCardStatText(face.Toughness),
			HasPowerToughness: hasCardPowerToughness(face.Power, face.Toughness),
			Loyalty:           formatCardStatText(face.Loyalty),
			Colors:            formatCardColorNames(face.Colors),
			ColorIdentity:     formatCardColorNames(face.ColorID),
		})
	}

	printingItems := make([]cardDetailPrintingData, 0, len(printings))
	for _, printing := range printings {
		imageURI := strings.TrimSpace(printing.ImageURI)
		artCropURI := strings.TrimSpace(printing.ArtCropURI)
		if imageURI == "" && len(printing.Faces) > 0 {
			imageURI = strings.TrimSpace(printing.Faces[0].ImageURI)
		}
		if artCropURI == "" && len(printing.Faces) > 0 {
			artCropURI = strings.TrimSpace(printing.Faces[0].ArtCropURI)
		}
		if artCropURI == "" {
			artCropURI = imageURI
		}

		printingFaces := make([]cardDetailFaceData, 0, len(printing.Faces))
		for _, face := range printing.Faces {
			printingFaces = append(printingFaces, cardDetailFaceData{
				Name:              cardMetaValue(face.Name, printing.Name),
				ManaCost:          strings.TrimSpace(face.ManaCost),
				TypeLine:          cardMetaValue(face.TypeLine, printing.TypeLine),
				OracleText:        cardMetaValue(face.OracleText, "No oracle text."),
				FlavorText:        strings.TrimSpace(face.FlavorText),
				ImageURI:          strings.TrimSpace(face.ImageURI),
				ArtCropURI:        strings.TrimSpace(face.ArtCropURI),
				Artist:            cardMetaValue(face.Artist, printing.Artist),
				Power:             formatCardStatText(face.Power),
				Toughness:         formatCardStatText(face.Toughness),
				HasPowerToughness: hasCardPowerToughness(face.Power, face.Toughness),
				Loyalty:           formatCardStatText(face.Loyalty),
				Colors:            formatCardColorNames(face.Colors),
				ColorIdentity:     formatCardColorNames(face.ColorID),
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
			ScryfallID:        strings.TrimSpace(printing.ID),
			OracleID:          strings.TrimSpace(printing.OracleID),
			Name:              cardMetaValue(printing.Name, card.Name),
			ManaCost:          strings.TrimSpace(printing.ManaCost),
			TypeLine:          cardMetaValue(printing.TypeLine, card.TypeLine),
			OracleText:        cardMetaValue(printing.OracleText, "No oracle text."),
			FlavorText:        printingFlavorText,
			ImageURI:          imageURI,
			ArtCropURI:        artCropURI,
			SetName:           cardMetaValue(printing.SetName, "Unknown set"),
			SetCode:           strings.ToUpper(strings.TrimSpace(printing.SetCode)),
			CollectorNumber:   cardMetaValue(printing.CollectorNumber, "N/A"),
			Rarity:            cardMetaValue(printing.Rarity, "N/A"),
			ReleasedAt:        cardMetaValue(printing.ReleasedAt, "Unknown"),
			Artist:            cardMetaValue(printing.Artist, "Unknown"),
			PriceUSD:          formatCardPrice(printing.PriceUSD),
			PriceSort:         cardPriceSortValue(printing.PriceUSD),
			Lang:              cardMetaValue(strings.ToUpper(strings.TrimSpace(printing.Lang)), "EN"),
			ScryfallURI:       strings.TrimSpace(printing.ScryfallURI),
			Layout:            cardMetaValue(printing.Layout, "Unknown"),
			Colors:            formatCardColorNames(printing.Colors),
			ColorIdentity:     formatCardColorNames(printing.ColorIdentity),
			ManaValue:         formatCardManaValue(printing.CMC),
			Power:             formatCardStatText(printing.Power),
			Toughness:         formatCardStatText(printing.Toughness),
			HasPowerToughness: hasCardPowerToughness(printing.Power, printing.Toughness),
			Loyalty:           formatCardStatText(printing.Loyalty),
			EDHRecRank:        formatCardEDHRecRank(printing.EDHRecRank),
			Legalities:        formatCardLegalities(printing.Legalities),
			Faces:             printingFaces,
		})
	}

	return cardDetailPageData{
		Card: cardDetailData{
			ScryfallID:        strings.TrimSpace(card.ID),
			OracleID:          card.OracleID,
			Name:              cardMetaValue(card.Name, "Unknown card"),
			ManaCost:          strings.TrimSpace(card.ManaCost),
			TypeLine:          cardMetaValue(card.TypeLine, "Type unknown"),
			OracleText:        cardMetaValue(card.OracleText, "No oracle text."),
			FlavorText:        cardFlavorText,
			ImageURI:          cardImageURI,
			ArtCropURI:        cardArtCropURI,
			PriceUSD:          formatCardPrice(card.PriceUSD),
			Artist:            cardMetaValue(cardArtist, "Unknown"),
			SetCode:           strings.ToUpper(strings.TrimSpace(card.SetCode)),
			SetName:           cardMetaValue(card.SetName, "Unknown set"),
			CollectorNumber:   cardMetaValue(card.CollectorNumber, "N/A"),
			Rarity:            cardMetaValue(card.Rarity, "N/A"),
			ReleasedAt:        cardMetaValue(card.ReleasedAt, "Unknown"),
			Lang:              cardMetaValue(strings.ToUpper(strings.TrimSpace(card.Lang)), "EN"),
			Layout:            cardMetaValue(card.Layout, "Unknown"),
			Colors:            formatCardColorNames(card.Colors),
			ColorIdentity:     formatCardColorNames(card.ColorIdentity),
			ManaValue:         formatCardManaValue(card.CMC),
			Power:             formatCardStatText(card.Power),
			Toughness:         formatCardStatText(card.Toughness),
			HasPowerToughness: hasCardPowerToughness(card.Power, card.Toughness),
			Loyalty:           formatCardStatText(card.Loyalty),
			EDHRecRank:        formatCardEDHRecRank(card.EDHRecRank),
			Legalities:        formatCardLegalities(card.Legalities),
			ScryfallURI:       strings.TrimSpace(card.ScryfallURI),
			Faces:             faces,
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

func copyCardSearchStatFilters(filters []cardSearchStatFilter) []cardSearchStatFilter {
	if len(filters) == 0 {
		return nil
	}
	out := make([]cardSearchStatFilter, len(filters))
	copy(out, filters)
	return out
}

func copyCardSearchPriceFilters(filters []cardSearchPriceFilter) []cardSearchPriceFilter {
	if len(filters) == 0 {
		return nil
	}
	out := make([]cardSearchPriceFilter, len(filters))
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
	req.StatFilters = copyCardSearchStatFilters(req.StatFilters)
	req.PriceFilters = copyCardSearchPriceFilters(req.PriceFilters)
	req.ColorParams = copyStringSlice(req.ColorParams)
	req.Rarities = copyStringSlice(req.Rarities)
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

func normalizeCardSearchRarities(values []string) []string {
	allowed := []string{"common", "uncommon", "rare", "mythic"}
	seen := make(map[string]bool, len(allowed))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		for _, allowedValue := range allowed {
			if value == allowedValue {
				seen[value] = true
				break
			}
		}
	}

	out := make([]string, 0, len(allowed))
	for _, value := range allowed {
		if seen[value] {
			out = append(out, value)
		}
	}
	return out
}

func selectedRarityMap(values []string) map[string]bool {
	selected := map[string]bool{
		"common":   false,
		"uncommon": false,
		"rare":     false,
		"mythic":   false,
	}
	for _, value := range normalizeCardSearchRarities(values) {
		selected[value] = true
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

func normalizeCardSearchSort(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "alphabetical":
		return "alphabetical"
	case "mana_value":
		return "mana_value"
	case "newest_printing":
		return "newest_printing"
	case "oldest_printing":
		return "oldest_printing"
	case "rarity":
		return "rarity"
	default:
		return "relevance"
	}
}

func defaultCardSearchSortDirection(sortMode string, hasNameQuery bool) string {
	switch normalizeCardSearchSort(sortMode) {
	case "newest_printing":
		return "desc"
	case "relevance":
		if hasNameQuery {
			return "desc"
		}
		return "asc"
	default:
		return "asc"
	}
}

func normalizeCardSearchSortDirection(sortMode string, hasNameQuery bool, raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "desc":
		return "desc"
	case "asc":
		return "asc"
	default:
		return defaultCardSearchSortDirection(sortMode, hasNameQuery)
	}
}

func hasExplicitCardSearchSortDirection(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "asc", "desc":
		return true
	default:
		return false
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

type cardSearchNumericConstraint struct {
	Operator string
	Value    float64
}

func cardSearchNumericRangeImpossible(constraints []cardSearchNumericConstraint) bool {
	lower := math.Inf(-1)
	upper := math.Inf(1)
	lowerInclusive := true
	upperInclusive := true
	excluded := make(map[float64]struct{})

	updateLower := func(value float64, inclusive bool) {
		if value > lower {
			lower = value
			lowerInclusive = inclusive
		} else if value == lower {
			lowerInclusive = lowerInclusive && inclusive
		}
	}
	updateUpper := func(value float64, inclusive bool) {
		if value < upper {
			upper = value
			upperInclusive = inclusive
		} else if value == upper {
			upperInclusive = upperInclusive && inclusive
		}
	}

	for _, constraint := range constraints {
		switch normalizeCardSearchStatOperator(constraint.Operator) {
		case "lt":
			updateUpper(constraint.Value, false)
		case "gt":
			updateLower(constraint.Value, false)
		case "lte":
			updateUpper(constraint.Value, true)
		case "gte":
			updateLower(constraint.Value, true)
		case "neq":
			excluded[constraint.Value] = struct{}{}
		default:
			updateLower(constraint.Value, true)
			updateUpper(constraint.Value, true)
		}
	}

	if lower > upper {
		return true
	}
	if lower != upper {
		return false
	}
	if !lowerInclusive || !upperInclusive {
		return true
	}
	_, excludedOnlyValue := excluded[lower]
	return excludedOnlyValue
}

func cardSearchRepeatedRangeImpossible(req cardSearchRequest) bool {
	statsByName := make(map[string][]cardSearchNumericConstraint)
	for _, filter := range req.StatFilters {
		if filter.Value == nil {
			continue
		}
		stat := normalizeCardSearchStat(filter.Stat)
		statsByName[stat] = append(statsByName[stat], cardSearchNumericConstraint{
			Operator: filter.Operator,
			Value:    float64(*filter.Value),
		})
	}
	for _, constraints := range statsByName {
		if cardSearchNumericRangeImpossible(constraints) {
			return true
		}
	}

	priceConstraints := make([]cardSearchNumericConstraint, 0, len(req.PriceFilters))
	for _, filter := range req.PriceFilters {
		if filter.Value == nil {
			continue
		}
		priceConstraints = append(priceConstraints, cardSearchNumericConstraint{
			Operator: filter.Operator,
			Value:    *filter.Value,
		})
	}
	return cardSearchNumericRangeImpossible(priceConstraints)
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
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, trimmed, fmt.Errorf("must be finite")
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

func normalizeCardSearchPageNumber(raw string) int {
	page, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func parseCardSearchRequest(q url.Values) (cardSearchRequest, string) {
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
		StatOperator:   normalizeCardSearchStatOperator(q.Get("stat_op")),
		StatValueRaw:   strings.TrimSpace(q.Get("stat_value")),
		StatMinRaw:     statMinRaw,
		StatMaxRaw:     statMaxRaw,
		SetQuery:       strings.TrimSpace(q.Get("set")),
		ArtistQuery:    strings.TrimSpace(q.Get("artist")),
		CommanderOnly:  strings.TrimSpace(q.Get("commander")) == "1",
		CommanderLegal: strings.TrimSpace(q.Get("commander_legal")) == "1",
		IncludeTokens:  strings.TrimSpace(q.Get("include_tokens")) == "1",
		Sort:           normalizeCardSearchSort(q.Get("sort")),
		PriceOperator:  normalizeCardSearchStatOperator(priceOperatorRaw),
		PriceValueRaw:  priceValueRaw,
		PriceMinRaw:    q.Get("price_min"),
		PriceMaxRaw:    q.Get("price_max"),
		ResultsMode:    normalizeCardResultsMode(q.Get("search_mode")),
		Page:           normalizeCardSearchPageNumber(q.Get("page")),
	}
	req.SortDirection = normalizeCardSearchSortDirection(req.Sort, req.NameQuery != "", q.Get("sort_dir"))
	req.SortDirectionExplicit = hasExplicitCardSearchSortDirection(q.Get("sort_dir"))
	req.Rarities = normalizeCardSearchRarities(q["rarity"])
	if len(req.Rarities) > 0 {
		req.Rarity = req.Rarities[0]
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

	var errMsg string
	statNames := q["stat"]
	statOperators := q["stat_op"]
	for idx, rawValue := range q["stat_value"] {
		value, valueText, valueErr := parseOptionalIntFilter(rawValue)
		if valueText == "" {
			continue
		}
		statName := ""
		if idx < len(statNames) {
			statName = statNames[idx]
		}
		statOperator := ""
		if idx < len(statOperators) {
			statOperator = statOperators[idx]
		}
		req.StatFilters = append(req.StatFilters, cardSearchStatFilter{
			Stat:     normalizeCardSearchStat(statName),
			Operator: normalizeCardSearchStatOperator(statOperator),
			Value:    value,
			ValueRaw: valueText,
		})
		if errMsg == "" && valueErr != nil {
			errMsg = "Stat values must be valid whole numbers."
		}
	}
	if len(req.StatFilters) > 0 {
		first := req.StatFilters[0]
		req.Stat = first.Stat
		req.StatOperator = first.Operator
		req.StatValue = first.Value
		req.StatValueRaw = first.ValueRaw
		req.StatMinRaw = ""
		req.StatMaxRaw = ""
	} else {
		req.StatValue = nil
		req.StatValueRaw = ""
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

	priceOperators := q["price_op"]
	for idx, rawValue := range q["price_value"] {
		value, valueText, valueErr := parseOptionalFloatFilter(rawValue)
		if valueText == "" {
			continue
		}
		priceOperator := ""
		if idx < len(priceOperators) {
			priceOperator = priceOperators[idx]
		}
		req.PriceFilters = append(req.PriceFilters, cardSearchPriceFilter{
			Operator: normalizeCardSearchStatOperator(priceOperator),
			Value:    value,
			ValueRaw: valueText,
		})
		if errMsg == "" && valueErr != nil {
			errMsg = "Price values must be valid numbers."
		}
	}
	if len(req.PriceFilters) > 0 {
		first := req.PriceFilters[0]
		req.PriceOperator = first.Operator
		req.PriceValue = first.Value
		req.PriceValueRaw = first.ValueRaw
		req.PriceMinRaw = ""
		req.PriceMaxRaw = ""
	} else {
		req.PriceValue = nil
		req.PriceValueRaw = ""
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
	if errMsg == "" && cardSearchRepeatedRangeImpossible(req) {
		errMsg = "Impossible range."
	}

	hasFilters := req.ManaCostQuery != "" ||
		req.TextQuery != "" ||
		len(req.TypeFilters) > 0 ||
		req.Layout != "" ||
		len(req.StatFilters) > 0 ||
		strings.TrimSpace(req.StatMinRaw) != "" ||
		strings.TrimSpace(req.StatMaxRaw) != "" ||
		len(req.ColorParams) > 0 ||
		len(req.PriceFilters) > 0 ||
		strings.TrimSpace(req.PriceMinRaw) != "" ||
		strings.TrimSpace(req.PriceMaxRaw) != "" ||
		len(req.Rarities) > 0 ||
		req.SetQuery != "" ||
		req.ArtistQuery != "" ||
		req.CommanderOnly ||
		req.CommanderLegal ||
		req.IncludeTokens
	req.HasSearched = req.NameQuery != "" || hasFilters

	return req, errMsg
}

func cardSearchQueryValues(req cardSearchRequest) url.Values {
	values := url.Values{}

	if normalizeCardResultsMode(string(req.ResultsMode)) == cardResultsModeAdvanced {
		values.Set("search_mode", string(cardResultsModeAdvanced))
	}
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
	if len(req.StatFilters) > 0 {
		for _, filter := range req.StatFilters {
			value := strings.TrimSpace(filter.ValueRaw)
			if value == "" {
				continue
			}
			values.Add("stat", normalizeCardSearchStat(filter.Stat))
			values.Add("stat_op", normalizeCardSearchStatOperator(filter.Operator))
			values.Add("stat_value", value)
		}
	} else {
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
	}
	if len(req.PriceFilters) > 0 {
		for _, filter := range req.PriceFilters {
			value := strings.TrimSpace(filter.ValueRaw)
			if value == "" {
				continue
			}
			values.Add("price_op", normalizeCardSearchStatOperator(filter.Operator))
			values.Add("price_value", value)
		}
	} else {
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
	}
	rarities := normalizeCardSearchRarities(req.Rarities)
	if len(rarities) == 0 {
		rarities = normalizeCardSearchRarities([]string{req.Rarity})
	}
	for _, rarity := range rarities {
		values.Add("rarity", rarity)
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
	if req.Page > 1 {
		values.Set("page", strconv.Itoa(req.Page))
	}
	values.Set("sort", normalizeCardSearchSort(req.Sort))
	if req.SortDirectionExplicit {
		values.Set("sort_dir", normalizeCardSearchSortDirection(req.Sort, req.NameQuery != "", req.SortDirection))
	}

	return values
}

func cardSearchQueryFields(req cardSearchRequest, excluded ...string) []cardSearchQueryField {
	values := cardSearchQueryValues(req)
	exclude := make(map[string]struct{}, len(excluded))
	for _, key := range excluded {
		exclude[key] = struct{}{}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		if _, skip := exclude[key]; skip {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fields := make([]cardSearchQueryField, 0, len(keys))
	for _, key := range keys {
		for _, value := range values[key] {
			fields = append(fields, cardSearchQueryField{
				Name:  key,
				Value: value,
			})
		}
	}
	return fields
}

func cardSearchPath(req cardSearchRequest) string {
	values := cardSearchQueryValues(req)
	if encoded := values.Encode(); encoded != "" {
		return "/cards?" + encoded
	}
	return "/cards"
}

func cardSearchQueryNeedsCanonicalRedirect(raw url.Values, canonical url.Values) bool {
	for key, rawValues := range raw {
		canonicalValues, ok := canonical[key]
		if !ok || len(rawValues) != len(canonicalValues) {
			return true
		}
		for index := range rawValues {
			if rawValues[index] != canonicalValues[index] {
				return true
			}
		}
	}
	return false
}

func cardSearchPagePath(req cardSearchRequest, page int) string {
	req.Page = page
	return cardSearchPath(req)
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
	req.Page = 1
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

	if len(req.StatFilters) > 0 {
		for idx, filter := range req.StatFilters {
			value := strings.TrimSpace(filter.ValueRaw)
			if value == "" {
				continue
			}
			next := cloneCardSearchRequest(req)
			next.StatFilters = append(copyCardSearchStatFilters(req.StatFilters[:idx]), req.StatFilters[idx+1:]...)
			if len(next.StatFilters) == 0 {
				next.StatValue = nil
				next.StatValueRaw = ""
				next.StatOperator = "eq"
			}
			chips = append(chips, cardSearchFilterChip{
				Label: formatCardSearchStatLabel(filter.Stat) + " " +
					formatCardSearchStatOperatorLabel(filter.Operator) + " " + value,
				RemovePath: cardSearchPath(next),
			})
		}
	} else if value := strings.TrimSpace(req.StatValueRaw); value != "" {
		next := cloneCardSearchRequest(req)
		next.StatValue = nil
		next.StatValueRaw = ""
		next.StatOperator = "eq"
		chips = append(chips, cardSearchFilterChip{
			Label:      formatCardSearchStatLabel(req.Stat) + " " + formatCardSearchStatOperatorLabel(req.StatOperator) + " " + value,
			RemovePath: cardSearchPath(next),
		})
	}

	statLabel := formatCardSearchStatLabel(req.Stat)
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

	if len(req.PriceFilters) > 0 {
		for idx, filter := range req.PriceFilters {
			value := strings.TrimSpace(filter.ValueRaw)
			if value == "" {
				continue
			}
			next := cloneCardSearchRequest(req)
			next.PriceFilters = append(copyCardSearchPriceFilters(req.PriceFilters[:idx]), req.PriceFilters[idx+1:]...)
			if len(next.PriceFilters) == 0 {
				next.PriceValue = nil
				next.PriceValueRaw = ""
				next.PriceOperator = "eq"
			}
			chips = append(chips, cardSearchFilterChip{
				Label:      "Price " + formatCardSearchPriceOperatorLabel(filter.Operator) + " $" + value,
				RemovePath: cardSearchPath(next),
			})
		}
	} else {
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
	}

	rarities := normalizeCardSearchRarities(req.Rarities)
	if len(rarities) == 0 {
		rarities = normalizeCardSearchRarities([]string{req.Rarity})
	}
	for idx, rarity := range rarities {
		next := cloneCardSearchRequest(req)
		next.Rarities = append(copyStringSlice(rarities[:idx]), rarities[idx+1:]...)
		next.Rarity = ""
		if len(next.Rarities) > 0 {
			next.Rarity = next.Rarities[0]
		}
		chips = append(chips, cardSearchFilterChip{
			Label:      "Rarity: " + formatCardSearchRarityLabel(rarity),
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

func formatCardSearchSortLabel(raw string) string {
	value := normalizeCardSearchSort(raw)
	for _, option := range advancedCardSortOptions {
		if option.Value == value {
			return option.Label
		}
	}
	return strings.TrimSpace(raw)
}

func formatCardSearchSortDirectionLabel(rawSort string, hasNameQuery bool, rawDirection string) string {
	value := normalizeCardSearchSortDirection(rawSort, hasNameQuery, rawDirection)
	for _, option := range advancedCardSortDirectionOptions {
		if option.Value == value {
			return option.Label
		}
	}
	return strings.TrimSpace(rawDirection)
}

func (req cardSearchRequest) searchParams(limit int) cards.CardSearchParams {
	var statValue *float64
	if req.StatValue != nil {
		value := float64(*req.StatValue)
		statValue = &value
	}
	statFilters := make([]cards.CardStatFilter, 0, len(req.StatFilters))
	for _, filter := range req.StatFilters {
		if filter.Value == nil {
			continue
		}
		statFilters = append(statFilters, cards.CardStatFilter{
			Stat:     filter.Stat,
			Operator: filter.Operator,
			Value:    float64(*filter.Value),
		})
	}
	priceFilters := make([]cards.CardPriceFilter, 0, len(req.PriceFilters))
	for _, filter := range req.PriceFilters {
		if filter.Value == nil {
			continue
		}
		priceFilters = append(priceFilters, cards.CardPriceFilter{
			Operator: filter.Operator,
			Value:    *filter.Value,
		})
	}
	rarities := normalizeCardSearchRarities(req.Rarities)
	if len(rarities) == 0 {
		rarities = normalizeCardSearchRarities([]string{req.Rarity})
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
		StatFilters:    statFilters,
		PriceOperator:  req.PriceOperator,
		PriceValue:     req.PriceValue,
		PriceFilters:   priceFilters,
		StatMin:        req.StatMin,
		StatMax:        req.StatMax,
		PriceUSDMin:    req.PriceMin,
		PriceUSDMax:    req.PriceMax,
		Rarity:         req.Rarity,
		Rarities:       rarities,
		SetQuery:       req.SetQuery,
		ArtistQuery:    req.ArtistQuery,
		Layout:         req.Layout,
		CommanderLegal: req.CommanderLegal,
		CommanderOnly:  req.CommanderOnly,
		IncludeTokens:  req.IncludeTokens,
		Sort:           req.Sort,
		SortDirection:  req.SortDirection,
		Limit:          limit,
	}
}

func cardSearchStatRowsForPage(req cardSearchRequest) []cardSearchStatFilter {
	rows := copyCardSearchStatFilters(req.StatFilters)
	if len(rows) == 0 && strings.TrimSpace(req.StatValueRaw) != "" {
		rows = append(rows, cardSearchStatFilter{
			Stat:     normalizeCardSearchStat(req.Stat),
			Operator: normalizeCardSearchStatOperator(req.StatOperator),
			Value:    req.StatValue,
			ValueRaw: req.StatValueRaw,
		})
	}
	rows = append(rows, cardSearchStatFilter{
		Stat:     "mana_value",
		Operator: "eq",
	})
	return rows
}

func cardSearchPriceRowsForPage(req cardSearchRequest) []cardSearchPriceFilter {
	rows := copyCardSearchPriceFilters(req.PriceFilters)
	if len(rows) == 0 && strings.TrimSpace(req.PriceValueRaw) != "" {
		rows = append(rows, cardSearchPriceFilter{
			Operator: normalizeCardSearchStatOperator(req.PriceOperator),
			Value:    req.PriceValue,
			ValueRaw: req.PriceValueRaw,
		})
	}
	rows = append(rows, cardSearchPriceFilter{Operator: "eq"})
	return rows
}

func clearedAdvancedCardSearchPath(req cardSearchRequest) string {
	cleared := cardSearchRequest{
		Sort:                  normalizeCardSearchSort(req.Sort),
		SortDirection:         normalizeCardSearchSortDirection(req.Sort, false, req.SortDirection),
		SortDirectionExplicit: req.SortDirectionExplicit,
		ResultsMode:           cardResultsModeAdvanced,
	}
	return cardSearchPath(cleared)
}

func buildCardSearchPageData(
	req cardSearchRequest,
	typeOptions []string,
	setSuggestions []string,
	inlineResults bool,
	showCurrentSort bool,
) cardSearchPageData {
	searchActionPath := "/cards/search"
	clearPath := "/cards/search"
	submitLabel := "View Results"
	if inlineResults {
		searchActionPath = "/cards"
		clearPath = clearedAdvancedCardSearchPath(req)
		submitLabel = "Apply filters"
	}

	return cardSearchPageData{
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
		StatFilters:           cardSearchStatRowsForPage(req),
		PriceOperatorOptions:  advancedCardStatOperatorOptions,
		SelectedPriceOperator: normalizeCardSearchStatOperator(req.PriceOperator),
		PriceValue:            req.PriceValueRaw,
		PriceFilters:          cardSearchPriceRowsForPage(req),
		ColorsSelected:        selectedColorMap(req.ColorParams),
		ColorMode:             req.ColorMode,
		Rarity:                req.Rarity,
		RaritiesSelected:      selectedRarityMap(req.Rarities),
		SetQuery:              req.SetQuery,
		ArtistQuery:           req.ArtistQuery,
		CommanderOnly:         req.CommanderOnly,
		CommanderLegal:        req.CommanderLegal,
		IncludeTokens:         req.IncludeTokens,
		Sort:                  normalizeCardSearchSort(req.Sort),
		SortDirection:         normalizeCardSearchSortDirection(req.Sort, req.NameQuery != "", req.SortDirection),
		SortDirectionExplicit: req.SortDirectionExplicit,
		CurrentSortLabel:      formatCardSearchSortLabel(req.Sort),
		CurrentDirectionLabel: formatCardSearchSortDirectionLabel(req.Sort, req.NameQuery != "", req.SortDirection),
		ShowCurrentSort:       showCurrentSort,
		SearchActionPath:      searchActionPath,
		ClearPath:             clearPath,
		ResultsMode:           cardResultsModeAdvanced,
		InlineResults:         inlineResults,
		SubmitLabel:           submitLabel,
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
	if canonicalValues := cardSearchQueryValues(req); cardSearchQueryNeedsCanonicalRedirect(r.URL.Query(), canonicalValues) {
		http.Redirect(w, r, cardSearchPath(req), http.StatusSeeOther)
		return
	}

	user := CurrentUser(r)

	typeOptions, typeOptionsErr := loadCardSearchTypeSuggestions(r, a.DB)
	if typeOptionsErr != nil || len(typeOptions) == 0 {
		typeOptions = fallbackCardTypeSuggestions()
	}
	setSuggestions, _ := loadCardSearchSetSuggestions(r, a.DB)

	var results []cards.Card
	searchParams := req.searchParams(48)
	searchParams.Page = req.Page
	outcome, err := cards.SearchCardsWithOutcome(r.Context(), a.DB, searchParams)
	if err != nil {
		errMsg = "We couldn't search for cards right now. Please try again."
	} else {
		if singlePath := cardSearchRedirectPath(
			req.ResultsMode,
			outcome.ExactNameMatch,
			outcome.MatchingPrintings,
			outcome.Cards,
		); singlePath != "" {
			http.Redirect(w, r, singlePath, http.StatusSeeOther)
			return
		}
		results = outcome.Cards
	}

	page := outcome.Page
	if outcome.TotalPages == 0 {
		page = 1
	}
	if page < 1 {
		page = req.Page
	}
	if page < 1 {
		page = 1
	}
	req.Page = page
	currentPath := cardSearchPath(req)

	resultStart := 0
	resultEnd := 0
	if len(results) > 0 {
		resultStart = ((page - 1) * outcome.PageSize) + 1
		resultEnd = resultStart + len(results) - 1
	}
	previousPath := ""
	if outcome.TotalPages > 0 && page > 1 {
		previousPath = cardSearchPagePath(req, page-1)
	}
	nextPath := ""
	if outcome.TotalPages > 0 && page < outcome.TotalPages {
		nextPath = cardSearchPagePath(req, page+1)
	}

	flash := readFlash(w, r)
	filterForm := buildCardSearchPageData(req, typeOptions, setSuggestions, true, false)
	meta := defaultPageMeta("cards_list")
	if query := truncateShareText(req.NameQuery, 48); query != "" {
		meta.Title = query + " — Card Results"
		meta.Description = "Browse Magic cards matching " + query + "."
	}

	data := TemplateData{
		CurrentUser: user,
		Meta:        meta,
		Data: cardListPageData{
			Results:                buildSearchResults(results),
			HasSearched:            req.HasSearched,
			NameQuery:              req.NameQuery,
			AppliedFilters:         buildCardSearchFilterChips(req),
			CurrentPath:            currentPath,
			ClearPath:              filterForm.ClearPath,
			EditFiltersPath:        cardSearchEditPath(req),
			FilterForm:             filterForm,
			FiltersOpen:            len(results) == 0 || errMsg != "",
			SortOptions:            advancedCardSortOptions,
			DirectionOptions:       advancedCardSortDirectionOptions,
			SelectedSort:           normalizeCardSearchSort(req.Sort),
			SelectedSortDirection:  normalizeCardSearchSortDirection(req.Sort, req.NameQuery != "", req.SortDirection),
			CurrentSortLabel:       formatCardSearchSortLabel(req.Sort),
			CurrentDirectionLabel:  formatCardSearchSortDirectionLabel(req.Sort, req.NameQuery != "", req.SortDirection),
			SortFields:             cardSearchQueryFields(req, "sort", "sort_dir", "page"),
			ShowOldestPrintingNote: normalizeCardSearchSort(req.Sort) == "oldest_printing",
			ResultsMode:            req.ResultsMode,
			TotalResults:           outcome.Total,
			Page:                   page,
			TotalPages:             outcome.TotalPages,
			ResultStart:            resultStart,
			ResultEnd:              resultEnd,
			PreviousPath:           previousPath,
			NextPath:               nextPath,
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
	req.ResultsMode = cardResultsModeAdvanced
	editRequested := strings.TrimSpace(r.URL.Query().Get("edit")) == "1"
	viewRequested := strings.TrimSpace(r.URL.Query().Get("view")) == "1"
	if (req.HasSearched || viewRequested) && errMsg == "" && !editRequested {
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
		Data:        buildCardSearchPageData(req, typeOptions, setSuggestions, false, editRequested),
		Flash:       flash,
		Error:       errMsg,
	}

	a.Renderer.Render(w, "cards_search", data)
}

func (a *App) HandleRandomCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	card, err := cards.RandomCard(r.Context(), a.DB)
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			a.RenderNotFound(w, r)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	http.Redirect(w, r, "/cards/view/"+url.PathEscape(card.OracleID), http.StatusSeeOther)
}

func (a *App) HandleCardShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := CurrentUser(r)
	if user != nil {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Add("Vary", "Cookie")
	}
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

	requestedPrintingID := strings.TrimSpace(r.URL.Query().Get("printing"))
	if requestedPrintingID != "" {
		if _, err := uuid.Parse(requestedPrintingID); err != nil {
			a.RenderNotFound(w, r)
			return
		}
	}

	selectedCard := *card
	if requestedPrintingID != "" {
		selectedPrinting, err := cards.GetCardPrintingByID(r.Context(), a.DB, oracleID, requestedPrintingID)
		if err != nil {
			if errors.Is(err, cards.ErrCardNotFound) {
				a.RenderNotFound(w, r)
				return
			}
			a.RenderServerError(w, r, err)
			return
		}
		selectedCard = *selectedPrinting
	}

	printings, err := cards.ListCardVersionsByOracleID(r.Context(), a.DB, oracleID, 500)
	if err != nil {
		if !errors.Is(err, cards.ErrCardNotFound) {
			a.RenderServerError(w, r, err)
			return
		}
		printings = []cards.Card{selectedCard}
	} else {
		// A normal search should always land on the newest real printing. The
		// oracle-card projection can retain an older (or, after a partial sync,
		// stale) default print id; using the first version returned by the
		// newest-first printing query also guarantees that favorite/profile-art
		// actions reference a row that exists in card_prints.
		selectedCard = selectCardDetailPrinting(selectedCard, printings, requestedPrintingID)
		printings = ensureCardDetailPrinting(printings, selectedCard)
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

	page := buildCardDetailPageData(selectedCard, printings)
	page.SavedDecks = savedDecks
	page.CurrentPath = r.URL.RequestURI()
	if user != nil {
		favoriteIDs, err := favoritePrintingIDsForOracle(r.Context(), a.DB, user.ID, oracleID)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		applyPrintingFavoriteStatus(&page, favoriteIDs)
	}

	data := TemplateData{
		CurrentUser: user,
		Data:        page,
		Meta:        buildCardShareMeta(a.PublicBaseURL, r, selectedCard),
		Flash:       flash,
		WideLayout:  true,
	}

	a.Renderer.Render(w, "card_show", data)
}

func buildCardShareMeta(publicBaseURL string, r *http.Request, card cards.Card) *PageMeta {
	name := strings.TrimSpace(card.Name)
	if name == "" {
		name = "Card Detail"
	}
	canonicalPath := cardDetailPath(card.OracleID)
	if r != nil && strings.TrimSpace(r.URL.Query().Get("printing")) != "" {
		canonicalPath = cardPrintingDetailPath(card.OracleID, card.ID)
	}

	description := compactShareText(strings.TrimSpace(card.TypeLine))
	oracle := compactShareText(strings.TrimSpace(card.OracleText))
	if description != "" && oracle != "" {
		description += " - " + oracle
	} else if oracle != "" {
		description = oracle
	}
	description = truncateShareText(description, 180)
	if description == "" {
		description = "View card details, printings, and deck tools on ManaTomb."
	}
	imageURL := strings.TrimSpace(card.ArtCropURI)
	imageAlt := name + " artwork"
	if imageURL == "" {
		imageURL = strings.TrimSpace(card.ImageURI)
		imageAlt = name + " card image"
	}

	return &PageMeta{
		Title:        name,
		Description:  description,
		CanonicalURL: absoluteSiteURL(publicBaseURL, canonicalPath),
		ImageURL:     imageURL,
		ImageAlt:     imageAlt,
		Type:         "article",
	}
}

func compactShareText(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func truncateShareText(raw string, limit int) string {
	raw = compactShareText(raw)
	runes := []rune(raw)
	if limit <= 0 || len(runes) <= limit {
		return raw
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-3])) + "..."
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
		ScryfallID:           card.ID,
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
	if card.ImageURI != "" || card.ArtCropURI != "" {
		resp.ImageURIs = map[string]string{}
		if card.ImageURI != "" {
			resp.ImageURIs["normal"] = card.ImageURI
		}
		if card.ArtCropURI != "" {
			resp.ImageURIs["art_crop"] = card.ArtCropURI
		}
	}
	resp.ArtCropURI = card.ArtCropURI
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
			OracleID:        c.OracleID,
			Lang:            c.Lang,
			Name:            c.Name,
			ManaCost:        c.ManaCost,
			TypeLine:        c.TypeLine,
			OracleText:      c.OracleText,
			ImageURI:        c.ImageURI,
			ArtCropURI:      c.ArtCropURI,
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
