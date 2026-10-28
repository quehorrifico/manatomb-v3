package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"manatomb/app/internal/cards"
)

func TestEditorBulkResolvePinsDefaultPrintingPostgres(t *testing.T) {
	databaseURL := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	if err := cards.EnsureCardsTable(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	const oracleID = "00000000-0000-0000-0000-000000000081"
	const printID = "00000000-0000-0000-0000-000000000082"
	if _, err := database.Exec(`
 CREATE TEMP TABLE oracle_cards (LIKE public.oracle_cards INCLUDING DEFAULTS INCLUDING GENERATED INCLUDING INDEXES);
 CREATE TEMP TABLE card_prints (LIKE public.card_prints INCLUDING DEFAULTS INCLUDING GENERATED INCLUDING INDEXES);
 INSERT INTO oracle_cards(oracle_id,name,default_print_id,default_set_code,default_set_name) VALUES ('` + oracleID + `','Bulk Test Card','` + printID + `','tst','Test Set');
 INSERT INTO card_prints(scryfall_id,oracle_id,name,set_code,collector_number) VALUES ('` + printID + `','` + oracleID + `','Bulk Test Card','tst','7');
 `); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: database}
	for _, text := range []string{"4 Bulk Test Card", "Format: Commander\nCommander:\n1 Bulk Test Card\nMainboard:\n3 Bulk Test Card"} {
		req := httptest.NewRequest(http.MethodPost, "/decks/import", strings.NewReader(url.Values{"intent": {"resolve"}, "decklist": {text}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		app.HandleDeckImportText(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("resolve returned %d: %s", rec.Code, rec.Body.String())
		}
		var result workbenchImportPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.CommanderName != "" || len(result.Cards) == 0 {
			t.Fatalf("bulk add changed commander or lost cards: %#v", result)
		}
		for _, card := range result.Cards {
			if card.PreferredPrintID != printID || card.Meta.PrintID != printID || card.Meta.SetCode != "tst" || card.Meta.CollectorNumber != "7" {
				t.Fatalf("bulk resolve lost default-print identity: %#v", card)
			}
		}
	}
	// A single unresolved entry rejects the whole list with actionable feedback.
	req := httptest.NewRequest(http.MethodPost, "/decks/import", strings.NewReader(url.Values{"intent": {"resolve"}, "decklist": {"1 Bulk Test Card\n1 Definitely Missing Card"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.HandleDeckImportText(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Definitely Missing Card") {
		t.Fatalf("bad list was not rejected: %d %s", rec.Code, rec.Body.String())
	}
}
