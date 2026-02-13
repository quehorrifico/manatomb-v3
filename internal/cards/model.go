package cards

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode"

	"github.com/lib/pq"
)

var ErrCardNotFound = errors.New("card not found")

type DBCard struct {
	OracleID             string
	Name                 string
	ManaCost             string
	TypeLine             string
	OracleText           string
	ImageURI             string
	ColorIdentity        string
	CMC                  float64
	IsCommanderCandidate bool
}

// NormalizeName normalizes user-entered card names into a search-friendly form.
// DB-side normalization also applies unaccent; this function keeps local
// normalization consistent for batching and map keys.
func NormalizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(value))
	lastWasSpace := true
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastWasSpace = false
			continue
		}
		if !lastWasSpace {
			b.WriteByte(' ')
			lastWasSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func scanDBCardRow(
	oracleID, name, manaCost, typeLine, oracleText, imageURI string,
	colorIdentity []string,
	cmc float64,
	isCommanderCandidate bool,
) DBCard {
	return DBCard{
		OracleID:             strings.TrimSpace(oracleID),
		Name:                 strings.TrimSpace(name),
		ManaCost:             strings.TrimSpace(manaCost),
		TypeLine:             strings.TrimSpace(typeLine),
		OracleText:           strings.TrimSpace(oracleText),
		ImageURI:             strings.TrimSpace(imageURI),
		ColorIdentity:        strings.Join(colorIdentity, ","),
		CMC:                  cmc,
		IsCommanderCandidate: isCommanderCandidate,
	}
}

func lookupCardsByNameSearches(ctx context.Context, db *sql.DB, searches []string) (map[string]DBCard, error) {
	if len(searches) == 0 {
		return map[string]DBCard{}, nil
	}

	rows, err := db.QueryContext(ctx, `
		WITH input AS (
			SELECT DISTINCT unnest($1::text[]) AS q
		),
		ranked AS (
			SELECT
				i.q AS name_search,
				oc.oracle_id::text,
				oc.name,
				COALESCE(oc.mana_cost, '') AS mana_cost,
				COALESCE(oc.type_line, '') AS type_line,
				COALESCE(oc.oracle_text, '') AS oracle_text,
				COALESCE(oc.default_image_uri, '') AS image_uri,
				COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
				COALESCE(oc.cmc, 0) AS cmc,
				COALESCE(oc.is_commander_candidate, false) AS is_commander_candidate,
				ROW_NUMBER() OVER (
					PARTITION BY i.q
					ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
				) AS rn
			FROM input i
			JOIN oracle_cards oc
			  ON oc.name_search = i.q
		)
		SELECT
			name_search,
			oracle_id,
			name,
			mana_cost,
			type_line,
			oracle_text,
			image_uri,
			color_identity,
			cmc,
			is_commander_candidate
		FROM ranked
		WHERE rn = 1
	`, pq.Array(searches))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]DBCard, len(searches))
	for rows.Next() {
		var (
			nameSearch, oracleID, name, manaCost, typeLine, oracleText, imageURI string
			colorIdentity                                                        []string
			cmc                                                                  float64
			isCommanderCandidate                                                 bool
		)
		if err := rows.Scan(
			&nameSearch,
			&oracleID,
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&imageURI,
			pq.Array(&colorIdentity),
			&cmc,
			&isCommanderCandidate,
		); err != nil {
			return nil, err
		}
		out[nameSearch] = scanDBCardRow(
			oracleID,
			name,
			manaCost,
			typeLine,
			oracleText,
			imageURI,
			colorIdentity,
			cmc,
			isCommanderCandidate,
		)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// EnsureCardByName resolves a card from the canonical oracle_cards table by name.
func EnsureCardByName(ctx context.Context, db *sql.DB, name string) (*DBCard, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrCardNotFound
	}

	var (
		oracleID, rowName, manaCost, typeLine, oracleText, imageURI string
		colorIdentity                                               []string
		cmc                                                         float64
		isCommanderCandidate                                        bool
	)
	err := db.QueryRowContext(ctx, `
		SELECT
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, ''),
			COALESCE(oc.type_line, ''),
			COALESCE(oc.oracle_text, ''),
			COALESCE(oc.default_image_uri, ''),
			COALESCE(oc.color_identity, ARRAY[]::text[]),
			COALESCE(oc.cmc, 0),
			COALESCE(oc.is_commander_candidate, false)
		FROM oracle_cards oc
		WHERE oc.name_search = normalize_card_name($1)
		ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
		LIMIT 1
	`, name).Scan(
		&oracleID,
		&rowName,
		&manaCost,
		&typeLine,
		&oracleText,
		&imageURI,
		pq.Array(&colorIdentity),
		&cmc,
		&isCommanderCandidate,
	)
	if err == sql.ErrNoRows {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, err
	}

	card := scanDBCardRow(
		oracleID,
		rowName,
		manaCost,
		typeLine,
		oracleText,
		imageURI,
		colorIdentity,
		cmc,
		isCommanderCandidate,
	)
	return &card, nil
}

// LookupCardsByNames fetches exact canonical card matches keyed by normalized
// input name (lower+trim). It performs one batch query.
func LookupCardsByNames(ctx context.Context, db *sql.DB, names []string) (map[string]DBCard, error) {
	type lookupInput struct {
		key    string
		search string
	}

	seenKey := make(map[string]struct{}, len(names))
	inputs := make([]lookupInput, 0, len(names))
	uniqueSearches := make([]string, 0, len(names))
	seenSearch := make(map[string]struct{}, len(names))

	for _, raw := range names {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" {
			continue
		}
		if _, exists := seenKey[key]; exists {
			continue
		}
		seenKey[key] = struct{}{}

		search := NormalizeName(raw)
		if search == "" {
			continue
		}

		inputs = append(inputs, lookupInput{key: key, search: search})
		if _, exists := seenSearch[search]; !exists {
			seenSearch[search] = struct{}{}
			uniqueSearches = append(uniqueSearches, search)
		}
	}

	if len(uniqueSearches) == 0 {
		return map[string]DBCard{}, nil
	}

	bySearch, err := lookupCardsByNameSearches(ctx, db, uniqueSearches)
	if err != nil {
		return nil, err
	}

	out := make(map[string]DBCard, len(inputs))
	for _, input := range inputs {
		if card, ok := bySearch[input.search]; ok {
			out[input.key] = card
		}
	}
	return out, nil
}

func EnsureCardsTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm;`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS unaccent;`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION normalize_card_name(input TEXT)
		RETURNS TEXT
		LANGUAGE SQL
		IMMUTABLE
		PARALLEL SAFE
		AS $$
			SELECT trim(
				regexp_replace(
					regexp_replace(
						lower(unaccent(COALESCE(input, ''))),
						'[^a-z0-9]+',
						' ',
						'g'
					),
					'\s+',
					' ',
					'g'
				)
			)
		$$;
	`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS oracle_cards (
			oracle_id UUID PRIMARY KEY,
			name TEXT NOT NULL,
			name_search TEXT GENERATED ALWAYS AS (normalize_card_name(name)) STORED,
			mana_cost TEXT,
			cmc DOUBLE PRECISION NOT NULL DEFAULT 0,
			type_line TEXT,
			oracle_text TEXT,
			colors TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			color_identity TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			layout TEXT,
			card_faces JSONB NOT NULL DEFAULT '[]'::jsonb,
			commander_legal BOOLEAN NOT NULL DEFAULT FALSE,
			is_commander_candidate BOOLEAN NOT NULL DEFAULT FALSE,
			edhrec_rank INTEGER,
			default_print_id UUID NULL,
			default_image_uri TEXT,
			default_price_usd TEXT,
			default_artist TEXT,
			default_set_code TEXT,
			default_set_name TEXT,
			default_released_at DATE,
			default_scryfall_uri TEXT
		);
	`); err != nil {
		return err
	}
	alterOracleStmts := []string{
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_image_uri TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_price_usd TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_artist TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_set_code TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_set_name TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_released_at DATE;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_scryfall_uri TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS is_commander_candidate BOOLEAN NOT NULL DEFAULT FALSE;`,
	}
	for _, stmt := range alterOracleStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS card_prints (
			scryfall_id UUID PRIMARY KEY,
			oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			set_code TEXT NOT NULL,
			collector_number TEXT NOT NULL,
			lang TEXT NOT NULL DEFAULT 'en',
			released_at DATE,
			image_uris JSONB NOT NULL DEFAULT '{}'::jsonb,
			image_uri TEXT,
			card_faces_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			set_name TEXT,
			rarity TEXT,
			artist TEXT,
			price_usd TEXT,
			scryfall_uri TEXT
		);
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS card_faces_json JSONB NOT NULL DEFAULT '[]'::jsonb`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'fk_oracle_cards_default_print'
			) THEN
				ALTER TABLE oracle_cards
				ADD CONSTRAINT fk_oracle_cards_default_print
				FOREIGN KEY (default_print_id)
				REFERENCES card_prints(scryfall_id)
				ON DELETE SET NULL;
			END IF;
		END $$;
	`); err != nil {
		return err
	}

	indexStmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_name_search ON oracle_cards (name_search);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_name_search_trgm ON oracle_cards USING GIN (name_search gin_trgm_ops);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_commander_legal ON oracle_cards (commander_legal);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_is_commander_candidate ON oracle_cards (is_commander_candidate);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_edhrec_rank ON oracle_cards (edhrec_rank);`,
		`CREATE INDEX IF NOT EXISTS idx_card_prints_oracle_id ON card_prints (oracle_id);`,
		`CREATE INDEX IF NOT EXISTS idx_card_prints_oracle_released ON card_prints (oracle_id, released_at DESC);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_card_prints_set_collector_lang ON card_prints (set_code, collector_number, lang);`,
	}
	for _, stmt := range indexStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

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
