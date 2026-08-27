package decks

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
)

var cardBoardUndoDriverSequence atomic.Uint64

type cardBoardUndoDriver struct {
	mu        sync.Mutex
	states    map[string]CardBoardState
	execCount int
	committed bool
}

func (d *cardBoardUndoDriver) Open(string) (driver.Conn, error) {
	return &cardBoardUndoConn{driver: d}, nil
}

type cardBoardUndoConn struct {
	driver *cardBoardUndoDriver
}

func (*cardBoardUndoConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (*cardBoardUndoConn) Close() error {
	return nil
}

func (c *cardBoardUndoConn) Begin() (driver.Tx, error) {
	return &cardBoardUndoTx{driver: c.driver}, nil
}

func (c *cardBoardUndoConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}

func (c *cardBoardUndoConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, "FROM decks") {
		return &cardBoardUndoRows{
			columns: []string{"id"},
			values:  [][]driver.Value{{int64(42)}},
		}, nil
	}
	if strings.Contains(query, "FROM deck_cards") && strings.Contains(query, "FOR UPDATE") {
		board := args[2].Value.(string)
		c.driver.mu.Lock()
		state, ok := c.driver.states[board]
		c.driver.mu.Unlock()
		if !ok || state.Quantity <= 0 {
			return &cardBoardUndoRows{columns: []string{"qty", "preferred_print_id"}}, nil
		}
		var printValue driver.Value
		if state.PreferredPrintID != "" {
			printValue = state.PreferredPrintID
		}
		return &cardBoardUndoRows{
			columns: []string{"qty", "preferred_print_id"},
			values:  [][]driver.Value{{int64(state.Quantity), printValue}},
		}, nil
	}
	if strings.Contains(query, "SELECT EXISTS") {
		return &cardBoardUndoRows{
			columns: []string{"exists"},
			values:  [][]driver.Value{{true}},
		}, nil
	}
	return nil, errors.New("unexpected query: " + query)
}

func (c *cardBoardUndoConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.driver.mu.Lock()
	defer c.driver.mu.Unlock()
	c.driver.execCount++

	if strings.Contains(query, "DELETE FROM deck_cards") {
		board := args[2].Value.(string)
		delete(c.driver.states, board)
		return driver.RowsAffected(1), nil
	}
	if strings.Contains(query, "INSERT INTO deck_cards") {
		quantity := int(args[2].Value.(int64))
		board := args[3].Value.(string)
		printID, _ := args[4].Value.(string)
		c.driver.states[board] = CardBoardState{
			Board:            board,
			Quantity:         quantity,
			PreferredPrintID: printID,
		}
		return driver.RowsAffected(1), nil
	}
	if strings.Contains(query, "UPDATE decks") {
		return driver.RowsAffected(1), nil
	}
	return nil, errors.New("unexpected exec: " + query)
}

type cardBoardUndoTx struct {
	driver *cardBoardUndoDriver
}

func (tx *cardBoardUndoTx) Commit() error {
	tx.driver.mu.Lock()
	defer tx.driver.mu.Unlock()
	tx.driver.committed = true
	return nil
}

func (*cardBoardUndoTx) Rollback() error {
	return nil
}

type cardBoardUndoRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *cardBoardUndoRows) Columns() []string {
	return r.columns
}

func (*cardBoardUndoRows) Close() error {
	return nil
}

func (r *cardBoardUndoRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func openCardBoardUndoTestDB(t *testing.T, states map[string]CardBoardState) (*sql.DB, *cardBoardUndoDriver) {
	t.Helper()

	dbDriver := &cardBoardUndoDriver{states: states}
	driverName := "card-board-undo-" + strconv.FormatUint(cardBoardUndoDriverSequence.Add(1), 10)
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

func TestRestoreCardBoardStatesRestoresQuantitiesAndPrintings(t *testing.T) {
	const mainPrint = "11111111-1111-1111-1111-111111111111"
	const sidePrint = "22222222-2222-2222-2222-222222222222"

	db, dbDriver := openCardBoardUndoTestDB(t, map[string]CardBoardState{
		"side": {Board: "side", Quantity: 3, PreferredPrintID: sidePrint},
	})
	err := RestoreCardBoardStatesIfCurrent(
		context.Background(),
		db,
		42,
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		[]CardBoardState{
			{Board: "main", Quantity: 0},
			{Board: "side", Quantity: 3, PreferredPrintID: sidePrint},
		},
		[]CardBoardState{
			{Board: "main", Quantity: 1, PreferredPrintID: mainPrint},
			{Board: "side", Quantity: 2, PreferredPrintID: sidePrint},
		},
	)
	if err != nil {
		t.Fatalf("RestoreCardBoardStatesIfCurrent: %v", err)
	}

	dbDriver.mu.Lock()
	defer dbDriver.mu.Unlock()
	if !dbDriver.committed {
		t.Fatal("restore transaction did not commit")
	}
	if got := dbDriver.states["main"]; got.Quantity != 1 || got.PreferredPrintID != mainPrint {
		t.Fatalf("restored main state = %#v, want quantity 1 and exact printing", got)
	}
	if got := dbDriver.states["side"]; got.Quantity != 2 || got.PreferredPrintID != sidePrint {
		t.Fatalf("restored side state = %#v, want quantity 2 and original printing", got)
	}
}

func TestRestoreCardBoardStatesRejectsStaleState(t *testing.T) {
	db, dbDriver := openCardBoardUndoTestDB(t, map[string]CardBoardState{
		"side": {Board: "side", Quantity: 4},
	})
	err := RestoreCardBoardStatesIfCurrent(
		context.Background(),
		db,
		42,
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		[]CardBoardState{{Board: "side", Quantity: 3}},
		[]CardBoardState{{Board: "side", Quantity: 2}},
	)
	if !errors.Is(err, ErrCardBoardStateConflict) {
		t.Fatalf("RestoreCardBoardStatesIfCurrent error = %v, want ErrCardBoardStateConflict", err)
	}

	dbDriver.mu.Lock()
	defer dbDriver.mu.Unlock()
	if dbDriver.execCount != 0 {
		t.Fatalf("stale restore executed %d writes, want none", dbDriver.execCount)
	}
	if dbDriver.committed {
		t.Fatal("stale restore committed")
	}
}
