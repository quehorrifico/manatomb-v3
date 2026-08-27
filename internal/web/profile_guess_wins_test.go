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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"manatomb/app/internal/account"
)

var profileGuessWinsDriverSequence atomic.Uint64

type profileGuessWinsDriver struct {
	mu      sync.Mutex
	queries []string
}

func (d *profileGuessWinsDriver) Open(string) (driver.Conn, error) {
	return &profileGuessWinsConn{driver: d}, nil
}

func (d *profileGuessWinsDriver) record(query string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.queries = append(d.queries, query)
}

func (d *profileGuessWinsDriver) allQueries() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.Join(d.queries, "\n")
}

type profileGuessWinsConn struct {
	driver *profileGuessWinsDriver
}

func (*profileGuessWinsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (*profileGuessWinsConn) Close() error { return nil }

func (*profileGuessWinsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

func (c *profileGuessWinsConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.driver.record(query)
	if strings.Contains(query, "SELECT COUNT(*)") {
		return &profileGuessWinsRows{
			columns: []string{"count"},
			values:  [][]driver.Value{{int64(2)}},
		}, nil
	}
	if !strings.Contains(query, "FROM user_guess_card_games AS game") {
		return nil, fmt.Errorf("unexpected profile Guess query: %s", query)
	}
	wonAt := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	return &profileGuessWinsRows{
		columns: []string{
			"id", "user_id", "oracle_id", "scryfall_id", "name", "image_uri",
			"won_at", "question_count", "guess_count", "is_daily",
		},
		values: [][]driver.Value{
			{int64(12), int64(7), "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "Farseek", "https://example.test/farseek.jpg", wonAt, int64(3), int64(1), true},
			{int64(11), int64(7), "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "Farseek", "https://example.test/farseek.jpg", wonAt.Add(-time.Minute), int64(4), int64(2), false},
		},
	}, nil
}

type profileGuessWinsRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *profileGuessWinsRows) Columns() []string { return r.columns }
func (*profileGuessWinsRows) Close() error        { return nil }

func (r *profileGuessWinsRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func openProfileGuessWinsDB(t *testing.T) (*sql.DB, *profileGuessWinsDriver) {
	t.Helper()
	dbDriver := &profileGuessWinsDriver{}
	driverName := "profile-guess-wins-" + strconv.FormatUint(profileGuessWinsDriverSequence.Add(1), 10)
	sql.Register(driverName, dbDriver)
	database, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database, dbDriver
}

func TestProfileGuessWinHistoryIncludesEveryCompletedGame(t *testing.T) {
	database, dbDriver := openProfileGuessWinsDB(t)

	wins, err := listGuessCardWinsPage(context.Background(), database, 7, 24, 0)
	if err != nil {
		t.Fatalf("list Guess the Card wins: %v", err)
	}
	if len(wins) != 2 {
		t.Fatalf("Guess the Card wins = %d, want both completed games", len(wins))
	}
	if !wins[0].IsDaily || wins[1].IsDaily {
		t.Fatalf("daily/practice modes = %v/%v, want true/false", wins[0].IsDaily, wins[1].IsDaily)
	}
	if wins[0].OracleID != wins[1].OracleID {
		t.Fatal("test setup should preserve repeated-target wins as separate game rows")
	}

	total, err := countGuessCardWins(context.Background(), database, 7)
	if err != nil {
		t.Fatalf("count Guess the Card wins: %v", err)
	}
	if total != len(wins) {
		t.Fatalf("Guess win count = %d, list length = %d", total, len(wins))
	}

	queries := dbDriver.allQueries()
	for _, required := range []string{
		"FROM user_guess_card_games AS game",
		"game.status = 'won'",
		"FROM user_guess_card_games",
		"status = 'won'",
	} {
		if !strings.Contains(queries, required) {
			t.Fatalf("profile Guess win query missing %q: %s", required, queries)
		}
	}
	if strings.Contains(queries, "user_guess_card_awards") || strings.Contains(queries, "DISTINCT") {
		t.Fatalf("profile Guess history must not be reward-only or collapse repeated games: %s", queries)
	}
}

func TestProfileGuessWinsLabelDailyAndPracticeGames(t *testing.T) {
	wins := buildProfileGuessWinViews([]guessCardWin{
		{OracleID: "11111111-1111-1111-1111-111111111111", CardName: "Daily Card", GuessCount: 2, IsDaily: true},
		{OracleID: "22222222-2222-2222-2222-222222222222", CardName: "Practice Card", GuessCount: 4, IsDaily: false},
	})
	if len(wins) != 2 || wins[0].ModeLabel != "Daily" || wins[1].ModeLabel != "Practice" {
		t.Fatalf("Guess win labels = %#v", wins)
	}

	body := renderTemplate(t, "profile_show", TemplateData{Data: profilePageData{
		Profile:         account.PublicProfile{ID: 7, DisplayName: "Player"},
		ActiveTab:       "achievements",
		AwardTotal:      2,
		GuessWins:       wins,
		GuessPagination: profilePagination{Total: 2},
	}})
	for _, expected := range []string{
		`id="guess-awards-title">Guess the Card Wins</h3>`,
		`Daily and practice wins`,
		`Daily · 2 guesses`,
		`Practice · 4 guesses`,
		`2 game wins across ManaTomb`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("profile Guess win history missing %q: %s", expected, body)
		}
	}
}
