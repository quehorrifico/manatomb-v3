CREATE TABLE IF NOT EXISTS user_card_printing_favorites (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  scryfall_id UUID NOT NULL REFERENCES card_prints(scryfall_id) ON DELETE CASCADE,
  oracle_id UUID NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, scryfall_id)
);

ALTER TABLE user_card_printing_favorites
ADD COLUMN IF NOT EXISTS oracle_id UUID;

UPDATE user_card_printing_favorites AS favorite
SET oracle_id = print.oracle_id
FROM card_prints AS print
WHERE print.scryfall_id = favorite.scryfall_id
  AND favorite.oracle_id IS DISTINCT FROM print.oracle_id;

CREATE INDEX IF NOT EXISTS idx_user_card_printing_favorites_oracle
ON user_card_printing_favorites (user_id, oracle_id);

CREATE INDEX IF NOT EXISTS idx_user_card_printing_favorites_user_created
ON user_card_printing_favorites (user_id, created_at DESC);
