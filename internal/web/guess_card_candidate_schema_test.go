package web

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var guessCardCandidateSchemaDriverSequence atomic.Uint64

type guessCardCandidateSchemaDriver struct {
	mu         sync.Mutex
	statements []string
}

type guessCardCandidateScanRow struct{}

func (guessCardCandidateScanRow) Scan(dest ...any) error {
	if len(dest) != 17 {
		return errors.New("unexpected guess-card scan column count")
	}
	*dest[0].(*int64) = 42
	*dest[1].(*sql.NullInt64) = sql.NullInt64{Int64: 7, Valid: true}
	*dest[2].(*string) = ""
	*dest[3].(*string) = "11111111-1111-1111-1111-111111111111"
	*dest[4].(*string) = "22222222-2222-2222-2222-222222222222"
	*dest[5].(*string) = "active"
	*dest[6].(*int) = 1
	*dest[7].(*int) = 2
	*dest[8].(*bool) = true
	*dest[9].(*string) = "2026-08-05"
	*dest[10].(*bool) = false
	*dest[11].(*int) = 8
	if err := dest[12].(interface{ Scan(any) error }).Scan(`{color_w}`); err != nil {
		return err
	}
	if err := dest[13].(interface{ Scan(any) error }).Scan(`{33333333-3333-3333-3333-333333333333}`); err != nil {
		return err
	}
	if err := dest[14].(interface{ Scan(any) error }).Scan(`{question:color_w,"guess:Polluted Delta"}`); err != nil {
		return err
	}
	*dest[15].(*time.Time) = time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	*dest[16].(*sql.NullTime) = sql.NullTime{}
	return nil
}

func (d *guessCardCandidateSchemaDriver) Open(string) (driver.Conn, error) {
	return &guessCardCandidateSchemaConn{driver: d}, nil
}

func (d *guessCardCandidateSchemaDriver) record(statement string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.statements = append(d.statements, statement)
}

func (d *guessCardCandidateSchemaDriver) joinedStatements() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.Join(d.statements, "\n")
}

type guessCardCandidateSchemaConn struct {
	driver *guessCardCandidateSchemaDriver
}

func (*guessCardCandidateSchemaConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (*guessCardCandidateSchemaConn) Close() error {
	return nil
}

func (*guessCardCandidateSchemaConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

func (c *guessCardCandidateSchemaConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.driver.record(query)
	return driver.RowsAffected(0), nil
}

func TestEnsureFeatureTablesPersistsResolvedWrongGuessOracleIDs(t *testing.T) {
	dbDriver := &guessCardCandidateSchemaDriver{}
	driverName := "guess-card-candidate-schema-" + strconv.FormatUint(guessCardCandidateSchemaDriverSequence.Add(1), 10)
	sql.Register(driverName, dbDriver)

	database, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := EnsureFeatureTables(context.Background(), database); err != nil {
		t.Fatalf("EnsureFeatureTables: %v", err)
	}

	schemaSQL := dbDriver.joinedStatements()
	for _, required := range []string{
		"wrong_guess_oracle_ids UUID[] NOT NULL DEFAULT ARRAY[]::UUID[]",
		"ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS wrong_guess_oracle_ids UUID[] NOT NULL DEFAULT ARRAY[]::UUID[]",
		"history_events TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]",
		"ALTER TABLE user_guess_card_games ADD COLUMN IF NOT EXISTS history_events TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]",
		"SET history_events = ARRAY(",
		"SELECT 'question:' || question_id",
	} {
		if !strings.Contains(schemaSQL, required) {
			t.Fatalf("guess-card candidate schema is missing %q", required)
		}
	}
}

func TestScanGuessCardGameLoadsResolvedWrongGuessOracleIDs(t *testing.T) {
	t.Parallel()

	game, err := scanGuessCardGame(guessCardCandidateScanRow{})
	if err != nil {
		t.Fatalf("scanGuessCardGame: %v", err)
	}
	if len(game.AskedQuestions) != 1 || game.AskedQuestions[0] != "color_w" {
		t.Fatalf("asked questions = %#v", game.AskedQuestions)
	}
	if len(game.WrongGuessOracleIDs) != 1 || game.WrongGuessOracleIDs[0] != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("wrong guess oracle IDs = %#v", game.WrongGuessOracleIDs)
	}
	if len(game.HistoryEvents) != 2 || game.HistoryEvents[0] != "question:color_w" || game.HistoryEvents[1] != "guess:Polluted Delta" {
		t.Fatalf("history events = %#v", game.HistoryEvents)
	}
}
