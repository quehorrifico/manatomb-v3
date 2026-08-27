package web

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

	"manatomb/app/internal/cards"
)

var importPrintingLookupDriverSequence atomic.Uint64

type importPrintingLookupDriver struct {
	printingID string
	query      string
	args       []driver.NamedValue
}

func (d *importPrintingLookupDriver) Open(string) (driver.Conn, error) {
	return &importPrintingLookupConn{driver: d}, nil
}

type importPrintingLookupConn struct {
	driver *importPrintingLookupDriver
}

func (c *importPrintingLookupConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (c *importPrintingLookupConn) Close() error {
	return nil
}

func (c *importPrintingLookupConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

func (c *importPrintingLookupConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.driver.query = query
	c.driver.args = append([]driver.NamedValue(nil), args...)
	return &importPrintingLookupRows{printingID: c.driver.printingID}, nil
}

type importPrintingLookupRows struct {
	printingID string
	read       bool
}

func (r *importPrintingLookupRows) Columns() []string {
	return []string{"scryfall_id"}
}

func (r *importPrintingLookupRows) Close() error {
	return nil
}

func (r *importPrintingLookupRows) Next(dest []driver.Value) error {
	if r.read || r.printingID == "" {
		return io.EOF
	}
	r.read = true
	dest[0] = r.printingID
	return nil
}

func openImportPrintingLookupDB(t *testing.T, printingID string) (*sql.DB, *importPrintingLookupDriver) {
	t.Helper()
	dbDriver := &importPrintingLookupDriver{printingID: printingID}
	driverName := "import-printing-lookup-" + strconv.FormatUint(importPrintingLookupDriverSequence.Add(1), 10)
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

func TestLookupImportedPrintingIDUsesSetAsBestEffortConstraint(t *testing.T) {
	const (
		oracleID   = "11111111-1111-1111-1111-111111111111"
		printingID = "22222222-2222-2222-2222-222222222222"
	)
	db, dbDriver := openImportPrintingLookupDB(t, printingID)

	got, err := lookupImportedPrintingID(context.Background(), db, oracleID, "CMM", "")
	if err != nil {
		t.Fatalf("lookupImportedPrintingID: %v", err)
	}
	if got != printingID {
		t.Fatalf("printing ID = %q, want %q", got, printingID)
	}
	if len(dbDriver.args) != 3 || dbDriver.args[0].Value != oracleID || dbDriver.args[1].Value != "CMM" || dbDriver.args[2].Value != "" {
		t.Fatalf("query args = %#v", dbDriver.args)
	}
	for _, fragment := range []string{
		"$3 = '' OR lower(cp.collector_number) = lower($3)",
		"COALESCE(cp.lang, 'en') = 'en'",
		"cp.released_at DESC NULLS LAST",
	} {
		if !strings.Contains(dbDriver.query, fragment) {
			t.Fatalf("printing lookup query missing %q: %s", fragment, dbDriver.query)
		}
	}
}

func TestLookupImportedPrintingIDFallsBackOnlyForMissingSetPreference(t *testing.T) {
	db, _ := openImportPrintingLookupDB(t, "")

	got, err := lookupImportedPrintingID(
		context.Background(),
		db,
		"11111111-1111-1111-1111-111111111111",
		"CMM",
		"",
	)
	if err != nil || got != "" {
		t.Fatalf("set-only fallback = (%q, %v), want empty ID and nil error", got, err)
	}

	_, err = lookupImportedPrintingID(
		context.Background(),
		db,
		"11111111-1111-1111-1111-111111111111",
		"CMM",
		"396",
	)
	if !errors.Is(err, cards.ErrCardNotFound) {
		t.Fatalf("exact printing error = %v, want cards.ErrCardNotFound", err)
	}
}

func TestApplyCanonicalImportedPrintingPreservesDefaultMetadata(t *testing.T) {
	item := deckImportReviewItem{
		SetCode:        "MISSING",
		StatusDetail:   "Default printing",
		NeedsAttention: false,
	}
	applyCanonicalImportedPrinting(&item, cards.DBCard{
		SetCode:         "fdn",
		CollectorNumber: "231",
	})

	if item.SetCode != "FDN" || item.CollectorNumber != "231" {
		t.Fatalf("canonical metadata = %s %s, want FDN 231", item.SetCode, item.CollectorNumber)
	}
	if item.PrintingLabel != "FDN · 231" {
		t.Fatalf("printing label = %q", item.PrintingLabel)
	}
	if item.StatusDetail != "Default printing" || item.NeedsAttention {
		t.Fatalf("fallback incorrectly marked attention: %#v", item)
	}
}
