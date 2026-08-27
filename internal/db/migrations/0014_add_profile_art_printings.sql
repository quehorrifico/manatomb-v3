ALTER TABLE users
ADD COLUMN IF NOT EXISTS profile_picture_print_id UUID NULL;

ALTER TABLE users
ADD COLUMN IF NOT EXISTS profile_background_print_id UUID NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'users_profile_picture_print_id_fkey'
  ) THEN
    ALTER TABLE users
    ADD CONSTRAINT users_profile_picture_print_id_fkey
    FOREIGN KEY (profile_picture_print_id)
    REFERENCES card_prints(scryfall_id)
    ON DELETE SET NULL;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'users_profile_background_print_id_fkey'
  ) THEN
    ALTER TABLE users
    ADD CONSTRAINT users_profile_background_print_id_fkey
    FOREIGN KEY (profile_background_print_id)
    REFERENCES card_prints(scryfall_id)
    ON DELETE SET NULL;
  END IF;
END $$;
