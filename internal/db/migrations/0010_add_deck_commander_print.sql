ALTER TABLE decks
ADD COLUMN IF NOT EXISTS commander_print_id UUID NULL;

UPDATE decks AS d
SET commander_print_id = COALESCE(
    (
        SELECT COALESCE(dc.preferred_print_id, oc.default_print_id)
        FROM deck_cards AS dc
        JOIN oracle_cards AS oc
          ON oc.oracle_id = dc.oracle_id
        WHERE dc.deck_id = d.id
          AND dc.board = 'main'
          AND oc.name_search = normalize_card_name(d.commander_name)
        ORDER BY (dc.preferred_print_id IS NOT NULL) DESC
        LIMIT 1
    ),
    (
        SELECT oc.default_print_id
        FROM oracle_cards AS oc
        WHERE oc.name_search = normalize_card_name(d.commander_name)
        ORDER BY COALESCE(oc.edhrec_rank, 2147483647), oc.name
        LIMIT 1
    )
)
WHERE d.commander_print_id IS NULL
  AND btrim(COALESCE(d.commander_name, '')) <> '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_decks_commander_print'
          AND conrelid = 'decks'::regclass
    ) THEN
        ALTER TABLE decks
        ADD CONSTRAINT fk_decks_commander_print
        FOREIGN KEY (commander_print_id)
        REFERENCES card_prints(scryfall_id)
        ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_decks_commander_print_id
ON decks (commander_print_id)
WHERE commander_print_id IS NOT NULL;
