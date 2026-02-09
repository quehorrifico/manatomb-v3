package cards

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
)

type CardSearchParams struct {
	Query         string
	TypeFilter    string
	ColorIdentity []string
	CommanderOnly bool
	Limit         int
}

func normalizeCSV(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
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

func scanCardFromRow(
	name, manaCost, typeLine, oracleText, imageURI, colors, colorIdentity string,
	cmc float64,
	layout string,
	commanderLegal bool,
	priceUSD, artist string,
	edhrecRank int,
	scryfallURI, setCode, setName, scryfallID, oracleID, facesJSON string,
) Card {
	return Card{
		ID:             scryfallID,
		OracleID:       oracleID,
		Name:           name,
		ManaCost:       manaCost,
		TypeLine:       typeLine,
		OracleText:     oracleText,
		ImageURI:       imageURI,
		Colors:         normalizeCSV(colors),
		ColorIdentity:  normalizeCSV(colorIdentity),
		CMC:            cmc,
		Layout:         layout,
		CommanderLegal: commanderLegal,
		PriceUSD:       priceUSD,
		Artist:         artist,
		EDHRecRank:     edhrecRank,
		ScryfallURI:    scryfallURI,
		SetCode:        setCode,
		SetName:        setName,
		Faces:          decodeCardFacesJSON(facesJSON),
	}
}

func GetCardByName(ctx context.Context, db *sql.DB, name string) (*Card, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrCardNotFound
	}

	var (
		rowName, manaCost, typeLine, oracleText, imageURI, colors, colorIdentity      string
		layout, priceUSD, artist, scryfallURI, setCode, setName, scryfallID, oracleID string
		facesJSON                                                                     string
		cmc                                                                           float64
		commanderLegal                                                                bool
		edhrecRank                                                                    int
	)

	err := db.QueryRowContext(ctx, `
		SELECT
			name,
			COALESCE(mana_cost, ''),
			COALESCE(type_line, ''),
			COALESCE(oracle_text, ''),
			COALESCE(image_uri, ''),
			COALESCE(colors, ''),
			COALESCE(color_identity, ''),
			COALESCE(cmc, 0),
			COALESCE(layout, ''),
			COALESCE(commander_legal, false),
			COALESCE(price_usd, ''),
			COALESCE(artist, ''),
			COALESCE(edhrec_rank, 0),
			COALESCE(scryfall_uri, ''),
			COALESCE(set_code, ''),
			COALESCE(set_name, ''),
			COALESCE(scryfall_id, ''),
			COALESCE(oracle_id, ''),
			COALESCE(card_faces_json::text, '')
		FROM cards
		WHERE lower(name) = lower($1)
		ORDER BY id
		LIMIT 1
	`, name).Scan(
		&rowName,
		&manaCost,
		&typeLine,
		&oracleText,
		&imageURI,
		&colors,
		&colorIdentity,
		&cmc,
		&layout,
		&commanderLegal,
		&priceUSD,
		&artist,
		&edhrecRank,
		&scryfallURI,
		&setCode,
		&setName,
		&scryfallID,
		&oracleID,
		&facesJSON,
	)
	if err == sql.ErrNoRows {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, err
	}

	card := scanCardFromRow(
		rowName,
		manaCost,
		typeLine,
		oracleText,
		imageURI,
		colors,
		colorIdentity,
		cmc,
		layout,
		commanderLegal,
		priceUSD,
		artist,
		edhrecRank,
		scryfallURI,
		setCode,
		setName,
		scryfallID,
		oracleID,
		facesJSON,
	)
	return &card, nil
}

func SearchCards(ctx context.Context, db *sql.DB, params CardSearchParams) ([]Card, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 120
	}
	if limit > 300 {
		limit = 300
	}

	query := strings.TrimSpace(params.Query)
	typeFilter := strings.TrimSpace(params.TypeFilter)
	colorFilters := normalizeColorIdentityFilters(params.ColorIdentity)

	var sb strings.Builder
	sb.WriteString(`
		SELECT
			name,
			COALESCE(mana_cost, ''),
			COALESCE(type_line, ''),
			COALESCE(oracle_text, ''),
			COALESCE(image_uri, ''),
			COALESCE(colors, ''),
			COALESCE(color_identity, ''),
			COALESCE(cmc, 0),
			COALESCE(layout, ''),
			COALESCE(commander_legal, false),
			COALESCE(price_usd, ''),
			COALESCE(artist, ''),
			COALESCE(edhrec_rank, 0),
			COALESCE(scryfall_uri, ''),
			COALESCE(set_code, ''),
			COALESCE(set_name, ''),
			COALESCE(scryfall_id, ''),
			COALESCE(oracle_id, ''),
			COALESCE(card_faces_json::text, '')
		FROM cards
		WHERE 1=1
	`)

	args := make([]any, 0, 8)
	argN := 1

	if query != "" {
		sb.WriteString(" AND name ILIKE '%' || $" + intPlaceholder(argN) + " || '%'")
		args = append(args, query)
		argN++
	}
	if typeFilter != "" {
		sb.WriteString(" AND type_line ILIKE '%' || $" + intPlaceholder(argN) + " || '%'")
		args = append(args, typeFilter)
		argN++
	}
	if params.CommanderOnly {
		sb.WriteString(" AND commander_legal = TRUE")
	}
	for _, color := range colorFilters {
		sb.WriteString(" AND color_identity ILIKE '%' || $" + intPlaceholder(argN) + " || '%'")
		args = append(args, color)
		argN++
	}

	sb.WriteString(" ORDER BY name ASC")
	sb.WriteString(" LIMIT $" + intPlaceholder(argN))
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Card, 0, limit)
	for rows.Next() {
		var (
			name, manaCost, typeLine, oracleText, imageURI, colors, colorIdentity         string
			layout, priceUSD, artist, scryfallURI, setCode, setName, scryfallID, oracleID string
			facesJSON                                                                     string
			cmc                                                                           float64
			commanderLegal                                                                bool
			edhrecRank                                                                    int
		)
		if err := rows.Scan(
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&imageURI,
			&colors,
			&colorIdentity,
			&cmc,
			&layout,
			&commanderLegal,
			&priceUSD,
			&artist,
			&edhrecRank,
			&scryfallURI,
			&setCode,
			&setName,
			&scryfallID,
			&oracleID,
			&facesJSON,
		); err != nil {
			return nil, err
		}

		out = append(out, scanCardFromRow(
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
			priceUSD,
			artist,
			edhrecRank,
			scryfallURI,
			setCode,
			setName,
			scryfallID,
			oracleID,
			facesJSON,
		))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func intPlaceholder(n int) string {
	return strconv.Itoa(n)
}
