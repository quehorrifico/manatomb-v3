package decks

import (
	"context"
	"database/sql"
	"fmt"
	"manatomb/app/internal/cards"
)

func EnsureDeckRoleTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS deck_card_role_overrides(deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,role TEXT NOT NULL,PRIMARY KEY(deck_id,oracle_id))`)
	return err
}

func SetCardRole(ctx context.Context, db *sql.DB, deckID, userID int64, oracleID, role string) error {
	if !cards.ValidRole(role) {
		return fmt.Errorf("invalid card role")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owned int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM decks WHERE id=$1 AND user_id=$2 FOR UPDATE`, deckID, userID).Scan(&owned); err != nil {
		return err
	}
	var present bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM deck_cards WHERE deck_id=$1 AND oracle_id=$2::uuid) OR EXISTS(SELECT 1 FROM decks d JOIN oracle_cards oc ON oc.name_search=normalize_card_name(d.commander_name) WHERE d.id=$1 AND oc.oracle_id=$2::uuid)`, deckID, oracleID).Scan(&present); err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("card is not in this deck")
	}
	if role == "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM deck_card_role_overrides WHERE deck_id=$1 AND oracle_id=$2::uuid`, deckID, oracleID)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO deck_card_role_overrides(deck_id,oracle_id,role) VALUES($1,$2::uuid,$3) ON CONFLICT(deck_id,oracle_id) DO UPDATE SET role=EXCLUDED.role`, deckID, oracleID, role)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func validImportedRole(role string) bool { return cards.ValidRole(role) }
