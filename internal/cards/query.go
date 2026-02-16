package cards

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type CardSearchParams struct {
	Query         string
	TypeFilter    string
	ColorIdentity []string
	CommanderOnly bool
	Limit         int
}

type NameResolution struct {
	Card       DBCard
	Exact      bool
	Similarity float64
}

func normalizeColorIdentityFilters(colors []string) []string {
	if len(colors) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(colors))
	for _, c := range colors {
		upper := strings.ToUpper(strings.TrimSpace(c))
		switch upper {
		case "W", "U", "B", "R", "G":
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

func scanCanonicalCard(
	oracleID, name, manaCost, typeLine, oracleText, imageURI string,
	colors, colorIdentity []string,
	cmc float64,
	layout string,
	commanderLegal bool,
	isCommanderCandidate bool,
	priceUSD, artist string,
	edhrecRank int,
	scryfallURI, setCode, setName, releasedAt, facesJSON string,
) Card {
	return Card{
		ID:                   strings.TrimSpace(oracleID),
		OracleID:             strings.TrimSpace(oracleID),
		Name:                 strings.TrimSpace(name),
		ManaCost:             strings.TrimSpace(manaCost),
		TypeLine:             strings.TrimSpace(typeLine),
		OracleText:           strings.TrimSpace(oracleText),
		ImageURI:             strings.TrimSpace(imageURI),
		Colors:               colors,
		ColorIdentity:        colorIdentity,
		CMC:                  cmc,
		Layout:               strings.TrimSpace(layout),
		CommanderLegal:       commanderLegal,
		IsCommanderCandidate: isCommanderCandidate,
		PriceUSD:             strings.TrimSpace(priceUSD),
		Artist:               strings.TrimSpace(artist),
		EDHRecRank:           edhrecRank,
		ScryfallURI:          strings.TrimSpace(scryfallURI),
		SetCode:              strings.TrimSpace(setCode),
		SetName:              strings.TrimSpace(setName),
		ReleasedAt:           strings.TrimSpace(releasedAt),
		Faces:                decodeCardFacesJSON(facesJSON),
	}
}

func canonicalCardSelectSQL() string {
	return `
		SELECT
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, '') AS mana_cost,
			COALESCE(oc.type_line, '') AS type_line,
			COALESCE(oc.oracle_text, '') AS oracle_text,
			COALESCE(oc.default_image_uri, '') AS image_uri,
			COALESCE(oc.colors, ARRAY[]::text[]) AS colors,
			COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
			COALESCE(oc.cmc, 0) AS cmc,
			COALESCE(oc.layout, '') AS layout,
			COALESCE(oc.commander_legal, false) AS commander_legal,
			COALESCE(oc.is_commander_candidate, false) AS is_commander_candidate,
			COALESCE(oc.default_price_usd, '') AS price_usd,
			COALESCE(oc.default_artist, '') AS artist,
			COALESCE(oc.edhrec_rank, 0) AS edhrec_rank,
			COALESCE(oc.default_scryfall_uri, '') AS scryfall_uri,
			COALESCE(oc.default_set_code, '') AS set_code,
			COALESCE(oc.default_set_name, '') AS set_name,
			COALESCE(to_char(oc.default_released_at, 'YYYY-MM-DD'), '') AS released_at,
			COALESCE(oc.card_faces::text, '')
		FROM oracle_cards oc
	`
}

func buildCardSearchFilters(params CardSearchParams, startArg int) (string, []any) {
	typeFilter := strings.TrimSpace(params.TypeFilter)
	colorFilters := normalizeColorIdentityFilters(params.ColorIdentity)

	// Keep generic card search results focused on playable cards.
	clauses := []string{
		"lower(btrim(COALESCE(oc.layout, ''))) <> 'token'",
		"lower(btrim(COALESCE(oc.type_line, ''))) <> 'card'",
	}
	args := make([]any, 0, 1+len(colorFilters))
	argN := startArg

	if params.CommanderOnly {
		clauses = append(clauses, "oc.is_commander_candidate = TRUE")
	}
	if typeFilter != "" {
		clauses = append(clauses, "oc.type_line ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
		args = append(args, typeFilter)
		argN++
	}
	for _, color := range colorFilters {
		clauses = append(clauses, "oc.color_identity @> ARRAY[$"+fmt.Sprint(argN)+"]::text[]")
		args = append(args, color)
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
			oracleID, name, manaCost, typeLine, oracleText, imageURI string
			layout, priceUSD, artist, scryfallURI, setCode, setName  string
			releasedAt, facesJSON                                    string
			colors, colorIdentity                                    []string
			cmc                                                      float64
			commanderLegal, isCommanderCandidate                     bool
			edhrecRank                                               int
		)
		if err := rows.Scan(
			&oracleID,
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&imageURI,
			pq.Array(&colors),
			pq.Array(&colorIdentity),
			&cmc,
			&layout,
			&commanderLegal,
			&isCommanderCandidate,
			&priceUSD,
			&artist,
			&edhrecRank,
			&scryfallURI,
			&setCode,
			&setName,
			&releasedAt,
			&facesJSON,
		); err != nil {
			return nil, err
		}
		out = append(out, scanCanonicalCard(
			oracleID,
			name,
			manaCost,
			typeLine,
			oracleText,
			imageURI,
			colors,
			colorIdentity,
			cmc,
			layout,
			commanderLegal,
			isCommanderCandidate,
			priceUSD,
			artist,
			edhrecRank,
			scryfallURI,
			setCode,
			setName,
			releasedAt,
			facesJSON,
		))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
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

// SearchCards first returns one exact normalized-name match when present.
// If no exact match exists, it returns fuzzy matches ordered by closeness.
func SearchCards(ctx context.Context, db *sql.DB, params CardSearchParams) ([]Card, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 120
	}
	if limit > 300 {
		limit = 300
	}

	query := strings.TrimSpace(params.Query)
	if query == "" {
		filterSQL, filterArgs := buildCardSearchFilters(params, 1)
		sqlText := canonicalCardSelectSQL() + `
			WHERE 1=1` + filterSQL + `
			ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
			LIMIT $` + fmt.Sprint(len(filterArgs)+1)
		args := append(filterArgs, limit)

		rows, err := db.QueryContext(ctx, sqlText, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanCanonicalCards(rows, limit)
	}

	// 1) exact normalized match
	filterSQL, filterArgs := buildCardSearchFilters(params, 2)
	exactSQL := canonicalCardSelectSQL() + `
		WHERE oc.name_search = normalize_card_name($1)` + filterSQL + `
		ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
		LIMIT 1
	`
	exactArgs := append([]any{query}, filterArgs...)
	exactRows, err := db.QueryContext(ctx, exactSQL, exactArgs...)
	if err != nil {
		return nil, err
	}
	exactCards, err := scanCanonicalCards(exactRows, 1)
	exactRows.Close()
	if err != nil {
		return nil, err
	}
	if len(exactCards) > 0 {
		return exactCards, nil
	}

	// 2) fuzzy list
	normalizedQuery := NormalizeName(query)
	if normalizedQuery == "" {
		return nil, nil
	}

	filterSQL, filterArgs = buildCardSearchFilters(params, 2)
	fuzzySQL := canonicalCardSelectSQL() + `
		WHERE oc.name_search % $1` + filterSQL + `
		ORDER BY
			(oc.name_search = $1) DESC,
			(oc.name_search LIKE $1 || '%') DESC,
			similarity(oc.name_search, $1) DESC,
			COALESCE(oc.edhrec_rank, 999999) ASC,
			oc.name ASC
		LIMIT $` + fmt.Sprint(len(filterArgs)+2)
	fuzzyArgs := append([]any{normalizedQuery}, filterArgs...)
	fuzzyArgs = append(fuzzyArgs, limit)

	rows, err := db.QueryContext(ctx, fuzzySQL, fuzzyArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCanonicalCards(rows, limit)
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

// SearchCardNamesExactThenFuzzy powers deck-builder autocomplete style search.
// It returns a single exact match when present, else a fuzzy list.
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
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

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
	`
	args := []any{query}
	if commanderOnly {
		exactSQL += ` AND oc.is_commander_candidate = TRUE`
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
	if len(exactOut) > 0 {
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
		WHERE oc.name_search % $1
	`
	fuzzyArgs := []any{normalizedQuery}
	if commanderOnly {
		fuzzySQL += ` AND oc.is_commander_candidate = TRUE`
	}
	fuzzySQL += `
		ORDER BY
			(oc.name_search = $1) DESC,
			(oc.name_search LIKE $1 || '%') DESC,
			similarity(oc.name_search, $1) DESC,
			COALESCE(oc.edhrec_rank, 999999) ASC,
			oc.name ASC
		LIMIT $2
	`
	fuzzyArgs = append(fuzzyArgs, limit)

	rows, err := db.QueryContext(ctx, fuzzySQL, fuzzyArgs...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	out := make([]DBCard, 0, limit)
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

	return out, false, nil
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

	rows, err := db.QueryContext(ctx, `
		SELECT
			cp.scryfall_id::text AS id,
			oc.oracle_id::text AS oracle_id,
			COALESCE(cp.lang, 'en') AS lang,
			oc.name,
			COALESCE(oc.mana_cost, '') AS mana_cost,
			COALESCE(oc.type_line, '') AS type_line,
			COALESCE(oc.oracle_text, '') AS oracle_text,
			COALESCE(cp.image_uri, '') AS image_uri,
			COALESCE(oc.colors, ARRAY[]::text[]) AS colors,
			COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
			COALESCE(oc.cmc, 0) AS cmc,
			COALESCE(oc.layout, '') AS layout,
			COALESCE(oc.commander_legal, false) AS commander_legal,
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
			)
		FROM card_prints cp
		JOIN oracle_cards oc
		  ON oc.oracle_id = cp.oracle_id
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
		var (
			id, rowOracleID, lang, name, manaCost, typeLine, oracleText, imageURI string
			layout, priceUSD, artist, scryfallURI, setCode, setName               string
			collectorNumber, rarity, releasedAt, facesJSON                        string
			colors, colorIdentity                                                 []string
			cmc                                                                   float64
			commanderLegal                                                        bool
			edhrecRank                                                            int
		)
		if err := rows.Scan(
			&id,
			&rowOracleID,
			&lang,
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&imageURI,
			pq.Array(&colors),
			pq.Array(&colorIdentity),
			&cmc,
			&layout,
			&commanderLegal,
			&priceUSD,
			&artist,
			&edhrecRank,
			&scryfallURI,
			&setCode,
			&setName,
			&collectorNumber,
			&rarity,
			&releasedAt,
			&facesJSON,
		); err != nil {
			return nil, err
		}

		out = append(out, Card{
			ID:              id,
			OracleID:        rowOracleID,
			Lang:            lang,
			Name:            name,
			ManaCost:        manaCost,
			TypeLine:        typeLine,
			OracleText:      oracleText,
			ImageURI:        imageURI,
			Colors:          colors,
			ColorIdentity:   colorIdentity,
			CMC:             cmc,
			Layout:          layout,
			CommanderLegal:  commanderLegal,
			PriceUSD:        priceUSD,
			Artist:          artist,
			EDHRecRank:      edhrecRank,
			ScryfallURI:     scryfallURI,
			SetCode:         setCode,
			SetName:         setName,
			CollectorNumber: collectorNumber,
			Rarity:          rarity,
			ReleasedAt:      releasedAt,
			Faces:           decodeCardFacesJSON(facesJSON),
		})
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
