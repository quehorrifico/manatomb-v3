package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/forge"
)

// Uses an explicitly supplied disposable database. Fixture rows are temporary.
func TestCPUPrintingsAndTokenArtworkPostgres(t *testing.T) {
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
	if err = cards.EnsureCardsTable(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
 CREATE TEMP TABLE oracle_cards (LIKE public.oracle_cards INCLUDING DEFAULTS INCLUDING GENERATED);
 CREATE TEMP TABLE card_prints (LIKE public.card_prints INCLUDING DEFAULTS INCLUDING GENERATED);
 ALTER TABLE pg_temp.card_prints ALTER COLUMN name SET DEFAULT 'Fixture';
 CREATE TEMP TABLE decks (id bigint,user_id bigint,name text,description text,tags text,format text,commander_name text,commander_print_id uuid,is_public boolean,public_slug text,published_at timestamptz,power_bracket text,created_at timestamptz,updated_at timestamptz);
 CREATE TEMP TABLE deck_cards (deck_id bigint,oracle_id uuid,qty integer,board text,preferred_print_id uuid);
 INSERT INTO oracle_cards (oracle_id,name,type_line,layout,power_text,toughness_text,default_print_id) VALUES
 ('00000000-0000-0000-0000-000000000001','Isamaru, Hound of Konda','Legendary Creature — Dog','normal','2','2','00000000-0000-0000-0000-000000000101'),
 ('00000000-0000-0000-0000-000000000002','Plains','Basic Land — Plains','normal',null,null,'00000000-0000-0000-0000-000000000102'),
 ('00000000-0000-0000-0000-000000000003','Egg','Token Creature — Egg','token','0','1','00000000-0000-0000-0000-000000000103'),
 ('00000000-0000-0000-0000-000000000004','Egg','Creature — Egg','normal','0','1','00000000-0000-0000-0000-000000000104');
 INSERT INTO card_prints (scryfall_id,oracle_id,set_code,collector_number,image_uri,released_at) VALUES
 ('00000000-0000-0000-0000-000000000101','00000000-0000-0000-0000-000000000001','old','1','https://example.test/old-commander.jpg','2004-10-01'),
 ('00000000-0000-0000-0000-000000000102','00000000-0000-0000-0000-000000000002','new','2','https://example.test/new-plains.jpg','2026-01-01'),
 ('00000000-0000-0000-0000-000000000105','00000000-0000-0000-0000-000000000002','old','5','https://example.test/saved-plains.jpg','2004-10-01'),
 ('00000000-0000-0000-0000-000000000103','00000000-0000-0000-0000-000000000003','tc19','3','https://example.test/egg-token.jpg','2019-08-01'),
 ('00000000-0000-0000-0000-000000000104','00000000-0000-0000-0000-000000000004','new','4','https://example.test/ordinary-egg.jpg','2026-01-01');
 INSERT INTO decks VALUES (1,42,'Saved printing deck','','','Commander','Isamaru, Hound of Konda','00000000-0000-0000-0000-000000000101',false,'',null,'',now(),now());
 INSERT INTO deck_cards VALUES (1,'00000000-0000-0000-0000-000000000002',99,'main','00000000-0000-0000-0000-000000000105');
 `)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{DB: db, Forge: &forge.Client{}}
	r := httptest.NewRequest("GET", "/api/cpu/decks/1", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser, &account.User{ID: 42}))
	w := httptest.NewRecorder()
	app.CPUAPI(w, r)
	if w.Code != 200 {
		t.Fatalf("deck snapshot: %d %s", w.Code, w.Body.String())
	}
	var snapshot forge.Deck
	if err = json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Main[0].Printing != "00000000-0000-0000-0000-000000000105" || snapshot.Commanders[0].Printing != "00000000-0000-0000-0000-000000000101" {
		t.Fatal("saved printing lost", w.Body.String())
	}
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser, &account.User{ID: 99}))
	w = httptest.NewRecorder()
	app.CPUAPI(w, r)
	if w.Code != 404 {
		t.Fatal("another owner read saved deck", w.Code)
	}
	for _, tc := range []struct {
		path, want string
		status     int
	}{
		{"/cards/resolve?q=Plains&printing=00000000-0000-0000-0000-000000000105", "saved-plains.jpg", 200},
		{"/cards/resolve?q=Egg&token=1&type=Creature%20%E2%80%94%20Egg&pt=0%2F1", "egg-token.jpg", 200},
		{"/cards/resolve?q=Egg&token=1&type=Creature%20%E2%80%94%20Egg&pt=2%2F2", "", 404},
		{"/cards/resolve?q=Forest&printing=00000000-0000-0000-0000-000000000105", "", 404},
		{"/cards/resolve?q=Plains&printing=https://example.test/anything", "", 400},
	} {
		w = httptest.NewRecorder()
		app.HandleCardResolve(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status || tc.want != "" && !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}
