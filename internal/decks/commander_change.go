package decks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// CommanderChange describes a commander swap inside an existing deck. The new
// commander must already be present in the mainboard; the operation moves one
// copy into the commander slot and returns the previous commander to the
// mainboard without exposing an intermediate, partially-updated deck.
type CommanderChange struct {
	NewCommanderName     string
	NewCommanderOracleID string
	NewCommanderPrintID  string
}

// ChangeCommander atomically updates the commander slot and the corresponding
// mainboard quantities/printing choices.
func ChangeCommander(ctx context.Context, db *sql.DB, deckID int64, change CommanderChange) error {
	if deckID <= 0 {
		return fmt.Errorf("invalid deck id")
	}

	newName := strings.TrimSpace(change.NewCommanderName)
	newOracleID := strings.TrimSpace(change.NewCommanderOracleID)
	newPrintID := strings.TrimSpace(change.NewCommanderPrintID)
	if newName == "" || newOracleID == "" {
		return fmt.Errorf("commander name and oracle id are required")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		oldName    string
		oldPrintID sql.NullString
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			COALESCE(commander_name, ''),
			commander_print_id::text
		FROM decks
		WHERE id = $1
		FOR UPDATE
	`, deckID).Scan(&oldName, &oldPrintID)
	if err != nil {
		return err
	}
	oldName = strings.TrimSpace(oldName)

	var newCandidate bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM oracle_cards
			WHERE oracle_id = $1::uuid
			  AND name_search = normalize_card_name($2)
			  AND is_commander_candidate = TRUE
		)
	`, newOracleID, newName).Scan(&newCandidate)
	if err != nil {
		return err
	}
	if !newCandidate {
		return fmt.Errorf("card is not a valid commander")
	}

	if newPrintID != "" {
		if err := validateCommanderPrintTx(ctx, tx, newName, newPrintID); err != nil {
			return err
		}
	}

	if strings.EqualFold(oldName, newName) {
		result, err := tx.ExecContext(ctx, `
			UPDATE decks
			SET commander_name = $2,
			    commander_print_id = NULLIF($3, '')::uuid,
			    updated_at = NOW()
			WHERE id = $1
		`, deckID, newName, newPrintID)
		if err != nil {
			return err
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if updated == 0 {
			return sql.ErrNoRows
		}
		return tx.Commit()
	}

	newQty, err := deckCardQtyInBoard(ctx, tx, deckID, newOracleID, "main")
	if err != nil {
		return err
	}
	if newQty <= 0 {
		return fmt.Errorf("new commander is not in the mainboard")
	}

	var oldOracleID string
	if oldName != "" {
		err = tx.QueryRowContext(ctx, `
			SELECT oracle_id::text
			FROM oracle_cards
			WHERE name_search = normalize_card_name($1)
			ORDER BY COALESCE(edhrec_rank, 2147483647), name
			LIMIT 1
		`, oldName).Scan(&oldOracleID)
		if err != nil {
			return fmt.Errorf("resolve previous commander: %w", err)
		}
		oldOracleID = strings.TrimSpace(oldOracleID)
	}

	if err := adjustDeckCardQty(ctx, tx, deckID, newOracleID, "main", -1); err != nil {
		return err
	}

	if oldOracleID != "" {
		if err := adjustDeckCardQty(ctx, tx, deckID, oldOracleID, "main", 1); err != nil {
			return err
		}
		if oldPrintID.Valid && strings.TrimSpace(oldPrintID.String) != "" {
			result, err := tx.ExecContext(ctx, `
				UPDATE deck_cards
				SET preferred_print_id = $4::uuid
				WHERE deck_id = $1
				  AND oracle_id = $2::uuid
				  AND board = $3
			`, deckID, oldOracleID, "main", strings.TrimSpace(oldPrintID.String))
			if err != nil {
				return err
			}
			updated, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if updated == 0 {
				return fmt.Errorf("could not preserve previous commander printing")
			}
		}
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE decks
		SET commander_name = $2,
		    commander_print_id = NULLIF($3, '')::uuid,
		    updated_at = NOW()
		WHERE id = $1
	`, deckID, newName, newPrintID)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return sql.ErrNoRows
	}

	return tx.Commit()
}
