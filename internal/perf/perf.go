// Package perf provides lightweight server-side timing diagnostics without
// recording SQL text, arguments, URLs, cookies, or request bodies.
package perf

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type logger struct {
	enabled                bool
	slowQuery, slowRequest time.Duration
	write                  func(string, ...any)
}

var diagnostics = logger{
	enabled:     os.Getenv("PERF_LOG_ENABLED") != "false",
	slowQuery:   envMilliseconds("PERF_SLOW_QUERY_MS", 100),
	slowRequest: envMilliseconds("PERF_SLOW_REQUEST_MS", 500),
	write:       log.Printf,
}

func envMilliseconds(name string, fallback int) time.Duration {
	n, err := strconv.Atoi(os.Getenv(name))
	if err != nil || n < 1 {
		n = fallback
	}
	return time.Duration(n) * time.Millisecond
}

func (l logger) emit(event string, fields map[string]any) {
	fields["event"] = event
	b, _ := json.Marshal(fields)
	l.write("perf %s", b)
}

type traceKey struct{}
type queryTiming struct {
	ID     string  `json:"query_id"`
	Source string  `json:"source"`
	Count  int     `json:"count"`
	MS     float64 `json:"duration_ms"`
}
type trace struct {
	mu      sync.Mutex
	id      string
	count   int
	dbTime  time.Duration
	queries map[string]*queryTiming
	stages  map[string]float64
}

func milliseconds(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// Stage records elapsed wall time. Nested stages may overlap database time.
func Stage(ctx context.Context, name string) func() {
	t, _ := ctx.Value(traceKey{}).(*trace)
	if t == nil {
		return func() {}
	}
	start := time.Now()
	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.stages[name] += milliseconds(time.Since(start))
	}
}

// WriterContext lets renderers share the request trace without changing their API.
func WriterContext(w http.ResponseWriter) context.Context {
	for {
		if p, ok := w.(interface{ TimingContext() context.Context }); ok {
			return p.TimingContext()
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return context.Background()
		}
		w = u.Unwrap()
	}
}

type responseWriter struct {
	http.ResponseWriter
	ctx           context.Context
	status, bytes int
}

func (w *responseWriter) TimingContext() context.Context { return w.ctx }
func (w *responseWriter) Unwrap() http.ResponseWriter    { return w.ResponseWriter }
func (w *responseWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 && status != 101 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Requests logs every dynamic request, plus slow or failing static/health
// requests. Use the mux's registered pattern rather than potentially private URLs.
func Requests(next http.Handler, mux *http.ServeMux, db *sql.DB) http.Handler {
	return diagnostics.requests(next, mux, db)
}
func (l logger) requests(next http.Handler, mux *http.ServeMux, db *sql.DB) http.Handler {
	if !l.enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		t := &trace{id: uuid.NewString(), queries: make(map[string]*queryTiming), stages: make(map[string]float64)}
		r = r.WithContext(context.WithValue(r.Context(), traceKey{}, t))
		rw := &responseWriter{ResponseWriter: w, ctx: r.Context()}
		rw.Header().Set("X-Request-ID", t.id)
		route := "unmatched"
		if mux != nil {
			if _, pattern := mux.Handler(r); pattern != "" {
				route = pattern
			}
		}
		defer func() {
			duration := time.Since(start)
			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}
			quiet := strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == "/healthz"
			if quiet && duration < l.slowRequest && status < 400 {
				return
			}
			t.mu.Lock()
			defer t.mu.Unlock()
			queries := make([]queryTiming, 0, len(t.queries))
			for _, q := range t.queries {
				queries = append(queries, *q)
			}
			sort.Slice(queries, func(i, j int) bool { return queries[i].MS > queries[j].MS })
			if len(queries) > 5 {
				queries = queries[:5]
			}
			fields := map[string]any{"request_id": t.id, "method": r.Method, "route": route,
				"status": status, "duration_ms": milliseconds(duration), "slow": duration >= l.slowRequest,
				"response_bytes": rw.bytes, "db_queries": t.count, "db_ms": milliseconds(t.dbTime),
				"top_queries": queries, "stages_ms": t.stages}
			if db != nil {
				s := db.Stats()
				fields["db_pool_in_use"] = s.InUse
				fields["db_pool_open"] = s.OpenConnections
				// Cumulative process-wide counters, not this request's wait time.
				fields["db_pool_wait_count_total"] = s.WaitCount
				fields["db_pool_wait_ms_total"] = milliseconds(s.WaitDuration)
			}
			l.emit("request", fields)
		}()
		next.ServeHTTP(rw, r)
	})
}

type queryMeasurement struct {
	ctx        context.Context
	id, source string
	elapsed    time.Duration
	once       sync.Once
}

func measureQuery(ctx context.Context, query string) *queryMeasurement {
	if !diagnostics.enabled {
		return nil
	}
	hash := sha256.Sum256([]byte(query))
	source := "unknown"
	pcs := make([]uintptr, 20)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, "manatomb/app/") && !strings.Contains(frame.Function, "/internal/perf.") {
			file := frame.File
			if i := strings.LastIndex(file, "/internal/"); i >= 0 {
				file = file[i+1:]
			}
			if i := strings.LastIndex(file, "/cmd/"); i >= 0 {
				file = file[i+1:]
			}
			source = fmt.Sprintf("%s:%d", file, frame.Line)
			break
		}
		if !more {
			break
		}
	}
	return &queryMeasurement{ctx: ctx, id: fmt.Sprintf("%x", hash[:6]), source: source}
}

func (q *queryMeasurement) add(start time.Time) {
	if q != nil {
		q.elapsed += time.Since(start)
	}
}
func (q *queryMeasurement) finish(err error) {
	if q == nil {
		return
	}
	q.once.Do(func() {
		id := ""
		if t, ok := q.ctx.Value(traceKey{}).(*trace); ok {
			id = t.id
			t.mu.Lock()
			t.count++
			t.dbTime += q.elapsed
			key := q.source + q.id
			entry := t.queries[key]
			if entry == nil && len(t.queries) < 128 {
				entry = &queryTiming{ID: q.id, Source: q.source}
				t.queries[key] = entry
			}
			if entry != nil {
				entry.Count++
				entry.MS += milliseconds(q.elapsed)
			}
			t.mu.Unlock()
		}
		if q.elapsed >= diagnostics.slowQuery {
			diagnostics.emit("slow_query", map[string]any{"request_id": id, "query_id": q.id,
				"source": q.source, "duration_ms": milliseconds(q.elapsed), "failed": err != nil})
		}
	})
}
