package web

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"manatomb/app/internal/account"
)

// Run only against an explicitly supplied disposable PostgreSQL database.
// All deck rows and sequences are temporary and disappear when this connection closes.
func TestDeckAutosavePostgres(t *testing.T) {
	url := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set MANATOMB_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TEMP TABLE decks (
 id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, name TEXT NOT NULL,
 description TEXT, tags TEXT, format TEXT, commander_name TEXT, commander_print_id UUID,
 is_public BOOLEAN DEFAULT FALSE, public_slug TEXT, published_at TIMESTAMPTZ,
 power_bracket TEXT, created_at TIMESTAMPTZ DEFAULT NOW(), updated_at TIMESTAMPTZ DEFAULT NOW());`)
	if err != nil {
		t.Fatal(err)
	}
	withRendererRoot(t)
	app := &App{DB: db, Renderer: NewRenderer()}
	owner := &account.User{ID: 7}
	request := httptest.NewRequest(http.MethodGet, "/decks/new/workbench?format=Sandbox&reset=1", nil)
	request = request.WithContext(context.WithValue(request.Context(), ctxKeyUser, owner))
	response := httptest.NewRecorder()
	app.HandleDeckWorkbench(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/decks/1" {
		t.Fatalf("editor entry did not redirect to a committed account deck: %d %s", response.Code, response.Header().Get("Location"))
	}
	var ownerID int64
	var name, format string
	var public bool
	if err := db.QueryRow("SELECT user_id,name,format,is_public FROM decks WHERE id=1").Scan(&ownerID, &name, &format, &public); err != nil {
		t.Fatal(err)
	}
	if ownerID != owner.ID || name == "" || format != "Sandbox" || public {
		t.Fatalf("wrong initial deck: %d %q %q public=%v", ownerID, name, format, public)
	}
	// Autosave the name, notes and tags in the same account transaction.
	post := func(user *account.User, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/decks/1", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser, user))
		w := httptest.NewRecorder()
		app.HandleDeckShow(w, r)
		return w
	}
	if w := post(owner, "action=save_overview&name=Recovered+Deck&description=Durable+notes&tags=Aggro&format=Sandbox"); w.Code != http.StatusSeeOther {
		t.Fatalf("autosave failed: %d %s", w.Code, w.Body.String())
	}
	var description, tags string
	if err := db.QueryRow("SELECT name,description,tags FROM decks WHERE id=1").Scan(&name, &description, &tags); err != nil {
		t.Fatal(err)
	}
	if name != "Recovered Deck" || description != "Durable notes" || tags != "Aggro" {
		t.Fatalf("metadata not saved: %q %q %q", name, description, tags)
	}
	if w := post(owner, "action=save_overview&name=&description=Erase"); w.Code != http.StatusBadRequest {
		t.Fatalf("blank name status=%d", w.Code)
	}
	if w := post(&account.User{ID: 8}, "action=save_overview&name=Another+owner"); w.Code != http.StatusNotFound {
		t.Fatalf("non-owner save status=%d", w.Code)
	}
	if err := db.QueryRow("SELECT name,description FROM decks WHERE id=1").Scan(&name, &description); err != nil {
		t.Fatal(err)
	}
	if name != "Recovered Deck" || description != "Durable notes" {
		t.Fatal("rejected saves changed the deck")
	}
	// Explicit guest/import handoffs must keep their local draft rather than
	// silently replacing imported cards with a new empty account deck.
	r := httptest.NewRequest(http.MethodGet, "/decks/new/workbench?format=Sandbox&save_guest=1", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser, owner))
	w := httptest.NewRecorder()
	app.HandleDeckWorkbench(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "importDraftToAccount()") {
		t.Fatalf("import recovery lost: %d", w.Code)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM decks").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("import handoff created %d decks before restoring the local draft", count)
	}
}
