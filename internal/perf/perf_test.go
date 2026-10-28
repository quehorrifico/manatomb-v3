package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	l := logger{enabled: true, slowQuery: time.Hour, slowRequest: time.Hour, write: func(f string, args ...any) { fmt.Fprintf(&buf, f, args...) }}
	mux := http.NewServeMux()
	mux.HandleFunc("/decks/{id}", func(w http.ResponseWriter, r *http.Request) {
		defer Stage(r.Context(), "deck_workspace")()
		defer Stage(WriterContext(w), "render.deck_show")()
		for i := 0; i < 2; i++ {
			q := measureQuery(r.Context(), "SELECT private_literal FROM cards WHERE name = $1")
			q.elapsed = time.Millisecond
			q.finish(nil)
			q.finish(nil) // Query completion must only be recorded once.
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/decks/123?token=private_token", nil)
	req.Header.Set("X-Request-ID", "untrusted_request_id")
	l.requests(mux, mux, nil).ServeHTTP(rec, req)
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimPrefix(buf.Bytes(), []byte("perf ")), &event); err != nil {
		t.Fatal(err)
	}
	if event["route"] != "/decks/{id}" || event["status"] != float64(201) || event["response_bytes"] != float64(2) {
		t.Fatalf("incorrect response fields: %v", event)
	}
	if event["db_queries"] != float64(2) || event["db_ms"] != float64(2) {
		t.Fatalf("incorrect SQL totals: %v", event)
	}
	if event["request_id"] != rec.Header().Get("X-Request-ID") || event["request_id"] == "untrusted_request_id" {
		t.Fatalf("incorrect request ID: %v", event)
	}
	stages := event["stages_ms"].(map[string]any)
	if _, ok := stages["render.deck_show"]; !ok {
		t.Fatal("missing render stage")
	}
	for _, private := range []string{"/decks/123", "private_token", "private_literal", "untrusted_request_id"} {
		if strings.Contains(buf.String(), private) {
			t.Fatalf("logged private value %q", private)
		}
	}
}

func TestQuietRequestsAndDisabledDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		path             string
		status           int
		enabled, wantLog bool
	}{
		{"/assets/test.js", 200, true, false}, {"/healthz", 200, true, false},
		{"/healthz", 503, true, true}, {"/assets/missing.js", 404, true, true},
		{"/decks", 200, true, true}, {"/decks", 500, false, false},
	} {
		t.Run(fmt.Sprintf("%s_%d_%t", tc.path, tc.status, tc.enabled), func(t *testing.T) {
			calls := 0
			l := logger{enabled: tc.enabled, slowRequest: time.Hour, write: func(string, ...any) { calls++ }}
			h := l.requests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status) }), nil, nil)
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", tc.path, nil))
			if (calls > 0) != tc.wantLog {
				t.Fatalf("log calls = %d, want log %t", calls, tc.wantLog)
			}
		})
	}
	Stage(context.Background(), "no_trace")()
}
