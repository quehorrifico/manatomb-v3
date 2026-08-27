package web

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type healthDriver struct {
	err error
}

func (d healthDriver) Open(string) (driver.Conn, error) {
	return healthConnection{err: d.err}, nil
}

type healthConnection struct {
	err error
}

func (healthConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("health test does not prepare statements")
}
func (healthConnection) Close() error { return nil }
func (healthConnection) Begin() (driver.Tx, error) {
	return nil, errors.New("health test does not begin transactions")
}
func (c healthConnection) Ping(context.Context) error {
	return c.err
}

func TestHealthzChecksDatabaseReadiness(t *testing.T) {
	for _, test := range []struct {
		name       string
		driverName string
		pingErr    error
		wantStatus int
	}{
		{name: "ready", driverName: "manatomb-health-ready", wantStatus: http.StatusOK},
		{name: "database unavailable", driverName: "manatomb-health-unavailable", pingErr: errors.New("offline"), wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			sql.Register(test.driverName, healthDriver{err: test.pingErr})
			database, err := sql.Open(test.driverName, "")
			if err != nil {
				t.Fatalf("open health database: %v", err)
			}
			t.Cleanup(func() { _ = database.Close() })
			app := &App{DB: database}
			recorder := httptest.NewRecorder()
			app.HandleHealthz(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("health status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("health cache control = %q", got)
			}
		})
	}
}

func TestHealthzRejectsWrites(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&App{}).HandleHealthz(recorder, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("health write response = %d Allow=%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}
