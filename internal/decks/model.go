package decks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Deck struct {
	ID            int64
	UserID        int64
	Name          string
	Description   string
	Tags          string
	Format        string
	CommanderName string
	IsPublic      bool
	PublicSlug    string
	PublishedAt   *time.Time
	PowerBracket  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DeckInput struct {
	Name          string
	Description   string
	Tags          string
	Format        string
	CommanderName string
	IsPublic      bool
	PublicSlug    string
	PowerBracket  string
}

type DeckCard struct {
	CardID        string
	CardName      string
	ManaCost      string
	ImageURI      string
	TypeLine      string
	OracleText    string
	AllPartsJSON  string
	CMC           float64
	PriceUSD      string
	ColorIdentity string
	Quantity      int
}

func normalizeBoard(board string) string {
	switch strings.ToLower(strings.TrimSpace(board)) {
	case "main":
		return "main"
	case "maybe":
		return "maybe"
	default:
		return ""
	}
}

func adjustDeckCardQty(ctx context.Context, tx *sql.Tx, deckID int64, oracleID, board string, delta int) error {
	if deckID <= 0 {
		return fmt.Errorf("invalid deck id")
	}
	oracleID = strings.TrimSpace(oracleID)
	if oracleID == "" {
		return fmt.Errorf("missing oracle id")
	}
	board = normalizeBoard(board)
	if board == "" {
		return fmt.Errorf("invalid board")
	}

	var currentQty int
	err := tx.QueryRowContext(ctx, `
		SELECT qty
		FROM deck_cards
		WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
	`, deckID, oracleID, board).Scan(&currentQty)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	newQty := currentQty + delta
	if newQty <= 0 {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM deck_cards
			WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
		`, deckID, oracleID, board)
		return err
	}
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO deck_cards (deck_id, oracle_id, qty, board)
			VALUES ($1, $2::uuid, $3, $4)
		`, deckID, oracleID, newQty, board)
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE deck_cards
		SET qty = $4
		WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
	`, deckID, oracleID, board, newQty)
	return err
}

func deckCardQtyInBoard(ctx context.Context, tx *sql.Tx, deckID int64, oracleID, board string) (int, error) {
	oracleID = strings.TrimSpace(oracleID)
	board = normalizeBoard(board)
	if oracleID == "" || board == "" {
		return 0, nil
	}

	var qty int
	err := tx.QueryRowContext(ctx, `
		SELECT qty
		FROM deck_cards
		WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
	`, deckID, oracleID, board).Scan(&qty)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return qty, nil
}

func AddCard(ctx context.Context, db *sql.DB, deckID int64, oracleID string, delta int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "main", delta); err != nil {
		return err
	}
	return tx.Commit()
}

func AddMaybeCard(ctx context.Context, db *sql.DB, deckID int64, oracleID string, delta int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "maybe", delta); err != nil {
		return err
	}
	return tx.Commit()
}

func MoveCardToMaybe(ctx context.Context, db *sql.DB, deckID int64, oracleID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentQty, err := deckCardQtyInBoard(ctx, tx, deckID, oracleID, "main")
	if err != nil {
		return err
	}
	if currentQty <= 0 {
		return tx.Commit()
	}

	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "main", -1); err != nil {
		return err
	}
	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "maybe", 1); err != nil {
		return err
	}
	return tx.Commit()
}

func MoveMaybeToDeck(ctx context.Context, db *sql.DB, deckID int64, oracleID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentQty, err := deckCardQtyInBoard(ctx, tx, deckID, oracleID, "maybe")
	if err != nil {
		return err
	}
	if currentQty <= 0 {
		return tx.Commit()
	}

	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "maybe", -1); err != nil {
		return err
	}
	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "main", 1); err != nil {
		return err
	}
	return tx.Commit()
}

func listDeckCardsByBoard(ctx context.Context, db *sql.DB, deckID int64, board string) ([]DeckCard, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			dc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, ''),
			COALESCE(cp.image_uri, ''),
			COALESCE(oc.type_line, ''),
			COALESCE(oc.oracle_text, ''),
			COALESCE(oc.all_parts::text, '[]'),
			COALESCE(oc.cmc, 0),
			COALESCE(cp.price_usd, ''),
			COALESCE(array_to_string(oc.color_identity, ','), ''),
			dc.qty
		FROM deck_cards dc
		JOIN oracle_cards oc
		  ON oc.oracle_id = dc.oracle_id
		LEFT JOIN card_prints cp
		  ON cp.scryfall_id = COALESCE(dc.preferred_print_id, oc.default_print_id)
		WHERE dc.deck_id = $1
		  AND dc.board = $2
		ORDER BY oc.name
	`, deckID, board)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DeckCard, 0)
	for rows.Next() {
		var dc DeckCard
		if err := rows.Scan(
			&dc.CardID,
			&dc.CardName,
			&dc.ManaCost,
			&dc.ImageURI,
			&dc.TypeLine,
			&dc.OracleText,
			&dc.AllPartsJSON,
			&dc.CMC,
			&dc.PriceUSD,
			&dc.ColorIdentity,
			&dc.Quantity,
		); err != nil {
			return nil, err
		}
		out = append(out, dc)
	}
	return out, rows.Err()
}

func ListDeckCards(ctx context.Context, db *sql.DB, deckID int64) ([]DeckCard, error) {
	return listDeckCardsByBoard(ctx, db, deckID, "main")
}

func ListDeckMaybeCards(ctx context.Context, db *sql.DB, deckID int64) ([]DeckCard, error) {
	return listDeckCardsByBoard(ctx, db, deckID, "maybe")
}

func CreateDeck(ctx context.Context, db *sql.DB, userID int64, name, description, commanderName string) (*Deck, error) {
	return CreateDeckWithOptions(ctx, db, userID, DeckInput{
		Name:          name,
		Description:   description,
		Tags:          "",
		Format:        "Commander",
		CommanderName: commanderName,
	})
}

func ListDecksByUser(ctx context.Context, db *sql.DB, userID int64) ([]Deck, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Deck
	for rows.Next() {
		d, err := scanDeck(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func GetDeck(ctx context.Context, db *sql.DB, id, userID int64) (*Deck, error) {
	return scanDeck(db.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan)
}

func UpdateDeck(ctx context.Context, db *sql.DB, deckID int64, name, description, commanderName string) error {
	return UpdateDeckWithOptions(ctx, db, deckID, DeckInput{
		Name:          name,
		Description:   description,
		Tags:          "",
		Format:        "Commander",
		CommanderName: commanderName,
	})
}

func DeleteDeck(ctx context.Context, db *sql.DB, deckID int64) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM decks
		WHERE id = $1
	`, deckID)
	return err
}

var supportedDeckTags = []string{
	"Aggro",
	"Midrange",
	"Control",
	"Combo",
	"Ramp",
	"Stax",
	"Aristocrats",
	"Spellslinger",
	"Tokens",
	"Reanimator",
	"Voltron",
	"Tribal",
}

func SupportedDeckTags() []string {
	out := make([]string, len(supportedDeckTags))
	copy(out, supportedDeckTags)
	return out
}

func NormalizeDeckTag(raw string) string {
	switch strings.ToLower(normalizeLooseLabel(raw)) {
	case "aggro":
		return "Aggro"
	case "midrange":
		return "Midrange"
	case "control":
		return "Control"
	case "combo":
		return "Combo"
	case "ramp":
		return "Ramp"
	case "stax":
		return "Stax"
	case "aristocrats":
		return "Aristocrats"
	case "spellslinger":
		return "Spellslinger"
	case "tokens":
		return "Tokens"
	case "reanimator":
		return "Reanimator"
	case "voltron":
		return "Voltron"
	case "tribal":
		return "Tribal"
	default:
		return ""
	}
}

func SplitTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})

	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		tag := NormalizeDeckTag(field)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func NormalizeTags(raw string) string {
	return strings.Join(SplitTags(raw), ", ")
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func tableHasColumn(ctx context.Context, db *sql.DB, tableName, columnName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = $1
			  AND column_name = $2
		)
	`, tableName, columnName).Scan(&exists)
	return exists, err
}

func createDeckCardsV2(ctx context.Context, db *sql.DB, tableName string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
			oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE RESTRICT,
			qty INT NOT NULL DEFAULT 0,
			board TEXT NOT NULL DEFAULT 'main',
			preferred_print_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
			PRIMARY KEY (deck_id, oracle_id, board),
			CHECK (board IN ('main', 'maybe'))
		);
	`, tableName))
	return err
}

func migrateLegacyDeckCards(ctx context.Context, db *sql.DB) error {
	if err := createDeckCardsV2(ctx, db, "deck_cards_v2"); err != nil {
		return err
	}

	oldCardsExists, err := tableExists(ctx, db, "cards")
	if err != nil {
		return err
	}
	if oldCardsExists {
		_, _ = db.ExecContext(ctx, `
			INSERT INTO deck_cards_v2 (deck_id, oracle_id, qty, board)
			SELECT
				dc.deck_id,
				c.oracle_id::uuid,
				dc.quantity,
				'main'
			FROM deck_cards dc
			JOIN cards c
			  ON c.id = dc.card_id
			WHERE dc.quantity > 0
			  AND COALESCE(c.oracle_id, '') <> ''
			  AND c.oracle_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
			ON CONFLICT (deck_id, oracle_id, board)
			DO UPDATE SET qty = deck_cards_v2.qty + EXCLUDED.qty
		`)

		maybeExists, err := tableExists(ctx, db, "deck_maybe_cards")
		if err != nil {
			return err
		}
		if maybeExists {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO deck_cards_v2 (deck_id, oracle_id, qty, board)
				SELECT
					dmc.deck_id,
					c.oracle_id::uuid,
					dmc.quantity,
					'maybe'
				FROM deck_maybe_cards dmc
				JOIN cards c
				  ON c.id = dmc.card_id
				WHERE dmc.quantity > 0
				  AND COALESCE(c.oracle_id, '') <> ''
				  AND c.oracle_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
				ON CONFLICT (deck_id, oracle_id, board)
				DO UPDATE SET qty = deck_cards_v2.qty + EXCLUDED.qty
			`)
		}
	}

	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS deck_maybe_cards`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS deck_cards`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards_v2 RENAME TO deck_cards`); err != nil {
		return err
	}
	return nil
}

func EnsureDeckTables(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS decks (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '',
			format TEXT NOT NULL DEFAULT 'Commander',
			commander_name TEXT,
			is_public BOOLEAN NOT NULL DEFAULT FALSE,
			public_slug TEXT,
			published_at TIMESTAMPTZ,
			power_bracket TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`); err != nil {
		return err
	}

	for _, stmt := range []string{
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS tags TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS format TEXT`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS commander_name TEXT`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS public_slug TEXT`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS power_bracket TEXT NOT NULL DEFAULT ''`,
		`UPDATE decks SET description = '' WHERE description IS NULL`,
		`UPDATE decks SET tags = '' WHERE tags IS NULL`,
		`UPDATE decks SET format = 'Commander' WHERE COALESCE(btrim(format), '') = ''`,
		`ALTER TABLE decks ALTER COLUMN format SET DEFAULT 'Commander'`,
		`ALTER TABLE decks ALTER COLUMN format SET NOT NULL`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	deckCardsExists, err := tableExists(ctx, db, "deck_cards")
	if err != nil {
		return err
	}
	if deckCardsExists {
		hasLegacyCardID, err := tableHasColumn(ctx, db, "deck_cards", "card_id")
		if err != nil {
			return err
		}
		if hasLegacyCardID {
			if err := migrateLegacyDeckCards(ctx, db); err != nil {
				return err
			}
		}
	}

	if err := createDeckCardsV2(ctx, db, "deck_cards"); err != nil {
		return err
	}
	// Keep older installations with "quantity" compatible.
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards ADD COLUMN IF NOT EXISTS qty INT NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards ADD COLUMN IF NOT EXISTS board TEXT NOT NULL DEFAULT 'main'`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards ADD COLUMN IF NOT EXISTS preferred_print_id UUID NULL`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'deck_cards_board_check'
			) THEN
				ALTER TABLE deck_cards
				ADD CONSTRAINT deck_cards_board_check CHECK (board IN ('main', 'maybe'));
			END IF;
		END $$;
	`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_decks_user_updated
		ON decks (user_id, updated_at DESC)
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_decks_public_published
		ON decks (is_public, published_at DESC, updated_at DESC)
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_decks_public_slug_unique
		ON decks (public_slug)
		WHERE public_slug IS NOT NULL
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_deck_cards_deck_board
		ON deck_cards (deck_id, board)
	`); err != nil {
		return err
	}

	return nil
}

func scanDeck(scan func(dest ...any) error) (*Deck, error) {
	var (
		d           Deck
		publishedAt sql.NullTime
	)
	if err := scan(
		&d.ID,
		&d.UserID,
		&d.Name,
		&d.Description,
		&d.Tags,
		&d.Format,
		&d.CommanderName,
		&d.IsPublic,
		&d.PublicSlug,
		&publishedAt,
		&d.PowerBracket,
		&d.CreatedAt,
		&d.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if publishedAt.Valid {
		t := publishedAt.Time
		d.PublishedAt = &t
	}
	return &d, nil
}

func CreateDeckWithOptions(ctx context.Context, db *sql.DB, userID int64, input DeckInput) (*Deck, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	d, err := insertDeckTx(ctx, tx, userID, input)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d, nil
}

func UpdateDeckWithOptions(ctx context.Context, db *sql.DB, deckID int64, input DeckInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	current, err := scanDeck(tx.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE id = $1
	`, deckID).Scan)
	if err != nil {
		return err
	}

	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	tags := NormalizeTags(input.Tags)
	format := NormalizeFormat(input.Format)
	commanderName := strings.TrimSpace(input.CommanderName)
	isPublic := input.IsPublic
	powerBracket := NormalizePowerBracket(input.PowerBracket)
	publicSlug := NormalizePublicSlug(input.PublicSlug)
	if isPublic && publicSlug == "" {
		publicSlug, err = reserveUniquePublicSlugTx(ctx, tx, deckID, name, "")
		if err != nil {
			return err
		}
	}

	var publishedAt any
	if isPublic {
		if current.PublishedAt != nil {
			publishedAt = *current.PublishedAt
		} else {
			publishedAt = time.Now().UTC()
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE decks
		SET name = $1,
		    description = $2,
		    tags = $3,
		    format = $4,
		    commander_name = $5,
		    is_public = $6,
		    public_slug = NULLIF($7, ''),
		    published_at = $8,
		    power_bracket = $9,
		    updated_at = NOW()
		WHERE id = $10
	`, name, description, tags, format, commanderName, isPublic, publicSlug, publishedAt, powerBracket, deckID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func insertDeckTx(ctx context.Context, tx *sql.Tx, userID int64, input DeckInput) (*Deck, error) {
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	tags := NormalizeTags(input.Tags)
	format := NormalizeFormat(input.Format)
	commanderName := strings.TrimSpace(input.CommanderName)
	if !FormatRequiresCommander(format) {
		commanderName = strings.TrimSpace(input.CommanderName)
	}
	powerBracket := NormalizePowerBracket(input.PowerBracket)

	d, err := scanDeck(tx.QueryRowContext(ctx, `
		INSERT INTO decks (
			user_id,
			name,
			description,
			tags,
			format,
			commander_name,
			is_public,
			public_slug,
			published_at,
			power_bracket
		)
		VALUES ($1, $2, $3, $4, $5, $6, FALSE, NULL, NULL, $7)
		RETURNING
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
	`, userID, name, description, tags, format, commanderName, powerBracket).Scan)
	if err != nil {
		return nil, err
	}

	if !input.IsPublic {
		return d, nil
	}

	slug, err := reserveUniquePublicSlugTx(ctx, tx, d.ID, name, input.PublicSlug)
	if err != nil {
		return nil, err
	}
	publishedAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE decks
		SET is_public = TRUE,
		    public_slug = $2,
		    published_at = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, d.ID, slug, publishedAt); err != nil {
		return nil, err
	}

	d.IsPublic = true
	d.PublicSlug = slug
	d.PublishedAt = &publishedAt
	return d, nil
}
