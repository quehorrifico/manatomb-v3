package decks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Deck struct {
	ID               int64
	UserID           int64
	Name             string
	Description      string
	Tags             string
	Format           string
	CommanderName    string
	CommanderPrintID string
	IsPublic         bool
	PublicSlug       string
	PublishedAt      *time.Time
	PowerBracket     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DeckInput struct {
	Name             string
	Description      string
	Tags             string
	Format           string
	CommanderName    string
	CommanderPrintID string
	IsPublic         bool
	PublicSlug       string
	PowerBracket     string
}

type DeckCard struct {
	CardID           string
	CardName         string
	ManaCost         string
	ImageURI         string
	TypeLine         string
	OracleText       string
	FlavorText       string
	AllPartsJSON     string
	CMC              float64
	PriceUSD         string
	Colors           string
	ColorIdentity    string
	Quantity         int
	PreferredPrintID string
	PrintID          string
	SetCode          string
	SetName          string
	CollectorNumber  string
	Rarity           string
	ReleasedAt       string
	Artist           string
}

type DeckCardInput struct {
	OracleID string
	Qty      int
}

type CardBoardState struct {
	Board            string `json:"board"`
	Quantity         int    `json:"quantity"`
	PreferredPrintID string `json:"preferred_print_id,omitempty"`
}

var (
	ErrCardBoardStateConflict = errors.New("card board state changed")
	ErrInvalidCardBoardState  = errors.New("invalid card board state")
)

func normalizeBoard(board string) string {
	switch strings.ToLower(strings.TrimSpace(board)) {
	case "main", "mainboard", "deck":
		return "main"
	case "maybe", "maybeboard":
		return "maybe"
	case "side", "sideboard":
		return "side"
	default:
		return ""
	}
}

func adjustDeckCardQty(ctx context.Context, tx *sql.Tx, deckID int64, oracleID, board string, delta int) error {
	if deckID <= 0 {
		return fmt.Errorf("invalid deck id")
	}
	oracleID = strings.TrimSpace(oracleID)
	if oracleID == "" {
		return fmt.Errorf("missing oracle id")
	}
	board = normalizeBoard(board)
	if board == "" {
		return fmt.Errorf("invalid board")
	}

	var currentQty int
	err := tx.QueryRowContext(ctx, `
		SELECT qty
		FROM deck_cards
		WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
	`, deckID, oracleID, board).Scan(&currentQty)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	newQty := currentQty + delta
	if newQty <= 0 {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM deck_cards
			WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
		`, deckID, oracleID, board)
		return err
	}
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
				INSERT INTO deck_cards (deck_id, oracle_id, qty, board, preferred_print_id)
				VALUES (
					$1,
					$2::uuid,
					$3,
					$4,
					(
						SELECT preferred_print_id
						FROM deck_cards
						WHERE deck_id = $1
						  AND oracle_id = $2::uuid
						  AND preferred_print_id IS NOT NULL
						ORDER BY board
						LIMIT 1
					)
				)
			`, deckID, oracleID, newQty, board)
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE deck_cards
		SET qty = $4
		WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
	`, deckID, oracleID, board, newQty)
	return err
}

func deckCardQtyInBoard(ctx context.Context, tx *sql.Tx, deckID int64, oracleID, board string) (int, error) {
	oracleID = strings.TrimSpace(oracleID)
	board = normalizeBoard(board)
	if oracleID == "" || board == "" {
		return 0, nil
	}

	var qty int
	err := tx.QueryRowContext(ctx, `
		SELECT qty
		FROM deck_cards
		WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
	`, deckID, oracleID, board).Scan(&qty)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return qty, nil
}

func deckCardPreferredPrintInBoard(ctx context.Context, tx *sql.Tx, deckID int64, oracleID, board string) (sql.NullString, error) {
	oracleID = strings.TrimSpace(oracleID)
	board = normalizeBoard(board)
	if oracleID == "" || board == "" {
		return sql.NullString{}, nil
	}

	var printID sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT preferred_print_id::text
		FROM deck_cards
		WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
	`, deckID, oracleID, board).Scan(&printID)
	if err == sql.ErrNoRows {
		return sql.NullString{}, nil
	}
	if err != nil {
		return sql.NullString{}, err
	}
	return printID, nil
}

func AddCard(ctx context.Context, db *sql.DB, deckID int64, oracleID string, delta int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "main", delta); err != nil {
		return err
	}
	return tx.Commit()
}

func AddMaybeCard(ctx context.Context, db *sql.DB, deckID int64, oracleID string, delta int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "maybe", delta); err != nil {
		return err
	}
	return tx.Commit()
}

func AddSideboardCard(ctx context.Context, db *sql.DB, deckID int64, oracleID string, delta int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, "side", delta); err != nil {
		return err
	}
	return tx.Commit()
}

func SetCardQuantity(ctx context.Context, db *sql.DB, deckID int64, oracleID, board string, qty int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	board = normalizeBoard(board)
	if board == "" {
		return fmt.Errorf("invalid board")
	}
	if qty < 0 {
		qty = 0
	}

	currentQty, err := deckCardQtyInBoard(ctx, tx, deckID, oracleID, board)
	if err != nil {
		return err
	}
	if currentQty == qty {
		return tx.Commit()
	}
	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, board, qty-currentQty); err != nil {
		return err
	}
	return tx.Commit()
}

func SetCardPreferredPrint(ctx context.Context, db *sql.DB, deckID int64, oracleID, board, printID string) error {
	if deckID <= 0 {
		return fmt.Errorf("invalid deck id")
	}
	oracleID = strings.TrimSpace(oracleID)
	if oracleID == "" {
		return fmt.Errorf("missing oracle id")
	}
	board = normalizeBoard(board)
	if board == "" {
		return fmt.Errorf("invalid board")
	}
	printID = strings.TrimSpace(printID)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentQty int
	err = tx.QueryRowContext(ctx, `
		SELECT qty
		FROM deck_cards
		WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
	`, deckID, oracleID, board).Scan(&currentQty)
	if err == sql.ErrNoRows {
		return fmt.Errorf("card is not in board")
	}
	if err != nil {
		return err
	}

	if printID == "" || strings.EqualFold(printID, "default") || strings.EqualFold(printID, "latest") {
		if _, err := tx.ExecContext(ctx, `
			UPDATE deck_cards
			SET preferred_print_id = NULL
			WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
		`, deckID, oracleID, board); err != nil {
			return err
		}
	} else {
		var printOracleID string
		err = tx.QueryRowContext(ctx, `
			SELECT oracle_id::text
			FROM card_prints
			WHERE scryfall_id = $1::uuid
		`, printID).Scan(&printOracleID)
		if err == sql.ErrNoRows {
			return fmt.Errorf("card print not found")
		}
		if err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(printOracleID), oracleID) {
			return fmt.Errorf("card print does not match card")
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE deck_cards
			SET preferred_print_id = $4::uuid
			WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
		`, deckID, oracleID, board, printID); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE decks
		SET updated_at = NOW()
		WHERE id = $1
	`, deckID); err != nil {
		return err
	}

	return tx.Commit()
}

// SetCommanderPreferredPrint persists the exact printing used for a deck's
// commander. Commander cards live outside the 99, so their printing choice
// cannot safely depend on a deck_cards row being present.
func SetCommanderPreferredPrint(ctx context.Context, db *sql.DB, deckID int64, oracleID, printID string) error {
	if deckID <= 0 {
		return fmt.Errorf("invalid deck id")
	}
	oracleID = strings.TrimSpace(oracleID)
	if oracleID == "" {
		return fmt.Errorf("missing oracle id")
	}
	printID = strings.TrimSpace(printID)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var commanderMatches bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM decks d
			JOIN oracle_cards oc
			  ON oc.oracle_id::text = $2
			WHERE d.id = $1
			  AND oc.name_search = normalize_card_name(COALESCE(d.commander_name, ''))
		)
	`, deckID, oracleID).Scan(&commanderMatches); err != nil {
		return err
	}
	if !commanderMatches {
		return fmt.Errorf("card does not match deck commander")
	}

	commanderPrintID := ""
	if printID != "" && !strings.EqualFold(printID, "default") && !strings.EqualFold(printID, "latest") {
		var matchedPrintID string
		err := tx.QueryRowContext(ctx, `
			SELECT scryfall_id::text
			FROM card_prints
			WHERE scryfall_id::text = $1
			  AND oracle_id::text = $2
		`, printID, oracleID).Scan(&matchedPrintID)
		if err == sql.ErrNoRows {
			return fmt.Errorf("card print does not match commander")
		}
		if err != nil {
			return err
		}
		commanderPrintID = strings.TrimSpace(matchedPrintID)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE decks
		SET commander_print_id = NULLIF($2, '')::uuid,
		    updated_at = NOW()
		WHERE id = $1
	`, deckID, commanderPrintID)
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

	if _, err := tx.ExecContext(ctx, `
		UPDATE deck_cards
		SET preferred_print_id = NULLIF($3, '')::uuid
		WHERE deck_id = $1
		  AND oracle_id::text = $2
		  AND board = 'main'
	`, deckID, oracleID, commanderPrintID); err != nil {
		return err
	}

	return tx.Commit()
}

func MoveCardBetweenBoards(ctx context.Context, db *sql.DB, deckID int64, oracleID, fromBoard, toBoard string) error {
	return MoveCardQuantityBetweenBoards(ctx, db, deckID, oracleID, fromBoard, toBoard, 1)
}

func MoveCardQuantityBetweenBoards(ctx context.Context, db *sql.DB, deckID int64, oracleID, fromBoard, toBoard string, qty int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	fromBoard = normalizeBoard(fromBoard)
	toBoard = normalizeBoard(toBoard)
	if fromBoard == "" || toBoard == "" {
		return fmt.Errorf("invalid board")
	}
	if fromBoard == toBoard {
		return tx.Commit()
	}

	currentQty, err := deckCardQtyInBoard(ctx, tx, deckID, oracleID, fromBoard)
	if err != nil {
		return err
	}
	if currentQty <= 0 {
		return tx.Commit()
	}
	sourcePreferredPrint, err := deckCardPreferredPrintInBoard(ctx, tx, deckID, oracleID, fromBoard)
	if err != nil {
		return err
	}
	targetQty, err := deckCardQtyInBoard(ctx, tx, deckID, oracleID, toBoard)
	if err != nil {
		return err
	}
	moveQty := qty
	if moveQty <= 0 || moveQty > currentQty {
		moveQty = currentQty
	}

	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, fromBoard, -moveQty); err != nil {
		return err
	}
	if err := adjustDeckCardQty(ctx, tx, deckID, oracleID, toBoard, moveQty); err != nil {
		return err
	}
	if targetQty <= 0 && sourcePreferredPrint.Valid && strings.TrimSpace(sourcePreferredPrint.String) != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE deck_cards
			SET preferred_print_id = $4::uuid
			WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
		`, deckID, oracleID, toBoard, strings.TrimSpace(sourcePreferredPrint.String)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func MoveCardToMaybe(ctx context.Context, db *sql.DB, deckID int64, oracleID string) error {
	return MoveCardBetweenBoards(ctx, db, deckID, oracleID, "main", "maybe")
}

func MoveMaybeToDeck(ctx context.Context, db *sql.DB, deckID int64, oracleID string) error {
	return MoveCardBetweenBoards(ctx, db, deckID, oracleID, "maybe", "main")
}

func RestoreCardBoardStatesIfCurrent(
	ctx context.Context,
	db *sql.DB,
	deckID int64,
	oracleID string,
	expected []CardBoardState,
	restore []CardBoardState,
) error {
	if deckID <= 0 || strings.TrimSpace(oracleID) == "" {
		return fmt.Errorf("%w: missing deck or card", ErrInvalidCardBoardState)
	}
	if len(expected) == 0 || len(expected) > 3 || len(restore) != len(expected) {
		return fmt.Errorf("%w: expected and restore states must describe the same boards", ErrInvalidCardBoardState)
	}

	normalizeStates := func(states []CardBoardState) (map[string]CardBoardState, error) {
		out := make(map[string]CardBoardState, len(states))
		for _, state := range states {
			board := normalizeBoard(state.Board)
			if board == "" || state.Quantity < 0 {
				return nil, fmt.Errorf("%w: invalid board or quantity", ErrInvalidCardBoardState)
			}
			if _, exists := out[board]; exists {
				return nil, fmt.Errorf("%w: duplicate board", ErrInvalidCardBoardState)
			}
			state.Board = board
			state.PreferredPrintID = strings.TrimSpace(state.PreferredPrintID)
			if state.Quantity == 0 {
				state.PreferredPrintID = ""
			}
			out[board] = state
		}
		return out, nil
	}

	expectedByBoard, err := normalizeStates(expected)
	if err != nil {
		return err
	}
	restoreByBoard, err := normalizeStates(restore)
	if err != nil {
		return err
	}
	for board := range expectedByBoard {
		if _, ok := restoreByBoard[board]; !ok {
			return fmt.Errorf("%w: board sets do not match", ErrInvalidCardBoardState)
		}
	}

	oracleID = strings.TrimSpace(oracleID)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var lockedDeckID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM decks
		WHERE id = $1
		FOR UPDATE
	`, deckID).Scan(&lockedDeckID); err != nil {
		return err
	}

	for board, state := range expectedByBoard {
		var currentQty int
		var currentPrint sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT qty, preferred_print_id::text
			FROM deck_cards
			WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
			FOR UPDATE
		`, deckID, oracleID, board).Scan(&currentQty, &currentPrint)
		if err == sql.ErrNoRows {
			currentQty = 0
			currentPrint = sql.NullString{}
		} else if err != nil {
			return err
		}
		if currentQty != state.Quantity ||
			!strings.EqualFold(strings.TrimSpace(currentPrint.String), state.PreferredPrintID) {
			return ErrCardBoardStateConflict
		}
	}

	for board, state := range restoreByBoard {
		if state.PreferredPrintID != "" {
			var printMatches bool
			if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM card_prints
					WHERE scryfall_id = $1::uuid
					  AND oracle_id = $2::uuid
				)
			`, state.PreferredPrintID, oracleID).Scan(&printMatches); err != nil {
				return err
			}
			if !printMatches {
				return fmt.Errorf("%w: printing does not match card", ErrInvalidCardBoardState)
			}
		}

		if state.Quantity == 0 {
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM deck_cards
				WHERE deck_id = $1 AND oracle_id = $2::uuid AND board = $3
			`, deckID, oracleID, board); err != nil {
				return err
			}
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO deck_cards (deck_id, oracle_id, qty, board, preferred_print_id)
			VALUES ($1, $2::uuid, $3, $4, NULLIF($5, '')::uuid)
			ON CONFLICT (deck_id, oracle_id, board)
			DO UPDATE SET
				qty = EXCLUDED.qty,
				preferred_print_id = EXCLUDED.preferred_print_id
		`, deckID, oracleID, state.Quantity, board, state.PreferredPrintID); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE decks
		SET updated_at = NOW()
		WHERE id = $1
	`, deckID); err != nil {
		return err
	}

	return tx.Commit()
}

func SetMainboard(ctx context.Context, db *sql.DB, deckID int64, cardsIn []DeckCardInput) error {
	if deckID <= 0 {
		return fmt.Errorf("invalid deck id")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM deck_cards
		WHERE deck_id = $1 AND board = 'main'
	`, deckID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO deck_cards (deck_id, oracle_id, qty, board)
		VALUES ($1, $2::uuid, $3, 'main')
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range cardsIn {
		oracleID := strings.TrimSpace(item.OracleID)
		if oracleID == "" || item.Qty <= 0 {
			continue
		}
		if _, err := stmt.ExecContext(ctx, deckID, oracleID, item.Qty); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE decks
		SET updated_at = NOW()
		WHERE id = $1
	`, deckID); err != nil {
		return err
	}

	return tx.Commit()
}

func listDeckCardsByBoard(ctx context.Context, db *sql.DB, deckID int64, board string) ([]DeckCard, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			dc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, ''),
			COALESCE(cp.image_uri, ''),
			COALESCE(oc.type_line, ''),
			COALESCE(oc.oracle_text, ''),
			COALESCE(cp.flavor_text, oc.flavor_text, ''),
			COALESCE(oc.all_parts::text, '[]'),
			COALESCE(oc.cmc, 0),
			COALESCE(cp.price_usd, ''),
			COALESCE(array_to_string(oc.colors, ','), ''),
			COALESCE(array_to_string(oc.color_identity, ','), ''),
			dc.qty,
			COALESCE(dc.preferred_print_id::text, ''),
			COALESCE(cp.scryfall_id::text, ''),
			COALESCE(cp.set_code, ''),
			COALESCE(cp.set_name, ''),
			COALESCE(cp.collector_number, ''),
			COALESCE(cp.rarity, ''),
			COALESCE(to_char(cp.released_at, 'YYYY-MM-DD'), ''),
			COALESCE(cp.artist, '')
		FROM deck_cards dc
		JOIN oracle_cards oc
		  ON oc.oracle_id = dc.oracle_id
		LEFT JOIN card_prints cp
		  ON cp.scryfall_id = COALESCE(dc.preferred_print_id, oc.default_print_id)
		WHERE dc.deck_id = $1
		  AND dc.board = $2
		ORDER BY oc.name
	`, deckID, board)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DeckCard, 0)
	for rows.Next() {
		var dc DeckCard
		if err := rows.Scan(
			&dc.CardID,
			&dc.CardName,
			&dc.ManaCost,
			&dc.ImageURI,
			&dc.TypeLine,
			&dc.OracleText,
			&dc.FlavorText,
			&dc.AllPartsJSON,
			&dc.CMC,
			&dc.PriceUSD,
			&dc.Colors,
			&dc.ColorIdentity,
			&dc.Quantity,
			&dc.PreferredPrintID,
			&dc.PrintID,
			&dc.SetCode,
			&dc.SetName,
			&dc.CollectorNumber,
			&dc.Rarity,
			&dc.ReleasedAt,
			&dc.Artist,
		); err != nil {
			return nil, err
		}
		out = append(out, dc)
	}
	return out, rows.Err()
}

func ListDeckCards(ctx context.Context, db *sql.DB, deckID int64) ([]DeckCard, error) {
	return listDeckCardsByBoard(ctx, db, deckID, "main")
}

func ListDeckMaybeCards(ctx context.Context, db *sql.DB, deckID int64) ([]DeckCard, error) {
	return listDeckCardsByBoard(ctx, db, deckID, "maybe")
}

func ListDeckSideboardCards(ctx context.Context, db *sql.DB, deckID int64) ([]DeckCard, error) {
	return listDeckCardsByBoard(ctx, db, deckID, "side")
}

func CreateDeck(ctx context.Context, db *sql.DB, userID int64, name, description, commanderName string) (*Deck, error) {
	return CreateDeckWithOptions(ctx, db, userID, DeckInput{
		Name:          name,
		Description:   description,
		Tags:          "",
		Format:        "Commander",
		CommanderName: commanderName,
	})
}

func ListDecksByUser(ctx context.Context, db *sql.DB, userID int64) ([]Deck, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(commander_print_id::text, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Deck
	for rows.Next() {
		d, err := scanDeck(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func GetDeck(ctx context.Context, db *sql.DB, id, userID int64) (*Deck, error) {
	return scanDeck(db.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(commander_print_id::text, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan)
}

func UpdateDeck(ctx context.Context, db *sql.DB, deckID int64, name, description, commanderName string) error {
	return UpdateDeckWithOptions(ctx, db, deckID, DeckInput{
		Name:          name,
		Description:   description,
		Tags:          "",
		Format:        "Commander",
		CommanderName: commanderName,
	})
}

func DeleteDeck(ctx context.Context, db *sql.DB, deckID int64) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM decks
		WHERE id = $1
	`, deckID)
	return err
}

var supportedDeckTags = []string{
	"Aggro",
	"Midrange",
	"Control",
	"Combo",
	"Ramp",
	"Stax",
	"Aristocrats",
	"Spellslinger",
	"Tokens",
	"Reanimator",
	"Voltron",
	"Tribal",
}

func SupportedDeckTags() []string {
	out := make([]string, len(supportedDeckTags))
	copy(out, supportedDeckTags)
	return out
}

func NormalizeDeckTag(raw string) string {
	switch strings.ToLower(normalizeLooseLabel(raw)) {
	case "aggro":
		return "Aggro"
	case "midrange":
		return "Midrange"
	case "control":
		return "Control"
	case "combo":
		return "Combo"
	case "ramp":
		return "Ramp"
	case "stax":
		return "Stax"
	case "aristocrats":
		return "Aristocrats"
	case "spellslinger":
		return "Spellslinger"
	case "tokens":
		return "Tokens"
	case "reanimator":
		return "Reanimator"
	case "voltron":
		return "Voltron"
	case "tribal":
		return "Tribal"
	default:
		return ""
	}
}

func SplitTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})

	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		tag := NormalizeDeckTag(field)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func NormalizeTags(raw string) string {
	return strings.Join(SplitTags(raw), ", ")
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func tableHasColumn(ctx context.Context, db *sql.DB, tableName, columnName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = $1
			  AND column_name = $2
		)
	`, tableName, columnName).Scan(&exists)
	return exists, err
}

type deckSchemaExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func createDeckCardsV2(ctx context.Context, executor deckSchemaExecer, tableName string) error {
	_, err := executor.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
			oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE RESTRICT,
			qty INT NOT NULL DEFAULT 0,
			board TEXT NOT NULL DEFAULT 'main',
			preferred_print_id UUID NULL REFERENCES card_prints(scryfall_id) ON DELETE SET NULL,
			PRIMARY KEY (deck_id, oracle_id, board),
			CHECK (board IN ('main', 'maybe', 'side'))
		);
	`, tableName))
	return err
}

func migrateLegacyDeckCards(ctx context.Context, db *sql.DB) error {
	oldCardsExists, err := tableExists(ctx, db, "cards")
	if err != nil {
		return fmt.Errorf("check legacy cards table: %w", err)
	}
	if !oldCardsExists {
		return errors.New("cannot migrate legacy deck cards without the legacy cards table")
	}

	maybeExists, err := tableExists(ctx, db, "deck_maybe_cards")
	if err != nil {
		return fmt.Errorf("check legacy maybeboard table: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin legacy deck-card migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS deck_cards_v2`); err != nil {
		return fmt.Errorf("reset deck-card migration table: %w", err)
	}
	if err := createDeckCardsV2(ctx, tx, "deck_cards_v2"); err != nil {
		return fmt.Errorf("create deck-card migration table: %w", err)
	}

	var unmappableMain int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM deck_cards dc
		LEFT JOIN cards c ON c.id = dc.card_id
		LEFT JOIN LATERAL (
			SELECT candidate.oracle_id
			FROM oracle_cards candidate
			WHERE candidate.name_search = normalize_card_name(c.name)
			ORDER BY candidate.name, candidate.oracle_id
			LIMIT 1
		) oc ON TRUE
		WHERE dc.quantity > 0
		  AND (c.id IS NULL OR oc.oracle_id IS NULL)
	`).Scan(&unmappableMain); err != nil {
		return fmt.Errorf("validate legacy mainboard cards: %w", err)
	}
	if unmappableMain > 0 {
		return fmt.Errorf("refusing to replace legacy deck cards: %d mainboard rows have no canonical card mapping", unmappableMain)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO deck_cards_v2 (deck_id, oracle_id, qty, board)
		SELECT dc.deck_id, oc.oracle_id, dc.quantity, 'main'
		FROM deck_cards dc
		JOIN cards c ON c.id = dc.card_id
		JOIN LATERAL (
			SELECT candidate.oracle_id
			FROM oracle_cards candidate
			WHERE candidate.name_search = normalize_card_name(c.name)
			ORDER BY candidate.name, candidate.oracle_id
			LIMIT 1
		) oc ON TRUE
		WHERE dc.quantity > 0
		ON CONFLICT (deck_id, oracle_id, board)
		DO UPDATE SET qty = deck_cards_v2.qty + EXCLUDED.qty
	`); err != nil {
		return fmt.Errorf("copy legacy mainboard cards: %w", err)
	}

	if maybeExists {
		var unmappableMaybe int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM deck_maybe_cards dmc
			LEFT JOIN cards c ON c.id = dmc.card_id
			LEFT JOIN LATERAL (
				SELECT candidate.oracle_id
				FROM oracle_cards candidate
				WHERE candidate.name_search = normalize_card_name(c.name)
				ORDER BY candidate.name, candidate.oracle_id
				LIMIT 1
			) oc ON TRUE
			WHERE dmc.quantity > 0
			  AND (c.id IS NULL OR oc.oracle_id IS NULL)
		`).Scan(&unmappableMaybe); err != nil {
			return fmt.Errorf("validate legacy maybeboard cards: %w", err)
		}
		if unmappableMaybe > 0 {
			return fmt.Errorf("refusing to replace legacy deck cards: %d maybeboard rows have no canonical card mapping", unmappableMaybe)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO deck_cards_v2 (deck_id, oracle_id, qty, board)
			SELECT dmc.deck_id, oc.oracle_id, dmc.quantity, 'maybe'
			FROM deck_maybe_cards dmc
			JOIN cards c ON c.id = dmc.card_id
			JOIN LATERAL (
				SELECT candidate.oracle_id
				FROM oracle_cards candidate
				WHERE candidate.name_search = normalize_card_name(c.name)
				ORDER BY candidate.name, candidate.oracle_id
				LIMIT 1
			) oc ON TRUE
			WHERE dmc.quantity > 0
			ON CONFLICT (deck_id, oracle_id, board)
			DO UPDATE SET qty = deck_cards_v2.qty + EXCLUDED.qty
		`); err != nil {
			return fmt.Errorf("copy legacy maybeboard cards: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS deck_maybe_cards`); err != nil {
		return fmt.Errorf("remove migrated maybeboard table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE deck_cards`); err != nil {
		return fmt.Errorf("replace migrated deck-card table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE deck_cards_v2 RENAME TO deck_cards`); err != nil {
		return fmt.Errorf("activate migrated deck-card table: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy deck-card migration: %w", err)
	}
	return nil
}

func EnsureDeckTables(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS decks (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '',
			format TEXT NOT NULL DEFAULT 'Commander',
			commander_name TEXT,
			commander_print_id UUID NULL,
			is_public BOOLEAN NOT NULL DEFAULT FALSE,
			public_slug TEXT,
			published_at TIMESTAMPTZ,
			power_bracket TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`); err != nil {
		return err
	}

	for _, stmt := range []string{
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS tags TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS format TEXT`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS commander_name TEXT`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS commander_print_id UUID NULL`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS public_slug TEXT`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ`,
		`ALTER TABLE decks ADD COLUMN IF NOT EXISTS power_bracket TEXT NOT NULL DEFAULT ''`,
		`UPDATE decks SET description = '' WHERE description IS NULL`,
		`UPDATE decks SET tags = '' WHERE tags IS NULL`,
		`UPDATE decks SET format = 'Commander' WHERE COALESCE(btrim(format), '') = ''`,
		`ALTER TABLE decks ALTER COLUMN format SET DEFAULT 'Commander'`,
		`ALTER TABLE decks ALTER COLUMN format SET NOT NULL`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	deckCardsExists, err := tableExists(ctx, db, "deck_cards")
	if err != nil {
		return err
	}
	if deckCardsExists {
		hasLegacyCardID, err := tableHasColumn(ctx, db, "deck_cards", "card_id")
		if err != nil {
			return err
		}
		if hasLegacyCardID {
			if err := migrateLegacyDeckCards(ctx, db); err != nil {
				return err
			}
		}
	}

	if err := createDeckCardsV2(ctx, db, "deck_cards"); err != nil {
		return err
	}
	// Keep older installations with "quantity" compatible.
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards ADD COLUMN IF NOT EXISTS qty INT NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards ADD COLUMN IF NOT EXISTS board TEXT NOT NULL DEFAULT 'main'`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards ADD COLUMN IF NOT EXISTS preferred_print_id UUID NULL`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE decks AS d
		SET commander_print_id = COALESCE(
			(
				SELECT COALESCE(dc.preferred_print_id, oc.default_print_id)
				FROM deck_cards AS dc
				JOIN oracle_cards AS oc
				  ON oc.oracle_id = dc.oracle_id
				WHERE dc.deck_id = d.id
				  AND dc.board = 'main'
				  AND oc.name_search = normalize_card_name(d.commander_name)
				ORDER BY (dc.preferred_print_id IS NOT NULL) DESC
				LIMIT 1
			),
			(
				SELECT oc.default_print_id
				FROM oracle_cards AS oc
				WHERE oc.name_search = normalize_card_name(d.commander_name)
				ORDER BY COALESCE(oc.edhrec_rank, 2147483647), oc.name
				LIMIT 1
			)
		)
		WHERE d.commander_print_id IS NULL
		  AND btrim(COALESCE(d.commander_name, '')) <> ''
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'fk_decks_commander_print'
				  AND conrelid = 'decks'::regclass
			) THEN
				ALTER TABLE decks
				ADD CONSTRAINT fk_decks_commander_print
				FOREIGN KEY (commander_print_id)
				REFERENCES card_prints(scryfall_id)
				ON DELETE SET NULL;
			END IF;
		END $$;
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards DROP CONSTRAINT IF EXISTS deck_cards_board_check`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE deck_cards DROP CONSTRAINT IF EXISTS deck_cards_v2_board_check`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE deck_cards
		ADD CONSTRAINT deck_cards_board_check CHECK (board IN ('main', 'maybe', 'side'))
	`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_decks_user_updated
		ON decks (user_id, updated_at DESC)
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_decks_public_published
		ON decks (is_public, published_at DESC, updated_at DESC)
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_decks_public_slug_unique
		ON decks (public_slug)
		WHERE public_slug IS NOT NULL
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_decks_commander_print_id
		ON decks (commander_print_id)
		WHERE commander_print_id IS NOT NULL
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_deck_cards_deck_board
		ON deck_cards (deck_id, board)
	`); err != nil {
		return err
	}

	return nil
}

func scanDeck(scan func(dest ...any) error) (*Deck, error) {
	var (
		d           Deck
		publishedAt sql.NullTime
	)
	if err := scan(
		&d.ID,
		&d.UserID,
		&d.Name,
		&d.Description,
		&d.Tags,
		&d.Format,
		&d.CommanderName,
		&d.CommanderPrintID,
		&d.IsPublic,
		&d.PublicSlug,
		&publishedAt,
		&d.PowerBracket,
		&d.CreatedAt,
		&d.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if publishedAt.Valid {
		t := publishedAt.Time
		d.PublishedAt = &t
	}
	d.PowerBracket = NormalizePowerBracket(d.PowerBracket)
	return &d, nil
}

func CreateDeckWithOptions(ctx context.Context, db *sql.DB, userID int64, input DeckInput) (*Deck, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	d, err := insertDeckTx(ctx, tx, userID, input)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d, nil
}

func validateCommanderPrintTx(ctx context.Context, tx *sql.Tx, commanderName, printID string) error {
	commanderName = strings.TrimSpace(commanderName)
	printID = strings.TrimSpace(printID)
	if commanderName == "" || printID == "" {
		return fmt.Errorf("commander and printing are required")
	}

	var matchedPrintID string
	err := tx.QueryRowContext(ctx, `
		SELECT cp.scryfall_id::text
		FROM card_prints cp
		JOIN oracle_cards oc
		  ON oc.oracle_id = cp.oracle_id
		WHERE cp.scryfall_id::text = $1
		  AND oc.name_search = normalize_card_name($2)
		LIMIT 1
	`, printID, commanderName).Scan(&matchedPrintID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("commander print does not match commander")
	}
	if err != nil {
		return err
	}
	return nil
}

func UpdateDeckWithOptions(ctx context.Context, db *sql.DB, deckID int64, input DeckInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	current, err := scanDeck(tx.QueryRowContext(ctx, `
		SELECT
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(commander_print_id::text, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
		FROM decks
		WHERE id = $1
	`, deckID).Scan)
	if err != nil {
		return err
	}

	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	tags := NormalizeTags(input.Tags)
	format := NormalizeFormat(input.Format)
	commanderName := strings.TrimSpace(input.CommanderName)
	commanderPrintID := strings.TrimSpace(input.CommanderPrintID)
	if !FormatRequiresCommander(format) {
		commanderName = ""
	}
	isPublic := input.IsPublic
	powerBracket := NormalizePowerBracket(input.PowerBracket)
	publicSlug := NormalizePublicSlug(input.PublicSlug)
	if commanderName == "" {
		commanderPrintID = ""
	} else if commanderPrintID == "" && strings.EqualFold(commanderName, current.CommanderName) {
		commanderPrintID = current.CommanderPrintID
	}
	if commanderPrintID != "" {
		if err := validateCommanderPrintTx(ctx, tx, commanderName, commanderPrintID); err != nil {
			return err
		}
	}
	if isPublic && publicSlug == "" {
		publicSlug, err = reserveUniquePublicSlugTx(ctx, tx, deckID, name, "")
		if err != nil {
			return err
		}
	}

	var publishedAt any
	if isPublic {
		if current.PublishedAt != nil {
			publishedAt = *current.PublishedAt
		} else {
			publishedAt = time.Now().UTC()
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE decks
		SET name = $1,
		    description = $2,
		    tags = $3,
		    format = $4,
		    commander_name = $5,
		    commander_print_id = NULLIF($6, '')::uuid,
		    is_public = $7,
		    public_slug = NULLIF($8, ''),
		    published_at = $9,
		    power_bracket = $10,
		    updated_at = NOW()
		WHERE id = $11
	`, name, description, tags, format, commanderName, commanderPrintID, isPublic, publicSlug, publishedAt, powerBracket, deckID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func insertDeckTx(ctx context.Context, tx *sql.Tx, userID int64, input DeckInput) (*Deck, error) {
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	tags := NormalizeTags(input.Tags)
	format := NormalizeFormat(input.Format)
	commanderName := strings.TrimSpace(input.CommanderName)
	if !FormatRequiresCommander(format) {
		commanderName = ""
	}
	powerBracket := NormalizePowerBracket(input.PowerBracket)
	commanderPrintID := strings.TrimSpace(input.CommanderPrintID)
	if !FormatRequiresCommander(format) || commanderName == "" {
		commanderPrintID = ""
	}
	if commanderPrintID != "" {
		if err := validateCommanderPrintTx(ctx, tx, commanderName, commanderPrintID); err != nil {
			return nil, err
		}
	}

	d, err := scanDeck(tx.QueryRowContext(ctx, `
		INSERT INTO decks (
			user_id,
			name,
			description,
			tags,
			format,
			commander_name,
			commander_print_id,
			is_public,
			public_slug,
			published_at,
			power_bracket
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, FALSE, NULL, NULL, $8)
		RETURNING
			id,
			user_id,
			name,
			COALESCE(description, ''),
			COALESCE(tags, ''),
			COALESCE(format, 'Commander'),
			COALESCE(commander_name, ''),
			COALESCE(commander_print_id::text, ''),
			COALESCE(is_public, FALSE),
			COALESCE(public_slug, ''),
			published_at,
			COALESCE(power_bracket, ''),
			created_at,
			updated_at
	`, userID, name, description, tags, format, commanderName, commanderPrintID, powerBracket).Scan)
	if err != nil {
		return nil, err
	}

	if !input.IsPublic {
		return d, nil
	}

	slug, err := reserveUniquePublicSlugTx(ctx, tx, d.ID, name, input.PublicSlug)
	if err != nil {
		return nil, err
	}
	publishedAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE decks
		SET is_public = TRUE,
		    public_slug = $2,
		    published_at = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, d.ID, slug, publishedAt); err != nil {
		return nil, err
	}

	d.IsPublic = true
	d.PublicSlug = slug
	d.PublishedAt = &publishedAt
	return d, nil
}
