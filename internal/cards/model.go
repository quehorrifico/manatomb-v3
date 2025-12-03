package cards

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrCardNotFound = errors.New("card not found")

type DBCard struct {
	ID   int64
	Name string
}

// EnsureCardByName ensures that the card exists in our DB.
// 1) Try to find it by exact name in the cards table.
// 2) If not found, query Scryfall using an exact-name search.
// 3) If Scryfall returns no results, return ErrCardNotFound.
// 4) If found, insert a full row and return the DBCard.
func EnsureCardByName(ctx context.Context, db *sql.DB, name string) (*DBCard, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrCardNotFound
	}

	// 1) Try to find card already stored in DB by exact name
	var existing DBCard
	err := db.QueryRowContext(ctx, `
		SELECT id, name
		FROM cards
		WHERE name = $1
	`, name).Scan(&existing.ID, &existing.Name)
	if err == nil {
		return &existing, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// 2) Not in DB → search Scryfall using exact-name search: !"Card Name"
	scry := NewScryfallClient()
	query := fmt.Sprintf(`!"%s"`, name)

	results, err := scry.SearchByName(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		// Do NOT insert junk; the name isn't a real Scryfall card.
		return nil, ErrCardNotFound
	}

	c := results[0] // Card from your scryfall.go: Name, ManaCost, TypeLine, OracleText, ImageURI

	// 3) Insert card into DB using fields that match your Card struct.
	var newID int64
	err = db.QueryRowContext(ctx, `
		INSERT INTO cards (name, mana_cost, type_line, oracle_text, image_uri)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, c.Name, c.ManaCost, c.TypeLine, c.OracleText, c.ImageURI).Scan(&newID)
	if err != nil {
		return nil, err
	}

	return &DBCard{
		ID:   newID,
		Name: c.Name,
	}, nil
}

func EnsureCardsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS cards (
            id BIGSERIAL PRIMARY KEY,
            name TEXT NOT NULL,
            mana_cost TEXT,
            type_line TEXT,
            oracle_text TEXT,
            image_uri TEXT
        );
    `)
	return err
}
