-- Cards table: minimal data for now, populated when adding to decks

CREATE TABLE IF NOT EXISTS cards (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    UNIQUE(name)
);

-- Deck cards table: many-to-many between decks and cards

CREATE TABLE IF NOT EXISTS deck_cards (
    deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    card_id BIGINT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    quantity INT NOT NULL DEFAULT 1,
    PRIMARY KEY (deck_id, card_id)
);