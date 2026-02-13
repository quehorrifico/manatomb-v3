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
  colors TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  color_identity TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  layout TEXT,
  card_faces JSONB NOT NULL DEFAULT '[]'::jsonb,
  all_parts JSONB NOT NULL DEFAULT '[]'::jsonb,
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

ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_image_uri TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_price_usd TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_artist TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_set_code TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_set_name TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_released_at DATE;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS default_scryfall_uri TEXT;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS is_commander_candidate BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE oracle_cards ADD COLUMN IF NOT EXISTS all_parts JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE card_prints ADD COLUMN IF NOT EXISTS card_faces_json JSONB NOT NULL DEFAULT '[]'::jsonb;

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
CREATE INDEX IF NOT EXISTS idx_oracle_cards_commander_legal ON oracle_cards (commander_legal);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_is_commander_candidate ON oracle_cards (is_commander_candidate);
CREATE INDEX IF NOT EXISTS idx_oracle_cards_edhrec_rank ON oracle_cards (edhrec_rank);
CREATE INDEX IF NOT EXISTS idx_card_prints_oracle_id ON card_prints (oracle_id);
CREATE INDEX IF NOT EXISTS idx_card_prints_oracle_released ON card_prints (oracle_id, released_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_card_prints_set_collector_lang ON card_prints (set_code, collector_number, lang);

DROP TABLE IF EXISTS deck_maybe_cards;
DROP TABLE IF EXISTS deck_cards;

CREATE TABLE IF NOT EXISTS deck_cards (
  deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
  oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE RESTRICT,
  qty INT NOT NULL DEFAULT 0,
  board TEXT NOT NULL DEFAULT 'main',
  preferred_print_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
  PRIMARY KEY (deck_id, oracle_id, board),
  CHECK (board IN ('main', 'maybe'))
);

CREATE INDEX IF NOT EXISTS idx_deck_cards_deck_board ON deck_cards (deck_id, board);
