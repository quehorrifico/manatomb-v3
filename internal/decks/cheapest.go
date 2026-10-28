package decks

import (
	"context"
	"database/sql"
)

// UseCheapestPrintings is an explicit, atomic printing change across every board
// and the commander. Unknown prices are ignored, and equal prices keep the user's version.
func UseCheapestPrintings(ctx context.Context, db *sql.DB, deckID, userID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owned int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM decks WHERE id=$1 AND user_id=$2 FOR UPDATE`, deckID, userID).Scan(&owned); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
 WITH candidates AS (
   SELECT dc.oracle_id, dc.board, candidate.scryfall_id
   FROM deck_cards dc
   LEFT JOIN oracle_cards oc ON oc.oracle_id=dc.oracle_id
   LEFT JOIN card_prints current ON current.scryfall_id=COALESCE(dc.preferred_print_id,oc.default_print_id)
   JOIN LATERAL (
     SELECT cp.scryfall_id, cp.price_usd FROM card_prints cp
     WHERE cp.oracle_id=dc.oracle_id AND NOT cp.digital AND cp.price_usd ~ '^[0-9]+([.][0-9]+)?$'
     ORDER BY (CASE WHEN cp.price_usd ~ '^[0-9]+([.][0-9]+)?$' THEN cp.price_usd::numeric END), cp.scryfall_id LIMIT 1
   ) candidate ON TRUE
   WHERE dc.deck_id=$1 AND (CASE WHEN current.price_usd ~ '^[0-9]+([.][0-9]+)?$' THEN candidate.price_usd::numeric < current.price_usd::numeric ELSE TRUE END)
 )
 UPDATE deck_cards dc SET preferred_print_id=c.scryfall_id FROM candidates c
 WHERE dc.deck_id=$1 AND dc.oracle_id=c.oracle_id AND dc.board=c.board
 `, deckID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
 WITH candidate AS (
   SELECT cp.scryfall_id, cp.price_usd FROM decks d
   JOIN oracle_cards oc ON oc.oracle_id=COALESCE(
     (SELECT oracle_id FROM card_prints WHERE scryfall_id=d.commander_print_id),
     (SELECT oracle_id FROM oracle_cards WHERE name_search=normalize_card_name(d.commander_name) ORDER BY oracle_id LIMIT 1))
   JOIN card_prints cp ON cp.oracle_id=oc.oracle_id
   WHERE d.id=$1 AND NOT cp.digital AND cp.price_usd ~ '^[0-9]+([.][0-9]+)?$'
   ORDER BY (CASE WHEN cp.price_usd ~ '^[0-9]+([.][0-9]+)?$' THEN cp.price_usd::numeric END),cp.scryfall_id LIMIT 1
 )
 UPDATE decks d SET commander_print_id=c.scryfall_id FROM candidate c
 LEFT JOIN card_prints current ON current.scryfall_id=(SELECT COALESCE(commander_print_id,oc.default_print_id) FROM decks dd JOIN oracle_cards oc ON oc.name_search=normalize_card_name(dd.commander_name) WHERE dd.id=$1 LIMIT 1)
 WHERE d.id=$1 AND (CASE WHEN current.price_usd ~ '^[0-9]+([.][0-9]+)?$' THEN c.price_usd::numeric < current.price_usd::numeric ELSE TRUE END)
 `, deckID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE decks SET updated_at=NOW() WHERE id=$1`, deckID); err != nil {
		return err
	}
	return tx.Commit()
}
