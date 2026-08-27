package cards

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

var cardLookupDriverSequence atomic.Uint64

type cardLookupCaptureDriver struct {
	mu    sync.Mutex
	query string
}

func (d *cardLookupCaptureDriver) Open(string) (driver.Conn, error) {
	return &cardLookupCaptureConn{driver: d}, nil
}

func (d *cardLookupCaptureDriver) setQuery(query string) {
	d.mu.Lock()
	d.query = query
	d.mu.Unlock()
}

func (d *cardLookupCaptureDriver) lastQuery() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.query
}

type cardLookupCaptureConn struct {
	driver *cardLookupCaptureDriver
}

func (c *cardLookupCaptureConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (c *cardLookupCaptureConn) Close() error { return nil }

func (c *cardLookupCaptureConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

func (c *cardLookupCaptureConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.driver.setQuery(query)
	return cardLookupEmptyRows{}, nil
}

type cardLookupEmptyRows struct{}

func (cardLookupEmptyRows) Columns() []string {
	return []string{
		"name_search", "oracle_id", "name", "mana_cost", "type_line", "oracle_text", "flavor_text",
		"all_parts_json", "image_uri", "art_crop_uri", "colors", "color_identity", "cmc", "price_usd",
		"artist", "set_code", "set_name", "collector_number", "is_commander_candidate",
	}
}

func (cardLookupEmptyRows) Close() error { return nil }

func (cardLookupEmptyRows) Next([]driver.Value) error { return io.EOF }

func openCardLookupCaptureDB(t *testing.T) (*sql.DB, *cardLookupCaptureDriver) {
	t.Helper()
	d := &cardLookupCaptureDriver{}
	driverName := "card_lookup_capture_" + strings.TrimSpace(strings.ToLower(t.Name())) + "_" +
		string(rune('a'+cardLookupDriverSequence.Add(1)%26))
	sql.Register(driverName, d)
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open lookup capture db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, d
}

func assertPlayableCardLookupOrder(t *testing.T, query string) {
	t.Helper()
	legal := strings.Index(query, "COALESCE(oc.legal_anywhere, true) DESC")
	token := strings.Index(query, "CASE WHEN lower(btrim(COALESCE(oc.layout, ''))) IN ('token', 'double_faced_token') THEN 1 ELSE 0 END ASC")
	rank := strings.Index(query, "COALESCE(oc.edhrec_rank, 999999) ASC")
	if legal < 0 || token < 0 || rank < 0 || !(legal < token && token < rank) {
		t.Fatalf("card lookup does not prefer playable, non-token cards before EDHREC rank: %s", query)
	}
}

func TestLookupCardsByNamesPrefersPlayableCardOverSameNameToken(t *testing.T) {
	db, capture := openCardLookupCaptureDB(t)
	if _, err := LookupCardsByNames(context.Background(), db, []string{"Llanowar Elves"}); err != nil {
		t.Fatalf("LookupCardsByNames: %v", err)
	}
	assertPlayableCardLookupOrder(t, capture.lastQuery())
}

func TestEnsureCardByNamePrefersPlayableCardOverSameNameToken(t *testing.T) {
	db, capture := openCardLookupCaptureDB(t)
	_, err := EnsureCardByName(context.Background(), db, "Llanowar Elves")
	if !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("EnsureCardByName error = %v, want ErrCardNotFound from empty capture rows", err)
	}
	assertPlayableCardLookupOrder(t, capture.lastQuery())
}

var _ driver.QueryerContext = (*cardLookupCaptureConn)(nil)
