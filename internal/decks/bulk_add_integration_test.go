package decks

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestBulkAddPreservesPrintingsAndRollsBackPostgres(t *testing.T) {
	url := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	const oracle = "00000000-0000-0000-0000-000000000001"
	const selected = "00000000-0000-0000-0000-000000000002"
	const latest = "00000000-0000-0000-0000-000000000003"
	_, err = db.Exec(`
 CREATE TEMP TABLE oracle_cards (oracle_id UUID PRIMARY KEY, default_print_id UUID);
 CREATE TEMP TABLE card_prints (scryfall_id UUID PRIMARY KEY, oracle_id UUID);
 CREATE TEMP TABLE decks (id BIGINT PRIMARY KEY, user_id BIGINT, updated_at TIMESTAMPTZ);
 CREATE TEMP TABLE deck_cards (deck_id BIGINT, oracle_id UUID REFERENCES oracle_cards, qty INT CHECK(qty > 0), board TEXT, preferred_print_id UUID REFERENCES card_prints, PRIMARY KEY(deck_id, oracle_id, board));
 INSERT INTO oracle_cards VALUES ('` + oracle + `','` + latest + `');
 INSERT INTO card_prints VALUES ('` + selected + `','` + oracle + `'),('` + latest + `','` + oracle + `');
 INSERT INTO decks VALUES (1,7,NOW());
 INSERT INTO deck_cards VALUES (1,'` + oracle + `',2,'main','` + selected + `');
 `)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddImportedDeckCards(ctx, db, 1, 7, []ImportedDeckCardInput{{OracleID: oracle, Qty: 3, Board: "main", PreferredPrintID: latest}, {OracleID: oracle, Qty: 1, Board: "side", PreferredPrintID: latest}}); err != nil {
		t.Fatal(err)
	}
	var qty int
	var printID string
	if err := db.QueryRow(`SELECT qty, preferred_print_id::text FROM deck_cards WHERE board='main'`).Scan(&qty, &printID); err != nil {
		t.Fatal(err)
	}
	if qty != 5 || printID != selected {
		t.Fatalf("bulk add changed selected printing: %d %s", qty, printID)
	}
	if err := db.QueryRow(`SELECT preferred_print_id::text FROM deck_cards WHERE board='side'`).Scan(&printID); err != nil {
		t.Fatal(err)
	}
	if printID != selected {
		t.Fatalf("new board lost selected printing: %s", printID)
	}
	// The second row fails after the first was inserted; neither may commit.
	err = AddImportedDeckCards(ctx, db, 1, 7, []ImportedDeckCardInput{{OracleID: oracle, Qty: 2, Board: "main"}, {OracleID: "00000000-0000-0000-0000-000000000099", Qty: 1, Board: "main"}})
	if err == nil {
		t.Fatal("expected unresolved oracle to fail")
	}
	if err := db.QueryRow(`SELECT qty FROM deck_cards WHERE board='main'`).Scan(&qty); err != nil {
		t.Fatal(err)
	}
	if qty != 5 {
		t.Fatalf("failed import partially committed: qty=%d", qty)
	}
	if err := AddImportedDeckCards(ctx, db, 1, 999, []ImportedDeckCardInput{{OracleID: oracle, Qty: 1, Board: "main"}}); err == nil {
		t.Fatal("another user imported into the deck")
	}
	// Ordinary additions pin the default immediately, too.
	if _, err := db.Exec(`DELETE FROM deck_cards`); err != nil {
		t.Fatal(err)
	}
	if err := AddCard(ctx, db, 1, oracle, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT preferred_print_id::text FROM deck_cards WHERE board='main'`).Scan(&printID); err != nil {
		t.Fatal(err)
	}
	if printID != latest {
		t.Fatalf("new card did not pin its default: %s", printID)
	}
}
