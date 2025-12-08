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
	ID       int64
	Name     string
	ImageURI string
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
		SELECT id, name, image_uri
		FROM cards
		WHERE name = $1
	`, name).Scan(&existing.ID, &existing.Name, &existing.ImageURI)
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

	c := results[0]

	// 3) Insert card into DB using fields that match the normalized Card struct.
	colors := ""
	if len(c.Colors) > 0 {
		colors = strings.Join(c.Colors, ",")
	}
	colorIdentity := ""
	if len(c.ColorIdentity) > 0 {
		colorIdentity = strings.Join(c.ColorIdentity, ",")
	}

	var newID int64
	err = db.QueryRowContext(ctx, `
		INSERT INTO cards (
			name,
			mana_cost,
			type_line,
			oracle_text,
			image_uri,
			colors,
			color_identity,
			cmc,
			layout,
			commander_legal,
			price_usd,
			artist,
			edhrec_rank,
			scryfall_uri,
			set_code,
			set_name,
			scryfall_id,
			oracle_id
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18
		)
		RETURNING id
	`,
		c.Name,
		c.ManaCost,
		c.TypeLine,
		c.OracleText,
		c.ImageURI,
		colors,
		colorIdentity,
		c.CMC,
		c.Layout,
		c.CommanderLegal,
		c.PriceUSD,
		c.Artist,
		c.EDHRecRank,
		c.ScryfallURI,
		c.SetCode,
		c.SetName,
		c.ID,
		c.OracleID,
	).Scan(&newID)
	if err != nil {
		return nil, err
	}

	return &DBCard{
		ID:       newID,
		Name:     c.Name,
		ImageURI: c.ImageURI,
	}, nil
}

func EnsureCardsTable(ctx context.Context, db *sql.DB) error {
	// Base table definition (for new databases).
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
	if err != nil {
		return err
	}

	// Add new commander/MDFC-related columns if they don't exist yet.
	alterStmts := []string{
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS colors TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS color_identity TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS cmc DOUBLE PRECISION;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS layout TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS commander_legal BOOLEAN;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS price_usd TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS artist TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS edhrec_rank INTEGER;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS scryfall_uri TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS set_code TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS set_name TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS scryfall_id TEXT;`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS oracle_id TEXT;`,
	}

	for _, stmt := range alterStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
