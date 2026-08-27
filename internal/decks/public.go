package decks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	"github.com/lib/pq"
)

type PublicDeckFilters struct {
	DeckName      string
	CommanderName string
	Format        string
	PowerBracket  string
	Archetypes    []string
	ColorIdentity []string
	ColorMode     string
	Sort          string
	Limit         int
	Offset        int
}

func NormalizePublicDeckArchetypes(archetypes []string) []string {
	selected := make(map[string]bool, len(archetypes))
	for _, raw := range archetypes {
		if archetype := NormalizeDeckTag(raw); archetype != "" {
			selected[archetype] = true
		}
	}

	supported := SupportedDeckTags()
	out := make([]string, 0, len(selected))
	for _, archetype := range supported {
		if selected[archetype] {
			out = append(out, archetype)
		}
	}
	return out
}

func NormalizePublicDeckColors(colors []string) []string {
	seen := make(map[string]bool, len(colors))
	for _, color := range colors {
		switch color = strings.ToUpper(strings.TrimSpace(color)); color {
		case "W", "U", "B", "R", "G", "C":
			seen[color] = true
		}
	}

	order := []string{"W", "U", "B", "R", "G"}
	out := make([]string, 0, len(order))
	for _, color := range order {
		if seen[color] {
			out = append(out, color)
		}
	}
	if len(out) == 0 && seen["C"] {
		return []string{"C"}
	}
	return out
}

func NormalizePublicDeckColorMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "exact":
		return "exact"
	case "at_most":
		return "at_most"
	default:
		return "includes"
	}
}

func NormalizePublicDeckSort(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "updated":
		return "updated"
	case "name":
		return "name"
	case "commander":
		return "commander"
	case "oldest":
		return "oldest"
	default:
		return "recent"
	}
}

func NormalizePublicSlug(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func ListPublicDecks(ctx context.Context, db *sql.DB, filters PublicDeckFilters) ([]Deck, error) {
	query, args := buildPublicDeckListQuery(filters)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	limit := normalizedPublicDeckLimit(filters.Limit)
	out := make([]Deck, 0, limit)
	for rows.Next() {
		d, err := scanDeck(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func normalizedPublicDeckLimit(raw int) int {
	limit := raw
	if limit <= 0 {
		limit = 60
	}
	if limit > 200 {
		limit = 200
	}
	return limit
}

func normalizedPublicDeckOffset(raw int) int {
	if raw < 0 {
		return 0
	}
	return raw
}

func publicDeckOrderBySQL(raw string) string {
	switch NormalizePublicDeckSort(raw) {
	case "updated":
		return "d.updated_at DESC, d.id DESC"
	case "name":
		return "lower(d.name) ASC, d.id ASC"
	case "commander":
		return "lower(COALESCE(d.commander_name, '')) ASC, lower(d.name) ASC, d.id ASC"
	case "oldest":
		return "d.published_at ASC NULLS LAST, d.updated_at ASC, d.id ASC"
	default:
		return "d.published_at DESC NULLS LAST, d.updated_at DESC, d.id DESC"
	}
}

func buildPublicDeckListQuery(filters PublicDeckFilters) (string, []any) {
	limit := filters.Limit
	limit = normalizedPublicDeckLimit(limit)

	args := make([]any, 0, 18)
	clauses := []string{"d.is_public = TRUE"}
	argN := 1

	if deckName := strings.TrimSpace(filters.DeckName); deckName != "" {
		clauses = append(clauses, "d.name ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
		args = append(args, deckName)
		argN++
	}
	if commander := strings.TrimSpace(filters.CommanderName); commander != "" {
		clauses = append(clauses, "d.commander_name ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
		args = append(args, commander)
		argN++
	}
	if rawFormat := strings.TrimSpace(filters.Format); rawFormat != "" {
		format := NormalizeFormat(rawFormat)
		clauses = append(clauses, "d.format = $"+fmt.Sprint(argN))
		args = append(args, format)
		argN++
	}
	if powerLabels := powerBracketFilterLabels(filters.PowerBracket); len(powerLabels) > 0 {
		placeholders := make([]string, 0, len(powerLabels))
		for _, label := range powerLabels {
			placeholders = append(placeholders, "$"+fmt.Sprint(argN))
			args = append(args, label)
			argN++
		}
		clauses = append(clauses, "d.power_bracket IN ("+strings.Join(placeholders, ", ")+")")
	}
	if archetypes := NormalizePublicDeckArchetypes(filters.Archetypes); len(archetypes) > 0 {
		placeholder := "$" + fmt.Sprint(argN) + "::text[]"
		clauses = append(clauses, "EXISTS (SELECT 1 FROM unnest(string_to_array(COALESCE(d.tags, ''), ',')) AS stored_tag(tag), unnest("+placeholder+") AS selected_tag(tag) WHERE lower(btrim(stored_tag.tag)) = lower(selected_tag.tag))")
		args = append(args, pq.Array(archetypes))
		argN++
	}
	colors := NormalizePublicDeckColors(filters.ColorIdentity)
	if len(colors) == 1 && colors[0] == "C" {
		clauses = append(clauses, "COALESCE(array_length(oc.color_identity, 1), 0) = 0")
	} else if len(colors) > 0 {
		identity := "COALESCE(oc.color_identity, ARRAY[]::text[])"
		placeholder := "$" + fmt.Sprint(argN) + "::text[]"
		switch NormalizePublicDeckColorMode(filters.ColorMode) {
		case "exact":
			clauses = append(clauses, identity+" @> "+placeholder, identity+" <@ "+placeholder)
		case "at_most":
			clauses = append(clauses, identity+" <@ "+placeholder)
		default:
			clauses = append(clauses, identity+" @> "+placeholder)
		}
		args = append(args, pq.Array(colors))
		argN++
	}

	query := `
		SELECT
			d.id,
			d.user_id,
			d.name,
			COALESCE(d.description, ''),
			COALESCE(d.tags, ''),
			COALESCE(d.format, 'Commander'),
			COALESCE(d.commander_name, ''),
			COALESCE(d.commander_print_id::text, ''),
			COALESCE(d.is_public, FALSE),
			COALESCE(d.public_slug, ''),
			d.published_at,
			COALESCE(d.power_bracket, ''),
			d.created_at,
			d.updated_at
		FROM decks d
		LEFT JOIN oracle_cards oc
		  ON oc.name_search = normalize_card_name(COALESCE(d.commander_name, ''))
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY ` + publicDeckOrderBySQL(filters.Sort) + `
		LIMIT $` + fmt.Sprint(argN) + `
		OFFSET $` + fmt.Sprint(argN+1)
	args = append(args, limit, normalizedPublicDeckOffset(filters.Offset))
	return query, args
}

func ListPublicDecksByUser(ctx context.Context, db *sql.DB, userID int64, limit int) ([]Deck, error) {
	if userID <= 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 60
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := db.QueryContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(commander_print_id::text, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE user_id = $1
		  AND is_public = TRUE
		ORDER BY published_at DESC NULLS LAST, updated_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Deck, 0, limit)
	for rows.Next() {
		d, err := scanDeck(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func GetPublicDeckBySlug(ctx context.Context, db *sql.DB, slug string) (*Deck, error) {
	slug = NormalizePublicSlug(slug)
	if slug == "" {
		return nil, sql.ErrNoRows
	}

	return scanDeck(db.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(commander_print_id::text, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE is_public = TRUE
		  AND public_slug = $1
	`, slug).Scan)
}

func ForkDeckToUser(ctx context.Context, db *sql.DB, publicSlug string, userID int64) (*Deck, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	source, err := scanDeck(tx.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(commander_print_id::text, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE is_public = TRUE
		  AND public_slug = $1
	`, NormalizePublicSlug(publicSlug)).Scan)
	if err != nil {
		return nil, err
	}

	newDeck, err := insertDeckTx(ctx, tx, userID, DeckInput{
		Name:             "Copy of " + source.Name,
		Description:      source.Description,
		Tags:             source.Tags,
		Format:           source.Format,
		CommanderName:    source.CommanderName,
		CommanderPrintID: source.CommanderPrintID,
		IsPublic:         false,
		PowerBracket:     source.PowerBracket,
	})
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO deck_cards (deck_id, oracle_id, qty, board, preferred_print_id)
		SELECT $1, oracle_id, qty, board, preferred_print_id
		FROM deck_cards
		WHERE deck_id = $2
	`, newDeck.ID, source.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return newDeck, nil
}

func reserveUniquePublicSlugTx(ctx context.Context, tx *sql.Tx, deckID int64, fallbackName, rawSlug string) (string, error) {
	base := NormalizePublicSlug(rawSlug)
	if base == "" {
		base = NormalizePublicSlug(fallbackName)
	}
	if base == "" {
		base = "deck"
	}

	candidates := []string{base}
	if deckID > 0 {
		candidates = append(candidates, fmt.Sprintf("%s-%d", base, deckID))
	}
	for i := 2; i <= 25; i++ {
		candidates = append(candidates, fmt.Sprintf("%s-%d", base, i))
	}

	for _, candidate := range candidates {
		var existingID int64
		err := tx.QueryRowContext(ctx, `
			SELECT id
			FROM decks
			WHERE public_slug = $1
			  AND id <> $2
			LIMIT 1
		`, candidate, deckID).Scan(&existingID)
		if err == sql.ErrNoRows {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%s-%d", base, deckID), nil
}
