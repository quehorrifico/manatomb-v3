package web

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"manatomb/app/internal/cards"
)

var spellifyCardGuessDriverSequence atomic.Uint64

type spellifyCardGuessDriver struct {
	mu             sync.Mutex
	statements     []string
	queryArguments [][]driver.NamedValue
	cardGuessCount int64
	exhausted      bool
}

func (d *spellifyCardGuessDriver) Open(string) (driver.Conn, error) {
	return &spellifyCardGuessConn{driver: d}, nil
}

func (d *spellifyCardGuessDriver) record(query string, args []driver.NamedValue) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.statements = append(d.statements, query)
	d.queryArguments = append(d.queryArguments, append([]driver.NamedValue(nil), args...))
}

func (d *spellifyCardGuessDriver) joinedStatements() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.Join(d.statements, "\n")
}

func (d *spellifyCardGuessDriver) lastQueryArguments() []driver.NamedValue {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.queryArguments) == 0 {
		return nil
	}
	return append([]driver.NamedValue(nil), d.queryArguments[len(d.queryArguments)-1]...)
}

type spellifyCardGuessConn struct {
	driver *spellifyCardGuessDriver
}

func (*spellifyCardGuessConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (*spellifyCardGuessConn) Close() error { return nil }

func (*spellifyCardGuessConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

func (c *spellifyCardGuessConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.driver.record(query, args)
	return driver.RowsAffected(0), nil
}

func (c *spellifyCardGuessConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.driver.record(query, args)
	return &spellifyCardGuessRows{
		cardGuessCount: c.driver.cardGuessCount,
		exhausted:      c.driver.exhausted,
	}, nil
}

type spellifyCardGuessRows struct {
	cardGuessCount int64
	exhausted      bool
	returned       bool
}

type spellifyReloadRow struct{}

func (spellifyReloadRow) Scan(dest ...any) error {
	if len(dest) != 14 {
		return errors.New("unexpected spellify scan width")
	}
	*dest[0].(*int64) = 42
	*dest[1].(*sql.NullInt64) = sql.NullInt64{Int64: 7, Valid: true}
	*dest[2].(*string) = ""
	*dest[3].(*string) = "11111111-1111-1111-1111-111111111111"
	*dest[4].(*string) = "22222222-2222-2222-2222-222222222222"
	*dest[5].(*string) = "active"
	if err := dest[6].(sql.Scanner).Scan([]byte(`{"a","{W}"}`)); err != nil {
		return err
	}
	*dest[7].(*int) = 2
	*dest[8].(*int) = 2
	if err := dest[9].(sql.Scanner).Scan([]byte(`{"Storm Crow","Black Lotus"}`)); err != nil {
		return err
	}
	*dest[10].(*bool) = true
	*dest[11].(*string) = "2026-08-12"
	*dest[12].(*time.Time) = time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	*dest[13].(*sql.NullTime) = sql.NullTime{}
	return nil
}

func (*spellifyCardGuessRows) Columns() []string {
	return []string{"card_guess_count", "exhausted"}
}

func (*spellifyCardGuessRows) Close() error { return nil }

func (r *spellifyCardGuessRows) Next(dest []driver.Value) error {
	if r.returned {
		return io.EOF
	}
	r.returned = true
	dest[0] = r.cardGuessCount
	dest[1] = r.exhausted
	return nil
}

func openSpellifyCardGuessDB(t *testing.T, count int64, exhausted bool) (*sql.DB, *spellifyCardGuessDriver) {
	t.Helper()
	dbDriver := &spellifyCardGuessDriver{cardGuessCount: count, exhausted: exhausted}
	driverName := "spellify-card-guess-" + strconv.FormatUint(spellifyCardGuessDriverSequence.Add(1), 10)
	sql.Register(driverName, dbDriver)
	database, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database, dbDriver
}

func TestEnsureFeatureTablesAddsSpellifyCardGuessCount(t *testing.T) {
	database, dbDriver := openSpellifyCardGuessDB(t, 0, false)
	if err := EnsureFeatureTables(t.Context(), database); err != nil {
		t.Fatalf("EnsureFeatureTables: %v", err)
	}

	schemaSQL := dbDriver.joinedStatements()
	for _, required := range []string{
		"card_guess_count INT NOT NULL DEFAULT 0",
		"ALTER TABLE user_spellify_games ADD COLUMN IF NOT EXISTS card_guess_count INT NOT NULL DEFAULT 0",
		"previous_wrong_guesses TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]",
		"ALTER TABLE user_spellify_games ADD COLUMN IF NOT EXISTS previous_wrong_guesses TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]",
	} {
		if !strings.Contains(schemaSQL, required) {
			t.Fatalf("Tombscript schema is missing %q", required)
		}
	}
}

func TestRecordSpellifyWrongCardGuessClosesThirdAttempt(t *testing.T) {
	database, dbDriver := openSpellifyCardGuessDB(t, spellifyMaxCardGuesses, true)
	result, err := recordSpellifyWrongCardGuess(
		t.Context(),
		database,
		42,
		gamePlayer{UserID: 7},
		"  Storm   Crow  ",
		spellifyMaxCardGuesses,
	)
	if err != nil {
		t.Fatalf("recordSpellifyWrongCardGuess: %v", err)
	}
	if result.CardGuessCount != spellifyMaxCardGuesses || !result.Exhausted {
		t.Fatalf("result = %#v, want exhausted third attempt", result)
	}

	query := dbDriver.joinedStatements()
	for _, required := range []string{
		"card_guess_count = card_guess_count + 1",
		"previous_wrong_guesses = array_append(previous_wrong_guesses, $4)",
		"WHEN card_guess_count + 1 >= $5 THEN 'lost'",
		"WHERE id = $1",
		"(user_id = $2 OR guest_id = NULLIF($3, '')::uuid)",
		"AND status = 'active'",
		"AND card_guess_count < $5",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("wrong-card-guess update is missing %q: %s", required, query)
		}
	}
	args := dbDriver.lastQueryArguments()
	if len(args) != 5 || args[3].Value != "Storm Crow" {
		t.Fatalf("wrong-card-guess query args = %#v, want normalized guess as fourth argument", args)
	}
}

func TestRecordSpellifyWrongCardGuessRejectsInvalidLimit(t *testing.T) {
	_, err := recordSpellifyWrongCardGuess(t.Context(), nil, 42, gamePlayer{UserID: 7}, "Storm Crow", 0)
	if !errors.Is(err, errSpellifyCardGuessUnavailable) {
		t.Fatalf("error = %v, want unavailable", err)
	}
}

func TestRecordSpellifyWrongCardGuessRejectsEmptyHistoryEntry(t *testing.T) {
	_, err := recordSpellifyWrongCardGuess(t.Context(), nil, 42, gamePlayer{UserID: 7}, " \t\n ", spellifyMaxCardGuesses)
	if !errors.Is(err, errSpellifyCardGuessUnavailable) {
		t.Fatalf("error = %v, want unavailable", err)
	}
}

func TestSpellifyPersistedWrongGuessIsSafeAndBounded(t *testing.T) {
	got := spellifyPersistedWrongGuess("  A\x00  very\nwrong\tguess  ")
	if got != "A� very wrong guess" {
		t.Fatalf("normalized guess = %q", got)
	}

	long := strings.Repeat("界", spellifyPersistedWrongGuessMaxRunes+10)
	if got := []rune(spellifyPersistedWrongGuess(long)); len(got) != spellifyPersistedWrongGuessMaxRunes {
		t.Fatalf("persisted guess length = %d, want %d", len(got), spellifyPersistedWrongGuessMaxRunes)
	}
}

func TestScanSpellifyGameReloadsPreviousWrongGuessesInOrder(t *testing.T) {
	game, err := scanSpellifyGame(spellifyReloadRow{})
	if err != nil {
		t.Fatalf("scanSpellifyGame: %v", err)
	}
	if got, want := strings.Join(game.PreviousWrongGuesses, "|"), "Storm Crow|Black Lotus"; got != want {
		t.Fatalf("reloaded previous wrong guesses = %q, want %q", got, want)
	}
	if game.CardGuessCount != 2 {
		t.Fatalf("reloaded card guess count = %d, want 2", game.CardGuessCount)
	}
}

func TestBuildSpellifyPageDataTracksSeparateCardGuessBudget(t *testing.T) {
	game := spellifyGame{
		Status:         "active",
		GuessCount:     6,
		CardGuessCount: 2,
		PreviousWrongGuesses: []string{
			"Storm Crow",
			"Black Lotus",
		},
	}
	page := buildSpellifyPageData(game, cards.Card{Name: "Sol Ring"})

	if page.GuessCount != 6 || page.RemainingGuesses != spellifyMaxGuesses-6 {
		t.Fatalf("character reveal budget changed: %#v", page)
	}
	if page.CardGuessCount != 2 || page.MaxCardGuesses != 3 || page.RemainingCardGuesses != 1 {
		t.Fatalf("card-guess budget = used %d max %d remaining %d", page.CardGuessCount, page.MaxCardGuesses, page.RemainingCardGuesses)
	}
	if !page.CanGuess {
		t.Fatal("second incorrect card guess disabled the final attempt")
	}
	if got, want := strings.Join(page.PreviousWrongGuesses, "|"), "Storm Crow|Black Lotus"; got != want {
		t.Fatalf("previous wrong guesses = %q, want %q", got, want)
	}
	page.PreviousWrongGuesses[0] = "mutated"
	if game.PreviousWrongGuesses[0] != "Storm Crow" {
		t.Fatal("page data aliases persisted wrong-guess history")
	}

	exhausted := buildSpellifyPageData(spellifyGame{
		Status:         "lost",
		GuessCount:     6,
		CardGuessCount: spellifyMaxCardGuesses,
	}, cards.Card{Name: "Sol Ring"})
	if exhausted.CanGuess || exhausted.RemainingCardGuesses != 0 {
		t.Fatalf("exhausted round still allows card guesses: %#v", exhausted)
	}
	if exhausted.RemainingGuesses != spellifyMaxGuesses-6 {
		t.Fatalf("card attempts altered character reveals: %d", exhausted.RemainingGuesses)
	}
}
