package web

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"manatomb/app/internal/account"
)

var deckOwnershipDriverSequence atomic.Uint64

type emptyDeckOwnershipDriver struct {
	queryCount  atomic.Int64
	beginCount  atomic.Int64
	queryDeckID atomic.Int64
	queryUserID atomic.Int64
}

func (d *emptyDeckOwnershipDriver) Open(string) (driver.Conn, error) {
	return &emptyDeckOwnershipConn{driver: d}, nil
}

type emptyDeckOwnershipConn struct {
	driver *emptyDeckOwnershipDriver
}

func (c *emptyDeckOwnershipConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (c *emptyDeckOwnershipConn) Close() error {
	return nil
}

func (c *emptyDeckOwnershipConn) Begin() (driver.Tx, error) {
	c.driver.beginCount.Add(1)
	return nil, errors.New("unexpected mutation transaction")
}

func (c *emptyDeckOwnershipConn) QueryContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Rows, error) {
	c.driver.queryCount.Add(1)
	if len(args) >= 2 {
		if deckID, ok := args[0].Value.(int64); ok {
			c.driver.queryDeckID.Store(deckID)
		}
		if userID, ok := args[1].Value.(int64); ok {
			c.driver.queryUserID.Store(userID)
		}
	}
	return emptyDeckOwnershipRows{}, nil
}

type emptyDeckOwnershipRows struct{}

func (emptyDeckOwnershipRows) Columns() []string {
	return []string{
		"id",
		"user_id",
		"name",
		"description",
		"tags",
		"format",
		"commander_name",
		"commander_print_id",
		"is_public",
		"public_slug",
		"published_at",
		"power_bracket",
		"created_at",
		"updated_at",
	}
}

func (emptyDeckOwnershipRows) Close() error {
	return nil
}

func (emptyDeckOwnershipRows) Next([]driver.Value) error {
	return io.EOF
}

func TestHandleDeckShowPostRejectsNonOwnerBeforeMutation(t *testing.T) {
	dbDriver := &emptyDeckOwnershipDriver{}
	driverName := "deck-ownership-" + strconv.FormatUint(deckOwnershipDriverSequence.Add(1), 10)
	sql.Register(driverName, dbDriver)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	notFoundRenderer := &Renderer{tmpl: template.Must(template.New("not_found").Parse(
		`{{define "not_found"}}not found{{end}}`,
	))}
	app := &App{
		DB:       db,
		Renderer: notFoundRenderer,
	}

	body := strings.NewReader("action=set_qty&card_id=owned-by-someone-else&quantity=4")
	req := httptest.NewRequest(http.MethodPost, "/decks/42", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUser, &account.User{ID: 7}))
	rec := httptest.NewRecorder()

	app.HandleDeckShow(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("HandleDeckShow() status = %d, want %d; body=%q", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if got := dbDriver.queryCount.Load(); got != 1 {
		t.Fatalf("ownership query count = %d, want 1", got)
	}
	if got := dbDriver.queryDeckID.Load(); got != 42 {
		t.Fatalf("ownership query deck ID = %d, want 42", got)
	}
	if got := dbDriver.queryUserID.Load(); got != 7 {
		t.Fatalf("ownership query user ID = %d, want 7", got)
	}
	if got := dbDriver.beginCount.Load(); got != 0 {
		t.Fatalf("mutation transaction count = %d, want 0", got)
	}
}
