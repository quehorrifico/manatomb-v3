package cards

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

var randomizedCommanderBatchDriverSequence atomic.Uint64

type randomizedCommanderBatchQueryDriver struct {
	query string
	args  []driver.NamedValue
}

func (d *randomizedCommanderBatchQueryDriver) Open(string) (driver.Conn, error) {
	return &randomizedCommanderBatchQueryConn{driver: d}, nil
}

type randomizedCommanderBatchQueryConn struct {
	driver *randomizedCommanderBatchQueryDriver
}

func (c *randomizedCommanderBatchQueryConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (c *randomizedCommanderBatchQueryConn) Close() error {
	return nil
}

func (c *randomizedCommanderBatchQueryConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

func (c *randomizedCommanderBatchQueryConn) QueryContext(
	_ context.Context,
	query string,
	args []driver.NamedValue,
) (driver.Rows, error) {
	c.driver.query = query
	c.driver.args = append([]driver.NamedValue(nil), args...)
	return randomizedCommanderBatchEmptyRows{}, nil
}

type randomizedCommanderBatchEmptyRows struct{}

func (randomizedCommanderBatchEmptyRows) Columns() []string {
	return []string{"unused"}
}

func (randomizedCommanderBatchEmptyRows) Close() error {
	return nil
}

func (randomizedCommanderBatchEmptyRows) Next([]driver.Value) error {
	return io.EOF
}

func openRandomizedCommanderBatchQueryDB(
	t *testing.T,
) (*sql.DB, *randomizedCommanderBatchQueryDriver) {
	t.Helper()

	queryDriver := &randomizedCommanderBatchQueryDriver{}
	driverName := fmt.Sprintf(
		"randomized_commander_batch_query_test_%d",
		randomizedCommanderBatchDriverSequence.Add(1),
	)
	sql.Register(driverName, queryDriver)
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db, queryDriver
}

func TestRandomizedTopCommanderBatchUsesSeededKeysetOrdering(t *testing.T) {
	db, queryDriver := openRandomizedCommanderBatchQueryDB(t)
	const (
		seed          = "picker-seed"
		afterOracleID = "123e4567-e89b-12d3-a456-426614174000"
	)

	got, err := RandomizedTopCommanderBatch(
		context.Background(),
		db,
		seed,
		afterOracleID,
		9,
		0,
	)
	if err != nil {
		t.Fatalf("RandomizedTopCommanderBatch: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("RandomizedTopCommanderBatch returned %d cards from an empty result", len(got))
	}

	normalizedQuery := strings.Join(strings.Fields(strings.ToLower(queryDriver.query)), " ")
	if strings.Contains(normalizedQuery, "random()") {
		t.Fatalf("batch query should keep one deterministic order per seed: %s", queryDriver.query)
	}
	for _, snippet := range []string{
		"oc.is_commander_candidate = true",
		"oc.edhrec_rank <= $2",
		"nullif($3::text, '')::uuid",
		"md5($1::text || oc.oracle_id::text)",
		"order by md5($1::text || oc.oracle_id::text), oc.oracle_id asc",
		"limit $4",
	} {
		if !strings.Contains(normalizedQuery, snippet) {
			t.Fatalf("batch query missing %q: %s", snippet, queryDriver.query)
		}
	}

	wantArgs := []any{seed, int64(1500), afterOracleID, int64(9)}
	if len(queryDriver.args) != len(wantArgs) {
		t.Fatalf("batch query args = %#v, want %#v", queryDriver.args, wantArgs)
	}
	for index, want := range wantArgs {
		if got := queryDriver.args[index].Value; got != want {
			t.Fatalf("batch query arg %d = %#v, want %#v", index+1, got, want)
		}
	}
}

func TestRandomizedTopCommanderBatchUsesCallerLimitAndRank(t *testing.T) {
	db, queryDriver := openRandomizedCommanderBatchQueryDB(t)

	_, err := RandomizedTopCommanderBatch(
		context.Background(),
		db,
		"  another-seed  ",
		"  ",
		17,
		725,
	)
	if err != nil {
		t.Fatalf("RandomizedTopCommanderBatch: %v", err)
	}

	wantArgs := []any{"another-seed", int64(725), "", int64(17)}
	if len(queryDriver.args) != len(wantArgs) {
		t.Fatalf("batch query args = %#v, want %#v", queryDriver.args, wantArgs)
	}
	for index, want := range wantArgs {
		if got := queryDriver.args[index].Value; got != want {
			t.Fatalf("batch query arg %d = %#v, want %#v", index+1, got, want)
		}
	}
}
