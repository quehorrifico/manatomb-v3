package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"manatomb/app/internal/cards"

	"github.com/lib/pq"
)

type favoritePrintingData struct {
	ScryfallID      string
	OracleID        string
	Name            string
	ImageURI        string
	ArtCropURI      string
	SetName         string
	SetCode         string
	CollectorNumber string
	Rarity          string
	ReleasedAt      string
	Artist          string
	PriceUSD        string
	CreatedAt       time.Time
}

type guessCardGame struct {
	ID               int64
	UserID           int64
	GuestID          string
	TargetOracleID   string
	TargetScryfallID string
	Status           string
	QuestionCount    int
	GuessCount       int
	IsDaily          bool
	DailyKey         string
	MaxQuestions     int
	AskedQuestions   []string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

type spellifyGame struct {
	ID               int64
	UserID           int64
	GuestID          string
	TargetOracleID   string
	TargetScryfallID string
	Status           string
	GuessedChars     []string
	GuessCount       int
	IsDaily          bool
	DailyKey         string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

type guessCardAward struct {
	ID            int64
	UserID        int64
	OracleID      string
	ScryfallID    string
	CardName      string
	ImageURI      string
	WonAt         time.Time
	QuestionCount int
	GuessCount    int
}

type spellifyAward struct {
	ID         int64
	UserID     int64
	OracleID   string
	ScryfallID string
	CardName   string
	ImageURI   string
	WonAt      time.Time
	GuessCount int
}

var errGuessCardQuestionUnavailable = errors.New("guess card question unavailable")
var errSpellifyGuessUnavailable = errors.New("spellify guess unavailable")

// EnsureFeatureTables creates account-scoped feature tables that are owned by
// the web layer: favorite printings, guess-card games, and game awards.
func EnsureFeatureTables(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`
		CREATE TABLE IF NOT EXISTS user_card_printing_favorites (
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			scryfall_id UUID NOT NULL REFERENCES card_prints(scryfall_id) ON DELETE CASCADE,
			oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, scryfall_id)
		);
		`,
		`CREATE INDEX IF NOT EXISTS idx_user_card_printing_favorites_user_created ON user_card_printing_favorites (user_id, created_at DESC);`,
		`
		CREATE TABLE IF NOT EXISTS daily_game_targets (
			mode TEXT NOT NULL,
			daily_key TEXT NOT NULL,
			target_oracle_id UUID NOT NULL,
			target_scryfall_id UUID NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (mode, daily_key)
		);
		`,
		`
		CREATE TABLE IF NOT EXISTS user_guess_card_games (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NULL REFERENCES users(id) ON DELETE CASCADE,
			guest_id UUID NULL,
			target_oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			target_scryfall_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
			status TEXT NOT NULL DEFAULT 'active',
			question_count INT NOT NULL DEFAULT 0,
			guess_count INT NOT NULL DEFAULT 0,
			is_daily BOOLEAN NOT NULL DEFAULT true,
			daily_key TEXT NOT NULL DEFAULT '',
			max_questions INT NOT NULL DEFAULT 0,
			asked_questions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			completed_at TIMESTAMPTZ NULL
		);
		`,
		`ALTER TABLE user_guess_card_games ALTER COLUMN user_id DROP NOT NULL;`,
		`ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS guest_id UUID NULL;`,
		`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'user_guess_card_games_owner_check'
			) THEN
				ALTER TABLE user_guess_card_games
				ADD CONSTRAINT user_guess_card_games_owner_check
				CHECK ((user_id IS NULL) <> (guest_id IS NULL));
			END IF;
		END $$;
		`,
		`CREATE INDEX IF NOT EXISTS idx_user_guess_card_games_guest_active ON user_guess_card_games (guest_id, status, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_user_guess_card_games_guest_daily ON user_guess_card_games (guest_id, daily_key, is_daily);`,
		`ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS guess_count INT NOT NULL DEFAULT 0;`,
		`ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS is_daily BOOLEAN NOT NULL DEFAULT true;`,
		`ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS daily_key TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE user_guess_card_games ALTER COLUMN max_questions SET DEFAULT 0;`,
		`CREATE INDEX IF NOT EXISTS idx_user_guess_card_games_active ON user_guess_card_games (user_id, status, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_user_guess_card_games_daily ON user_guess_card_games (user_id, daily_key, is_daily);`,
		`
		CREATE TABLE IF NOT EXISTS user_guess_card_awards (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			scryfall_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
			card_name TEXT NOT NULL,
			image_uri TEXT NOT NULL DEFAULT '',
			won_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			question_count INT NOT NULL DEFAULT 0,
			guess_count INT NOT NULL DEFAULT 0,
			UNIQUE (user_id, scryfall_id)
		);
		`,
		`ALTER TABLE user_guess_card_awards ADD COLUMN IF NOT EXISTS guess_count INT NOT NULL DEFAULT 0;`,
		`CREATE INDEX IF NOT EXISTS idx_user_guess_card_awards_user_won ON user_guess_card_awards (user_id, won_at DESC);`,
		`
		CREATE TABLE IF NOT EXISTS user_spellify_games (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NULL REFERENCES users(id) ON DELETE CASCADE,
			guest_id UUID NULL,
			target_oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			target_scryfall_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
			status TEXT NOT NULL DEFAULT 'active',
			guessed_chars TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			guess_count INT NOT NULL DEFAULT 0,
			is_daily BOOLEAN NOT NULL DEFAULT true,
			daily_key TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			completed_at TIMESTAMPTZ NULL
		);
		`,
		`ALTER TABLE user_spellify_games ALTER COLUMN user_id DROP NOT NULL;`,
		`ALTER TABLE user_spellify_games ADD COLUMN IF NOT EXISTS guest_id UUID NULL;`,
		`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'user_spellify_games_owner_check'
			) THEN
				ALTER TABLE user_spellify_games
				ADD CONSTRAINT user_spellify_games_owner_check
				CHECK ((user_id IS NULL) <> (guest_id IS NULL));
			END IF;
		END $$;
		`,
		`CREATE INDEX IF NOT EXISTS idx_user_spellify_games_guest_active ON user_spellify_games (guest_id, status, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_user_spellify_games_guest_daily ON user_spellify_games (guest_id, daily_key, is_daily);`,
		`ALTER TABLE user_spellify_games ADD COLUMN IF NOT EXISTS is_daily BOOLEAN NOT NULL DEFAULT true;`,
		`ALTER TABLE user_spellify_games ADD COLUMN IF NOT EXISTS daily_key TEXT NOT NULL DEFAULT '';`,
		`CREATE INDEX IF NOT EXISTS idx_user_spellify_games_active ON user_spellify_games (user_id, status, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_user_spellify_games_daily ON user_spellify_games (user_id, daily_key, is_daily);`,
		`
		CREATE TABLE IF NOT EXISTS user_spellify_awards (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			scryfall_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
			card_name TEXT NOT NULL,
			image_uri TEXT NOT NULL DEFAULT '',
			won_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			guess_count INT NOT NULL DEFAULT 0,
			daily_key TEXT NOT NULL DEFAULT '',
			UNIQUE (user_id, daily_key)
		);
		`,
		`CREATE INDEX IF NOT EXISTS idx_user_spellify_awards_user_won ON user_spellify_awards (user_id, won_at DESC);`,
		`
		INSERT INTO daily_game_targets (mode, daily_key, target_oracle_id, target_scryfall_id)
		SELECT DISTINCT ON (daily_key)
			'guess-card',
			daily_key,
			target_oracle_id,
			target_scryfall_id
		FROM user_guess_card_games
		WHERE is_daily = true AND daily_key <> ''
		ORDER BY daily_key, created_at ASC
		ON CONFLICT (mode, daily_key) DO NOTHING;
		`,
		`
		INSERT INTO daily_game_targets (mode, daily_key, target_oracle_id, target_scryfall_id)
		SELECT DISTINCT ON (daily_key)
			'tombscript',
			daily_key,
			target_oracle_id,
			target_scryfall_id
		FROM user_spellify_games
		WHERE is_daily = true AND daily_key <> ''
		ORDER BY daily_key, created_at ASC
		ON CONFLICT (mode, daily_key) DO NOTHING;
		`,
		`DELETE FROM user_guess_card_games WHERE guest_id IS NOT NULL AND created_at < now() - interval '90 days';`,
		`DELETE FROM user_spellify_games WHERE guest_id IS NOT NULL AND created_at < now() - interval '90 days';`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func setFavoritePrinting(ctx context.Context, db *sql.DB, userID int64, scryfallID string, favorite bool) error {
	scryfallID = strings.TrimSpace(scryfallID)
	if scryfallID == "" {
		return errors.New("scryfall id is required")
	}
	if favorite {
		result, err := db.ExecContext(ctx, `
			INSERT INTO user_card_printing_favorites (user_id, scryfall_id, oracle_id)
			SELECT $1, cp.scryfall_id, cp.oracle_id
			FROM card_prints cp
			WHERE cp.scryfall_id = $2::uuid
			ON CONFLICT (user_id, scryfall_id) DO NOTHING
		`, userID, scryfallID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			var exists bool
			if err := db.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM user_card_printing_favorites
					WHERE user_id = $1 AND scryfall_id = $2::uuid
				)
			`, userID, scryfallID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return cards.ErrCardNotFound
			}
		}
		return nil
	}
	_, err := db.ExecContext(ctx, `
		DELETE FROM user_card_printing_favorites
		WHERE user_id = $1 AND scryfall_id = $2::uuid
	`, userID, scryfallID)
	return err
}

func favoritePrintingIDsForOracle(ctx context.Context, db *sql.DB, userID int64, oracleID string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT scryfall_id::text
		FROM user_card_printing_favorites
		WHERE user_id = $1 AND oracle_id = $2::uuid
	`, userID, strings.TrimSpace(oracleID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func listFavoritePrintingsPage(ctx context.Context, db *sql.DB, userID int64, limit, offset int) ([]favoritePrintingData, error) {
	if limit <= 0 {
		limit = 120
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := db.QueryContext(ctx, `
		SELECT
			cp.scryfall_id::text,
			cp.oracle_id::text,
			cp.name,
			COALESCE(cp.image_uri, '') AS image_uri,
			COALESCE(cp.image_uris->>'art_crop', cp.image_uri, '') AS art_crop_uri,
			COALESCE(cp.set_name, '') AS set_name,
			COALESCE(cp.set_code, '') AS set_code,
			COALESCE(cp.collector_number, '') AS collector_number,
			COALESCE(cp.rarity, '') AS rarity,
			COALESCE(to_char(cp.released_at, 'YYYY-MM-DD'), '') AS released_at,
			COALESCE(cp.artist, '') AS artist,
			COALESCE(cp.price_usd, '') AS price_usd,
			f.created_at
		FROM user_card_printing_favorites f
		JOIN card_prints cp
		  ON cp.scryfall_id = f.scryfall_id
		WHERE f.user_id = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]favoritePrintingData, 0, limit)
	for rows.Next() {
		var item favoritePrintingData
		if err := rows.Scan(
			&item.ScryfallID,
			&item.OracleID,
			&item.Name,
			&item.ImageURI,
			&item.ArtCropURI,
			&item.SetName,
			&item.SetCode,
			&item.CollectorNumber,
			&item.Rarity,
			&item.ReleasedAt,
			&item.Artist,
			&item.PriceUSD,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func applyPrintingFavoriteStatus(page *cardDetailPageData, favoriteIDs map[string]bool) {
	if page == nil || len(favoriteIDs) == 0 {
		return
	}
	for i := range page.Printings {
		page.Printings[i].IsFavorited = favoriteIDs[page.Printings[i].ScryfallID]
	}
}

func loadActiveGuessCardGame(ctx context.Context, db *sql.DB, player gamePlayer) (*guessCardGame, error) {
	row := db.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			COALESCE(guest_id::text, ''),
			target_oracle_id::text,
			COALESCE(target_scryfall_id::text, ''),
			status,
			question_count,
			guess_count,
			is_daily,
			daily_key,
			max_questions,
			asked_questions,
			created_at,
			completed_at
		FROM user_guess_card_games
		WHERE (user_id = $1 OR guest_id = NULLIF($2, '')::uuid)
		  AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`, player.userIDValue(), player.guestIDValue())
	return scanGuessCardGame(row)
}

func loadGuessCardGameByID(ctx context.Context, db *sql.DB, player gamePlayer, gameID int64) (*guessCardGame, error) {
	row := db.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			COALESCE(guest_id::text, ''),
			target_oracle_id::text,
			COALESCE(target_scryfall_id::text, ''),
			status,
			question_count,
			guess_count,
			is_daily,
			daily_key,
			max_questions,
			asked_questions,
			created_at,
			completed_at
		FROM user_guess_card_games
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		LIMIT 1
	`, gameID, player.userIDValue(), player.guestIDValue())
	return scanGuessCardGame(row)
}

func scanGuessCardGame(row interface {
	Scan(dest ...any) error
}) (*guessCardGame, error) {
	var game guessCardGame
	var completedAt sql.NullTime
	var userID sql.NullInt64
	if err := row.Scan(
		&game.ID,
		&userID,
		&game.GuestID,
		&game.TargetOracleID,
		&game.TargetScryfallID,
		&game.Status,
		&game.QuestionCount,
		&game.GuessCount,
		&game.IsDaily,
		&game.DailyKey,
		&game.MaxQuestions,
		pq.Array(&game.AskedQuestions),
		&game.CreatedAt,
		&completedAt,
	); err != nil {
		return nil, err
	}
	if userID.Valid {
		game.UserID = userID.Int64
	}
	if completedAt.Valid {
		game.CompletedAt = &completedAt.Time
	}
	return &game, nil
}

func createGuessCardGame(ctx context.Context, db *sql.DB, player gamePlayer, isDaily bool, dailyKey string) (*guessCardGame, error) {
	dailyKey = strings.TrimSpace(dailyKey)
	var oracleID, scryfallID string
	var err error
	if isDaily {
		if dailyKey == "" {
			dailyKey = guessCardDailyKey(time.Now().UTC())
		}
		oracleID, scryfallID, err = getOrCreateDailyGameTarget(ctx, db, "guess-card", dailyKey)
	} else {
		if dailyKey == "" {
			dailyKey = guessCardDailyKey(time.Now().UTC())
		}
		oracleID, scryfallID, err = selectGuessCardTarget(ctx, db)
	}
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `
		INSERT INTO user_guess_card_games (user_id, guest_id, target_oracle_id, target_scryfall_id, is_daily, daily_key)
		VALUES ($1, NULLIF($2, '')::uuid, $3::uuid, NULLIF($4, '')::uuid, $5, $6)
		RETURNING
			id,
			user_id,
			COALESCE(guest_id::text, ''),
			target_oracle_id::text,
			COALESCE(target_scryfall_id::text, ''),
			status,
			question_count,
			guess_count,
			is_daily,
			daily_key,
			max_questions,
			asked_questions,
			created_at,
			completed_at
	`, player.userIDValue(), player.guestIDValue(), oracleID, scryfallID, isDaily, dailyKey)
	return scanGuessCardGame(row)
}

func guessCardDailyKey(now time.Time) string {
	return now.UTC().Format("2006-01-02")
}

func selectGuessCardTarget(ctx context.Context, db *sql.DB) (string, string, error) {
	var oracleID, scryfallID string
	if err := db.QueryRowContext(ctx, `
		SELECT
			oc.oracle_id::text,
			COALESCE(oc.default_print_id::text, '')
		FROM oracle_cards oc
		WHERE COALESCE(oc.legal_anywhere, true) = true
		  AND COALESCE(oc.default_image_uri, '') <> ''
		  AND oc.default_print_id IS NOT NULL
		  AND COALESCE(oc.edhrec_rank, 0) BETWEEN 1 AND 250
		  AND lower(COALESCE(oc.default_set_code, '')) <> 'unk'
		ORDER BY random()
		LIMIT 1
	`).Scan(&oracleID, &scryfallID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", cards.ErrCardNotFound
		}
		return "", "", err
	}
	return oracleID, scryfallID, nil
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func selectDailyGuessCardTarget(ctx context.Context, db queryRower, dailyKey string) (string, string, error) {
	dailyKey = strings.TrimSpace(dailyKey)
	if dailyKey == "" {
		return "", "", cards.ErrCardNotFound
	}

	var count int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM oracle_cards oc
		WHERE COALESCE(oc.legal_anywhere, true) = true
		  AND COALESCE(oc.default_image_uri, '') <> ''
		  AND oc.default_print_id IS NOT NULL
		  AND COALESCE(oc.edhrec_rank, 0) BETWEEN 1 AND 250
		  AND lower(COALESCE(oc.default_set_code, '')) <> 'unk'
	`).Scan(&count); err != nil {
		return "", "", err
	}
	if count <= 0 {
		return "", "", cards.ErrCardNotFound
	}

	offset := guessCardDailyOffset(dailyKey, count)
	var oracleID, scryfallID string
	if err := db.QueryRowContext(ctx, `
		SELECT
			oc.oracle_id::text,
			COALESCE(oc.default_print_id::text, '')
		FROM oracle_cards oc
		WHERE COALESCE(oc.legal_anywhere, true) = true
		  AND COALESCE(oc.default_image_uri, '') <> ''
		  AND oc.default_print_id IS NOT NULL
		  AND COALESCE(oc.edhrec_rank, 0) BETWEEN 1 AND 250
		  AND lower(COALESCE(oc.default_set_code, '')) <> 'unk'
		ORDER BY COALESCE(oc.edhrec_rank, 1000000), oc.name, oc.oracle_id::text
		OFFSET $1
		LIMIT 1
	`, offset).Scan(&oracleID, &scryfallID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", cards.ErrCardNotFound
		}
		return "", "", err
	}
	return oracleID, scryfallID, nil
}

func getOrCreateDailyGameTarget(ctx context.Context, db *sql.DB, mode, dailyKey string) (string, string, error) {
	mode = strings.TrimSpace(mode)
	dailyKey = strings.TrimSpace(dailyKey)
	if mode == "" || dailyKey == "" {
		return "", "", cards.ErrCardNotFound
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	var oracleID, scryfallID string
	err = tx.QueryRowContext(ctx, `
		SELECT target_oracle_id::text, COALESCE(target_scryfall_id::text, '')
		FROM daily_game_targets
		WHERE mode = $1 AND daily_key = $2
	`, mode, dailyKey).Scan(&oracleID, &scryfallID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return "", "", err
		}
		return oracleID, scryfallID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}

	oracleID, scryfallID, err = selectDailyGuessCardTarget(ctx, tx, mode+":"+dailyKey)
	if err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO daily_game_targets (mode, daily_key, target_oracle_id, target_scryfall_id)
		VALUES ($1, $2, $3::uuid, NULLIF($4, '')::uuid)
		ON CONFLICT (mode, daily_key) DO NOTHING
	`, mode, dailyKey, oracleID, scryfallID); err != nil {
		return "", "", err
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT target_oracle_id::text, COALESCE(target_scryfall_id::text, '')
		FROM daily_game_targets
		WHERE mode = $1 AND daily_key = $2
	`, mode, dailyKey).Scan(&oracleID, &scryfallID); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return oracleID, scryfallID, nil
}

func guessCardDailyOffset(dailyKey string, count int) int {
	if count <= 0 {
		return 0
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.TrimSpace(dailyKey)))
	return int(hash.Sum32() % uint32(count))
}

func activeOrNewGuessCardGame(ctx context.Context, db *sql.DB, player gamePlayer) (*guessCardGame, error) {
	dailyKey := guessCardDailyKey(time.Now().UTC())
	game, err := loadActiveGuessCardGame(ctx, db, player)
	if err == nil {
		if game.DailyKey != "" && game.DailyKey == dailyKey {
			return game, nil
		}
		if game.IsDaily && game.DailyKey == dailyKey {
			return game, nil
		}
		if err := abandonActiveGuessCardGames(ctx, db, player); err != nil {
			return nil, err
		}
		if attempted, err := hasGuessCardDailyAttempt(ctx, db, player, dailyKey); err != nil {
			return nil, err
		} else if attempted {
			return createGuessCardGame(ctx, db, player, false, "")
		}
		return createGuessCardGame(ctx, db, player, true, dailyKey)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	attempted, err := hasGuessCardDailyAttempt(ctx, db, player, dailyKey)
	if err != nil {
		return nil, err
	}
	if attempted {
		return createGuessCardGame(ctx, db, player, false, "")
	}
	return createGuessCardGame(ctx, db, player, true, dailyKey)
}

func hasGuessCardDailyAttempt(ctx context.Context, db *sql.DB, player gamePlayer, dailyKey string) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_guess_card_games
			WHERE (user_id = $1 OR guest_id = NULLIF($2, '')::uuid)
			  AND is_daily = true
			  AND daily_key = $3
			)
	`, player.userIDValue(), player.guestIDValue(), strings.TrimSpace(dailyKey)).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func createReplayGuessCardGame(ctx context.Context, db *sql.DB, player gamePlayer) (*guessCardGame, error) {
	return createGuessCardGame(ctx, db, player, false, guessCardDailyKey(time.Now().UTC()))
}

func abandonActiveGuessCardGames(ctx context.Context, db *sql.DB, player gamePlayer) error {
	_, err := db.ExecContext(ctx, `
		UPDATE user_guess_card_games
		SET status = 'abandoned', completed_at = now()
		WHERE (user_id = $1 OR guest_id = NULLIF($2, '')::uuid)
		  AND status = 'active'
	`, player.userIDValue(), player.guestIDValue())
	return err
}

func addGuessCardQuestion(ctx context.Context, db *sql.DB, gameID int64, player gamePlayer, questionID string) error {
	questionID = strings.TrimSpace(questionID)
	if questionByID(questionID) == nil {
		return fmt.Errorf("unknown question %q", questionID)
	}
	result, err := db.ExecContext(ctx, `
		UPDATE user_guess_card_games
		SET
			asked_questions = array_append(asked_questions, $4),
			question_count = question_count + 1
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
		  AND NOT ($4 = ANY(asked_questions))
	`, gameID, player.userIDValue(), player.guestIDValue(), questionID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errGuessCardQuestionUnavailable
	}
	return nil
}

func incrementGuessCardGuessCount(ctx context.Context, db *sql.DB, gameID int64, player gamePlayer) error {
	result, err := db.ExecContext(ctx, `
		UPDATE user_guess_card_games
		SET guess_count = guess_count + 1
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
	`, gameID, player.userIDValue(), player.guestIDValue())
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("guess could not be counted")
	}
	return nil
}

func completeGuessCardGame(ctx context.Context, db *sql.DB, gameID int64, player gamePlayer, won bool) error {
	status := "lost"
	if won {
		status = "won"
	}
	result, err := db.ExecContext(ctx, `
		UPDATE user_guess_card_games
		SET status = $4, completed_at = now()
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
	`, gameID, player.userIDValue(), player.guestIDValue(), status)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("game could not be completed")
	}
	return nil
}

func completeGuessCardGameWithAward(ctx context.Context, db *sql.DB, game guessCardGame, card cards.Card, guessCount int) error {
	if game.UserID <= 0 {
		return errors.New("guest games cannot create awards")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE user_guess_card_games
		SET status = 'won', completed_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'active'
	`, game.ID, game.UserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("game could not be completed")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_guess_card_awards (
			user_id, oracle_id, scryfall_id, card_name, image_uri, question_count, guess_count
		)
		VALUES ($1, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7)
		ON CONFLICT (user_id, scryfall_id) DO NOTHING
	`, game.UserID, card.OracleID, game.TargetScryfallID, card.Name, card.ImageURI, game.QuestionCount, guessCount); err != nil {
		return err
	}
	return tx.Commit()
}

func loadActiveSpellifyGame(ctx context.Context, db *sql.DB, player gamePlayer) (*spellifyGame, error) {
	row := db.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			COALESCE(guest_id::text, ''),
			target_oracle_id::text,
			COALESCE(target_scryfall_id::text, ''),
			status,
			guessed_chars,
			guess_count,
			is_daily,
			daily_key,
			created_at,
			completed_at
		FROM user_spellify_games
		WHERE (user_id = $1 OR guest_id = NULLIF($2, '')::uuid)
		  AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`, player.userIDValue(), player.guestIDValue())
	return scanSpellifyGame(row)
}

func loadSpellifyGameByID(ctx context.Context, db *sql.DB, player gamePlayer, gameID int64) (*spellifyGame, error) {
	row := db.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			COALESCE(guest_id::text, ''),
			target_oracle_id::text,
			COALESCE(target_scryfall_id::text, ''),
			status,
			guessed_chars,
			guess_count,
			is_daily,
			daily_key,
			created_at,
			completed_at
		FROM user_spellify_games
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		LIMIT 1
	`, gameID, player.userIDValue(), player.guestIDValue())
	return scanSpellifyGame(row)
}

func scanSpellifyGame(row interface {
	Scan(dest ...any) error
}) (*spellifyGame, error) {
	var game spellifyGame
	var completedAt sql.NullTime
	var userID sql.NullInt64
	if err := row.Scan(
		&game.ID,
		&userID,
		&game.GuestID,
		&game.TargetOracleID,
		&game.TargetScryfallID,
		&game.Status,
		pq.Array(&game.GuessedChars),
		&game.GuessCount,
		&game.IsDaily,
		&game.DailyKey,
		&game.CreatedAt,
		&completedAt,
	); err != nil {
		return nil, err
	}
	if userID.Valid {
		game.UserID = userID.Int64
	}
	if completedAt.Valid {
		game.CompletedAt = &completedAt.Time
	}
	return &game, nil
}

func createSpellifyGame(ctx context.Context, db *sql.DB, player gamePlayer, isDaily bool, dailyKey string) (*spellifyGame, error) {
	dailyKey = strings.TrimSpace(dailyKey)
	if dailyKey == "" {
		dailyKey = guessCardDailyKey(time.Now().UTC())
	}
	var oracleID, scryfallID string
	var err error
	if isDaily {
		oracleID, scryfallID, err = getOrCreateDailyGameTarget(ctx, db, "tombscript", dailyKey)
	} else {
		oracleID, scryfallID, err = selectGuessCardTarget(ctx, db)
	}
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `
		INSERT INTO user_spellify_games (user_id, guest_id, target_oracle_id, target_scryfall_id, is_daily, daily_key)
		VALUES ($1, NULLIF($2, '')::uuid, $3::uuid, NULLIF($4, '')::uuid, $5, $6)
		RETURNING
			id,
			user_id,
			COALESCE(guest_id::text, ''),
			target_oracle_id::text,
			COALESCE(target_scryfall_id::text, ''),
			status,
			guessed_chars,
			guess_count,
			is_daily,
			daily_key,
			created_at,
			completed_at
	`, player.userIDValue(), player.guestIDValue(), oracleID, scryfallID, isDaily, dailyKey)
	return scanSpellifyGame(row)
}

func activeOrNewSpellifyGame(ctx context.Context, db *sql.DB, player gamePlayer) (*spellifyGame, error) {
	dailyKey := guessCardDailyKey(time.Now().UTC())
	game, err := loadActiveSpellifyGame(ctx, db, player)
	if err == nil {
		if game.DailyKey == dailyKey {
			return game, nil
		}
		if err := abandonActiveSpellifyGames(ctx, db, player); err != nil {
			return nil, err
		}
		attempted, err := hasSpellifyDailyAttempt(ctx, db, player, dailyKey)
		if err != nil {
			return nil, err
		}
		if attempted {
			return createSpellifyGame(ctx, db, player, false, dailyKey)
		}
		return createSpellifyGame(ctx, db, player, true, dailyKey)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	attempted, err := hasSpellifyDailyAttempt(ctx, db, player, dailyKey)
	if err != nil {
		return nil, err
	}
	if attempted {
		return createSpellifyGame(ctx, db, player, false, dailyKey)
	}
	return createSpellifyGame(ctx, db, player, true, dailyKey)
}

func hasSpellifyDailyAttempt(ctx context.Context, db *sql.DB, player gamePlayer, dailyKey string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_spellify_games
			WHERE (user_id = $1 OR guest_id = NULLIF($2, '')::uuid)
			  AND is_daily = true
			  AND daily_key = $3
			)
	`, player.userIDValue(), player.guestIDValue(), strings.TrimSpace(dailyKey)).Scan(&exists)
	return exists, err
}

func createReplaySpellifyGame(ctx context.Context, db *sql.DB, player gamePlayer) (*spellifyGame, error) {
	return createSpellifyGame(ctx, db, player, false, guessCardDailyKey(time.Now().UTC()))
}

func abandonActiveSpellifyGames(ctx context.Context, db *sql.DB, player gamePlayer) error {
	_, err := db.ExecContext(ctx, `
		UPDATE user_spellify_games
		SET status = 'abandoned', completed_at = now()
		WHERE (user_id = $1 OR guest_id = NULLIF($2, '')::uuid)
		  AND status = 'active'
	`, player.userIDValue(), player.guestIDValue())
	return err
}

func addSpellifyGuessChar(ctx context.Context, db *sql.DB, gameID int64, player gamePlayer, guessChar string) error {
	guessChar = strings.ToLower(strings.TrimSpace(guessChar))
	if guessChar == "" {
		return errSpellifyGuessUnavailable
	}
	result, err := db.ExecContext(ctx, `
		UPDATE user_spellify_games
		SET
			guessed_chars = array_append(guessed_chars, $4),
			guess_count = guess_count + 1
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
		  AND guess_count < $5
		  AND NOT ($4 = ANY(guessed_chars))
	`, gameID, player.userIDValue(), player.guestIDValue(), guessChar, spellifyMaxGuesses)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errSpellifyGuessUnavailable
	}
	return nil
}

func completeSpellifyGame(ctx context.Context, db *sql.DB, gameID int64, player gamePlayer, won bool) error {
	status := "lost"
	if won {
		status = "won"
	}
	result, err := db.ExecContext(ctx, `
		UPDATE user_spellify_games
		SET status = $4, completed_at = now()
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
	`, gameID, player.userIDValue(), player.guestIDValue(), status)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("spellify game could not be completed")
	}
	return nil
}

func completeSpellifyGameWithAward(ctx context.Context, db *sql.DB, game spellifyGame, card cards.Card) error {
	if game.UserID <= 0 {
		return errors.New("guest games cannot create awards")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE user_spellify_games
		SET status = 'won', completed_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'active'
	`, game.ID, game.UserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("spellify game could not be completed")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_spellify_awards (
			user_id, oracle_id, scryfall_id, card_name, image_uri, guess_count, daily_key
		)
		VALUES ($1, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7)
		ON CONFLICT (user_id, daily_key) DO NOTHING
	`, game.UserID, card.OracleID, game.TargetScryfallID, card.Name, card.ImageURI, game.GuessCount, game.DailyKey); err != nil {
		return err
	}
	return tx.Commit()
}

func listGuessCardAwardsPage(ctx context.Context, db *sql.DB, userID int64, limit, offset int) ([]guessCardAward, error) {
	if limit <= 0 {
		limit = 60
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := db.QueryContext(ctx, `
		SELECT
			id,
			user_id,
			oracle_id::text,
			COALESCE(scryfall_id::text, ''),
			card_name,
			image_uri,
			won_at,
			question_count,
			guess_count
		FROM user_guess_card_awards
		WHERE user_id = $1
		ORDER BY won_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]guessCardAward, 0, limit)
	for rows.Next() {
		var award guessCardAward
		if err := rows.Scan(
			&award.ID,
			&award.UserID,
			&award.OracleID,
			&award.ScryfallID,
			&award.CardName,
			&award.ImageURI,
			&award.WonAt,
			&award.QuestionCount,
			&award.GuessCount,
		); err != nil {
			return nil, err
		}
		out = append(out, award)
	}
	return out, rows.Err()
}

func listSpellifyAwardsPage(ctx context.Context, db *sql.DB, userID int64, limit, offset int) ([]spellifyAward, error) {
	if limit <= 0 {
		limit = 60
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, oracle_id::text, COALESCE(scryfall_id::text, ''),
		       card_name, image_uri, won_at, guess_count
		FROM user_spellify_awards
		WHERE user_id = $1
		ORDER BY won_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]spellifyAward, 0, limit)
	for rows.Next() {
		var award spellifyAward
		if err := rows.Scan(&award.ID, &award.UserID, &award.OracleID, &award.ScryfallID, &award.CardName, &award.ImageURI, &award.WonAt, &award.GuessCount); err != nil {
			return nil, err
		}
		out = append(out, award)
	}
	return out, rows.Err()
}

func countFavoritePrintings(ctx context.Context, db *sql.DB, userID int64) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_card_printing_favorites WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

func countGuessCardAwards(ctx context.Context, db *sql.DB, userID int64) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_guess_card_awards WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

func countSpellifyAwards(ctx context.Context, db *sql.DB, userID int64) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_spellify_awards WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}
