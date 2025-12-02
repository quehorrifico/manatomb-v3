package account

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int64
	Email        string
	DisplayName  string
	PasswordHash string
}

type Session struct {
	ID        uuid.UUID
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

var ErrInvalidCredentials = errors.New("invalid credentials")

func CreateUser(ctx context.Context, db *sql.DB, email, displayName, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var u User
	err = db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id, email, display_name, password_hash
	`, email, string(hash), displayName).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash)

	return &u, err
}

func Authenticate(ctx context.Context, db *sql.DB, email, password string) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx, `
		SELECT id, email, display_name, password_hash
		FROM users
		WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash)
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

func GetUserBySession(ctx context.Context, db *sql.DB, sid uuid.UUID) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, u.password_hash
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > NOW()
	`, sid).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash)

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
	_, err := db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS users (
            id BIGSERIAL PRIMARY KEY,
            email TEXT NOT NULL UNIQUE,
            password_hash TEXT NOT NULL,
            display_name TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        );
    `)
	return err
}
