package decks

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestCardRoleOverridesAreDeckScopedPostgres(t *testing.T) {
	url := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	const oracle = "00000000-0000-0000-0000-000000000061"
	if _, err := db.Exec(`CREATE TEMP TABLE decks(id BIGINT PRIMARY KEY,user_id BIGINT,commander_name TEXT);
 CREATE TEMP TABLE oracle_cards(oracle_id UUID PRIMARY KEY,name_search TEXT);
 CREATE TEMP TABLE deck_cards(deck_id BIGINT,oracle_id UUID);
 CREATE TEMP TABLE deck_card_role_overrides(deck_id BIGINT,oracle_id UUID,role TEXT,PRIMARY KEY(deck_id,oracle_id));
 INSERT INTO decks VALUES(1,7,''),(2,7,'');INSERT INTO oracle_cards VALUES('` + oracle + `','role card');INSERT INTO deck_cards VALUES(1,'` + oracle + `'),(2,'` + oracle + `');`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := SetCardRole(ctx, db, 1, 999, oracle, "ramp"); err == nil {
		t.Fatal("another user changed the role")
	}
	if err := SetCardRole(ctx, db, 1, 7, oracle, "unknown-role"); err == nil {
		t.Fatal("invalid role was accepted")
	}
	if err := SetCardRole(ctx, db, 1, 7, oracle, "utility"); err != nil {
		t.Fatal(err)
	}
	var deckID int64
	var role string
	if err := db.QueryRow(`SELECT deck_id,role FROM deck_card_role_overrides`).Scan(&deckID, &role); err != nil {
		t.Fatal(err)
	}
	if deckID != 1 || role != "utility" {
		t.Fatalf("role applied to wrong deck: %d %s", deckID, role)
	}
	if err := SetCardRole(ctx, db, 1, 7, oracle, ""); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM deck_card_role_overrides`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("Automatic did not clear the override")
	}
}
