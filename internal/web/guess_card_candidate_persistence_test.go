package web

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"manatomb/app/internal/cards"
)

var guessCardCandidatePersistenceDriverSequence atomic.Uint64

type guessCardCandidatePersistenceDriver struct {
	updateQuery string
	updateArgs  []driver.NamedValue
	committed   atomic.Bool
}

func (d *guessCardCandidatePersistenceDriver) Open(string) (driver.Conn, error) {
	return &guessCardCandidatePersistenceConn{driver: d}, nil
}

type guessCardCandidatePersistenceConn struct {
	driver *guessCardCandidatePersistenceDriver
}

func (*guessCardCandidatePersistenceConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (*guessCardCandidatePersistenceConn) Close() error {
	return nil
}

func (c *guessCardCandidatePersistenceConn) Begin() (driver.Tx, error) {
	return &guessCardCandidatePersistenceTx{driver: c.driver}, nil
}

func (c *guessCardCandidatePersistenceConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}

func (*guessCardCandidatePersistenceConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "wrong_guess_oracle_ids") || !strings.Contains(query, "FOR UPDATE") {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	return &guessCardCandidatePersistenceRows{}, nil
}

func (c *guessCardCandidatePersistenceConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if !strings.Contains(query, "UPDATE user_guess_card_games") {
		return nil, fmt.Errorf("unexpected exec: %s", query)
	}
	c.driver.updateQuery = query
	c.driver.updateArgs = append([]driver.NamedValue(nil), args...)
	return driver.RowsAffected(1), nil
}

type guessCardCandidatePersistenceTx struct {
	driver *guessCardCandidatePersistenceDriver
}

func (tx *guessCardCandidatePersistenceTx) Commit() error {
	tx.driver.committed.Store(true)
	return nil
}

func (*guessCardCandidatePersistenceTx) Rollback() error {
	return nil
}

type guessCardCandidatePersistenceRows struct {
	returned bool
}

func (*guessCardCandidatePersistenceRows) Columns() []string {
	return []string{
		"guess_count",
		"question_count",
		"is_daily",
		"daily_key",
		"user_id",
		"target_oracle_id",
		"target_scryfall_id",
		"asked_questions",
		"wrong_guess_oracle_ids",
		"history_events",
	}
}

func (*guessCardCandidatePersistenceRows) Close() error {
	return nil
}

func (r *guessCardCandidatePersistenceRows) Next(dest []driver.Value) error {
	if r.returned {
		return io.EOF
	}
	r.returned = true
	copy(dest, []driver.Value{
		int64(0),
		int64(2),
		false,
		"2026-08-05",
		int64(7),
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		"{color_w,creature}",
		"{}",
		"{}",
	})
	return nil
}

func openGuessCardCandidatePersistenceDB(t *testing.T) (*sql.DB, *guessCardCandidatePersistenceDriver) {
	t.Helper()

	dbDriver := &guessCardCandidatePersistenceDriver{}
	driverName := "guess-card-candidate-persistence-" + strconv.FormatUint(guessCardCandidatePersistenceDriverSequence.Add(1), 10)
	sql.Register(driverName, dbDriver)
	database, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database, dbDriver
}

func TestRecordGuessCardFinalGuessPersistsResolvedWrongOracleID(t *testing.T) {
	const (
		targetOracleID = "11111111-1111-1111-1111-111111111111"
		wrongOracleID  = "33333333-3333-3333-3333-333333333333"
	)
	database, dbDriver := openGuessCardCandidatePersistenceDB(t)

	result, err := recordGuessCardFinalGuess(
		context.Background(),
		database,
		42,
		gamePlayer{UserID: 7},
		cards.Card{OracleID: targetOracleID},
		wrongOracleID,
		"Polluted Delta",
		1,
	)
	if err != nil {
		t.Fatalf("recordGuessCardFinalGuess: %v", err)
	}
	if result.Won || result.GuessCount != 1 {
		t.Fatalf("result = %#v, want one recorded wrong guess", result)
	}
	if !dbDriver.committed.Load() {
		t.Fatal("wrong guess was not committed")
	}
	if !strings.Contains(dbDriver.updateQuery, "wrong_guess_oracle_ids = $3::uuid[]") || !strings.Contains(dbDriver.updateQuery, "history_events = $4::text[]") {
		t.Fatalf("wrong-guess update does not persist the UUID array: %s", dbDriver.updateQuery)
	}
	if len(dbDriver.updateArgs) != 4 || !strings.Contains(fmt.Sprint(dbDriver.updateArgs[2].Value), wrongOracleID) {
		t.Fatalf("wrong-guess update args = %#v, want persisted %s", dbDriver.updateArgs, wrongOracleID)
	}
	historyArg := fmt.Sprint(dbDriver.updateArgs[3].Value)
	for _, event := range []string{"question:color_w", "question:creature", "guess:Polluted Delta"} {
		if !strings.Contains(historyArg, event) {
			t.Fatalf("history update arg = %q, missing %q", historyArg, event)
		}
	}
}

func TestRecordGuessCardFinalGuessPersistsUnmatchedTypedName(t *testing.T) {
	const targetOracleID = "11111111-1111-1111-1111-111111111111"
	database, dbDriver := openGuessCardCandidatePersistenceDB(t)

	result, err := recordGuessCardFinalGuess(
		context.Background(),
		database,
		42,
		gamePlayer{UserID: 7},
		cards.Card{OracleID: targetOracleID},
		"",
		"Definitely Not a Real Card",
		1,
	)
	if err != nil {
		t.Fatalf("recordGuessCardFinalGuess: %v", err)
	}
	if result.Won || result.GuessCount != 1 {
		t.Fatalf("result = %#v, want one recorded unmatched guess", result)
	}
	historyArg := fmt.Sprint(dbDriver.updateArgs[3].Value)
	if !strings.Contains(historyArg, "guess:Definitely Not a Real Card") {
		t.Fatalf("history update arg = %q, want unmatched typed name", historyArg)
	}
}

func TestRecordGuessCardFinalGuessDoesNotAddCorrectGuessToHistory(t *testing.T) {
	const targetOracleID = "11111111-1111-1111-1111-111111111111"
	database, dbDriver := openGuessCardCandidatePersistenceDB(t)

	result, err := recordGuessCardFinalGuess(
		context.Background(),
		database,
		42,
		gamePlayer{UserID: 7},
		cards.Card{OracleID: targetOracleID},
		targetOracleID,
		"Sample Flyer",
		1,
	)
	if err != nil {
		t.Fatalf("recordGuessCardFinalGuess: %v", err)
	}
	if !result.Won || result.GuessCount != 1 {
		t.Fatalf("result = %#v, want one correct guess", result)
	}
	if strings.Contains(dbDriver.updateQuery, "history_events") {
		t.Fatalf("correct-guess update unexpectedly changed history: %s", dbDriver.updateQuery)
	}
}
