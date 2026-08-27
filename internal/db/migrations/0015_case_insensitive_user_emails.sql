-- Prevent accounts that differ only by email letter casing. Review and merge
-- any existing case-only duplicates before applying this index.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_case_insensitive
ON users (lower(email));
