package cards

import (
	"context"
	"database/sql"
)

type DBCard struct {
	ID   int64
	Name string
}

// EnsureCardByName returns an existing DB card by name or creates it.
func EnsureCardByName(ctx context.Context, db *sql.DB, name string) (*DBCard, error) {
	var c DBCard

	// Try to find it first
	err := db.QueryRowContext(ctx, `
		SELECT id, name
		FROM cards
		WHERE name = $1
	`, name).Scan(&c.ID, &c.Name)
	if err == nil {
		return &c, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Insert if not found
	err = db.QueryRowContext(ctx, `
		INSERT INTO cards (name)
		VALUES ($1)
		RETURNING id, name
	`, name).Scan(&c.ID, &c.Name)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
