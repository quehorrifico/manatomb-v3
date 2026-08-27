-- Canonical card schema for fast exact/fuzzy search and deck references.

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

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

CREATE TABLE IF NOT EXISTS card_prints (
  scryfall_id UUID PRIMARY KEY,
  oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    set_code TEXT NOT NULL,
    set_type TEXT NOT NULL DEFAULT '',
    collector_number TEXT NOT NULL,
  lang TEXT NOT NULL DEFAULT 'en',
  released_at DATE,
  flavor_text TEXT,
  image_uris JSONB NOT NULL DEFAULT '{}'::jsonb,
  image_uri TEXT,
  card_faces_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  finishes_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  frame_effects_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  promo_types_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  set_name TEXT,
  rarity TEXT,
  border_color TEXT,
  frame TEXT,
  security_stamp TEXT,
  full_art BOOLEAN NOT NULL DEFAULT FALSE,
  textless BOOLEAN NOT NULL DEFAULT FALSE,
  booster BOOLEAN NOT NULL DEFAULT FALSE,
  digital BOOLEAN NOT NULL DEFAULT FALSE,
  variation BOOLEAN NOT NULL DEFAULT FALSE,
  artist TEXT,
  price_usd TEXT,
  scryfall_uri TEXT
);

ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_image_uri TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_price_usd TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS flavor_text TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_artist TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_set_code TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_set_name TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_released_at DATE;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_scryfall_uri TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS is_commander_candidate BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS all_parts JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS legal_anywhere BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS power_text TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS toughness_text TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS loyalty_text TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS power_value DOUBLE PRECISION;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS toughness_value DOUBLE PRECISION;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS loyalty_value DOUBLE PRECISION;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS card_faces_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS flavor_text TEXT;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS finishes_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS frame_effects_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS promo_types_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS border_color TEXT;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS frame TEXT;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS security_stamp TEXT;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS full_art BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS textless BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS booster BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS digital BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS variation BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS set_type TEXT NOT NULL DEFAULT '';
DROP INDEX IF EXISTS idx_card_prints_set_collector_lang;

UPDATE oracle_cards
SET legal_anywhere = FALSE
WHERE lower(COALESCE(default_set_code, '')) = 'unk';

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

CREATE INDEX IF NOT EXISTS idx_oracle_cards_name_search ON oracle_cards (name_search);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_name_search_trgm ON oracle_cards USING GIN (name_search gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_legal_anywhere ON oracle_cards (legal_anywhere);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_commander_legal ON oracle_cards (commander_legal);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_is_commander_candidate ON oracle_cards (is_commander_candidate);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_edhrec_rank ON oracle_cards (edhrec_rank);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_power_value ON oracle_cards (power_value);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_toughness_value ON oracle_cards (toughness_value);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_loyalty_value ON oracle_cards (loyalty_value);
CREATE INDEX IF NOT EXISTS idx_card_prints_oracle_id ON card_prints (oracle_id);
CREATE INDEX IF NOT EXISTS idx_card_prints_oracle_released ON card_prints (oracle_id, released_at DESC);
CREATE INDEX IF NOT EXISTS idx_card_prints_set_collector_lang ON card_prints (set_code, collector_number, lang);

CREATE TABLE IF NOT EXISTS card_sync_state (
  id SMALLINT PRIMARY KEY CHECK (id = 1),
  last_attempt_at TIMESTAMPTZ,
  last_success_at TIMESTAMPTZ,
  source_updated_at TIMESTAMPTZ,
  last_error TEXT,
  card_count INTEGER NOT NULL DEFAULT 0,
  data_version INTEGER NOT NULL DEFAULT 0
);

ALTER TABLE card_sync_state ADD COLUMN IF NOT EXISTS data_version INTEGER NOT NULL DEFAULT 0;

INSERT INTO card_sync_state (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;

-- Upgrade the legacy name-based deck tables without ever dropping them before
-- every positive-quantity row has a canonical match. A raised exception rolls
-- back this entire DO statement and leaves the original tables intact.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'deck_cards'
      AND column_name = 'card_id'
  ) THEN
    DROP TABLE IF EXISTS deck_cards_v2;
    CREATE TABLE deck_cards_v2 (
      deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
      oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE RESTRICT,
      qty INT NOT NULL DEFAULT 0,
      board TEXT NOT NULL DEFAULT 'main',
      preferred_print_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
      PRIMARY KEY (deck_id, oracle_id, board),
      CHECK (board IN ('main', 'maybe', 'side'))
    );

    IF EXISTS (
      SELECT 1
      FROM deck_cards dc
      LEFT JOIN cards c ON c.id = dc.card_id
      LEFT JOIN LATERAL (
        SELECT candidate.oracle_id
        FROM oracle_cards candidate
        WHERE candidate.name_search = normalize_card_name(c.name)
        ORDER BY candidate.name, candidate.oracle_id
        LIMIT 1
      ) oc ON TRUE
      WHERE dc.quantity > 0
        AND (c.id IS NULL OR oc.oracle_id IS NULL)
    ) THEN
      RAISE EXCEPTION 'legacy deck_cards contains rows without canonical card matches';
    END IF;

    INSERT INTO deck_cards_v2 (deck_id, oracle_id, qty, board)
    SELECT dc.deck_id, oc.oracle_id, dc.quantity, 'main'
    FROM deck_cards dc
    JOIN cards c ON c.id = dc.card_id
    JOIN LATERAL (
      SELECT candidate.oracle_id
      FROM oracle_cards candidate
      WHERE candidate.name_search = normalize_card_name(c.name)
      ORDER BY candidate.name, candidate.oracle_id
      LIMIT 1
    ) oc ON TRUE
    WHERE dc.quantity > 0
    ON CONFLICT (deck_id, oracle_id, board)
    DO UPDATE SET qty = deck_cards_v2.qty + EXCLUDED.qty;

    IF EXISTS (
      SELECT 1 FROM information_schema.tables
      WHERE table_schema = 'public' AND table_name = 'deck_maybe_cards'
    ) THEN
      IF EXISTS (
        SELECT 1
        FROM deck_maybe_cards dmc
        LEFT JOIN cards c ON c.id = dmc.card_id
        LEFT JOIN LATERAL (
          SELECT candidate.oracle_id
          FROM oracle_cards candidate
          WHERE candidate.name_search = normalize_card_name(c.name)
          ORDER BY candidate.name, candidate.oracle_id
          LIMIT 1
        ) oc ON TRUE
        WHERE dmc.quantity > 0
          AND (c.id IS NULL OR oc.oracle_id IS NULL)
      ) THEN
        RAISE EXCEPTION 'legacy deck_maybe_cards contains rows without canonical card matches';
      END IF;

      INSERT INTO deck_cards_v2 (deck_id, oracle_id, qty, board)
      SELECT dmc.deck_id, oc.oracle_id, dmc.quantity, 'maybe'
      FROM deck_maybe_cards dmc
      JOIN cards c ON c.id = dmc.card_id
      JOIN LATERAL (
        SELECT candidate.oracle_id
        FROM oracle_cards candidate
        WHERE candidate.name_search = normalize_card_name(c.name)
        ORDER BY candidate.name, candidate.oracle_id
        LIMIT 1
      ) oc ON TRUE
      WHERE dmc.quantity > 0
      ON CONFLICT (deck_id, oracle_id, board)
      DO UPDATE SET qty = deck_cards_v2.qty + EXCLUDED.qty;

      DROP TABLE deck_maybe_cards;
    END IF;

    DROP TABLE deck_cards;
    ALTER TABLE deck_cards_v2 RENAME TO deck_cards;
  ELSE
    CREATE TABLE IF NOT EXISTS deck_cards (
      deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
      oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE RESTRICT,
      qty INT NOT NULL DEFAULT 0,
      board TEXT NOT NULL DEFAULT 'main',
      preferred_print_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
      PRIMARY KEY (deck_id, oracle_id, board),
      CHECK (board IN ('main', 'maybe', 'side'))
    );
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_deck_cards_deck_board ON deck_cards (deck_id, board);
