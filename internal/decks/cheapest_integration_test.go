package decks

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestCheapestPrintingsPostgres(t *testing.T) {
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
	const oracle = "00000000-0000-0000-0000-000000000091"
	const selected = "00000000-0000-0000-0000-000000000092"
	const cheap = "00000000-0000-0000-0000-000000000093"
	_, err = db.Exec(`CREATE TEMP TABLE oracle_cards(oracle_id UUID PRIMARY KEY,name_search TEXT,default_print_id UUID);
 CREATE TEMP TABLE card_prints(scryfall_id UUID PRIMARY KEY,oracle_id UUID,price_usd TEXT,digital BOOLEAN DEFAULT FALSE);
 CREATE TEMP TABLE decks(id BIGINT PRIMARY KEY,user_id BIGINT,commander_name TEXT,commander_print_id UUID,updated_at TIMESTAMPTZ);
 CREATE TEMP TABLE deck_cards(deck_id BIGINT,oracle_id UUID,board TEXT,preferred_print_id UUID);
 INSERT INTO oracle_cards VALUES('` + oracle + `','cost card','` + selected + `');
 INSERT INTO card_prints VALUES('` + selected + `','` + oracle + `','10.00',FALSE),('` + cheap + `','` + oracle + `','1.00',FALSE),('00000000-0000-0000-0000-000000000094','` + oracle + `','',FALSE),('00000000-0000-0000-0000-000000000095','` + oracle + `','0.01',TRUE);
 INSERT INTO decks VALUES(1,7,'Cost Card','` + selected + `',NOW());
 INSERT INTO deck_cards VALUES(1,'` + oracle + `','main','` + selected + `'),(1,'` + oracle + `','side','` + selected + `'),(1,'` + oracle + `','maybe','` + selected + `');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := UseCheapestPrintings(context.Background(), db, 1, 999); err == nil {
		t.Fatal("another user changed printings")
	}
	if err := UseCheapestPrintings(context.Background(), db, 1, 7); err != nil {
		t.Fatal(err)
	}
	var count int
	var commander string
	if err := db.QueryRow(`SELECT COUNT(*) FROM deck_cards WHERE preferred_print_id=$1`, cheap).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT commander_print_id::text FROM decks WHERE id=1`).Scan(&commander); err != nil {
		t.Fatal(err)
	}
	if count != 3 || commander != cheap {
		t.Fatalf("did not choose cheapest paper printings: %d %s", count, commander)
	}
	// Equal-priced versions stay selected; missing prices never become zero-dollar candidates.
	for _, query := range []struct {
		sql string
		arg any
	}{
		{`UPDATE card_prints SET price_usd='10.00' WHERE scryfall_id=$1`, cheap},
		{`UPDATE deck_cards SET preferred_print_id=$1`, selected},
		{`UPDATE decks SET commander_print_id=$1`, selected},
	} {
		if _, err := db.Exec(query.sql, query.arg); err != nil {
			t.Fatal(err)
		}
	}
	if err := UseCheapestPrintings(context.Background(), db, 1, 7); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM deck_cards WHERE preferred_print_id=$1`, selected).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatal("ties changed a selected printing")
	}
}
