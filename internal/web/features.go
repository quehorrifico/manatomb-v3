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
	ID                  int64
	UserID              int64
	GuestID             string
	TargetOracleID      string
	TargetScryfallID    string
	Status              string
	QuestionCount       int
	GuessCount          int
	IsDaily             bool
	DailyKey            string
	AwardEarned         bool
	MaxQuestions        int
	AskedQuestions      []string
	WrongGuessOracleIDs []string
	HistoryEvents       []string
	CreatedAt           time.Time
	CompletedAt         *time.Time
}

type spellifyGame struct {
	ID                   int64
	UserID               int64
	GuestID              string
	TargetOracleID       string
	TargetScryfallID     string
	Status               string
	GuessedChars         []string
	GuessCount           int
	CardGuessCount       int
	PreviousWrongGuesses []string
	IsDaily              bool
	DailyKey             string
	CreatedAt            time.Time
	CompletedAt          *time.Time
}

type guessCardWin struct {
	ID            int64
	UserID        int64
	OracleID      string
	ScryfallID    string
	CardName      string
	ImageURI      string
	WonAt         time.Time
	QuestionCount int
	GuessCount    int
	IsDaily       bool
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
var errGuessCardGameUnavailable = errors.New("guess card game unavailable")
var errGuessCardAttemptDuplicate = errors.New("guess card attempt already recorded")
var errSpellifyGuessUnavailable = errors.New("spellify guess unavailable")
var errSpellifyCardGuessUnavailable = errors.New("spellify card guess unavailable")

const (
	guessCardHistoryQuestionPrefix = "question:"
	guessCardHistoryGuessPrefix    = "guess:"
	guessCardHistoryGuessMaxRunes  = 256
)

type guessCardAttemptResult struct {
	GuessCount int
	Awarded    bool
	Won        bool
}

type spellifyCardGuessResult struct {
	CardGuessCount int
	Exhausted      bool
}

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
		`ALTER TABLE user_card_printing_favorites ADD COLUMN IF NOT EXISTS oracle_id UUID;`,
		`
		UPDATE user_card_printing_favorites AS favorite
		SET oracle_id = print.oracle_id
		FROM card_prints AS print
		WHERE print.scryfall_id = favorite.scryfall_id
		  AND favorite.oracle_id IS DISTINCT FROM print.oracle_id;
		`,
		`CREATE INDEX IF NOT EXISTS idx_user_card_printing_favorites_oracle ON user_card_printing_favorites (user_id, oracle_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_card_printing_favorites_user_created ON user_card_printing_favorites (user_id, created_at DESC);`,
		`
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
		`,
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
			award_earned BOOLEAN NOT NULL DEFAULT false,
			max_questions INT NOT NULL DEFAULT 8,
			asked_questions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			wrong_guess_oracle_ids UUID[] NOT NULL DEFAULT ARRAY[]::UUID[],
			history_events TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
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
		`ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS award_earned BOOLEAN NOT NULL DEFAULT false;`,
		`ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS wrong_guess_oracle_ids UUID[] NOT NULL DEFAULT ARRAY[]::UUID[];`,
		`ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS history_events TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];`,
		`
		UPDATE user_guess_card_games
		SET history_events = ARRAY(
			SELECT 'question:' || question_id
			FROM unnest(asked_questions) WITH ORDINALITY AS asked(question_id, position)
			ORDER BY position
		)
		WHERE cardinality(history_events) = 0
		  AND cardinality(asked_questions) > 0;
		`,
		`ALTER TABLE user_guess_card_games ALTER COLUMN max_questions SET DEFAULT 8;`,
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
		UPDATE user_guess_card_games AS game
		SET award_earned = true
		WHERE game.award_earned = false
		  AND game.status = 'won'
		  AND game.completed_at IS NOT NULL
		  AND EXISTS (
			SELECT 1
			FROM user_guess_card_awards AS award
			WHERE award.user_id = game.user_id
			  AND award.scryfall_id = game.target_scryfall_id
			  AND award.won_at BETWEEN game.completed_at - interval '1 minute'
			                       AND game.completed_at + interval '1 minute'
		  );
		`,
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
			card_guess_count INT NOT NULL DEFAULT 0,
			previous_wrong_guesses TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
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
		`ALTER TABLE user_spellify_games ADD COLUMN IF NOT EXISTS card_guess_count INT NOT NULL DEFAULT 0;`,
		`ALTER TABLE user_spellify_games ADD COLUMN IF NOT EXISTS previous_wrong_guesses TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];`,
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
		var storedScryfallID string
		err := db.QueryRowContext(ctx, `
			INSERT INTO user_card_printing_favorites (user_id, scryfall_id, oracle_id)
			SELECT $1, cp.scryfall_id, cp.oracle_id
			FROM card_prints cp
			WHERE cp.scryfall_id = $2::uuid
			ON CONFLICT (user_id, scryfall_id) DO UPDATE
			SET oracle_id = EXCLUDED.oracle_id
			RETURNING scryfall_id::text
		`, userID, scryfallID).Scan(&storedScryfallID)
		if errors.Is(err, sql.ErrNoRows) {
			return cards.ErrCardNotFound
		}
		if err != nil {
			return err
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
	if page == nil {
		return
	}

	normalizedFavoriteIDs := make(map[string]bool, len(favoriteIDs))
	for id, favorited := range favoriteIDs {
		id = strings.ToLower(strings.TrimSpace(id))
		if id != "" && favorited {
			normalizedFavoriteIDs[id] = true
		}
	}

	page.FavoritePrintingCount = len(normalizedFavoriteIDs)
	selectedPrintingID := strings.ToLower(strings.TrimSpace(page.Card.ScryfallID))
	page.SelectedPrintingIsFavorited = normalizedFavoriteIDs[selectedPrintingID]
	page.OtherFavoritePrintingCount = page.FavoritePrintingCount
	if page.SelectedPrintingIsFavorited && page.OtherFavoritePrintingCount > 0 {
		page.OtherFavoritePrintingCount--
	}

	for i := range page.Printings {
		printingID := strings.ToLower(strings.TrimSpace(page.Printings[i].ScryfallID))
		page.Printings[i].IsFavorited = normalizedFavoriteIDs[printingID]
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
			award_earned,
			max_questions,
			asked_questions,
			wrong_guess_oracle_ids,
			history_events,
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
			award_earned,
			max_questions,
			asked_questions,
			wrong_guess_oracle_ids,
			history_events,
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
		&game.AwardEarned,
		&game.MaxQuestions,
		pq.Array(&game.AskedQuestions),
		pq.Array(&game.WrongGuessOracleIDs),
		pq.Array(&game.HistoryEvents),
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

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var lockResult any
	if err := tx.QueryRowContext(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
	`, guessCardOwnerLockKey(player)).Scan(&lockResult); err != nil {
		return nil, err
	}

	active, err := scanGuessCardGame(tx.QueryRowContext(ctx, `
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
			award_earned,
			max_questions,
			asked_questions,
			wrong_guess_oracle_ids,
			history_events,
			created_at,
			completed_at
		FROM user_guess_card_games
		WHERE (user_id = $1 OR guest_id = NULLIF($2, '')::uuid)
		  AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`, player.userIDValue(), player.guestIDValue()))
	if err == nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return active, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	game, err := scanGuessCardGame(tx.QueryRowContext(ctx, `
		INSERT INTO user_guess_card_games (user_id, guest_id, target_oracle_id, target_scryfall_id, is_daily, daily_key, max_questions)
		VALUES ($1, NULLIF($2, '')::uuid, $3::uuid, NULLIF($4, '')::uuid, $5, $6, $7)
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
			award_earned,
			max_questions,
			asked_questions,
			wrong_guess_oracle_ids,
			history_events,
			created_at,
			completed_at
	`, player.userIDValue(), player.guestIDValue(), oracleID, scryfallID, isDaily, dailyKey, guessCardDefaultMaxQuestions))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return game, nil
}

func guessCardOwnerLockKey(player gamePlayer) string {
	if player.UserID > 0 {
		return fmt.Sprintf("guess-card:user:%d", player.UserID)
	}
	return "guess-card:guest:" + strings.ToLower(strings.TrimSpace(player.GuestID))
}

func guessCardDailyKey(now time.Time) string {
	return now.UTC().Format("2006-01-02")
}

func selectGuessCardTarget(ctx context.Context, db *sql.DB) (string, string, error) {
	return selectRandomGameTarget(ctx, db, guessCardEligiblePoolPredicateSQL)
}

func selectTombscriptTarget(ctx context.Context, db *sql.DB) (string, string, error) {
	return selectRandomGameTarget(ctx, db, sharedGameEligiblePoolPredicateSQL)
}

func selectRandomGameTarget(ctx context.Context, db *sql.DB, eligiblePoolPredicate string) (string, string, error) {
	var oracleID, scryfallID string
	if err := db.QueryRowContext(ctx, `
		SELECT
			oc.oracle_id::text,
			COALESCE(oc.default_print_id::text, '')
		FROM oracle_cards oc
		WHERE `+eligiblePoolPredicate+`
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

func selectDailyGameTarget(ctx context.Context, db queryRower, dailyKey, eligiblePoolPredicate string) (string, string, error) {
	dailyKey = strings.TrimSpace(dailyKey)
	if dailyKey == "" {
		return "", "", cards.ErrCardNotFound
	}

	var count int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM oracle_cards oc
		WHERE `+eligiblePoolPredicate+`
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
		WHERE `+eligiblePoolPredicate+`
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
	eligiblePoolPredicate := sharedGameEligiblePoolPredicateSQL
	if mode == "guess-card" {
		eligiblePoolPredicate = guessCardEligiblePoolPredicateSQL
	}

	var oracleID, scryfallID string
	err = tx.QueryRowContext(ctx, `
		SELECT dt.target_oracle_id::text, COALESCE(dt.target_scryfall_id::text, '')
		FROM daily_game_targets dt
		JOIN oracle_cards oc ON oc.oracle_id = dt.target_oracle_id
		WHERE dt.mode = $1 AND dt.daily_key = $2
		  AND (`+eligiblePoolPredicate+`)
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

	oracleID, scryfallID, err = selectDailyGameTarget(ctx, tx, mode+":"+dailyKey, eligiblePoolPredicate)
	if err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO daily_game_targets (mode, daily_key, target_oracle_id, target_scryfall_id)
		VALUES ($1, $2, $3::uuid, NULLIF($4, '')::uuid)
		ON CONFLICT (mode, daily_key) DO UPDATE
		SET target_oracle_id = EXCLUDED.target_oracle_id,
		    target_scryfall_id = EXCLUDED.target_scryfall_id
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

func abandonGuessCardGame(ctx context.Context, db *sql.DB, player gamePlayer, gameID int64) error {
	_, err := db.ExecContext(ctx, `
		UPDATE user_guess_card_games
		SET status = 'abandoned', completed_at = now()
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
	`, gameID, player.userIDValue(), player.guestIDValue())
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
			history_events = (
				CASE
					WHEN cardinality(history_events) = 0 AND cardinality(asked_questions) > 0 THEN ARRAY(
						SELECT $7 || prior_question_id
						FROM unnest(asked_questions) WITH ORDINALITY AS prior(prior_question_id, position)
						ORDER BY position
					)
					ELSE history_events
				END
			) || ARRAY[$6],
			question_count = question_count + 1
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
		  AND NOT ($4 = ANY(asked_questions))
		  AND question_count < CASE WHEN max_questions > 0 THEN max_questions ELSE $5 END
	`, gameID, player.userIDValue(), player.guestIDValue(), questionID, guessCardDefaultMaxQuestions, guessCardHistoryQuestionPrefix+questionID, guessCardHistoryQuestionPrefix)
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

// recordGuessCardFinalGuess counts an exact-name guess and, when it is correct,
// completes the round and records its award in the same transaction. Keeping
// these writes together prevents duplicate browser submissions from counting
// twice or leaving a solved game active after an award has been inserted.
func recordGuessCardFinalGuess(
	ctx context.Context,
	db *sql.DB,
	gameID int64,
	player gamePlayer,
	card cards.Card,
	guessedOracleID string,
	guessedName string,
	expectedGuessNumber int,
) (guessCardAttemptResult, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return guessCardAttemptResult{}, err
	}
	defer tx.Rollback()

	var (
		guessCount          int
		questionCount       int
		isDaily             bool
		dailyKey            string
		userID              sql.NullInt64
		targetScryfallID    string
		targetOracleID      string
		askedQuestions      []string
		wrongGuessOracleIDs []string
		historyEvents       []string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			guess_count,
			question_count,
			is_daily,
			daily_key,
			user_id,
			target_oracle_id::text,
			COALESCE(target_scryfall_id::text, ''),
			asked_questions,
			wrong_guess_oracle_ids,
			history_events
		FROM user_guess_card_games
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
		FOR UPDATE
	`, gameID, player.userIDValue(), player.guestIDValue()).Scan(
		&guessCount,
		&questionCount,
		&isDaily,
		&dailyKey,
		&userID,
		&targetOracleID,
		&targetScryfallID,
		pq.Array(&askedQuestions),
		pq.Array(&wrongGuessOracleIDs),
		pq.Array(&historyEvents),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return guessCardAttemptResult{}, errGuessCardGameUnavailable
	}
	if err != nil {
		return guessCardAttemptResult{}, err
	}
	if guessCount+1 != expectedGuessNumber {
		return guessCardAttemptResult{}, errGuessCardAttemptDuplicate
	}
	if !strings.EqualFold(strings.TrimSpace(card.OracleID), strings.TrimSpace(targetOracleID)) {
		return guessCardAttemptResult{}, errors.New("guess card target does not match the locked round")
	}

	guessCount++
	won := strings.TrimSpace(guessedOracleID) != "" &&
		strings.EqualFold(strings.TrimSpace(guessedOracleID), strings.TrimSpace(targetOracleID))
	if !won {
		if len(historyEvents) == 0 {
			for _, questionID := range askedQuestions {
				questionID = strings.TrimSpace(questionID)
				if questionID != "" {
					historyEvents = append(historyEvents, guessCardHistoryQuestionPrefix+questionID)
				}
			}
		}
		if guessedName = guessCardPersistedGuessName(guessedName); guessedName != "" {
			historyEvents = append(historyEvents, guessCardHistoryGuessPrefix+guessedName)
		}
		wrongGuessOracleIDs = appendResolvedWrongGuessOracleID(
			wrongGuessOracleIDs,
			guessedOracleID,
			targetOracleID,
		)
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_guess_card_games
			SET guess_count = $2,
				wrong_guess_oracle_ids = $3::uuid[],
				history_events = $4::text[]
			WHERE id = $1 AND status = 'active'
		`, gameID, guessCount, pq.Array(wrongGuessOracleIDs), pq.Array(historyEvents)); err != nil {
			return guessCardAttemptResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return guessCardAttemptResult{}, err
		}
		return guessCardAttemptResult{GuessCount: guessCount, Won: false}, nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE user_guess_card_games
		SET guess_count = $2, status = 'won', completed_at = now()
		WHERE id = $1 AND status = 'active'
	`, gameID, guessCount); err != nil {
		return guessCardAttemptResult{}, err
	}

	result := guessCardAttemptResult{GuessCount: guessCount, Won: true}
	awardEligible := userID.Valid && userID.Int64 > 0 &&
		guessCardGameAwardEligible(guessCardGame{IsDaily: isDaily, DailyKey: dailyKey})
	if awardEligible {
		awardScryfallID := strings.TrimSpace(card.ID)
		if awardScryfallID == "" {
			awardScryfallID = strings.TrimSpace(targetScryfallID)
		}
		insertResult, err := tx.ExecContext(ctx, `
			INSERT INTO user_guess_card_awards (
				user_id, oracle_id, scryfall_id, card_name, image_uri, question_count, guess_count
			)
			VALUES ($1, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7)
			ON CONFLICT (user_id, scryfall_id) DO NOTHING
		`, userID.Int64, targetOracleID, awardScryfallID, card.Name, card.ImageURI, questionCount, guessCount)
		if err != nil {
			return guessCardAttemptResult{}, err
		}
		if affected, err := insertResult.RowsAffected(); err != nil {
			return guessCardAttemptResult{}, err
		} else {
			result.Awarded = affected > 0
		}
		if result.Awarded {
			if _, err := tx.ExecContext(ctx, `
				UPDATE user_guess_card_games
				SET award_earned = true
				WHERE id = $1
			`, gameID); err != nil {
				return guessCardAttemptResult{}, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return guessCardAttemptResult{}, err
	}
	return result, nil
}

// guessCardPersistedGuessName bounds and normalizes user-provided history
// before it reaches the round's PostgreSQL text array. Templates still escape
// the value at render time.
func guessCardPersistedGuessName(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > guessCardHistoryGuessMaxRunes {
		runes = runes[:guessCardHistoryGuessMaxRunes]
	}
	return string(runes)
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
		return errGuessCardGameUnavailable
	}
	return nil
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
			card_guess_count,
			previous_wrong_guesses,
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
			card_guess_count,
			previous_wrong_guesses,
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
		&game.CardGuessCount,
		pq.Array(&game.PreviousWrongGuesses),
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
		oracleID, scryfallID, err = selectTombscriptTarget(ctx, db)
	}
	if err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var lockResult any
	if err := tx.QueryRowContext(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
	`, spellifyOwnerLockKey(player)).Scan(&lockResult); err != nil {
		return nil, err
	}

	active, err := scanSpellifyGame(tx.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			COALESCE(guest_id::text, ''),
			target_oracle_id::text,
			COALESCE(target_scryfall_id::text, ''),
			status,
			guessed_chars,
			guess_count,
			card_guess_count,
			previous_wrong_guesses,
			is_daily,
			daily_key,
			created_at,
			completed_at
		FROM user_spellify_games
		WHERE (user_id = $1 OR guest_id = NULLIF($2, '')::uuid)
		  AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`, player.userIDValue(), player.guestIDValue()))
	if err == nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return active, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	game, err := scanSpellifyGame(tx.QueryRowContext(ctx, `
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
			card_guess_count,
			previous_wrong_guesses,
			is_daily,
			daily_key,
			created_at,
			completed_at
	`, player.userIDValue(), player.guestIDValue(), oracleID, scryfallID, isDaily, dailyKey))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return game, nil
}

func spellifyOwnerLockKey(player gamePlayer) string {
	if player.UserID > 0 {
		return fmt.Sprintf("spellify:user:%d", player.UserID)
	}
	return "spellify:guest:" + strings.ToLower(strings.TrimSpace(player.GuestID))
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

// recordSpellifyWrongCardGuess consumes one of the round's separate card-name
// attempts without changing its character-reveal count. The final failed
// attempt closes the round so it cannot remain active with no valid actions.
func recordSpellifyWrongCardGuess(
	ctx context.Context,
	db *sql.DB,
	gameID int64,
	player gamePlayer,
	wrongGuess string,
	maxCardGuesses int,
) (spellifyCardGuessResult, error) {
	if maxCardGuesses <= 0 {
		return spellifyCardGuessResult{}, errSpellifyCardGuessUnavailable
	}
	wrongGuess = spellifyPersistedWrongGuess(wrongGuess)
	if wrongGuess == "" {
		return spellifyCardGuessResult{}, errSpellifyCardGuessUnavailable
	}
	var result spellifyCardGuessResult
	err := db.QueryRowContext(ctx, `
		UPDATE user_spellify_games
		SET
			card_guess_count = card_guess_count + 1,
			previous_wrong_guesses = array_append(previous_wrong_guesses, $4),
			status = CASE
				WHEN card_guess_count + 1 >= $5 THEN 'lost'
				ELSE status
			END,
			completed_at = CASE
				WHEN card_guess_count + 1 >= $5 THEN now()
				ELSE completed_at
			END
		WHERE id = $1
		  AND (user_id = $2 OR guest_id = NULLIF($3, '')::uuid)
		  AND status = 'active'
		  AND card_guess_count < $5
		RETURNING card_guess_count, status = 'lost'
	`, gameID, player.userIDValue(), player.guestIDValue(), wrongGuess, maxCardGuesses).Scan(
		&result.CardGuessCount,
		&result.Exhausted,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return spellifyCardGuessResult{}, errSpellifyCardGuessUnavailable
	}
	return result, err
}

const spellifyPersistedWrongGuessMaxRunes = 256

// spellifyPersistedWrongGuess keeps user-submitted history safe for a
// PostgreSQL text array and bounded for later page/JSON rendering. It preserves
// the submitted wording while normalizing whitespace and invalid byte/NUL data.
func spellifyPersistedWrongGuess(value string) string {
	value = strings.ToValidUTF8(value, "�")
	value = strings.ReplaceAll(value, "\x00", "�")
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > spellifyPersistedWrongGuessMaxRunes {
		runes = runes[:spellifyPersistedWrongGuessMaxRunes]
	}
	return string(runes)
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
	if !game.IsDaily || strings.TrimSpace(game.DailyKey) != guessCardDailyKey(time.Now().UTC()) {
		return errors.New("only the first Tombscript game of the day can create awards")
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

func listGuessCardWinsPage(ctx context.Context, db *sql.DB, userID int64, limit, offset int) ([]guessCardWin, error) {
	if limit <= 0 {
		limit = 60
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := db.QueryContext(ctx, `
		SELECT
			game.id,
			game.user_id,
			game.target_oracle_id::text,
			COALESCE(game.target_scryfall_id::text, ''),
			oracle_card.name,
			COALESCE(
				NULLIF(print.image_uri, ''),
				NULLIF(oracle_card.default_image_uri, ''),
				''
			),
			COALESCE(game.completed_at, game.created_at),
			game.question_count,
			game.guess_count,
			game.is_daily
		FROM user_guess_card_games AS game
		JOIN oracle_cards AS oracle_card
		  ON oracle_card.oracle_id = game.target_oracle_id
		LEFT JOIN card_prints AS print
		  ON print.scryfall_id = game.target_scryfall_id
		WHERE game.user_id = $1
		  AND game.status = 'won'
		ORDER BY COALESCE(game.completed_at, game.created_at) DESC, game.id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]guessCardWin, 0, limit)
	for rows.Next() {
		var win guessCardWin
		if err := rows.Scan(
			&win.ID,
			&win.UserID,
			&win.OracleID,
			&win.ScryfallID,
			&win.CardName,
			&win.ImageURI,
			&win.WonAt,
			&win.QuestionCount,
			&win.GuessCount,
			&win.IsDaily,
		); err != nil {
			return nil, err
		}
		out = append(out, win)
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
		SELECT id, user_id, oracle_id, scryfall_id, card_name, image_uri, won_at, guess_count
		FROM (
			SELECT DISTINCT ON (
				COALESCE(NULLIF(game.daily_key, ''), 'legacy:' || game.id::text)
			)
				game.id,
				game.user_id,
				game.target_oracle_id::text AS oracle_id,
				COALESCE(game.target_scryfall_id::text, '') AS scryfall_id,
				oracle_card.name AS card_name,
				COALESCE(
					NULLIF(print.image_uri, ''),
					NULLIF(oracle_card.default_image_uri, ''),
					''
				) AS image_uri,
				COALESCE(game.completed_at, game.created_at) AS won_at,
				game.guess_count
			FROM user_spellify_games AS game
			JOIN oracle_cards AS oracle_card
			  ON oracle_card.oracle_id = game.target_oracle_id
			LEFT JOIN card_prints AS print
			  ON print.scryfall_id = game.target_scryfall_id
			WHERE game.user_id = $1
			  AND game.is_daily = true
			  AND game.status = 'won'
			ORDER BY
				COALESCE(NULLIF(game.daily_key, ''), 'legacy:' || game.id::text),
				COALESCE(game.completed_at, game.created_at) DESC,
				game.id DESC
		) AS daily_wins
		ORDER BY won_at DESC, id DESC
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

func countGuessCardWins(ctx context.Context, db *sql.DB, userID int64) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM user_guess_card_games
		WHERE user_id = $1
		  AND status = 'won'
	`, userID).Scan(&count)
	return count, err
}

func countSpellifyAwards(ctx context.Context, db *sql.DB, userID int64) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT COALESCE(NULLIF(daily_key, ''), 'legacy:' || id::text))
		FROM user_spellify_games
		WHERE user_id = $1
		  AND is_daily = true
		  AND status = 'won'
	`, userID).Scan(&count)
	return count, err
}
