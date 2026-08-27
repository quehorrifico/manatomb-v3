ALTER TABLE card_prints
    ADD COLUMN IF NOT EXISTS price_usd_nonfoil TEXT,
    ADD COLUMN IF NOT EXISTS price_usd_foil TEXT,
    ADD COLUMN IF NOT EXISTS price_usd_etched TEXT;
