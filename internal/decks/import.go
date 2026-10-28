package decks

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// ImportedDeckCardInput is a fully resolved card row ready to persist. Name
// resolution belongs in the review layer; this type only accepts stable IDs.
type ImportedDeckCardInput struct {
	OracleID         string
	Qty              int
	Board            string
	PreferredPrintID string
}

type importedDeckCardKey struct {
	oracleID string
	board    string
}

func normalizeImportedDeckCards(items []ImportedDeckCardInput) ([]ImportedDeckCardInput, error) {
	combined := make(map[importedDeckCardKey]ImportedDeckCardInput, len(items))
	for _, raw := range items {
		oracleID := strings.TrimSpace(raw.OracleID)
		board := normalizeBoard(raw.Board)
		printID := strings.TrimSpace(raw.PreferredPrintID)
		if oracleID == "" {
			return nil, fmt.Errorf("imported card is missing an oracle id")
		}
		if board == "" {
			return nil, fmt.Errorf("imported card has an invalid board")
		}
		if raw.Qty <= 0 {
			return nil, fmt.Errorf("imported card quantity must be positive")
		}

		key := importedDeckCardKey{oracleID: strings.ToLower(oracleID), board: board}
		item, exists := combined[key]
		if !exists {
			combined[key] = ImportedDeckCardInput{
				OracleID:         oracleID,
				Qty:              raw.Qty,
				Board:            board,
				PreferredPrintID: printID,
			}
			continue
		}

		if item.PreferredPrintID != "" && printID != "" && !strings.EqualFold(item.PreferredPrintID, printID) {
			return nil, fmt.Errorf("one card cannot use two printings on the same board")
		}
		item.Qty += raw.Qty
		if item.PreferredPrintID == "" {
			item.PreferredPrintID = printID
		}
		combined[key] = item
	}

	out := make([]ImportedDeckCardInput, 0, len(combined))
	for _, item := range combined {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Board != out[j].Board {
			return out[i].Board < out[j].Board
		}
		return strings.ToLower(out[i].OracleID) < strings.ToLower(out[j].OracleID)
	})
	return out, nil
}

// CreateImportedDeck persists the deck and every resolved board row in one
// transaction. Any validation or insert error rolls the entire import back.
func CreateImportedDeck(
	ctx context.Context,
	db *sql.DB,
	userID int64,
	input DeckInput,
	items []ImportedDeckCardInput,
) (*Deck, error) {
	normalized, err := normalizeImportedDeckCards(items)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 && strings.TrimSpace(input.CommanderName) == "" {
		return nil, fmt.Errorf("an imported deck must contain at least one card")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	deck, err := insertDeckTx(ctx, tx, userID, input)
	if err != nil {
		return nil, err
	}

	for _, item := range normalized {
		if item.PreferredPrintID != "" {
			var matches bool
			if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM card_prints
					WHERE scryfall_id = $1::uuid
					  AND oracle_id = $2::uuid
				)
			`, item.PreferredPrintID, item.OracleID).Scan(&matches); err != nil {
				return nil, err
			}
			if !matches {
				return nil, fmt.Errorf("preferred printing does not match imported card")
			}
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO deck_cards (deck_id, oracle_id, qty, board, preferred_print_id)
			VALUES ($1, $2::uuid, $3, $4, COALESCE(NULLIF($5, '')::uuid, (SELECT default_print_id FROM oracle_cards WHERE oracle_id = $2::uuid)))
		`, deck.ID, item.OracleID, item.Qty, item.Board, item.PreferredPrintID); err != nil {
			return nil, err
		}
	}

	for oracleID, role := range input.RoleOverrides {
		if role == "" {
			continue
		}
		if !validImportedRole(role) {
			return nil, fmt.Errorf("invalid imported card role")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO deck_card_role_overrides(deck_id,oracle_id,role) VALUES($1,$2::uuid,$3)`, deck.ID, oracleID, role); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return deck, nil
}

// AddImportedDeckCards adds a resolved list atomically without changing existing printings.
func AddImportedDeckCards(ctx context.Context, db *sql.DB, deckID, userID int64, items []ImportedDeckCardInput) error {
	normalized, err := normalizeImportedDeckCards(items)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		return fmt.Errorf("no cards to import")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owned int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM decks WHERE id = $1 AND user_id = $2 FOR UPDATE`, deckID, userID).Scan(&owned); err != nil {
		return err
	}
	for _, item := range normalized {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO deck_cards (deck_id, oracle_id, qty, board, preferred_print_id)
			VALUES ($1, $2::uuid, $3, $4, COALESCE(
				(SELECT preferred_print_id FROM deck_cards WHERE deck_id = $1 AND oracle_id = $2::uuid AND preferred_print_id IS NOT NULL ORDER BY board LIMIT 1),
				(SELECT scryfall_id FROM card_prints WHERE scryfall_id = NULLIF($5, '')::uuid AND oracle_id = $2::uuid),
				(SELECT default_print_id FROM oracle_cards WHERE oracle_id = $2::uuid)))
			ON CONFLICT (deck_id, oracle_id, board) DO UPDATE SET qty = deck_cards.qty + EXCLUDED.qty
		`, deckID, item.OracleID, item.Qty, item.Board, item.PreferredPrintID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE decks SET updated_at = NOW() WHERE id = $1`, deckID); err != nil {
		return err
	}
	return tx.Commit()
}
