package cards

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBulkRowStreamPostgres(t *testing.T) {
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
	if err := EnsureCardsTable(ctx, db); err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
 CREATE TEMP TABLE oracle_cards (LIKE public.oracle_cards INCLUDING DEFAULTS INCLUDING GENERATED INCLUDING INDEXES);
 CREATE TEMP TABLE card_prints (LIKE public.card_prints INCLUDING DEFAULTS INCLUDING GENERATED INCLUDING INDEXES);
 CREATE TEMP TABLE card_sync_state (LIKE public.card_sync_state INCLUDING DEFAULTS INCLUDING INDEXES);
 CREATE TEMP TABLE decks (commander_print_id UUID);
 CREATE TEMP TABLE deck_cards (preferred_print_id UUID, oracle_id UUID);
 INSERT INTO card_sync_state(id) VALUES(1);
 `)
	if err != nil {
		t.Fatal(err)
	}
	oracleID := "00000000-0000-0000-0000-000000000001"
	printID := "00000000-0000-0000-0000-000000000002"
	payload := `[{"id":"` + printID + `","oracle_id":"` + oracleID + `","name":"Stream Card","lang":"en","games":["paper"],"set":"tst","released_at":"2026-01-01","image_uris":{"normal":"https://example.test/card.jpg"},"legalities":{"commander":"legal"}}]`
	dec, cleanup, err := newBulkJSONDecoderFromReader(strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	oracles, err := decodeOracleRowsFromDecoder(dec, 0)
	if err != nil {
		t.Fatal(err)
	}
	dec, cleanup, err = newBulkJSONDecoderFromReader(strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	spool, err := spoolPaperPrints(ctx, dec, oracles)
	if err != nil {
		t.Fatal(err)
	}
	defer spool.close()
	prints := func(visit func(printBulkRow) error) error { return spool.walk(ctx, spool.paperIDs, visit) }
	if err := applyBulkRowStream(ctx, db, oracles, prints, time.Now(), true); err != nil {
		t.Fatal(err)
	}
	var name, image, defaultID string
	if err := db.QueryRow(`SELECT name,default_image_uri,default_print_id::text FROM oracle_cards WHERE oracle_id=$1`, oracleID).Scan(&name, &image, &defaultID); err != nil {
		t.Fatal(err)
	}
	if name != "Stream Card" || defaultID != printID || image != "https://example.test/card.jpg" {
		t.Fatalf("bad imported metadata: %s %s %s", name, image, defaultID)
	}
	var before time.Time
	if err := db.QueryRow(`SELECT last_success_at FROM card_sync_state WHERE id=1`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	want := errors.New("interrupted printing stream")
	oracles[0].Name = "Partial change must roll back"
	fail := func(visit func(printBulkRow) error) error {
		if err := prints(visit); err != nil {
			return err
		}
		return want
	}
	if err := applyBulkRowStream(ctx, db, oracles, fail, time.Now(), true); !errors.Is(err, want) {
		t.Fatalf("stream failure lost: %v", err)
	}
	var after time.Time
	if err := db.QueryRow(`SELECT name FROM oracle_cards WHERE oracle_id=$1`, oracleID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT last_success_at FROM card_sync_state WHERE id=1`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if name != "Stream Card" || !after.Equal(before) {
		t.Fatal("failed stream committed partial data or sync success")
	}
}
