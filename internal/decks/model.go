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
	Format        string
	CommanderName string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DeckCard struct {
	CardID        string
	CardName      string
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
	var d Deck
	err := db.QueryRowContext(ctx, `
		INSERT INTO decks (user_id, name, description, format, commander_name)
		VALUES ($1, $2, $3, 'commander', $4)
		RETURNING id, user_id, name, description, format, commander_name, created_at, updated_at
	`, userID, name, description, commanderName).
		Scan(&d.ID, &d.UserID, &d.Name, &d.Description, &d.Format, &d.CommanderName, &d.CreatedAt, &d.UpdatedAt)
	return &d, err
}

func ListDecksByUser(ctx context.Context, db *sql.DB, userID int64) ([]Deck, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, name, description, format, commander_name, created_at, updated_at
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
		var d Deck
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Description, &d.Format, &d.CommanderName, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func GetDeck(ctx context.Context, db *sql.DB, id, userID int64) (*Deck, error) {
	var d Deck
	err := db.QueryRowContext(ctx, `
		SELECT id, user_id, name, description, format, commander_name, created_at, updated_at
		FROM decks
		WHERE id = $1 AND user_id = $2
	`, id, userID).
		Scan(&d.ID, &d.UserID, &d.Name, &d.Description, &d.Format, &d.CommanderName, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func UpdateDeck(ctx context.Context, db *sql.DB, deckID int64, name, description, commanderName string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE decks
		SET name = $1,
		    description = $2,
		    commander_name = $3,
		    updated_at = NOW()
		WHERE id = $4
	`, name, description, commanderName, deckID)
	return err
}

func DeleteDeck(ctx context.Context, db *sql.DB, deckID int64) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM decks
		WHERE id = $1
	`, deckID)
	return err
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
			description TEXT,
			format TEXT,
			commander_name TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`); err != nil {
		return err
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
		CREATE INDEX IF NOT EXISTS idx_deck_cards_deck_board
		ON deck_cards (deck_id, board)
	`); err != nil {
		return err
	}

	return nil
}
