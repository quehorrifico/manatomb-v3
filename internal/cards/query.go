package cards

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lib/pq"
)

type CardSearchParams struct {
	Query          string
	NameExact      bool
	ManaCost       string
	OracleText     string
	OracleTextNot  bool
	TypeFilter     string
	TypeFilters    []CardTypeFilter
	TypePartial    bool
	Colors         []string
	ColorMode      string
	ManaValueMin   *float64
	ManaValueMax   *float64
	Stat           string
	StatOperator   string
	StatValue      *float64
	StatFilters    []CardStatFilter
	StatMin        *float64
	StatMax        *float64
	PriceOperator  string
	PriceValue     *float64
	PriceFilters   []CardPriceFilter
	PriceUSDMin    *float64
	PriceUSDMax    *float64
	Rarity         string
	Rarities       []string
	SetQuery       string
	ArtistQuery    string
	Layout         string
	CommanderLegal bool
	CommanderOnly  bool
	IncludeTokens  bool
	Sort           string
	SortDirection  string
	Limit          int
	Page           int
	AllMatches     bool
}

// CardSearchOutcome carries both the matching cards and metadata about how the
// name query resolved. Pagination metadata is included here so callers can
// preserve one search API as result paging is introduced.
type CardSearchOutcome struct {
	Cards             []Card
	ExactNameMatch    bool
	Total             int
	Page              int
	PageSize          int
	TotalPages        int
	MatchingPrintings bool
}

type NameResolution struct {
	Card       DBCard
	Exact      bool
	Similarity float64
}

type CardTypeFilter struct {
	Value   string
	Negated bool
}

type CardStatFilter struct {
	Stat     string
	Operator string
	Value    float64
}

type CardPriceFilter struct {
	Operator string
	Value    float64
}

func normalizeColorFilters(colors []string) []string {
	if len(colors) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(colors))
	for _, c := range colors {
		upper := strings.ToUpper(strings.TrimSpace(c))
		switch upper {
		case "W", "U", "B", "R", "G", "C":
		default:
			continue
		}
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		out = append(out, upper)
	}
	return out
}

func normalizeColorMatchMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "exact":
		return "exact"
	case "at_most":
		return "at_most"
	default:
		return "includes"
	}
}

func normalizeCardTypeFilters(filters []CardTypeFilter) []CardTypeFilter {
	if len(filters) == 0 {
		return nil
	}

	seen := map[string]int{}
	out := make([]CardTypeFilter, 0, len(filters))
	for _, filter := range filters {
		value := strings.TrimSpace(filter.Value)
		if value == "" {
			continue
		}

		key := strings.ToLower(value)
		if idx, ok := seen[key]; ok {
			out[idx].Value = value
			out[idx].Negated = filter.Negated
			continue
		}

		seen[key] = len(out)
		out = append(out, CardTypeFilter{
			Value:   value,
			Negated: filter.Negated,
		})
	}
	return out
}

func splitColorFilters(colors []string) ([]string, bool) {
	normalized := normalizeColorFilters(colors)
	if len(normalized) == 0 {
		return nil, false
	}

	out := make([]string, 0, len(normalized))
	colorless := false
	for _, color := range normalized {
		if color == "C" {
			colorless = true
			continue
		}
		out = append(out, color)
	}
	if colorless && len(out) > 0 {
		colorless = false
	}
	return out, colorless
}

func normalizeRarityFilter(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "common":
		return "common"
	case "uncommon":
		return "uncommon"
	case "rare":
		return "rare"
	case "mythic":
		return "mythic"
	default:
		return ""
	}
}

func normalizeRarityFilters(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		normalized := normalizeRarityFilter(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeLayoutFilter(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "normal":
		return "normal"
	case "split":
		return "split"
	case "flip":
		return "flip"
	case "transform":
		return "transform"
	case "modal_dfc":
		return "modal_dfc"
	case "meld":
		return "meld"
	case "leveler":
		return "leveler"
	case "class":
		return "class"
	case "adventure":
		return "adventure"
	case "saga":
		return "saga"
	case "token":
		return "token"
	case "double_faced_token":
		return "double_faced_token"
	default:
		return ""
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

func normalizeCardStatFilter(raw string) string {
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

func normalizeCardStatOperator(raw string) string {
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

func cardStatOperatorSQL(raw string) string {
	switch normalizeCardStatOperator(raw) {
	case "lt":
		return "<"
	case "gt":
		return ">"
	case "lte":
		return "<="
	case "gte":
		return ">="
	case "neq":
		return "<>"
	default:
		return "="
	}
}

func cardStatColumn(stat string) string {
	switch normalizeCardStatFilter(stat) {
	case "power":
		return "oc.power_value"
	case "toughness":
		return "oc.toughness_value"
	case "loyalty":
		return "oc.loyalty_value"
	default:
		return "oc.cmc"
	}
}

func exactTypePattern(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return `(^|[^[:alnum:]])` + regexp.QuoteMeta(value) + `([^[:alnum:]]|$)`
}

func decodeCardFacesJSON(raw string) []CardFace {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var faces []CardFace
	if err := json.Unmarshal([]byte(raw), &faces); err != nil {
		return nil
	}
	return faces
}

func decodeLegalitiesJSON(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}
	}
	var legalities map[string]string
	if err := json.Unmarshal([]byte(raw), &legalities); err != nil {
		return map[string]string{}
	}
	return normalizeLegalities(legalities)
}

func scanCanonicalCard(
	printingID, oracleID, name, manaCost, typeLine, oracleText, flavorText, imageURI, artCropURI string,
	colors, colorIdentity []string,
	cmc float64,
	power, toughness, loyalty string,
	layout string,
	legalAnywhere bool,
	commanderLegal bool,
	isCommanderCandidate bool,
	priceUSD, artist string,
	edhrecRank int,
	scryfallURI, setCode, setName, collectorNumber, rarity, releasedAt, lang, facesJSON, legalitiesJSON string,
) Card {
	return Card{
		ID:                   strings.TrimSpace(printingID),
		OracleID:             strings.TrimSpace(oracleID),
		Name:                 strings.TrimSpace(name),
		ManaCost:             strings.TrimSpace(manaCost),
		TypeLine:             strings.TrimSpace(typeLine),
		OracleText:           strings.TrimSpace(oracleText),
		FlavorText:           strings.TrimSpace(flavorText),
		ImageURI:             strings.TrimSpace(imageURI),
		ArtCropURI:           strings.TrimSpace(artCropURI),
		Colors:               colors,
		ColorIdentity:        colorIdentity,
		CMC:                  cmc,
		Power:                strings.TrimSpace(power),
		Toughness:            strings.TrimSpace(toughness),
		Loyalty:              strings.TrimSpace(loyalty),
		Layout:               strings.TrimSpace(layout),
		LegalAnywhere:        legalAnywhere,
		CommanderLegal:       commanderLegal,
		Legalities:           decodeLegalitiesJSON(legalitiesJSON),
		IsCommanderCandidate: isCommanderCandidate,
		PriceUSD:             strings.TrimSpace(priceUSD),
		Artist:               strings.TrimSpace(artist),
		EDHRecRank:           edhrecRank,
		ScryfallURI:          strings.TrimSpace(scryfallURI),
		SetCode:              strings.TrimSpace(setCode),
		SetName:              strings.TrimSpace(setName),
		CollectorNumber:      strings.TrimSpace(collectorNumber),
		Rarity:               strings.TrimSpace(rarity),
		ReleasedAt:           strings.TrimSpace(releasedAt),
		Lang:                 strings.TrimSpace(lang),
		Faces:                decodeCardFacesJSON(facesJSON),
	}
}

func canonicalCardSelectSQL() string {
	return `
		SELECT
			COALESCE(oc.default_print_id::text, '') AS id,
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, '') AS mana_cost,
			COALESCE(oc.type_line, '') AS type_line,
			COALESCE(oc.oracle_text, '') AS oracle_text,
			COALESCE(cp.flavor_text, '') AS flavor_text,
			COALESCE(oc.default_image_uri, '') AS image_uri,
			COALESCE(cp.image_uris->>'art_crop', oc.default_image_uri, '') AS art_crop_uri,
			COALESCE(oc.colors, ARRAY[]::text[]) AS colors,
			COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
			COALESCE(oc.cmc, 0) AS cmc,
			COALESCE(oc.power_text, '') AS power_text,
			COALESCE(oc.toughness_text, '') AS toughness_text,
			COALESCE(oc.loyalty_text, '') AS loyalty_text,
			COALESCE(oc.layout, '') AS layout,
			COALESCE(oc.legal_anywhere, true) AS legal_anywhere,
			COALESCE(oc.commander_legal, false) AS commander_legal,
			COALESCE(oc.is_commander_candidate, false) AS is_commander_candidate,
			COALESCE(oc.default_price_usd, '') AS price_usd,
			COALESCE(oc.default_artist, '') AS artist,
			COALESCE(oc.edhrec_rank, 0) AS edhrec_rank,
			COALESCE(oc.default_scryfall_uri, '') AS scryfall_uri,
			COALESCE(oc.default_set_code, '') AS set_code,
			COALESCE(oc.default_set_name, '') AS set_name,
			COALESCE(cp.collector_number, '') AS collector_number,
			COALESCE(cp.rarity, '') AS rarity,
			COALESCE(to_char(oc.default_released_at, 'YYYY-MM-DD'), '') AS released_at,
			COALESCE(cp.lang, 'en') AS lang,
			COALESCE(
				NULLIF(cp.card_faces_json::text, '[]'),
				oc.card_faces::text,
				'[]'
			),
			COALESCE(oc.legalities, '{}'::jsonb)::text AS legalities_json
		FROM oracle_cards oc
		LEFT JOIN card_prints cp ON cp.scryfall_id = oc.default_print_id
	`
}

func cardSearchNameOrderSQL() string {
	return "lower(oc.name) ASC, oc.name ASC, oc.oracle_id ASC"
}

func cardSearchNameOrderSQLForDirection(direction string) string {
	order := "ASC"
	if direction == "desc" {
		order = "DESC"
	}
	return "lower(oc.name) " + order + ", oc.name " + order + ", oc.oracle_id " + order
}

func cardSearchOldestEnglishPrintingExprSQL() string {
	return `(
			SELECT MIN(cp_oldest.released_at)
			FROM card_prints cp_oldest
			WHERE cp_oldest.oracle_id = oc.oracle_id
			  AND lower(COALESCE(cp_oldest.lang, 'en')) = 'en'
		)`
}

func cardSearchRarityRankExprSQL() string {
	return `CASE lower(COALESCE(cp.rarity, ''))
			WHEN 'common' THEN 0
			WHEN 'uncommon' THEN 1
			WHEN 'rare' THEN 2
			WHEN 'mythic' THEN 3
			ELSE 4
		END`
}

func cardSearchOrderBySQL(rawSort string, rawDirection string, hasNameQuery bool, fuzzy bool) string {
	direction := normalizeCardSearchSortDirection(rawSort, hasNameQuery, rawDirection)
	alphabeticalOrder := cardSearchNameOrderSQLForDirection(direction)
	nameOrder := cardSearchNameOrderSQL()
	relevanceOrder := "DESC"
	tiebreakerOrder := "ASC"
	if direction == "desc" {
		relevanceOrder = "DESC"
		tiebreakerOrder = "ASC"
	} else {
		relevanceOrder = "ASC"
		tiebreakerOrder = "DESC"
	}

	switch normalizeCardSearchSort(rawSort) {
	case "alphabetical":
		return alphabeticalOrder
	case "mana_value":
		return "COALESCE(oc.cmc, 0) " + strings.ToUpper(direction) + ", " + nameOrder
	case "newest_printing":
		return "oc.default_released_at " + strings.ToUpper(direction) + " NULLS LAST, " + nameOrder
	case "oldest_printing":
		return cardSearchOldestEnglishPrintingExprSQL() + " " + strings.ToUpper(direction) + " NULLS LAST, " + nameOrder
	case "rarity":
		rankDirection := strings.ToUpper(direction)
		return "CASE WHEN " + cardSearchRarityRankExprSQL() + " = 4 THEN 1 ELSE 0 END ASC, " +
			cardSearchRarityRankExprSQL() + " " + rankDirection + ", " +
			"lower(COALESCE(cp.rarity, '')) " + rankDirection + ", " +
			nameOrder
	default:
		if hasNameQuery && fuzzy {
			return "(oc.name_search = $1) " + relevanceOrder + ", " +
				"(oc.name_search LIKE $1 || '%') " + relevanceOrder + ", " +
				"similarity(oc.name_search, $1) " + relevanceOrder + ", " +
				"COALESCE(oc.edhrec_rank, 999999) " + tiebreakerOrder + ", " +
				nameOrder
		}
		return alphabeticalOrder
	}
}

func buildCardSearchFilters(params CardSearchParams, startArg int) (string, []any) {
	nameQuery := strings.TrimSpace(params.Query)
	manaCostQuery := strings.TrimSpace(params.ManaCost)
	textQuery := strings.TrimSpace(params.OracleText)
	typeFilter := strings.TrimSpace(params.TypeFilter)
	typeFilters := make([]CardTypeFilter, 0, len(params.TypeFilters)+1)
	if typeFilter != "" {
		typeFilters = append(typeFilters, CardTypeFilter{Value: typeFilter})
	}
	typeFilters = append(typeFilters, params.TypeFilters...)
	typeFilters = normalizeCardTypeFilters(typeFilters)
	colorFilters, colorlessOnly := splitColorFilters(params.Colors)
	colorMode := normalizeColorMatchMode(params.ColorMode)
	rarities := normalizeRarityFilters(params.Rarities)
	if len(rarities) == 0 {
		if rarity := normalizeRarityFilter(params.Rarity); rarity != "" {
			rarities = []string{rarity}
		}
	}
	setQuery := strings.TrimSpace(params.SetQuery)
	artistQuery := strings.TrimSpace(params.ArtistQuery)
	layout := normalizeLayoutFilter(params.Layout)
	stat := normalizeCardStatFilter(params.Stat)
	statOperator := normalizeCardStatOperator(params.StatOperator)
	statValue := params.StatValue
	statMin := params.StatMin
	statMax := params.StatMax
	priceOperator := normalizeCardStatOperator(params.PriceOperator)
	priceValue := params.PriceValue
	if stat == "mana_value" {
		if statValue == nil {
			statValue = params.StatValue
		}
		if statMin == nil {
			statMin = params.ManaValueMin
		}
		if statMax == nil {
			statMax = params.ManaValueMax
		}
	}

	// Keep generic card search results focused on playable cards.
	clauses := []string{
		"lower(btrim(COALESCE(oc.type_line, ''))) <> 'card'",
	}
	includeTokenLayouts := params.IncludeTokens || layout == "token" || layout == "double_faced_token"
	if includeTokenLayouts {
		clauses = append(clauses, "(COALESCE(oc.legal_anywhere, TRUE) = TRUE OR lower(btrim(COALESCE(oc.layout, ''))) IN ('token', 'double_faced_token'))")
	} else {
		clauses = append(clauses, "COALESCE(oc.legal_anywhere, TRUE) = TRUE")
		clauses = append(clauses, "lower(btrim(COALESCE(oc.layout, ''))) <> 'token'")
		clauses = append(clauses, "lower(btrim(COALESCE(oc.layout, ''))) <> 'double_faced_token'")
	}
	args := make([]any, 0, 1+len(colorFilters))
	argN := startArg

	if params.CommanderLegal {
		clauses = append(clauses, "oc.commander_legal = TRUE")
	}
	if params.CommanderOnly {
		clauses = append(clauses, "oc.is_commander_candidate = TRUE")
		clauses = append(clauses, "lower(COALESCE(oc.type_line, '')) NOT LIKE '%battle%'")
	}
	if nameQuery != "" {
		clauses = append(clauses, "(oc.name ILIKE '%' || $"+fmt.Sprint(argN)+" || '%')")
		args = append(args, nameQuery)
		argN++
	}
	if manaCostQuery != "" {
		clauses = append(clauses, "COALESCE(oc.mana_cost, '') ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
		args = append(args, manaCostQuery)
		argN++
	}
	if textQuery != "" {
		if params.OracleTextNot {
			clauses = append(clauses, "oc.oracle_text NOT ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
		} else {
			clauses = append(clauses, "oc.oracle_text ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
		}
		args = append(args, textQuery)
		argN++
	}
	for _, filter := range typeFilters {
		if params.TypePartial {
			if filter.Negated {
				clauses = append(clauses, "oc.type_line NOT ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
			} else {
				clauses = append(clauses, "oc.type_line ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
			}
			args = append(args, filter.Value)
		} else {
			if filter.Negated {
				clauses = append(clauses, "oc.type_line !~* $"+fmt.Sprint(argN))
			} else {
				clauses = append(clauses, "oc.type_line ~* $"+fmt.Sprint(argN))
			}
			args = append(args, exactTypePattern(filter.Value))
		}
		argN++
	}
	if colorlessOnly {
		clauses = append(clauses, "COALESCE(array_length(oc.color_identity, 1), 0) = 0")
	} else if len(colorFilters) > 0 {
		switch colorMode {
		case "exact":
			clauses = append(clauses, "oc.color_identity @> $"+fmt.Sprint(argN)+"::text[]")
			clauses = append(clauses, "oc.color_identity <@ $"+fmt.Sprint(argN)+"::text[]")
		case "at_most":
			clauses = append(clauses, "oc.color_identity <@ $"+fmt.Sprint(argN)+"::text[]")
		default:
			clauses = append(clauses, "oc.color_identity @> $"+fmt.Sprint(argN)+"::text[]")
		}
		args = append(args, pq.Array(colorFilters))
		argN++
	}
	if len(params.StatFilters) > 0 {
		for _, filter := range params.StatFilters {
			clauses = append(
				clauses,
				cardStatColumn(filter.Stat)+" "+cardStatOperatorSQL(filter.Operator)+" $"+fmt.Sprint(argN),
			)
			args = append(args, filter.Value)
			argN++
		}
	} else {
		if statValue != nil {
			clauses = append(clauses, cardStatColumn(stat)+" "+cardStatOperatorSQL(statOperator)+" $"+fmt.Sprint(argN))
			args = append(args, *statValue)
			argN++
		} else if statMin != nil {
			clauses = append(clauses, cardStatColumn(stat)+" >= $"+fmt.Sprint(argN))
			args = append(args, *statMin)
			argN++
		}
		if statValue == nil && statMax != nil {
			clauses = append(clauses, cardStatColumn(stat)+" <= $"+fmt.Sprint(argN))
			args = append(args, *statMax)
			argN++
		}
	}
	priceExpr := "NULLIF(regexp_replace(COALESCE(oc.default_price_usd, ''), '[^0-9.]', '', 'g'), '')::double precision"
	if len(params.PriceFilters) > 0 {
		for _, filter := range params.PriceFilters {
			clauses = append(clauses, priceExpr+" "+cardStatOperatorSQL(filter.Operator)+" $"+fmt.Sprint(argN))
			args = append(args, filter.Value)
			argN++
		}
	} else {
		if priceValue != nil {
			clauses = append(clauses, priceExpr+" "+cardStatOperatorSQL(priceOperator)+" $"+fmt.Sprint(argN))
			args = append(args, *priceValue)
			argN++
		} else if params.PriceUSDMin != nil {
			clauses = append(clauses, priceExpr+" >= $"+fmt.Sprint(argN))
			args = append(args, *params.PriceUSDMin)
			argN++
		}
		if priceValue == nil && params.PriceUSDMax != nil {
			clauses = append(clauses, priceExpr+" <= $"+fmt.Sprint(argN))
			args = append(args, *params.PriceUSDMax)
			argN++
		}
	}
	if len(rarities) == 1 {
		clauses = append(clauses, "lower(COALESCE(cp.rarity, '')) = $"+fmt.Sprint(argN))
		args = append(args, rarities[0])
		argN++
	} else if len(rarities) > 1 {
		clauses = append(clauses, "lower(COALESCE(cp.rarity, '')) = ANY($"+fmt.Sprint(argN)+"::text[])")
		args = append(args, pq.Array(rarities))
		argN++
	}
	if setQuery != "" {
		clauses = append(clauses, "(oc.default_set_code ILIKE '%' || $"+fmt.Sprint(argN)+" || '%' OR oc.default_set_name ILIKE '%' || $"+fmt.Sprint(argN)+" || '%')")
		args = append(args, setQuery)
		argN++
	}
	if artistQuery != "" {
		clauses = append(clauses, "oc.default_artist ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
		args = append(args, artistQuery)
		argN++
	}
	if layout != "" {
		clauses = append(clauses, "lower(COALESCE(oc.layout, '')) = $"+fmt.Sprint(argN))
		args = append(args, layout)
		argN++
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

func scanCanonicalCards(rows *sql.Rows, limit int) ([]Card, error) {
	out := make([]Card, 0, limit)
	for rows.Next() {
		var (
			printingID, oracleID, name, manaCost, typeLine, oracleText, flavorText, imageURI string
			artCropURI, power, toughness, loyalty                                            string
			layout, priceUSD, artist, scryfallURI, setCode, setName                          string
			collectorNumber, rarity, releasedAt, lang, facesJSON, legalitiesJSON             string
			colors, colorIdentity                                                            []string
			cmc                                                                              float64
			legalAnywhere, commanderLegal, isCommanderCandidate                              bool
			edhrecRank                                                                       int
		)
		if err := rows.Scan(
			&printingID,
			&oracleID,
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&flavorText,
			&imageURI,
			&artCropURI,
			pq.Array(&colors),
			pq.Array(&colorIdentity),
			&cmc,
			&power,
			&toughness,
			&loyalty,
			&layout,
			&legalAnywhere,
			&commanderLegal,
			&isCommanderCandidate,
			&priceUSD,
			&artist,
			&edhrecRank,
			&scryfallURI,
			&setCode,
			&setName,
			&collectorNumber,
			&rarity,
			&releasedAt,
			&lang,
			&facesJSON,
			&legalitiesJSON,
		); err != nil {
			return nil, err
		}
		out = append(out, scanCanonicalCard(
			printingID,
			oracleID,
			name,
			manaCost,
			typeLine,
			oracleText,
			flavorText,
			imageURI,
			artCropURI,
			colors,
			colorIdentity,
			cmc,
			power,
			toughness,
			loyalty,
			layout,
			legalAnywhere,
			commanderLegal,
			isCommanderCandidate,
			priceUSD,
			artist,
			edhrecRank,
			scryfallURI,
			setCode,
			setName,
			collectorNumber,
			rarity,
			releasedAt,
			lang,
			facesJSON,
			legalitiesJSON,
		))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func cardSearchLimit(params CardSearchParams) (int, bool) {
	if params.AllMatches {
		return 0, true
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 120
	}
	if limit > 300 {
		limit = 300
	}
	return limit, false
}

func cardNameSearchMatchSQL() string {
	return "(oc.name_search LIKE '%' || $1 || '%' OR oc.name_search % $1)"
}

func GetCardByName(ctx context.Context, db *sql.DB, name string) (*Card, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrCardNotFound
	}

	sqlText := canonicalCardSelectSQL() + `
		WHERE oc.name_search = normalize_card_name($1)
		ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
		LIMIT 1
	`
	rows, err := db.QueryContext(ctx, sqlText, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out, err := scanCanonicalCards(rows, 1)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrCardNotFound
	}
	return &out[0], nil
}

func GetCardByOracleID(ctx context.Context, db *sql.DB, oracleID string) (*Card, error) {
	oracleID = strings.TrimSpace(oracleID)
	if oracleID == "" {
		return nil, ErrCardNotFound
	}

	sqlText := canonicalCardSelectSQL() + `
		WHERE oc.oracle_id = $1::uuid
		LIMIT 1
	`
	rows, err := db.QueryContext(ctx, sqlText, oracleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out, err := scanCanonicalCards(rows, 1)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrCardNotFound
	}
	return &out[0], nil
}

func countCardSearchMatches(
	ctx context.Context,
	db *sql.DB,
	plan cardSearchSourcePlan,
	whereSQL string,
	args []any,
) (int, error) {
	var total int
	err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) `+plan.CountFromSQL+` WHERE `+whereSQL+plan.FilterSQL,
		args...,
	).Scan(&total)
	return total, err
}

func completedCardSearchOutcome(
	found []Card,
	exactNameMatch bool,
	matchingPrintings bool,
	window cardSearchPageWindow,
) CardSearchOutcome {
	return CardSearchOutcome{
		Cards:             found,
		ExactNameMatch:    exactNameMatch,
		Total:             window.Total,
		Page:              window.Page,
		PageSize:          window.PageSize,
		TotalPages:        window.TotalPages,
		MatchingPrintings: matchingPrintings,
	}
}

func unpagedCardSearchOutcome(found []Card, exactNameMatch, matchingPrintings bool) CardSearchOutcome {
	totalPages := 0
	if len(found) > 0 {
		totalPages = 1
	}
	return CardSearchOutcome{
		Cards:             found,
		ExactNameMatch:    exactNameMatch,
		Total:             len(found),
		Page:              1,
		PageSize:          len(found),
		TotalPages:        totalPages,
		MatchingPrintings: matchingPrintings,
	}
}

func exactCardSearchOutcome(found []Card, limit int, matchingPrintings bool) CardSearchOutcome {
	return completedCardSearchOutcome(
		found,
		len(found) > 0,
		matchingPrintings,
		normalizeCardSearchPage(1, limit, len(found)),
	)
}

// SearchCardsWithOutcome first returns one exact normalized-name match when
// present. If no exact match exists, it returns a counted, server-side page of
// fuzzy matches. Printing-specific filters select one deterministic matching
// printing per oracle card; ordinary name/oracle searches retain the canonical
// default printing.
func SearchCardsWithOutcome(ctx context.Context, db *sql.DB, params CardSearchParams) (CardSearchOutcome, error) {
	limit, unlimited := cardSearchLimit(params)
	scanCapacity := limit
	if unlimited || scanCapacity < 1 {
		scanCapacity = 64
	}

	query := strings.TrimSpace(params.Query)
	if query == "" {
		orderSQL := cardSearchOrderBySQL(params.Sort, params.SortDirection, false, false)
		plan := buildCardSearchSourcePlan(params, 1)
		sqlText := plan.SelectSQL + `
			WHERE 1=1` + plan.FilterSQL + `
			ORDER BY ` + orderSQL
		args := append([]any(nil), plan.Args...)

		if unlimited {
			rows, err := db.QueryContext(ctx, sqlText, args...)
			if err != nil {
				return CardSearchOutcome{}, err
			}
			defer rows.Close()
			found, err := scanCanonicalCards(rows, scanCapacity)
			if err != nil {
				return CardSearchOutcome{}, err
			}
			return unpagedCardSearchOutcome(found, false, plan.MatchingPrintings), nil
		}

		total, err := countCardSearchMatches(ctx, db, plan, "1=1", plan.Args)
		if err != nil {
			return CardSearchOutcome{}, err
		}
		window := normalizeCardSearchPage(params.Page, limit, total)
		if total == 0 {
			return completedCardSearchOutcome(nil, false, plan.MatchingPrintings, window), nil
		}

		sqlText += `
			LIMIT $` + fmt.Sprint(len(plan.Args)+1) + `
			OFFSET $` + fmt.Sprint(len(plan.Args)+2)
		args = append(args, window.PageSize, window.Offset)
		rows, err := db.QueryContext(ctx, sqlText, args...)
		if err != nil {
			return CardSearchOutcome{}, err
		}
		defer rows.Close()
		found, err := scanCanonicalCards(rows, scanCapacity)
		if err != nil {
			return CardSearchOutcome{}, err
		}
		return completedCardSearchOutcome(found, false, plan.MatchingPrintings, window), nil
	}

	if params.NameExact {
		exactParams := params
		exactParams.Query = ""
		plan := buildCardSearchSourcePlan(exactParams, 2)
		exactSQL := plan.SelectSQL + `
			WHERE oc.name_search = normalize_card_name($1)` + plan.FilterSQL + `
			ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
			LIMIT 1
		`
		exactArgs := append([]any{query}, plan.Args...)
		rows, err := db.QueryContext(ctx, exactSQL, exactArgs...)
		if err != nil {
			return CardSearchOutcome{}, err
		}
		defer rows.Close()
		found, err := scanCanonicalCards(rows, 1)
		if err != nil {
			return CardSearchOutcome{}, err
		}
		return exactCardSearchOutcome(found, limit, plan.MatchingPrintings), nil
	}

	// 1) exact normalized match
	exactParams := params
	exactParams.Query = ""
	exactPlan := buildCardSearchSourcePlan(exactParams, 2)
	exactSQL := exactPlan.SelectSQL + `
		WHERE oc.name_search = normalize_card_name($1)` + exactPlan.FilterSQL + `
		ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
		LIMIT 1
	`
	exactArgs := append([]any{query}, exactPlan.Args...)
	exactRows, err := db.QueryContext(ctx, exactSQL, exactArgs...)
	if err != nil {
		return CardSearchOutcome{}, err
	}
	exactCards, err := scanCanonicalCards(exactRows, 1)
	exactRows.Close()
	if err != nil {
		return CardSearchOutcome{}, err
	}
	if len(exactCards) > 0 {
		return exactCardSearchOutcome(exactCards, limit, exactPlan.MatchingPrintings), nil
	}

	// 2) fuzzy list
	normalizedQuery := NormalizeName(query)
	if normalizedQuery == "" {
		return completedCardSearchOutcome(
			nil,
			false,
			exactPlan.MatchingPrintings,
			normalizeCardSearchPage(params.Page, limit, 0),
		), nil
	}

	fuzzyParams := params
	fuzzyParams.Query = ""
	fuzzyPlan := buildCardSearchSourcePlan(fuzzyParams, 2)
	orderSQL := cardSearchOrderBySQL(params.Sort, params.SortDirection, true, true)
	fuzzySQL := fuzzyPlan.SelectSQL + `
		WHERE ` + cardNameSearchMatchSQL() + fuzzyPlan.FilterSQL + `
		ORDER BY ` + orderSQL
	fuzzyArgs := append([]any{normalizedQuery}, fuzzyPlan.Args...)

	if unlimited {
		rows, err := db.QueryContext(ctx, fuzzySQL, fuzzyArgs...)
		if err != nil {
			return CardSearchOutcome{}, err
		}
		defer rows.Close()
		found, err := scanCanonicalCards(rows, scanCapacity)
		if err != nil {
			return CardSearchOutcome{}, err
		}
		return unpagedCardSearchOutcome(found, false, fuzzyPlan.MatchingPrintings), nil
	}

	total, err := countCardSearchMatches(ctx, db, fuzzyPlan, cardNameSearchMatchSQL(), fuzzyArgs)
	if err != nil {
		return CardSearchOutcome{}, err
	}
	window := normalizeCardSearchPage(params.Page, limit, total)
	if total == 0 {
		return completedCardSearchOutcome(nil, false, fuzzyPlan.MatchingPrintings, window), nil
	}

	fuzzySQL += `
		LIMIT $` + fmt.Sprint(len(fuzzyPlan.Args)+2) + `
		OFFSET $` + fmt.Sprint(len(fuzzyPlan.Args)+3)
	fuzzyArgs = append(fuzzyArgs, window.PageSize, window.Offset)
	rows, err := db.QueryContext(ctx, fuzzySQL, fuzzyArgs...)
	if err != nil {
		return CardSearchOutcome{}, err
	}
	defer rows.Close()
	found, err := scanCanonicalCards(rows, scanCapacity)
	if err != nil {
		return CardSearchOutcome{}, err
	}
	return completedCardSearchOutcome(found, false, fuzzyPlan.MatchingPrintings, window), nil
}

// SearchCards keeps the original result-only API for existing callers.
func SearchCards(ctx context.Context, db *sql.DB, params CardSearchParams) ([]Card, error) {
	outcome, err := SearchCardsWithOutcome(ctx, db, params)
	if err != nil {
		return nil, err
	}
	return outcome.Cards, nil
}

// RandomCard returns one sensible random card for discovery flows.
func RandomCard(ctx context.Context, db *sql.DB) (*Card, error) {
	sqlText := canonicalCardSelectSQL() + `
		WHERE lower(btrim(COALESCE(oc.type_line, ''))) <> 'card'
		  AND lower(btrim(COALESCE(oc.layout, ''))) NOT IN ('token', 'double_faced_token')
		  AND COALESCE(oc.legal_anywhere, TRUE) = TRUE
		ORDER BY random()
		LIMIT 1
	`

	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found, err := scanCanonicalCards(rows, 1)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, ErrCardNotFound
	}
	return &found[0], nil
}

// RandomTopCommanders returns a random sample of strict commander-candidate
// cards constrained to strong EDHRec rank values (<= maxEDHRecRank).
func RandomTopCommanders(
	ctx context.Context,
	db *sql.DB,
	limit int,
	maxEDHRecRank int,
) ([]Card, error) {
	if limit <= 0 {
		limit = 6
	}
	if limit > 24 {
		limit = 24
	}
	if maxEDHRecRank <= 0 {
		maxEDHRecRank = 1500
	}

	sqlText := canonicalCardSelectSQL() + `
		WHERE oc.is_commander_candidate = TRUE
		  AND lower(COALESCE(oc.type_line, '')) NOT LIKE '%battle%'
		  AND oc.edhrec_rank IS NOT NULL
		  AND oc.edhrec_rank > 0
		  AND oc.edhrec_rank <= $1
		ORDER BY random()
		LIMIT $2
	`

	rows, err := db.QueryContext(ctx, sqlText, maxEDHRecRank, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCanonicalCards(rows, limit)
}

// RandomizedTopCommanderBatch returns one page from a deterministic shuffled
// ordering of popular commander candidates. Reusing seed and passing the final
// oracle ID from one batch as afterOracleID advances through that ordering
// without reshuffling previously returned cards.
func RandomizedTopCommanderBatch(
	ctx context.Context,
	db *sql.DB,
	seed string,
	afterOracleID string,
	limit int,
	maxEDHRecRank int,
) ([]Card, error) {
	seed = strings.TrimSpace(seed)
	afterOracleID = strings.TrimSpace(afterOracleID)
	if limit <= 0 {
		limit = 8
	}
	if maxEDHRecRank <= 0 {
		maxEDHRecRank = 1500
	}

	sqlText := canonicalCardSelectSQL() + `
		WHERE oc.is_commander_candidate = TRUE
		  AND lower(COALESCE(oc.type_line, '')) NOT LIKE '%battle%'
		  AND oc.edhrec_rank IS NOT NULL
		  AND oc.edhrec_rank > 0
		  AND oc.edhrec_rank <= $2
		  AND (
			NULLIF($3::text, '') IS NULL
			OR (
				md5($1::text || oc.oracle_id::text),
				oc.oracle_id
			) > (
				md5($1::text || NULLIF($3::text, '')::uuid::text),
				NULLIF($3::text, '')::uuid
			)
		  )
		ORDER BY md5($1::text || oc.oracle_id::text), oc.oracle_id ASC
		LIMIT $4
	`

	rows, err := db.QueryContext(
		ctx,
		sqlText,
		seed,
		maxEDHRecRank,
		afterOracleID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCanonicalCards(rows, limit)
}

// PopularCommanders returns strict commander candidates in stable EDHRec rank
// order. It powers surfaces that promise a predictable popular list rather
// than a shuffled set of ideas.
func PopularCommanders(
	ctx context.Context,
	db *sql.DB,
	limit int,
) ([]Card, error) {
	if limit <= 0 {
		limit = 24
	}
	if limit > 48 {
		limit = 48
	}

	sqlText := canonicalCardSelectSQL() + `
		WHERE oc.is_commander_candidate = TRUE
		  AND lower(COALESCE(oc.type_line, '')) NOT LIKE '%battle%'
		  AND oc.edhrec_rank IS NOT NULL
		  AND oc.edhrec_rank > 0
		ORDER BY oc.edhrec_rank ASC, lower(oc.name) ASC, oc.oracle_id ASC
		LIMIT $1
	`

	rows, err := db.QueryContext(ctx, sqlText, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCanonicalCards(rows, limit)
}

// SearchCardNamesExactThenFuzzy powers deck-builder autocomplete style search.
// Positive limits return one exact match when present, else a fuzzy list.
// A non-positive limit returns every fuzzy match, with any exact match first.
func SearchCardNamesExactThenFuzzy(
	ctx context.Context,
	db *sql.DB,
	query string,
	limit int,
	commanderOnly bool,
) ([]DBCard, bool, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, false, nil
	}
	unlimited := limit <= 0
	if limit > 100 && !unlimited {
		limit = 100
	}

	sensibleNameSearchSQL := `
		AND lower(btrim(COALESCE(oc.type_line, ''))) <> 'card'
		AND lower(btrim(COALESCE(oc.layout, ''))) NOT IN ('token', 'double_faced_token')
		AND COALESCE(oc.legal_anywhere, TRUE) = TRUE
	`
	exactSQL := `
		SELECT
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, ''),
			COALESCE(oc.type_line, ''),
			COALESCE(oc.oracle_text, ''),
			COALESCE(oc.default_image_uri, ''),
			COALESCE(oc.color_identity, ARRAY[]::text[]),
			COALESCE(oc.cmc, 0),
			COALESCE(oc.is_commander_candidate, false)
		FROM oracle_cards oc
		WHERE oc.name_search = normalize_card_name($1)
	` + sensibleNameSearchSQL
	args := []any{query}
	if commanderOnly {
		exactSQL += ` AND oc.is_commander_candidate = TRUE`
		exactSQL += ` AND lower(COALESCE(oc.type_line, '')) NOT LIKE '%battle%'`
	}
	exactSQL += ` ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC LIMIT 1`

	exactRows, err := db.QueryContext(ctx, exactSQL, args...)
	if err != nil {
		return nil, false, err
	}
	exactOut := make([]DBCard, 0, 1)
	for exactRows.Next() {
		var (
			oracleID, name, manaCost, typeLine, oracleText, imageURI string
			colorIdentity                                            []string
			cmc                                                      float64
			isCommanderCandidate                                     bool
		)
		if err := exactRows.Scan(
			&oracleID,
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&imageURI,
			pq.Array(&colorIdentity),
			&cmc,
			&isCommanderCandidate,
		); err != nil {
			exactRows.Close()
			return nil, false, err
		}
		exactOut = append(exactOut, scanDBCardRow(
			oracleID,
			name,
			manaCost,
			typeLine,
			oracleText,
			imageURI,
			colorIdentity,
			cmc,
			isCommanderCandidate,
		))
	}
	if err := exactRows.Err(); err != nil {
		exactRows.Close()
		return nil, false, err
	}
	exactRows.Close()
	hasExact := len(exactOut) > 0
	if hasExact && !unlimited {
		return exactOut, true, nil
	}

	normalizedQuery := NormalizeName(query)
	if normalizedQuery == "" {
		return nil, false, nil
	}

	fuzzySQL := `
		SELECT
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, ''),
			COALESCE(oc.type_line, ''),
			COALESCE(oc.oracle_text, ''),
			COALESCE(oc.default_image_uri, ''),
			COALESCE(oc.color_identity, ARRAY[]::text[]),
			COALESCE(oc.cmc, 0),
			COALESCE(oc.is_commander_candidate, false)
		FROM oracle_cards oc
		WHERE ` + cardNameSearchMatchSQL() + `
	` + sensibleNameSearchSQL
	fuzzyArgs := []any{normalizedQuery}
	if commanderOnly {
		fuzzySQL += ` AND oc.is_commander_candidate = TRUE`
		fuzzySQL += ` AND lower(COALESCE(oc.type_line, '')) NOT LIKE '%battle%'`
	}
	fuzzySQL += `
		ORDER BY
			(oc.name_search = $1) DESC,
			(oc.name_search LIKE $1 || '%') DESC,
			similarity(oc.name_search, $1) DESC,
			COALESCE(oc.edhrec_rank, 999999) ASC,
			oc.name ASC
	`
	if !unlimited {
		fuzzySQL += ` LIMIT $2`
		fuzzyArgs = append(fuzzyArgs, limit)
	}

	rows, err := db.QueryContext(ctx, fuzzySQL, fuzzyArgs...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	capacity := limit
	if unlimited || capacity < 1 {
		capacity = 64
	}
	out := make([]DBCard, 0, capacity)
	for rows.Next() {
		var (
			oracleID, name, manaCost, typeLine, oracleText, imageURI string
			colorIdentity                                            []string
			cmc                                                      float64
			isCommanderCandidate                                     bool
		)
		if err := rows.Scan(
			&oracleID,
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&imageURI,
			pq.Array(&colorIdentity),
			&cmc,
			&isCommanderCandidate,
		); err != nil {
			return nil, false, err
		}
		out = append(out, scanDBCardRow(
			oracleID,
			name,
			manaCost,
			typeLine,
			oracleText,
			imageURI,
			colorIdentity,
			cmc,
			isCommanderCandidate,
		))
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	return out, hasExact, nil
}

// ResolveCardByNameFuzzy returns an exact normalized-name match when possible,
// then falls back to the best fuzzy candidate.
func ResolveCardByNameFuzzy(ctx context.Context, db *sql.DB, name string) (*Card, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrCardNotFound
	}

	card, err := GetCardByName(ctx, db, name)
	if err == nil {
		return card, nil
	}
	if !errors.Is(err, ErrCardNotFound) {
		return nil, err
	}

	found, err := SearchCards(ctx, db, CardSearchParams{
		Query: name,
		Limit: 1,
	})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, ErrCardNotFound
	}
	best := found[0]
	return &best, nil
}

// ListCardVersionsByName returns printings for a card name by first resolving
// to its canonical oracle_id.
func ListCardVersionsByName(ctx context.Context, db *sql.DB, name string, limit int) ([]Card, error) {
	card, err := GetCardByName(ctx, db, name)
	if err != nil {
		return nil, err
	}
	return ListCardVersionsByOracleID(ctx, db, card.OracleID, limit)
}

func cardPrintingSelectSQL() string {
	return `
		SELECT
			cp.scryfall_id::text AS id,
			oc.oracle_id::text AS oracle_id,
			COALESCE(cp.lang, 'en') AS lang,
			oc.name,
			COALESCE(oc.mana_cost, '') AS mana_cost,
			COALESCE(oc.type_line, '') AS type_line,
			COALESCE(oc.oracle_text, '') AS oracle_text,
			COALESCE(cp.flavor_text, '') AS flavor_text,
			COALESCE(cp.image_uri, '') AS image_uri,
			COALESCE(cp.image_uris->>'art_crop', cp.image_uri, '') AS art_crop_uri,
			COALESCE(oc.colors, ARRAY[]::text[]) AS colors,
			COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
			COALESCE(oc.cmc, 0) AS cmc,
			COALESCE(oc.power_text, '') AS power_text,
			COALESCE(oc.toughness_text, '') AS toughness_text,
			COALESCE(oc.loyalty_text, '') AS loyalty_text,
			COALESCE(oc.layout, '') AS layout,
			COALESCE(oc.legal_anywhere, true) AS legal_anywhere,
			COALESCE(oc.commander_legal, false) AS commander_legal,
			COALESCE(oc.is_commander_candidate, false) AS is_commander_candidate,
			COALESCE(cp.price_usd, '') AS price_usd,
			COALESCE(cp.artist, '') AS artist,
			COALESCE(oc.edhrec_rank, 0) AS edhrec_rank,
			COALESCE(cp.scryfall_uri, '') AS scryfall_uri,
			COALESCE(cp.set_code, '') AS set_code,
			COALESCE(cp.set_name, '') AS set_name,
			COALESCE(cp.collector_number, '') AS collector_number,
			COALESCE(cp.rarity, '') AS rarity,
			COALESCE(to_char(cp.released_at, 'YYYY-MM-DD'), '') AS released_at,
			COALESCE(
				NULLIF(cp.card_faces_json::text, '[]'),
				oc.card_faces::text,
				'[]'
			),
			COALESCE(oc.legalities, '{}'::jsonb)::text AS legalities_json
		FROM card_prints cp
		JOIN oracle_cards oc
		  ON oc.oracle_id = cp.oracle_id
	`
}

type cardPrintingScanner interface {
	Scan(dest ...any) error
}

func scanCardPrinting(scanner cardPrintingScanner) (Card, error) {
	var (
		card                                                Card
		facesJSON, legalitiesJSON                           string
		colors, colorIdentity                               []string
		legalAnywhere, commanderLegal, isCommanderCandidate bool
	)
	err := scanner.Scan(
		&card.ID,
		&card.OracleID,
		&card.Lang,
		&card.Name,
		&card.ManaCost,
		&card.TypeLine,
		&card.OracleText,
		&card.FlavorText,
		&card.ImageURI,
		&card.ArtCropURI,
		pq.Array(&colors),
		pq.Array(&colorIdentity),
		&card.CMC,
		&card.Power,
		&card.Toughness,
		&card.Loyalty,
		&card.Layout,
		&legalAnywhere,
		&commanderLegal,
		&isCommanderCandidate,
		&card.PriceUSD,
		&card.Artist,
		&card.EDHRecRank,
		&card.ScryfallURI,
		&card.SetCode,
		&card.SetName,
		&card.CollectorNumber,
		&card.Rarity,
		&card.ReleasedAt,
		&facesJSON,
		&legalitiesJSON,
	)
	if err != nil {
		return Card{}, err
	}
	card.Colors = colors
	card.ColorIdentity = colorIdentity
	card.LegalAnywhere = legalAnywhere
	card.CommanderLegal = commanderLegal
	card.Legalities = decodeLegalitiesJSON(legalitiesJSON)
	card.IsCommanderCandidate = isCommanderCandidate
	card.Faces = decodeCardFacesJSON(facesJSON)
	return card, nil
}

// GetCardPrintingByID returns one exact printing when it belongs to oracleID.
func GetCardPrintingByID(ctx context.Context, db *sql.DB, oracleID, scryfallID string) (*Card, error) {
	oracleID = strings.TrimSpace(oracleID)
	scryfallID = strings.TrimSpace(scryfallID)
	if oracleID == "" || scryfallID == "" {
		return nil, ErrCardNotFound
	}

	row := db.QueryRowContext(ctx, cardPrintingSelectSQL()+`
		WHERE oc.oracle_id = $1::uuid
		  AND cp.scryfall_id = $2::uuid
		LIMIT 1
	`, oracleID, scryfallID)
	card, err := scanCardPrinting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, err
	}
	return &card, nil
}

// GetCardPrintingByScryfallID returns an exact printing without requiring the
// caller to already know its oracle card ID.
func GetCardPrintingByScryfallID(ctx context.Context, db *sql.DB, scryfallID string) (*Card, error) {
	scryfallID = strings.TrimSpace(scryfallID)
	if scryfallID == "" {
		return nil, ErrCardNotFound
	}

	row := db.QueryRowContext(ctx, cardPrintingSelectSQL()+`
		WHERE cp.scryfall_id = $1::uuid
		LIMIT 1
	`, scryfallID)
	card, err := scanCardPrinting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, err
	}
	return &card, nil
}

// ListCardVersionsByOracleID returns printings for an oracle card id, newest first.
func ListCardVersionsByOracleID(ctx context.Context, db *sql.DB, oracleID string, limit int) ([]Card, error) {
	oracleID = strings.TrimSpace(oracleID)
	if oracleID == "" {
		return nil, ErrCardNotFound
	}
	if limit <= 0 {
		limit = 120
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := db.QueryContext(ctx, cardPrintingSelectSQL()+`
		WHERE oc.oracle_id = $1::uuid
		ORDER BY cp.released_at DESC NULLS LAST, cp.set_code ASC, cp.collector_number ASC
		LIMIT $2
	`, oracleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Card, 0, limit)
	for rows.Next() {
		card, scanErr := scanCardPrinting(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrCardNotFound
	}
	return out, nil
}

// ResolveCardNamesBatch resolves many inputs with two DB round-trips:
// exact batch first, then fuzzy best-match for misses.
// Returned map keys are lower+trim versions of the original input names.
func ResolveCardNamesBatch(
	ctx context.Context,
	db *sql.DB,
	names []string,
	similarityThreshold float64,
) (map[string]NameResolution, error) {
	if similarityThreshold <= 0 {
		similarityThreshold = 0.40
	}

	type inputName struct {
		key    string
		search string
	}

	uniqueByKey := make(map[string]struct{}, len(names))
	inputs := make([]inputName, 0, len(names))
	uniqueSearches := make([]string, 0, len(names))
	seenSearch := make(map[string]struct{}, len(names))

	for _, raw := range names {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" {
			continue
		}
		if _, exists := uniqueByKey[key]; exists {
			continue
		}
		uniqueByKey[key] = struct{}{}

		search := NormalizeName(raw)
		if search == "" {
			continue
		}
		inputs = append(inputs, inputName{key: key, search: search})
		if _, exists := seenSearch[search]; !exists {
			seenSearch[search] = struct{}{}
			uniqueSearches = append(uniqueSearches, search)
		}
	}

	if len(uniqueSearches) == 0 {
		return map[string]NameResolution{}, nil
	}

	type exactRow struct {
		search string
		card   DBCard
	}

	exactMatches := make(map[string]NameResolution, len(uniqueSearches))
	exactRows, err := db.QueryContext(ctx, `
		WITH input AS (
			SELECT DISTINCT unnest($1::text[]) AS q
		),
		ranked AS (
			SELECT
				i.q,
				oc.oracle_id::text,
				oc.name,
				COALESCE(oc.mana_cost, '') AS mana_cost,
				COALESCE(oc.type_line, '') AS type_line,
				COALESCE(oc.oracle_text, '') AS oracle_text,
				COALESCE(oc.default_image_uri, '') AS image_uri,
				COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
				COALESCE(oc.cmc, 0) AS cmc,
				COALESCE(oc.is_commander_candidate, false) AS is_commander_candidate,
				ROW_NUMBER() OVER (
					PARTITION BY i.q
					ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
				) AS rn
			FROM input i
			JOIN oracle_cards oc
			  ON oc.name_search = i.q
		)
		SELECT
			q,
			oracle_id,
			name,
			mana_cost,
			type_line,
			oracle_text,
			image_uri,
			color_identity,
			cmc,
			is_commander_candidate
		FROM ranked
		WHERE rn = 1
	`, pq.Array(uniqueSearches))
	if err != nil {
		return nil, err
	}

	for exactRows.Next() {
		var (
			search, oracleID, name, manaCost, typeLine, oracleText, imageURI string
			colorIdentity                                                    []string
			cmc                                                              float64
			isCommanderCandidate                                             bool
		)
		if err := exactRows.Scan(
			&search,
			&oracleID,
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&imageURI,
			pq.Array(&colorIdentity),
			&cmc,
			&isCommanderCandidate,
		); err != nil {
			exactRows.Close()
			return nil, err
		}

		exactMatches[search] = NameResolution{
			Card: scanDBCardRow(
				oracleID,
				name,
				manaCost,
				typeLine,
				oracleText,
				imageURI,
				colorIdentity,
				cmc,
				isCommanderCandidate,
			),
			Exact:      true,
			Similarity: 1,
		}
	}
	if err := exactRows.Err(); err != nil {
		exactRows.Close()
		return nil, err
	}
	exactRows.Close()

	misses := make([]string, 0, len(uniqueSearches))
	for _, search := range uniqueSearches {
		if _, ok := exactMatches[search]; !ok {
			misses = append(misses, search)
		}
	}

	if len(misses) > 0 {
		fuzzyRows, err := db.QueryContext(ctx, `
			WITH input AS (
				SELECT DISTINCT unnest($1::text[]) AS q
			),
			ranked AS (
				SELECT
					i.q,
					oc.oracle_id::text,
					oc.name,
					COALESCE(oc.mana_cost, '') AS mana_cost,
					COALESCE(oc.type_line, '') AS type_line,
					COALESCE(oc.oracle_text, '') AS oracle_text,
					COALESCE(oc.default_image_uri, '') AS image_uri,
					COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
					COALESCE(oc.cmc, 0) AS cmc,
					COALESCE(oc.is_commander_candidate, false) AS is_commander_candidate,
					similarity(oc.name_search, i.q) AS sim,
					ROW_NUMBER() OVER (
						PARTITION BY i.q
						ORDER BY
							similarity(oc.name_search, i.q) DESC,
							COALESCE(oc.edhrec_rank, 999999) ASC,
							oc.name ASC
					) AS rn
				FROM input i
				JOIN oracle_cards oc
				  ON oc.name_search % i.q
			)
			SELECT
				q,
				oracle_id,
				name,
				mana_cost,
				type_line,
				oracle_text,
				image_uri,
				color_identity,
				cmc,
				is_commander_candidate,
				sim
			FROM ranked
			WHERE rn = 1
			  AND sim >= $2
		`, pq.Array(misses), similarityThreshold)
		if err != nil {
			return nil, err
		}

		for fuzzyRows.Next() {
			var (
				search, oracleID, name, manaCost, typeLine, oracleText, imageURI string
				colorIdentity                                                    []string
				cmc, sim                                                         float64
				isCommanderCandidate                                             bool
			)
			if err := fuzzyRows.Scan(
				&search,
				&oracleID,
				&name,
				&manaCost,
				&typeLine,
				&oracleText,
				&imageURI,
				pq.Array(&colorIdentity),
				&cmc,
				&isCommanderCandidate,
				&sim,
			); err != nil {
				fuzzyRows.Close()
				return nil, err
			}
			exactMatches[search] = NameResolution{
				Card: scanDBCardRow(
					oracleID,
					name,
					manaCost,
					typeLine,
					oracleText,
					imageURI,
					colorIdentity,
					cmc,
					isCommanderCandidate,
				),
				Exact:      false,
				Similarity: sim,
			}
		}
		if err := fuzzyRows.Err(); err != nil {
			fuzzyRows.Close()
			return nil, err
		}
		fuzzyRows.Close()
	}

	out := make(map[string]NameResolution, len(inputs))
	for _, input := range inputs {
		if resolved, ok := exactMatches[input.search]; ok {
			out[input.key] = resolved
		}
	}
	return out, nil
}
