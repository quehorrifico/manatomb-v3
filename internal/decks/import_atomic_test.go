package decks

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var importAtomicDriverSequence atomic.Uint64

type importAtomicDriver struct {
	failCardInsert bool
	committed      atomic.Bool
	rolledBack     atomic.Bool
}

func (d *importAtomicDriver) Open(string) (driver.Conn, error) {
	return &importAtomicConn{driver: d}, nil
}

type importAtomicConn struct {
	driver *importAtomicDriver
}

func (c *importAtomicConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (c *importAtomicConn) Close() error {
	return nil
}

func (c *importAtomicConn) Begin() (driver.Tx, error) {
	return &importAtomicTx{driver: c.driver}, nil
}

func (c *importAtomicConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}

func (c *importAtomicConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "INSERT INTO decks") {
		return nil, errors.New("unexpected query")
	}
	name := "Imported Deck"
	if len(args) > 1 {
		name, _ = args[1].Value.(string)
	}
	now := time.Now().UTC()
	return &importAtomicRows{
		columns: []string{
			"id", "user_id", "name", "description", "tags", "format",
			"commander_name", "commander_print_id", "is_public", "public_slug",
			"published_at", "power_bracket", "created_at", "updated_at",
		},
		values: []driver.Value{
			int64(41), int64(7), name, "", "", "Sandbox", "", "", false, "",
			nil, "", now, now,
		},
	}, nil
}

func (c *importAtomicConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "INSERT INTO deck_cards") && c.driver.failCardInsert {
		return nil, errors.New("forced card insert failure")
	}
	return driver.RowsAffected(1), nil
}

type importAtomicTx struct {
	driver *importAtomicDriver
}

func (tx *importAtomicTx) Commit() error {
	tx.driver.committed.Store(true)
	return nil
}

func (tx *importAtomicTx) Rollback() error {
	tx.driver.rolledBack.Store(true)
	return nil
}

type importAtomicRows struct {
	columns  []string
	values   []driver.Value
	returned bool
}

func (r *importAtomicRows) Columns() []string {
	return r.columns
}

func (*importAtomicRows) Close() error {
	return nil
}

func (r *importAtomicRows) Next(dest []driver.Value) error {
	if r.returned {
		return io.EOF
	}
	r.returned = true
	copy(dest, r.values)
	return nil
}

func openImportAtomicDB(t *testing.T, failCardInsert bool) (*sql.DB, *importAtomicDriver) {
	t.Helper()

	dbDriver := &importAtomicDriver{failCardInsert: failCardInsert}
	driverName := "deck-import-atomic-" + strconv.FormatUint(importAtomicDriverSequence.Add(1), 10)
	sql.Register(driverName, dbDriver)
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db, dbDriver
}

func TestCreateImportedDeckRollsBackEveryRowOnInsertFailure(t *testing.T) {
	db, dbDriver := openImportAtomicDB(t, true)
	_, err := CreateImportedDeck(context.Background(), db, 7, DeckInput{
		Name:   "Imported Deck",
		Format: "Sandbox",
	}, []ImportedDeckCardInput{
		{
			OracleID: "123e4567-e89b-12d3-a456-426614174000",
			Qty:      1,
			Board:    "main",
		},
	})
	if err == nil {
		t.Fatal("CreateImportedDeck returned nil error")
	}
	if dbDriver.committed.Load() {
		t.Fatal("failed import committed its transaction")
	}
	if !dbDriver.rolledBack.Load() {
		t.Fatal("failed import did not roll its transaction back")
	}
}

func TestCreateImportedDeckCommitsOnlyAfterAllRowsPersist(t *testing.T) {
	db, dbDriver := openImportAtomicDB(t, false)
	deck, err := CreateImportedDeck(context.Background(), db, 7, DeckInput{
		Name:   "Imported Deck",
		Format: "Sandbox",
	}, []ImportedDeckCardInput{
		{
			OracleID: "123e4567-e89b-12d3-a456-426614174000",
			Qty:      1,
			Board:    "main",
		},
	})
	if err != nil {
		t.Fatalf("CreateImportedDeck: %v", err)
	}
	if deck.ID != 41 {
		t.Fatalf("deck.ID = %d, want 41", deck.ID)
	}
	if !dbDriver.committed.Load() {
		t.Fatal("successful import did not commit")
	}
}
