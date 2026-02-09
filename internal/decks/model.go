package decks

import (
	"context"
	"database/sql"
	"fmt"
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
	CardID        int64
	CardName      string
	ImageURI      string
	TypeLine      string
	OracleText    string
	CMC           float64
	PriceUSD      string
	ColorIdentity string
	Quantity      int
}

func adjustDeckCardQty(ctx context.Context, tx *sql.Tx, table string, deckID int64, cardID int64, delta int) error {
	var currentQty int
	err := tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT quantity
		FROM %s
		WHERE deck_id = $1 AND card_id = $2
	`, table), deckID, cardID).Scan(&currentQty)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	newQty := currentQty + delta
	if newQty <= 0 {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			DELETE FROM %s
			WHERE deck_id = $1 AND card_id = $2
		`, table), deckID, cardID)
	} else if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO %s (deck_id, card_id, quantity)
			VALUES ($1, $2, $3)
		`, table), deckID, cardID, newQty)
	} else {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s
			SET quantity = $3
			WHERE deck_id = $1 AND card_id = $2
		`, table), deckID, cardID, newQty)
	}

	return err
}

func deckCardQtyInTable(ctx context.Context, tx *sql.Tx, table string, deckID int64, cardID int64) (int, error) {
	var qty int
	err := tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT quantity
		FROM %s
		WHERE deck_id = $1 AND card_id = $2
	`, table), deckID, cardID).Scan(&qty)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return qty, nil
}

func AddCard(ctx context.Context, db *sql.DB, deckID int64, cardID int64, delta int) error {
	// delta can be +1 or -1 for now

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = adjustDeckCardQty(ctx, tx, "deck_cards", deckID, cardID, delta)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func AddMaybeCard(ctx context.Context, db *sql.DB, deckID int64, cardID int64, delta int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = adjustDeckCardQty(ctx, tx, "deck_maybe_cards", deckID, cardID, delta)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func MoveCardToMaybe(ctx context.Context, db *sql.DB, deckID int64, cardID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentQty, err := deckCardQtyInTable(ctx, tx, "deck_cards", deckID, cardID)
	if err != nil {
		return err
	}
	if currentQty <= 0 {
		return tx.Commit()
	}

	if err := adjustDeckCardQty(ctx, tx, "deck_cards", deckID, cardID, -1); err != nil {
		return err
	}
	if err := adjustDeckCardQty(ctx, tx, "deck_maybe_cards", deckID, cardID, 1); err != nil {
		return err
	}

	return tx.Commit()
}

func MoveMaybeToDeck(ctx context.Context, db *sql.DB, deckID int64, cardID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentQty, err := deckCardQtyInTable(ctx, tx, "deck_maybe_cards", deckID, cardID)
	if err != nil {
		return err
	}
	if currentQty <= 0 {
		return tx.Commit()
	}

	if err := adjustDeckCardQty(ctx, tx, "deck_maybe_cards", deckID, cardID, -1); err != nil {
		return err
	}
	if err := adjustDeckCardQty(ctx, tx, "deck_cards", deckID, cardID, 1); err != nil {
		return err
	}

	return tx.Commit()
}

func ListDeckCards(ctx context.Context, db *sql.DB, deckID int64) ([]DeckCard, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT dc.card_id, c.name, COALESCE(c.image_uri, ''), COALESCE(c.type_line, ''), COALESCE(c.oracle_text, ''), COALESCE(c.cmc, 0), COALESCE(c.price_usd, ''), COALESCE(c.color_identity, ''), dc.quantity
		FROM deck_cards dc
		JOIN cards c ON c.id = dc.card_id
		WHERE dc.deck_id = $1
		ORDER BY c.name
	`, deckID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DeckCard
	for rows.Next() {
		var dc DeckCard
		if err := rows.Scan(&dc.CardID, &dc.CardName, &dc.ImageURI, &dc.TypeLine, &dc.OracleText, &dc.CMC, &dc.PriceUSD, &dc.ColorIdentity, &dc.Quantity); err != nil {
			return nil, err
		}
		out = append(out, dc)
	}
	return out, rows.Err()
}

func ListDeckMaybeCards(ctx context.Context, db *sql.DB, deckID int64) ([]DeckCard, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT dc.card_id, c.name, COALESCE(c.image_uri, ''), COALESCE(c.type_line, ''), COALESCE(c.oracle_text, ''), COALESCE(c.cmc, 0), COALESCE(c.price_usd, ''), COALESCE(c.color_identity, ''), dc.quantity
		FROM deck_maybe_cards dc
		JOIN cards c ON c.id = dc.card_id
		WHERE dc.deck_id = $1
		ORDER BY c.name
	`, deckID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DeckCard
	for rows.Next() {
		var dc DeckCard
		if err := rows.Scan(&dc.CardID, &dc.CardName, &dc.ImageURI, &dc.TypeLine, &dc.OracleText, &dc.CMC, &dc.PriceUSD, &dc.ColorIdentity, &dc.Quantity); err != nil {
			return nil, err
		}
		out = append(out, dc)
	}
	return out, rows.Err()
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

func EnsureDeckTables(ctx context.Context, db *sql.DB) error {
	// Decks table
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

	// Deck cards table
	if _, err := db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS deck_cards (
            deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
            card_id BIGINT NOT NULL,
            quantity INT NOT NULL DEFAULT 0,
            PRIMARY KEY (deck_id, card_id)
        );
    `); err != nil {
		return err
	}

	// Deck maybeboard cards table
	if _, err := db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS deck_maybe_cards (
            deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
            card_id BIGINT NOT NULL,
            quantity INT NOT NULL DEFAULT 0,
            PRIMARY KEY (deck_id, card_id)
        );
    `); err != nil {
		return err
	}

	return nil
}
