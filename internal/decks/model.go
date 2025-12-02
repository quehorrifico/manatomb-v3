package decks

import (
	"context"
	"database/sql"
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
	CardID   int64
	CardName string
	Quantity int
}

func AddCard(ctx context.Context, db *sql.DB, deckID int64, cardID int64, delta int) error {
	// delta can be +1 or -1 for now

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentQty int
	err = tx.QueryRowContext(ctx, `
		SELECT quantity
		FROM deck_cards
		WHERE deck_id = $1 AND card_id = $2
	`, deckID, cardID).Scan(&currentQty)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	newQty := currentQty + delta
	if newQty <= 0 {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM deck_cards
			WHERE deck_id = $1 AND card_id = $2
		`, deckID, cardID)
	} else if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO deck_cards (deck_id, card_id, quantity)
			VALUES ($1, $2, $3)
		`, deckID, cardID, newQty)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE deck_cards
			SET quantity = $3
			WHERE deck_id = $1 AND card_id = $2
		`, deckID, cardID, newQty)
	}

	if err != nil {
		return err
	}

	return tx.Commit()
}

func ListDeckCards(ctx context.Context, db *sql.DB, deckID int64) ([]DeckCard, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT dc.card_id, c.name, dc.quantity
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
		if err := rows.Scan(&dc.CardID, &dc.CardName, &dc.Quantity); err != nil {
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

	return nil
}
