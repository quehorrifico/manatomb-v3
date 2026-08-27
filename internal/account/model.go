package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidPassword = errors.New("invalid password")
var ErrInvalidResetToken = errors.New("invalid password reset token")

type User struct {
	ID           int64
	Email        string
	DisplayName  string
	PasswordHash string
	SiteTheme    string
}

type PublicProfile struct {
	ID                       int64
	DisplayName              string
	ProfileAvatarCommander   string
	ProfilePicturePrintID    string
	ProfileBackgroundPrintID string
	CreatedAt                time.Time
}

type Session struct {
	ID        uuid.UUID
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type PasswordResetToken struct {
	Token     string
	UserID    int64
	Email     string
	ExpiresAt time.Time
}

var ErrInvalidCredentials = errors.New("invalid credentials")

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func CreateUser(ctx context.Context, db *sql.DB, email, displayName, password string) (*User, error) {
	email = normalizeEmail(email)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var u User
	err = db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id, email, display_name, password_hash, site_theme
	`, email, string(hash), displayName).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.SiteTheme)

	return &u, err
}

func Authenticate(ctx context.Context, db *sql.DB, email, password string) (*User, error) {
	email = normalizeEmail(email)
	var u User
	err := db.QueryRowContext(ctx, `
		SELECT id, email, display_name, password_hash, site_theme
		FROM users
		WHERE lower(email) = $1
	`, email).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.SiteTheme)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return &u, nil
}

func CreateSession(ctx context.Context, db *sql.DB, userID int64, ttl time.Duration) (*Session, error) {
	id := uuid.New()
	now := time.Now()
	s := &Session{
		ID:        id,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, created_at, expires_at)
		VALUES ($1, $2, $3, $4)
	`, s.ID, s.UserID, s.CreatedAt, s.ExpiresAt)
	return s, err
}

func newPasswordResetToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func passwordResetTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func CreatePasswordResetToken(ctx context.Context, db *sql.DB, email string, ttl time.Duration) (*PasswordResetToken, bool, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, false, nil
	}
	if ttl <= 0 {
		ttl = time.Hour
	}

	var userID int64
	var canonicalEmail string
	err := db.QueryRowContext(ctx, `
		SELECT id, email
		FROM users
		WHERE lower(email) = lower($1)
	`, email).Scan(&userID, &canonicalEmail)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	token, err := newPasswordResetToken()
	if err != nil {
		return nil, false, err
	}
	reset := &PasswordResetToken{
		Token:     token,
		UserID:    userID,
		Email:     canonicalEmail,
		ExpiresAt: time.Now().Add(ttl),
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE user_id = $1
		  AND used_at IS NULL
	`, userID); err != nil {
		return nil, false, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, passwordResetTokenHash(token), reset.ExpiresAt); err != nil {
		return nil, false, err
	}

	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return reset, true, nil
}

func ResetPasswordWithToken(ctx context.Context, db *sql.DB, token, newPassword string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrInvalidResetToken
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx, `
		SELECT user_id
		FROM password_reset_tokens
		WHERE token_hash = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
		FOR UPDATE
	`, passwordResetTokenHash(token)).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrInvalidResetToken
		}
		return err
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1
		WHERE id = $2
	`, string(newHash), userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE token_hash = $1
	`, passwordResetTokenHash(token)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE user_id = $1
	`, userID); err != nil {
		return err
	}

	return tx.Commit()
}

func GetUserBySession(ctx context.Context, db *sql.DB, sid uuid.UUID) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, u.password_hash, u.site_theme
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > NOW()
	`, sid).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.SiteTheme)

	if err != nil {
		return nil, err
	}
	return &u, nil
}

func DeleteSession(ctx context.Context, db *sql.DB, sid uuid.UUID) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sid)
	return err
}

func EnsureUserTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS users (
            id BIGSERIAL PRIMARY KEY,
            email TEXT NOT NULL UNIQUE,
            password_hash TEXT NOT NULL,
            display_name TEXT NOT NULL,
            profile_avatar_commander TEXT NOT NULL DEFAULT '',
			profile_picture_print_id UUID NULL,
			profile_background_print_id UUID NULL,
            site_theme TEXT NOT NULL DEFAULT 'tomb',
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        );
    `); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS profile_avatar_commander TEXT NOT NULL DEFAULT ''
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS profile_picture_print_id UUID NULL
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS profile_background_print_id UUID NULL
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS site_theme TEXT NOT NULL DEFAULT 'tomb'
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE users
		ALTER COLUMN site_theme SET DEFAULT 'tomb'
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_case_insensitive
		ON users (lower(email))
	`); err != nil {
		return fmt.Errorf("ensure case-insensitive email uniqueness (resolve any existing case-only duplicates first): %w", err)
	}
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens (user_id);
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens (token_hash);
	`)
	return err
}

func EnsureSessionsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS sessions (
            id TEXT PRIMARY KEY,
            user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            expires_at TIMESTAMPTZ NOT NULL
        );
		CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);
		CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions (expires_at);
		DELETE FROM sessions WHERE expires_at <= NOW();
    `)
	return err
}

func UpdateProfile(ctx context.Context, db *sql.DB, userID int64, displayName, email string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE users
		 SET display_name = $1,
		     email = $2
		 WHERE id = $3`,
		displayName, normalizeEmail(email), userID,
	)
	return err
}

func UpdateProfileAvatarCommander(ctx context.Context, db *sql.DB, userID int64, commanderName string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE users
		 SET profile_avatar_commander = $1
		 WHERE id = $2`,
		strings.TrimSpace(commanderName), userID,
	)
	return err
}

func UpdateProfilePicturePrint(ctx context.Context, db *sql.DB, userID int64, scryfallID string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE users
		SET profile_picture_print_id = NULLIF($1, '')::uuid
		WHERE id = $2
	`, strings.TrimSpace(scryfallID), userID)
	return err
}

func UpdateProfileBackgroundPrint(ctx context.Context, db *sql.DB, userID int64, scryfallID string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE users
		SET profile_background_print_id = NULLIF($1, '')::uuid
		WHERE id = $2
	`, strings.TrimSpace(scryfallID), userID)
	return err
}

func UpdateSiteTheme(ctx context.Context, db *sql.DB, userID int64, siteTheme string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE users
		 SET site_theme = $1
		 WHERE id = $2`,
		strings.TrimSpace(siteTheme), userID,
	)
	return err
}

func ChangePassword(ctx context.Context, db *sql.DB, userID int64, currentPassword, newPassword string) error {
	// Fetch hash
	var hash string
	err := db.QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE id = $1`,
		userID,
	).Scan(&hash)
	if err != nil {
		return err
	}

	// Check current password
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)); err != nil {
		return ErrInvalidPassword
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		string(newHash), userID,
	)
	return err
}

func DeleteAccount(ctx context.Context, db *sql.DB, userID int64) error {
	// Order matters if you don't have ON DELETE CASCADE.
	// If you do have foreign keys with cascade, some of this may be redundant.

	// Delete deck_cards via decks
	if _, err := db.ExecContext(ctx,
		`DELETE FROM deck_cards WHERE deck_id IN (SELECT id FROM decks WHERE user_id = $1)`,
		userID,
	); err != nil {
		return err
	}

	// Delete decks
	if _, err := db.ExecContext(ctx,
		`DELETE FROM decks WHERE user_id = $1`,
		userID,
	); err != nil {
		return err
	}

	// Delete sessions
	if _, err := db.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = $1`,
		userID,
	); err != nil {
		return err
	}

	// Delete user
	if _, err := db.ExecContext(ctx,
		`DELETE FROM users WHERE id = $1`,
		userID,
	); err != nil {
		return err
	}

	return nil
}

func GetPublicProfileByID(ctx context.Context, db *sql.DB, userID int64) (*PublicProfile, error) {
	var profile PublicProfile
	err := db.QueryRowContext(ctx, `
		SELECT id,
		       display_name,
		       COALESCE(profile_avatar_commander, ''),
		       COALESCE(profile_picture_print_id::text, ''),
		       COALESCE(profile_background_print_id::text, ''),
		       created_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&profile.ID,
		&profile.DisplayName,
		&profile.ProfileAvatarCommander,
		&profile.ProfilePicturePrintID,
		&profile.ProfileBackgroundPrintID,
		&profile.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func ListPublicProfilesByIDs(ctx context.Context, db *sql.DB, userIDs []int64) (map[int64]PublicProfile, error) {
	out := make(map[int64]PublicProfile)
	if len(userIDs) == 0 {
		return out, nil
	}

	seen := make(map[int64]struct{}, len(userIDs))
	args := make([]any, 0, len(userIDs))
	placeholders := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		args = append(args, userID)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
	}
	if len(args) == 0 {
		return out, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id,
		       display_name,
		       COALESCE(profile_avatar_commander, ''),
		       COALESCE(profile_picture_print_id::text, ''),
		       COALESCE(profile_background_print_id::text, ''),
		       created_at
		FROM users
		WHERE id IN (`+strings.Join(placeholders, ", ")+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var profile PublicProfile
		if err := rows.Scan(
			&profile.ID,
			&profile.DisplayName,
			&profile.ProfileAvatarCommander,
			&profile.ProfilePicturePrintID,
			&profile.ProfileBackgroundPrintID,
			&profile.CreatedAt,
		); err != nil {
			return nil, err
		}
		out[profile.ID] = profile
	}
	return out, rows.Err()
}
