package cards

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrCardNotFound = errors.New("card not found")

type DBCard struct {
	ID            int64
	Name          string
	ManaCost      string
	TypeLine      string
	OracleText    string
	ImageURI      string
	ColorIdentity string
	CMC           float64
}

// EnsureCardByName resolves a card from the local cards table by name.
// Runtime card reads are DB-only; bulk sync is responsible for refreshing data.
func EnsureCardByName(ctx context.Context, db *sql.DB, name string) (*DBCard, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrCardNotFound
	}

	var existing DBCard
	err := db.QueryRowContext(ctx, `
		SELECT id, name, COALESCE(mana_cost, ''), COALESCE(type_line, ''), COALESCE(oracle_text, ''), COALESCE(image_uri, ''), COALESCE(color_identity, ''), COALESCE(cmc, 0)
		FROM cards
		WHERE lower(name) = lower($1)
		ORDER BY id
		LIMIT 1
	`, name).Scan(&existing.ID, &existing.Name, &existing.ManaCost, &existing.TypeLine, &existing.OracleText, &existing.ImageURI, &existing.ColorIdentity, &existing.CMC)
	if err == nil {
		return &existing, nil
	}
	if err == sql.ErrNoRows {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, err
	}
	return nil, ErrCardNotFound
}

func EnsureCardsTable(ctx context.Context, db *sql.DB) error {
	// Full cards table definition for new databases.
	_, err := db.ExecContext(ctx, `
	        CREATE TABLE IF NOT EXISTS cards (
	            id BIGSERIAL PRIMARY KEY,
	            name TEXT NOT NULL,
	            mana_cost TEXT,
	            type_line TEXT,
	            oracle_text TEXT,
	            image_uri TEXT,
	            card_faces_json JSONB,
	            colors TEXT,
	            color_identity TEXT,
	            cmc DOUBLE PRECISION,
	            layout TEXT,
	            commander_legal BOOLEAN,
	            price_usd TEXT,
	            artist TEXT,
	            edhrec_rank INTEGER,
	            scryfall_uri TEXT,
	            set_code TEXT,
	            set_name TEXT,
	            scryfall_id TEXT,
	            oracle_id TEXT
	        );
	    `)
	if err != nil {
		return err
	}

	// Existing installs might have older cards schema; keep it forward-compatible.
	alterStmts := []string{
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS card_faces_json JSONB;`,
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

	// Runtime query indexes for DB-backed search/lookups.
	indexStmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_cards_name_lower ON cards (LOWER(name));`,
		`CREATE INDEX IF NOT EXISTS idx_cards_commander_legal ON cards (commander_legal);`,
		`CREATE INDEX IF NOT EXISTS idx_cards_type_line_lower ON cards (LOWER(type_line));`,
		`CREATE INDEX IF NOT EXISTS idx_cards_scryfall_id ON cards (scryfall_id);`,
	}
	for _, stmt := range indexStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	// Single-row metadata table used by the daily bulk sync scheduler.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS card_sync_state (
			id SMALLINT PRIMARY KEY CHECK (id = 1),
			last_attempt_at TIMESTAMPTZ,
			last_success_at TIMESTAMPTZ,
			source_updated_at TIMESTAMPTZ,
			last_error TEXT,
			card_count INTEGER NOT NULL DEFAULT 0
		);
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO card_sync_state (id) VALUES (1)
		ON CONFLICT (id) DO NOTHING;
	`); err != nil {
		return err
	}

	return nil
}
