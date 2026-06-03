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
	FlavorText           string
	AllPartsJSON         string
	ImageURI             string
	Colors               string
	ColorIdentity        string
	CMC                  float64
	PriceUSD             string
	Artist               string
	SetCode              string
	SetName              string
	CollectorNumber      string
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
	return scanDBCardRowWithAllParts(
		oracleID,
		name,
		manaCost,
		typeLine,
		oracleText,
		"[]",
		imageURI,
		colorIdentity,
		cmc,
		isCommanderCandidate,
	)
}

func scanDBCardRowWithAllParts(
	oracleID, name, manaCost, typeLine, oracleText, allPartsJSON, imageURI string,
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
		AllPartsJSON:         strings.TrimSpace(allPartsJSON),
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
				COALESCE(cp.flavor_text, oc.flavor_text, '') AS flavor_text,
				COALESCE(oc.all_parts::text, '[]') AS all_parts_json,
				COALESCE(oc.default_image_uri, '') AS image_uri,
				COALESCE(oc.colors, ARRAY[]::text[]) AS colors,
				COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
				COALESCE(oc.cmc, 0) AS cmc,
				COALESCE(oc.default_price_usd, '') AS price_usd,
				COALESCE(oc.default_artist, '') AS artist,
				COALESCE(oc.default_set_code, '') AS set_code,
				COALESCE(oc.default_set_name, '') AS set_name,
				COALESCE(cp.collector_number, '') AS collector_number,
				COALESCE(oc.is_commander_candidate, false) AS is_commander_candidate,
				ROW_NUMBER() OVER (
					PARTITION BY i.q
					ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
				) AS rn
			FROM input i
			JOIN oracle_cards oc
			  ON oc.name_search = i.q
			LEFT JOIN card_prints cp
			  ON cp.scryfall_id = oc.default_print_id
		)
		SELECT
			name_search,
			oracle_id,
			name,
			mana_cost,
			type_line,
			oracle_text,
			flavor_text,
			all_parts_json,
			image_uri,
			colors,
			color_identity,
			cmc,
			price_usd,
			artist,
			set_code,
			set_name,
			collector_number,
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
			nameSearch, oracleID, name, manaCost, typeLine, oracleText, flavorText, allPartsJSON, imageURI string
			priceUSD, artist, setCode, setName, collectorNumber                                            string
			colors, colorIdentity                                                                          []string
			cmc                                                                                            float64
			isCommanderCandidate                                                                           bool
		)
		if err := rows.Scan(
			&nameSearch,
			&oracleID,
			&name,
			&manaCost,
			&typeLine,
			&oracleText,
			&flavorText,
			&allPartsJSON,
			&imageURI,
			pq.Array(&colors),
			pq.Array(&colorIdentity),
			&cmc,
			&priceUSD,
			&artist,
			&setCode,
			&setName,
			&collectorNumber,
			&isCommanderCandidate,
		); err != nil {
			return nil, err
		}
		card := scanDBCardRowWithAllParts(
			oracleID,
			name,
			manaCost,
			typeLine,
			oracleText,
			allPartsJSON,
			imageURI,
			colorIdentity,
			cmc,
			isCommanderCandidate,
		)
		card.FlavorText = strings.TrimSpace(flavorText)
		card.Colors = strings.Join(colors, ",")
		card.PriceUSD = strings.TrimSpace(priceUSD)
		card.Artist = strings.TrimSpace(artist)
		card.SetCode = strings.TrimSpace(setCode)
		card.SetName = strings.TrimSpace(setName)
		card.CollectorNumber = strings.TrimSpace(collectorNumber)
		out[nameSearch] = card
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
		oracleID, rowName, manaCost, typeLine, oracleText, flavorText, allPartsJSON, imageURI string
		priceUSD, artist, setCode, setName, collectorNumber                                   string
		colors, colorIdentity                                                                 []string
		cmc                                                                                   float64
		isCommanderCandidate                                                                  bool
	)
	err := db.QueryRowContext(ctx, `
		SELECT
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, ''),
			COALESCE(oc.type_line, ''),
			COALESCE(oc.oracle_text, ''),
			COALESCE(cp.flavor_text, oc.flavor_text, ''),
			COALESCE(oc.all_parts::text, '[]'),
			COALESCE(oc.default_image_uri, ''),
			COALESCE(oc.colors, ARRAY[]::text[]),
			COALESCE(oc.color_identity, ARRAY[]::text[]),
			COALESCE(oc.cmc, 0),
			COALESCE(oc.default_price_usd, ''),
			COALESCE(oc.default_artist, ''),
			COALESCE(oc.default_set_code, ''),
			COALESCE(oc.default_set_name, ''),
			COALESCE(cp.collector_number, ''),
			COALESCE(oc.is_commander_candidate, false)
		FROM oracle_cards oc
		LEFT JOIN card_prints cp
		  ON cp.scryfall_id = oc.default_print_id
		WHERE oc.name_search = normalize_card_name($1)
		ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
		LIMIT 1
	`, name).Scan(
		&oracleID,
		&rowName,
		&manaCost,
		&typeLine,
		&oracleText,
		&flavorText,
		&allPartsJSON,
		&imageURI,
		pq.Array(&colors),
		pq.Array(&colorIdentity),
		&cmc,
		&priceUSD,
		&artist,
		&setCode,
		&setName,
		&collectorNumber,
		&isCommanderCandidate,
	)
	if err == sql.ErrNoRows {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, err
	}

	card := scanDBCardRowWithAllParts(
		oracleID,
		rowName,
		manaCost,
		typeLine,
		oracleText,
		allPartsJSON,
		imageURI,
		colorIdentity,
		cmc,
		isCommanderCandidate,
	)
	card.FlavorText = strings.TrimSpace(flavorText)
	card.Colors = strings.Join(colors, ",")
	card.PriceUSD = strings.TrimSpace(priceUSD)
	card.Artist = strings.TrimSpace(artist)
	card.SetCode = strings.TrimSpace(setCode)
	card.SetName = strings.TrimSpace(setName)
	card.CollectorNumber = strings.TrimSpace(collectorNumber)
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
			flavor_text TEXT,
			colors TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			color_identity TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			power_text TEXT,
			toughness_text TEXT,
			loyalty_text TEXT,
			power_value DOUBLE PRECISION,
			toughness_value DOUBLE PRECISION,
			loyalty_value DOUBLE PRECISION,
			layout TEXT,
			card_faces JSONB NOT NULL DEFAULT '[]'::jsonb,
			all_parts JSONB NOT NULL DEFAULT '[]'::jsonb,
			legal_anywhere BOOLEAN NOT NULL DEFAULT TRUE,
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
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS flavor_text TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_artist TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_set_code TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_set_name TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_released_at DATE;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_scryfall_uri TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS is_commander_candidate BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS all_parts JSONB NOT NULL DEFAULT '[]'::jsonb;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS legal_anywhere BOOLEAN NOT NULL DEFAULT TRUE;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS power_text TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS toughness_text TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS loyalty_text TEXT;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS power_value DOUBLE PRECISION;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS toughness_value DOUBLE PRECISION;`,
		`ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS loyalty_value DOUBLE PRECISION;`,
	}
	for _, stmt := range alterOracleStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE oracle_cards
		SET legal_anywhere = FALSE
		WHERE lower(COALESCE(default_set_code, '')) = 'unk'
	`); err != nil {
		return err
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
			flavor_text TEXT,
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
	if _, err := db.ExecContext(ctx, `ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS flavor_text TEXT`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_card_prints_set_collector_lang`); err != nil {
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
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_legal_anywhere ON oracle_cards (legal_anywhere);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_commander_legal ON oracle_cards (commander_legal);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_is_commander_candidate ON oracle_cards (is_commander_candidate);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_edhrec_rank ON oracle_cards (edhrec_rank);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_power_value ON oracle_cards (power_value);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_toughness_value ON oracle_cards (toughness_value);`,
		`CREATE INDEX IF NOT EXISTS idx_oracle_cards_loyalty_value ON oracle_cards (loyalty_value);`,
		`CREATE INDEX IF NOT EXISTS idx_card_prints_oracle_id ON card_prints (oracle_id);`,
		`CREATE INDEX IF NOT EXISTS idx_card_prints_oracle_released ON card_prints (oracle_id, released_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_card_prints_set_collector_lang ON card_prints (set_code, collector_number, lang);`,
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
			card_count INTEGER NOT NULL DEFAULT 0,
			data_version INTEGER NOT NULL DEFAULT 0
		);
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE card_sync_state ADD COLUMN IF NOT EXISTS data_version INTEGER NOT NULL DEFAULT 0`); err != nil {
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
