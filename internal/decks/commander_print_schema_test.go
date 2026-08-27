package decks

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

var commanderPrintSchemaDriverSequence atomic.Uint64

type commanderPrintSchemaDriver struct {
	mu         sync.Mutex
	statements []string
}

func (d *commanderPrintSchemaDriver) Open(string) (driver.Conn, error) {
	return &commanderPrintSchemaConn{driver: d}, nil
}

func (d *commanderPrintSchemaDriver) record(query string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.statements = append(d.statements, query)
}

func (d *commanderPrintSchemaDriver) joinedStatements() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.Join(d.statements, "\n")
}

type commanderPrintSchemaConn struct {
	driver *commanderPrintSchemaDriver
}

func (c *commanderPrintSchemaConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (c *commanderPrintSchemaConn) Close() error {
	return nil
}

func (c *commanderPrintSchemaConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

func (c *commanderPrintSchemaConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.driver.record(query)
	return driver.RowsAffected(0), nil
}

func (c *commanderPrintSchemaConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.driver.record(query)
	return &commanderPrintSchemaBoolRows{}, nil
}

type commanderPrintSchemaBoolRows struct {
	returned bool
}

func (*commanderPrintSchemaBoolRows) Columns() []string {
	return []string{"exists"}
}

func (*commanderPrintSchemaBoolRows) Close() error {
	return nil
}

func (r *commanderPrintSchemaBoolRows) Next(dest []driver.Value) error {
	if r.returned {
		return io.EOF
	}
	r.returned = true
	dest[0] = false
	return nil
}

func assertCommanderPrintSchemaSQL(t *testing.T, schemaSQL string) {
	t.Helper()

	for _, snippet := range []string{
		"ADD COLUMN IF NOT EXISTS commander_print_id UUID NULL",
		"SELECT COALESCE(dc.preferred_print_id, oc.default_print_id)",
		"SELECT oc.default_print_id",
		"ADD CONSTRAINT fk_decks_commander_print",
		"REFERENCES card_prints(scryfall_id)",
		"ON DELETE SET NULL",
		"CREATE INDEX IF NOT EXISTS idx_decks_commander_print_id",
	} {
		if !strings.Contains(schemaSQL, snippet) {
			t.Fatalf("commander-print schema is missing %q", snippet)
		}
	}
}

func TestEnsureDeckTablesIncludesCommanderPrintCompatibility(t *testing.T) {
	dbDriver := &commanderPrintSchemaDriver{}
	driverName := "commander-print-schema-" + strconv.FormatUint(commanderPrintSchemaDriverSequence.Add(1), 10)
	sql.Register(driverName, dbDriver)

	database, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := EnsureDeckTables(context.Background(), database); err != nil {
		t.Fatalf("EnsureDeckTables: %v", err)
	}

	schemaSQL := dbDriver.joinedStatements()
	assertCommanderPrintSchemaSQL(t, schemaSQL)
	if !strings.Contains(schemaSQL, "AND conrelid = 'decks'::regclass") {
		t.Fatal("commander-print foreign-key lookup is not scoped to the decks table")
	}
}

func TestCommanderPrintMigrationMatchesRuntimeCompatibility(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate commander-print schema test")
	}
	migrationPath := filepath.Join(filepath.Dir(currentFile), "..", "db", "migrations", "0010_add_deck_commander_print.sql")
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read commander-print migration: %v", err)
	}

	assertCommanderPrintSchemaSQL(t, string(migrationSQL))
}
