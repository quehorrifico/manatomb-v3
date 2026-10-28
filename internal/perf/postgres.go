package perf

import (
	"context"
	"database/sql/driver"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/lib/pq"
)

// PostgresConnector instruments lib/pq without replacing its cancellation,
// transaction, COPY, connection validation, or session reset behavior.
func PostgresConnector(dsn string) (driver.Connector, error) {
	c, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	if !diagnostics.enabled {
		return c, nil
	}
	return &connector{Connector: c}, nil
}

type connector struct{ driver.Connector }

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &connection{Conn: conn}, nil
}

type connection struct{ driver.Conn }

func (c *connection) Ping(ctx context.Context) error { return c.Conn.(driver.Pinger).Ping(ctx) }
func (c *connection) ResetSession(ctx context.Context) error {
	return c.Conn.(driver.SessionResetter).ResetSession(ctx)
}
func (c *connection) IsValid() bool { return c.Conn.(driver.Validator).IsValid() }
func (c *connection) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	q := measureQuery(ctx, "BEGIN")
	start := time.Now()
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	q.add(start)
	q.finish(err)
	if err != nil {
		return nil, err
	}
	return &transaction{Tx: tx, ctx: ctx}, nil
}

type transaction struct {
	driver.Tx
	ctx context.Context
}

func (t *transaction) Commit() error {
	q := measureQuery(t.ctx, "COMMIT")
	start := time.Now()
	err := t.Tx.Commit()
	q.add(start)
	q.finish(err)
	return err
}
func (t *transaction) Rollback() error {
	q := measureQuery(t.ctx, "ROLLBACK")
	start := time.Now()
	err := t.Tx.Rollback()
	q.add(start)
	q.finish(err)
	return err
}
func (c *connection) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}
func (c *connection) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	q := measureQuery(ctx, query)
	start := time.Now()
	s, err := c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
	q.add(start)
	if err != nil {
		q.finish(err)
		return nil, err
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "COPY ") {
		// Aggregate an entire COPY stream, avoiding per-row stack walks and logs.
		return &statement{Stmt: s, query: query, copyMeasurement: q}, nil
	}
	q.finish(nil)
	return &statement{Stmt: s, query: query}, nil
}
func (c *connection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q := measureQuery(ctx, query)
	start := time.Now()
	result, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
	q.add(start)
	if err != driver.ErrSkip {
		q.finish(err)
	}
	return result, err
}
func (c *connection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q := measureQuery(ctx, query)
	start := time.Now()
	r, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
	q.add(start)
	if err != nil {
		if err != driver.ErrSkip {
			q.finish(err)
		}
		return nil, err
	}
	return &rows{Rows: r, measurement: q}, nil
}

type statement struct {
	driver.Stmt
	query           string
	copyMeasurement *queryMeasurement
}

func (s *statement) Close() error {
	start := time.Now()
	err := s.Stmt.Close()
	s.copyMeasurement.add(start)
	s.copyMeasurement.finish(err)
	return err
}

func namedValues(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, v := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}
func plainValues(args []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, len(args))
	for i, v := range args {
		out[i] = v.Value
	}
	return out
}
func (s *statement) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), namedValues(args))
}
func (s *statement) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), namedValues(args))
}
func (s *statement) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	q := s.copyMeasurement
	if q == nil {
		q = measureQuery(ctx, s.query)
	}
	start := time.Now()
	var result driver.Result
	var err error
	if stmt, ok := s.Stmt.(driver.StmtExecContext); ok {
		result, err = stmt.ExecContext(ctx, args)
	} else {
		// pq's COPY statement implements the older interface.
		if err = ctx.Err(); err == nil {
			result, err = s.Stmt.Exec(plainValues(args))
		}
	}
	q.add(start)
	if s.copyMeasurement == nil || err != nil {
		q.finish(err)
	}
	return result, err
}
func (s *statement) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	q := measureQuery(ctx, s.query)
	start := time.Now()
	var r driver.Rows
	var err error
	if stmt, ok := s.Stmt.(driver.StmtQueryContext); ok {
		r, err = stmt.QueryContext(ctx, args)
	} else {
		if err = ctx.Err(); err == nil {
			r, err = s.Stmt.Query(plainValues(args))
		}
	}
	q.add(start)
	if err != nil {
		q.finish(err)
		return nil, err
	}
	return &rows{Rows: r, measurement: q}, nil
}

// Time driver work only; application time between Next calls is excluded.
// Finalize on Close so streaming/fetch time and early closes are included.
type rows struct {
	driver.Rows
	measurement *queryMeasurement
	err         error
}

func (r *rows) Next(dest []driver.Value) error {
	start := time.Now()
	err := r.Rows.Next(dest)
	r.measurement.add(start)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return err
}
func (r *rows) Close() error {
	start := time.Now()
	err := r.Rows.Close()
	r.measurement.add(start)
	if err != nil {
		r.err = err
	}
	r.measurement.finish(r.err)
	return err
}
func (r *rows) HasNextResultSet() bool {
	if v, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return v.HasNextResultSet()
	}
	return false
}
func (r *rows) NextResultSet() error {
	start := time.Now()
	var err error = io.EOF
	if v, ok := r.Rows.(driver.RowsNextResultSet); ok {
		err = v.NextResultSet()
	}
	r.measurement.add(start)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return err
}
func (r *rows) ColumnTypeDatabaseTypeName(i int) string {
	if v, ok := r.Rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
		return v.ColumnTypeDatabaseTypeName(i)
	}
	return ""
}
func (r *rows) ColumnTypeLength(i int) (int64, bool) {
	if v, ok := r.Rows.(driver.RowsColumnTypeLength); ok {
		return v.ColumnTypeLength(i)
	}
	return 0, false
}
func (r *rows) ColumnTypeNullable(i int) (bool, bool) {
	if v, ok := r.Rows.(driver.RowsColumnTypeNullable); ok {
		return v.ColumnTypeNullable(i)
	}
	return false, false
}
func (r *rows) ColumnTypePrecisionScale(i int) (int64, int64, bool) {
	if v, ok := r.Rows.(driver.RowsColumnTypePrecisionScale); ok {
		return v.ColumnTypePrecisionScale(i)
	}
	return 0, 0, false
}
func (r *rows) ColumnTypeScanType(i int) reflect.Type {
	if v, ok := r.Rows.(driver.RowsColumnTypeScanType); ok {
		return v.ColumnTypeScanType(i)
	}
	return reflect.TypeOf((*any)(nil)).Elem()
}
