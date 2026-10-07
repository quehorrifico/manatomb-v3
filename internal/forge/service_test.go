package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testSecret = "local-transport-test-secret-32-characters"

func testService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	launch := filepath.Join(dir, "launch")
	if err := os.WriteFile(launch, []byte("#!/bin/sh\nexec sleep 120\n"), 0700); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(ServiceConfig{Secret: testSecret, Launcher: launch, Runtime: dir, Cap: 1, Build: "test-runtime-manifest-hash"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return s
}
func request(s http.Handler, owner, method, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Authorization", "Bearer "+testSecret)
	r.Header.Set("X-ManaTomb-Owner", owner)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func create(s *Service, owner string) *httptest.ResponseRecorder {
	return request(s, owner, "POST", "/v1/sessions", Create{Request: "creation-" + owner, Human: validDeck(), CPU: validDeck()})
}
func TestAtomicGlobalAdmissionOwnershipCleanup(t *testing.T) {
	s := testService(t)
	var wg sync.WaitGroup
	success := make(chan string, 2)
	for _, owner := range []string{"1", "2"} {
		wg.Add(1)
		go func(o string) {
			defer wg.Done()
			w := create(s, o)
			if w.Code == 200 {
				success <- o
			} else if w.Code != 409 {
				t.Errorf("unexpected %d %s", w.Code, w.Body)
			}
		}(owner)
	}
	wg.Wait()
	close(success)
	owners := []string{}
	for o := range success {
		owners = append(owners, o)
	}
	if len(owners) != 1 {
		t.Fatalf("admitted %d games", len(owners))
	}
	owner := owners[0]
	w := request(s, owner, "GET", "/v1/sessions", nil)
	var v View
	json.Unmarshal(w.Body.Bytes(), &v)
	if v.Protocol != Protocol || v.Forge != Pin || v.Build != "test-runtime-manifest-hash" {
		t.Fatalf("missing service identity: %+v", v)
	}
	other := "2"
	if owner == "2" {
		other = "1"
	}
	if request(s, other, "GET", "/v1/sessions/"+v.ID, nil).Code != 404 {
		t.Fatal("owner isolation failed")
	}
	if request(s, owner, "DELETE", "/v1/sessions/"+v.ID, nil).Code != 200 {
		t.Fatal("cancel failed")
	}
	s.mu.Lock()
	old := s.sessions[v.ID]
	s.mu.Unlock()
	if !isDone(old.done) {
		t.Fatal("slot released before worker reaped")
	}
	if _, err := os.Stat(old.dir); !os.IsNotExist(err) {
		t.Fatal("worker files not removed")
	}
	if create(s, other).Code != 200 {
		t.Fatal("replacement not admitted")
	}
	if request(s, owner, "POST", "/v1/sessions/"+v.ID+"/actions", Action{Request: "old-action", Session: "old", Revision: 1, Prompt: "old", Action: "ok"}).Code != 410 {
		t.Fatal("retired session accepted action")
	}
}
func TestReceiptsCompetingAndChangedPayload(t *testing.T) {
	s := testService(t)
	var calls atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer worker" {
			t.Error("missing worker authentication")
		}
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		io.WriteString(w, `{"accepted":true}`)
	}))
	defer worker.Close()
	v := &session{id: "id", owner: "1", token: "worker", url: worker.URL, status: "input", done: make(chan struct{}), receipts: map[string]receipt{}}
	s.sessions[v.id] = v
	a := Action{Request: "same-request", Session: "engine", Revision: 1, Prompt: "prompt", Action: "ok"}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request(s, "1", "POST", "/v1/sessions/id/actions", a)
			if w.Code != 200 {
				t.Errorf("%d", w.Code)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate reached engine", calls.Load())
	}
	a.Action = "cancel"
	if request(s, "1", "POST", "/v1/sessions/id/actions", a).Code != 409 {
		t.Fatal("changed payload accepted")
	}
	close(v.done)
}

// Use the real monitor and HTTP transport without starting a child process.
// These tests finish before any watchdog deadline and explicitly stop monitoring.
func monitoredTestSession(t *testing.T, worker *httptest.Server) *Service {
	t.Helper()
	s, err := NewService(ServiceConfig{Secret: testSecret, Launcher: "unused", Runtime: t.TempDir(), Cap: 1})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	v := &session{id: "id", owner: "1", token: "worker", url: worker.URL, status: "input", created: now, lastSeen: now, lastAction: now, done: make(chan struct{}), poll: make(chan struct{}, 1), receipts: map[string]receipt{}}
	s.sessions[v.id] = v
	stopped := make(chan struct{})
	go func() { s.monitor(v); close(stopped) }()
	t.Cleanup(func() { close(v.done); <-stopped; s.client.CloseIdleConnections() })
	return s
}

func awaitWorkerRevision(t *testing.T, s *Service, revision int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		w := request(s, "1", "GET", "/v1/sessions/id", nil)
		var view View
		var state struct {
			Revision int `json:"revision"`
		}
		if w.Code == 200 && json.Unmarshal(w.Body.Bytes(), &view) == nil && json.Unmarshal(view.State, &state) == nil && state.Revision == revision {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("worker revision %d not published within %s", revision, within)
}

func TestAcceptedActionPromptPropagatesBeforeIdlePoll(t *testing.T) {
	var acted atomic.Bool
	var activeReads atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/action" {
			acted.Store(true)
			io.WriteString(w, `{"accepted":true}`)
			return
		}
		if !acted.Load() {
			io.WriteString(w, `{"status":"input","revision":1}`)
		} else if activeReads.Add(1) == 1 {
			io.WriteString(w, `{"status":"working","revision":2}`)
		} else {
			io.WriteString(w, `{"status":"input","revision":3}`)
		}
	}))
	t.Cleanup(worker.Close)
	s := monitoredTestSession(t, worker)
	awaitWorkerRevision(t, s, 1, time.Second)
	w := request(s, "1", "POST", "/v1/sessions/id/actions", Action{Request: "advance-action", Session: "engine", Revision: 1, Prompt: "prompt", Action: "ok"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"accepted":true`) {
		t.Fatalf("action acknowledgment: %d %s", w.Code, w.Body)
	}
	// Both the immediate wake and the subsequent working-state sample must fit
	// well within the previous one-second monitor interval.
	awaitWorkerRevision(t, s, 3, 700*time.Millisecond)
}

func TestPhasePreferenceReachesWorkerAndRefreshesWithoutConsumingChoice(t *testing.T) {
	var changed atomic.Bool
	var calls atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/action" {
			calls.Add(1)
			var action Action
			if err := json.NewDecoder(r.Body).Decode(&action); err != nil || action.Action != "setPhaseStop" || action.Enabled == nil || *action.Enabled || action.Seat != "CPU" || action.Phase != "DRAW" || action.Revision != 0 || action.Prompt != "" {
				t.Errorf("phase preference changed in transport: %+v, %v", action, err)
			}
			changed.Store(true)
			io.WriteString(w, `{"accepted":true}`)
			return
		}
		cpuStops := []string{"DRAW"}
		if changed.Load() {
			cpuStops = []string{}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status": "choice", "session": "engine-incarnation", "revision": 4,
			"prompt":     map[string]any{"id": "forced-choice", "kind": "chooseEntitiesForEffect"},
			"phaseStops": map[string]any{"Human": []string{"MAIN1"}, "CPU": cpuStops},
		})
	}))
	t.Cleanup(worker.Close)
	s := monitoredTestSession(t, worker)
	awaitWorkerRevision(t, s, 4, time.Second)
	enabled := false
	action := Action{Request: "phase-stop-change", Session: "engine-incarnation", Action: "setPhaseStop", Seat: "CPU", Phase: "DRAW", Enabled: &enabled}
	if w := request(s, "2", "POST", "/v1/sessions/id/actions", action); w.Code != http.StatusNotFound {
		t.Fatalf("other owner changed phase preferences: %d", w.Code)
	}
	for i := 0; i < 2; i++ {
		if w := request(s, "1", "POST", "/v1/sessions/id/actions", action); w.Code != http.StatusOK {
			t.Fatalf("phase preference rejected: %d %s", w.Code, w.Body)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("preference receipt reached worker %d times", calls.Load())
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		w := request(s, "1", "GET", "/v1/sessions/id", nil)
		var view View
		var state struct {
			Revision   int
			Prompt     struct{ ID string }
			PhaseStops map[string][]string
		}
		if json.Unmarshal(w.Body.Bytes(), &view) == nil && json.Unmarshal(view.State, &state) == nil && changed.Load() && len(state.PhaseStops["CPU"]) == 0 {
			if view.Status != "choice" || state.Revision != 4 || state.Prompt.ID != "forced-choice" || len(state.PhaseStops["Human"]) != 1 {
				t.Fatalf("preference consumed or changed the decision: %s", view.State)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("phase preference did not propagate with the unchanged choice revision")
}

func TestWaitingHumanPromptRetainsIdlePollRate(t *testing.T) {
	var reads atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		io.WriteString(w, `{"status":"input","revision":1}`)
	}))
	t.Cleanup(worker.Close)
	s := monitoredTestSession(t, worker)
	awaitWorkerRevision(t, s, 1, time.Second)
	time.Sleep(650 * time.Millisecond)
	if got := reads.Load(); got != 1 {
		t.Fatalf("unchanged human prompt fetched %d times before idle heartbeat", got)
	}
	deadline := time.Now().Add(700 * time.Millisecond)
	for reads.Load() == 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := reads.Load(); got != 2 {
		t.Fatalf("idle heartbeat did not continue at one-second interval: %d reads", got)
	}
}

func TestActionDuringWorkerPollQueuesSerializedRefresh(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var acted atomic.Bool
	var first sync.Once
	var activeReads atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/action" {
			acted.Store(true)
			io.WriteString(w, `{"accepted":true}`)
			return
		}
		if activeReads.Add(1) != 1 {
			t.Error("overlapping worker state requests can publish snapshots out of order")
		}
		defer activeReads.Add(-1)
		updated := acted.Load()
		first.Do(func() { close(entered); <-release })
		if updated {
			io.WriteString(w, `{"status":"input","revision":2}`)
		} else {
			io.WriteString(w, `{"status":"input","revision":1}`)
		}
	}))
	t.Cleanup(worker.Close)
	s := monitoredTestSession(t, worker)
	// Unblock the handler even if a test assertion fails before the normal release.
	var unblock sync.Once
	t.Cleanup(func() { unblock.Do(func() { close(release) }) })
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("initial worker poll did not start")
	}
	w := request(s, "1", "POST", "/v1/sessions/id/actions", Action{Request: "advance-action", Session: "engine", Revision: 1, Prompt: "prompt", Action: "ok"})
	if w.Code != 200 {
		t.Fatalf("action blocked by in-flight snapshot: %d %s", w.Code, w.Body)
	}
	unblock.Do(func() { close(release) })
	awaitWorkerRevision(t, s, 2, 700*time.Millisecond)
}

func TestDisabledClientAndUntrustedRequests(t *testing.T) {
	c, err := NewClient(false, "", "")
	if c != nil || err != nil {
		t.Fatal("disabled config")
	}
	for _, u := range []string{"file:///tmp/engine", "https://example.test/path", "http://name:pass@localhost", "http://localhost?x=1"} {
		if _, err := NewClient(true, u, testSecret); err == nil {
			t.Fatal("accepted", u)
		}
	}
	s := testService(t)
	r := httptest.NewRequest("GET", "/v1/sessions", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "/v1/sessions", strings.NewReader(`{"unknown":true}`))
	r.Header.Set("Authorization", "Bearer "+testSecret)
	r.Header.Set("X-ManaTomb-Owner", strconv.Itoa(1))
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func TestEmptyReplySelectionSurvivesTransport(t *testing.T) {
	a := Action{Request: "reveal-ack", Session: "engine", Revision: 1, Prompt: "reveal", Action: "reply", Selected: []int{}}
	raw, _ := json.Marshal(a)
	if !bytes.Contains(raw, []byte(`"selected":[]`)) {
		t.Fatalf("empty reveal acknowledgment lost: %s", raw)
	}
}

func TestDefaultInspectionUsesFixedAllowlistWithoutAdmission(t *testing.T) {
	s := testService(t)
	s.cfg.Defaults = t.TempDir()
	data, _ := json.Marshal(validDeck())
	if err := os.WriteFile(filepath.Join(s.cfg.Defaults, "feline-ferocity.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if w := request(s, "1", "GET", "/v1/defaults/feline-ferocity", nil); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("commanders")) {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(s.sessions) != 0 {
		t.Fatal("Inspecting a list reserved capacity")
	}
	for _, path := range []string{"/v1/defaults/../../secret", "/v1/defaults/unknown"} {
		if request(s, "1", "GET", path, nil).Code != 404 {
			t.Fatal("Unlisted file accessible")
		}
	}
	if request(s, "1", "GET", "/v1/defaults/open-hostility", nil).Code != 503 {
		t.Fatal("Missing list not reported")
	}
}

func TestNoPromptWatchdogDistinguishesHumanWaitAndEngineStall(t *testing.T) {
	for _, mode := range []string{"working", "unreachable", "human-input"} {
		t.Run(mode, func(t *testing.T) {
			s := testService(t)
			s.cfg.NoPrompt = 1500 * time.Millisecond
			worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "unreachable" {
					w.WriteHeader(503)
					return
				}
				if mode == "working" {
					io.WriteString(w, `{"status":"working"}`)
					return
				}
				io.WriteString(w, `{"status":"input","prompt":{"id":"await-human"}}`)
			}))
			defer worker.Close()
			var view View
			if err := json.Unmarshal(create(s, "1").Body.Bytes(), &view); err != nil {
				t.Fatal(err)
			}
			s.mu.Lock()
			v := s.sessions[view.ID]
			s.mu.Unlock()
			v.mu.Lock()
			v.url = worker.URL
			v.status = "input"
			v.mu.Unlock()
			if mode == "human-input" {
				time.Sleep(2200 * time.Millisecond)
				v.mu.Lock()
				status := v.status
				v.mu.Unlock()
				if status != "input" || isDone(v.done) {
					t.Fatal("human thinking time counted as engine stall", status)
				}
				return
			}
			select {
			case <-v.done:
			case <-time.After(5 * time.Second):
				t.Fatal("watchdog failed to reap worker")
			}
			v.mu.Lock()
			status, message := v.status, v.message
			v.mu.Unlock()
			if status != "failed" || !strings.Contains(message, "time without a human decision") {
				t.Fatal(status, message)
			}
			if _, err := os.Stat(v.dir); !os.IsNotExist(err) {
				t.Fatal("stalled worker files retained")
			}
		})
	}
}

func TestDrainStopsAdmissionButPreservesExistingSession(t *testing.T) {
	s := testService(t)
	var view View
	json.Unmarshal(create(s, "1").Body.Bytes(), &view)
	if request(s, "", "POST", "/v1/admin/drain", map[string]bool{"drain": true}).Code != 200 {
		t.Fatal("drain failed")
	}
	if request(s, "2", "POST", "/v1/sessions", Create{Request: "during-drain", Human: validDeck(), CPU: validDeck()}).Code != 503 {
		t.Fatal("admitted during drain")
	}
	if request(s, "1", "GET", "/v1/sessions/"+view.ID, nil).Code != 200 {
		t.Fatal("drain lost existing session")
	}
	if request(s, "", "GET", "/readyz", nil).Code != 503 || request(s, "", "GET", "/healthz", nil).Code != 200 {
		t.Fatal("health/readiness not split")
	}
	if request(s, "1", "DELETE", "/v1/sessions/"+view.ID, nil).Code != 200 {
		t.Fatal("drain prevented cancellation")
	}
	var status map[string]any
	json.Unmarshal(request(s, "", "GET", "/v1/admin/status", nil).Body.Bytes(), &status)
	if status["activeWorkers"] != float64(0) || status["draining"] != true {
		t.Fatal(status)
	}
	if request(s, "", "POST", "/v1/admin/drain", map[string]bool{"drain": false}).Code != 200 || create(s, "2").Code != 200 {
		t.Fatal("resume failed")
	}
	for _, path := range []string{"/v1/admin/status", "/v1/admin/drain"} {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatal("unauthenticated operator access", path, w.Code)
		}
	}
}

func TestSessionLimitsReapAndFreeCapacity(t *testing.T) {
	for _, mode := range []string{"idle", "disconnected", "maximum"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := testService(t)
			var clock atomic.Int64
			clock.Store(time.Now().UnixNano())
			s.now = func() time.Time { return time.Unix(0, clock.Load()) }
			s.cfg.Idle = 24 * time.Hour
			s.cfg.Disconnect = 24 * time.Hour
			s.cfg.Maximum = 24 * time.Hour
			s.cfg.Startup = 24 * time.Hour
			s.cfg.NoPrompt = 24 * time.Hour
			switch mode {
			case "idle":
				s.cfg.Idle = time.Minute
			case "disconnected":
				s.cfg.Disconnect = time.Minute
			case "maximum":
				s.cfg.Maximum = time.Minute
			}
			var view View
			json.Unmarshal(create(s, "1").Body.Bytes(), &view)
			s.mu.Lock()
			v := s.sessions[view.ID]
			s.mu.Unlock()
			clock.Add(int64(2 * time.Minute))
			select {
			case <-v.done:
			case <-time.After(4 * time.Second):
				t.Fatal("session deadline did not reap worker")
			}
			v.mu.Lock()
			status := v.status
			v.mu.Unlock()
			if status != "expired" {
				t.Fatal("wrong terminal status", status)
			}
			if _, err := os.Stat(v.dir); !os.IsNotExist(err) {
				t.Fatal("expired session files remain")
			}
			if create(s, "2").Code != 200 {
				t.Fatal("expired capacity not returned")
			}
		})
	}
}
