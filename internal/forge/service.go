package forge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ServiceConfig struct {
	Secret, Launcher, Runtime, Defaults          string
	Build                                        string
	Cap                                          int
	Disconnect, Idle, Maximum, Startup, NoPrompt time.Duration
}
type receipt struct {
	Hash   [32]byte
	Status int
	Body   []byte
}
type session struct {
	actionMu                                             sync.Mutex
	mu                                                   sync.Mutex
	id, owner, request, dir, token, url, status, message string
	created, lastSeen, lastAction                        time.Time
	command                                              *exec.Cmd
	done                                                 chan struct{}
	poll                                                 chan struct{}
	state                                                json.RawMessage
	receipts                                             map[string]receipt
	receiptOrder                                         []string
	createHash                                           [32]byte
	ending                                               bool
}
type Service struct {
	cfg      ServiceConfig
	draining bool
	stopping bool
	mu       sync.Mutex
	sessions map[string]*session
	client   *http.Client
	now      func() time.Time
}

func NewService(c ServiceConfig) (*Service, error) {
	if len(c.Secret) < 32 || c.Launcher == "" || c.Runtime == "" || c.Cap < 1 {
		return nil, errors.New("engine requires a secret of at least 32 characters, launcher, runtime and positive cap")
	}
	if c.Disconnect == 0 {
		c.Disconnect = 5 * time.Minute
	}
	if c.Idle == 0 {
		c.Idle = 15 * time.Minute
	}
	if c.Maximum == 0 {
		c.Maximum = 2 * time.Hour
	}
	if c.NoPrompt == 0 {
		c.NoPrompt = 90 * time.Second
	}
	if c.Startup == 0 {
		c.Startup = 3 * time.Minute
	}
	if err := os.MkdirAll(c.Runtime, 0700); err != nil {
		return nil, err
	}
	return &Service{cfg: c, sessions: map[string]*session{}, client: &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil, MaxIdleConnsPerHost: 2}}, now: time.Now}, nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func serviceError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("expected one object")
	}
	return nil
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" && r.Method == "GET" {
		writeJSON(w, 200, map[string]string{"status": "ok", "protocol": Protocol, "forge": Pin})
		return
	}
	if r.URL.Path == "/readyz" && r.Method == "GET" {
		s.mu.Lock()
		draining := s.draining
		s.mu.Unlock()
		if draining {
			serviceError(w, 503, "admission is draining")
		} else {
			writeJSON(w, 200, map[string]string{"status": "accepting", "build": s.cfg.Build})
		}
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.cfg.Secret)) != 1 {
		serviceError(w, 401, "unauthorized")
		return
	}
	// Private operator endpoints are never proxied to the browser.
	if r.URL.Path == "/v1/admin/drain" && r.Method == "POST" {
		var body struct {
			Drain *bool `json:"drain"`
		}
		if decode(w, r, &body) != nil || body.Drain == nil {
			serviceError(w, 400, "explicit drain boolean required")
			return
		}
		s.mu.Lock()
		if s.stopping && !*body.Drain {
			s.mu.Unlock()
			serviceError(w, 503, "service is stopping")
			return
		}
		s.draining = *body.Drain
		s.mu.Unlock()
		writeJSON(w, 200, map[string]bool{"draining": *body.Drain})
		return
	}
	if r.URL.Path == "/v1/admin/status" && r.Method == "GET" {
		s.mu.Lock()
		active := 0
		for _, v := range s.sessions {
			if !isDone(v.done) {
				active++
			}
		}
		status := map[string]any{"draining": s.draining, "activeWorkers": active, "retainedSessions": len(s.sessions), "capacity": s.cfg.Cap, "protocol": Protocol, "forge": Pin, "build": s.cfg.Build}
		s.mu.Unlock()
		writeJSON(w, 200, status)
		return
	}
	owner := r.Header.Get("X-ManaTomb-Owner")
	if id, err := strconv.ParseInt(owner, 10, 64); err != nil || id <= 0 {
		serviceError(w, 400, "invalid owner")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v1/defaults/") && r.Method == "GET" {
		id := strings.TrimPrefix(r.URL.Path, "/v1/defaults/")
		if !validDefault(id) {
			serviceError(w, 404, "default deck not found")
			return
		}
		data, err := os.ReadFile(filepath.Join(s.cfg.Defaults, id+".json"))
		var deck Deck
		if err != nil || json.Unmarshal(data, &deck) != nil || deck.Validate() != nil {
			serviceError(w, 503, "default deck unavailable")
			return
		}
		writeJSON(w, 200, deck)
		return
	}
	if r.URL.Path == "/v1/sessions" {
		if r.Method == "POST" {
			var c Create
			if err := decode(w, r, &c); err != nil {
				serviceError(w, 400, "invalid creation request")
				return
			}
			s.create(w, owner, c)
			return
		}
		if r.Method == "GET" {
			s.mu.Lock()
			var found *session
			for _, v := range s.sessions {
				if v.owner == owner && (found == nil || v.created.After(found.created)) {
					found = v
				}
			}
			s.mu.Unlock()
			if found == nil {
				serviceError(w, 404, "no session")
			} else {
				s.view(w, found)
			}
			return
		}
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || len(parts) > 4 || parts[0] != "v1" || parts[1] != "sessions" {
		serviceError(w, 404, "not found")
		return
	}
	s.mu.Lock()
	v := s.sessions[parts[2]]
	s.mu.Unlock()
	if v == nil || v.owner != owner {
		serviceError(w, 404, "session not found")
		return
	}
	if len(parts) == 3 && r.Method == "GET" {
		s.view(w, v)
		return
	}
	if len(parts) == 3 && r.Method == "DELETE" {
		s.end(v, "cancelled", "Cancelled by owner")
		s.view(w, v)
		return
	}
	if len(parts) == 4 && parts[3] == "actions" && r.Method == "POST" {
		var a Action
		if err := decode(w, r, &a); err != nil || a.Validate() != nil {
			serviceError(w, 400, "invalid action")
			return
		}
		s.action(w, v, a)
		return
	}
	serviceError(w, 405, "method not allowed")
}
func terminal(status string) bool {
	return status == "finished" || status == "cancelled" || status == "expired" || status == "failed"
}
func (s *Service) create(w http.ResponseWriter, owner string, c Create) {
	if len(c.Request) < 8 || len(c.Request) > 80 {
		serviceError(w, 400, "creation requires a stable request identifier")
		return
	}
	if c.Default != "" {
		if !validDefault(c.Default) {
			serviceError(w, 400, "unknown default deck")
			return
		}
		data, err := os.ReadFile(filepath.Join(s.cfg.Defaults, c.Default+".json"))
		if err != nil || json.Unmarshal(data, &c.CPU) != nil {
			serviceError(w, 503, "default deck unavailable")
			return
		}
	}
	for _, d := range []Deck{c.Human, c.CPU} {
		if err := d.Validate(); err != nil {
			serviceError(w, 422, err.Error())
			return
		}
	}
	encoded, _ := json.Marshal(c)
	hash := sha256.Sum256(encoded)
	s.mu.Lock()
	if s.draining {
		s.mu.Unlock()
		serviceError(w, 503, "CPU play is temporarily closed to new games; existing games can continue.")
		return
	}
	active := 0
	for _, v := range s.sessions {
		v.mu.Lock()
		live := !terminal(v.status) || !isDone(v.done)
		same := v.owner == owner && v.request == c.Request
		v.mu.Unlock()
		if same {
			s.mu.Unlock()
			if v.createHash != hash {
				serviceError(w, 409, "request identifier reused with different decks")
			} else {
				s.view(w, v)
			}
			return
		}
		if live {
			active++
			if v.owner == owner {
				s.mu.Unlock()
				serviceError(w, 409, "you already have an active game; reconnect or cancel it")
				return
			}
		}
	}
	if active >= s.cfg.Cap {
		s.mu.Unlock()
		serviceError(w, 409, "All game slots are occupied. The site currently supports one active game; try again after it ends.")
		return
	}
	dir, err := os.MkdirTemp(s.cfg.Runtime, "game-")
	if err != nil {
		s.mu.Unlock()
		serviceError(w, 503, "cannot reserve game worker")
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.RemoveAll(dir)
		s.mu.Unlock()
		serviceError(w, 503, "cannot reserve worker port")
		return
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	now := s.now()
	v := &session{id: uuid.NewString(), owner: owner, request: c.Request, dir: dir, token: uuid.NewString(), url: fmt.Sprintf("http://127.0.0.1:%d", port), status: "starting", created: now, lastSeen: now, lastAction: now, done: make(chan struct{}), poll: make(chan struct{}, 1), receipts: map[string]receipt{}, createHash: hash}
	for name, d := range map[string]Deck{"human": c.Human, "cpu": c.CPU} {
		b, _ := json.Marshal(d)
		if err = os.WriteFile(filepath.Join(dir, name+".json"), b, 0600); err != nil {
			os.RemoveAll(dir)
			s.mu.Unlock()
			serviceError(w, 503, "cannot initialize worker")
			return
		}
	}
	cmd := exec.Command(s.cfg.Launcher, dir, strconv.Itoa(port))
	cmd.Env = append(os.Environ(), "FORGE_WORKER_TOKEN="+v.token)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard // No card identities or unbounded per-action logs retained.
	v.command = cmd
	if err = cmd.Start(); err != nil {
		os.RemoveAll(dir)
		s.mu.Unlock()
		serviceError(w, 503, "worker launch failed")
		return
	}
	// Retain at most 16 terminal receipts/views; never retain an unbounded game history.
	for len(s.sessions) >= 16 {
		var oldest *session
		for _, candidate := range s.sessions {
			if isDone(candidate.done) && (oldest == nil || candidate.created.Before(oldest.created)) {
				oldest = candidate
			}
		}
		if oldest == nil {
			break
		}
		delete(s.sessions, oldest.id)
	}
	s.sessions[v.id] = v
	s.mu.Unlock()
	go func() {
		err := cmd.Wait()
		v.mu.Lock()
		if !terminal(v.status) {
			v.status = "failed"
			v.message = "Engine process stopped. Start a fresh game."
			if data, readErr := os.ReadFile(filepath.Join(v.dir, "failure.json")); readErr == nil && len(data) < 4096 {
				var failure struct {
					Error string `json:"error"`
				}
				if json.Unmarshal(data, &failure) == nil {
					v.message = failure.Error
				}
			}
		}
		_ = err
		v.mu.Unlock()
		os.RemoveAll(dir)
		close(v.done)
	}()
	go s.monitor(v)
	s.view(w, v)
}
func validDefault(id string) bool {
	switch id {
	case "feline-ferocity", "open-hostility", "arcane-wizardry", "draconic-domination", "vampiric-bloodlust":
		return true
	}
	return false
}
func isDone(done chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
func (s *Service) worker(v *session, method, path string, body []byte) (int, []byte, error) {
	v.mu.Lock()
	origin, token := v.url, v.token
	v.mu.Unlock()
	req, err := http.NewRequest(method, origin+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if len(data) > 2<<20 {
		return 0, nil, errors.New("oversized worker view")
	}
	return res.StatusCode, data, err
}
func (s *Service) monitor(v *session) {
	var workingSince time.Time
	lastWorker := s.now()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-v.done:
			return
		case <-timer.C:
		case <-v.poll:
		}
		// Only this goroutine fetches and publishes worker views. An accepted
		// action wakes it without racing a second cache writer or waiting for
		// human/CPU computation to finish before returning the acknowledgment.
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		v.mu.Lock()
		now := s.now()
		status := v.status
		expired := now.Sub(v.created) > s.cfg.Maximum || now.Sub(v.lastSeen) > s.cfg.Disconnect || now.Sub(v.lastAction) > s.cfg.Idle
		startingExpired := status == "starting" && now.Sub(v.created) > s.cfg.Startup
		stalled := !workingSince.IsZero() && now.Sub(workingSince) > s.cfg.NoPrompt || status != "starting" && now.Sub(lastWorker) > s.cfg.NoPrompt
		v.mu.Unlock()
		if terminal(status) {
			return
		}
		if stalled {
			s.end(v, "failed", "Forge stopped responding or exceeded its time without a human decision. The worker was stopped; start a fresh game.")
			return
		}
		if expired || startingExpired {
			s.end(v, "expired", "Session expired; start a fresh game.")
			return
		}
		code, data, err := s.worker(v, "GET", "/state", nil)
		if err != nil || code != 200 {
			timer.Reset(workerPollInterval(status))
			continue
		}
		var state struct {
			Status      string `json:"status"`
			FailureCode string `json:"failureCode"`
		}
		if json.Unmarshal(data, &state) != nil {
			timer.Reset(workerPollInterval(status))
			continue
		}
		v.mu.Lock()
		if terminal(v.status) {
			v.mu.Unlock()
			return
		}
		lastWorker = s.now()
		if state.Status == "working" {
			if workingSince.IsZero() {
				workingSince = lastWorker
			}
		} else {
			workingSince = time.Time{}
		}
		v.state = append(v.state[:0], data...)
		v.status = state.Status
		v.mu.Unlock()
		if state.Status == "finished" {
			s.end(v, "finished", "")
			return
		}
		if state.Status == "blocked" {
			message := "Forge reached an unsupported or failed input. This game cannot continue."
			if validFailureCode(state.FailureCode) {
				message += " Reference: " + state.FailureCode
			}
			s.end(v, "failed", message)
			return
		}
		timer.Reset(workerPollInterval(state.Status))
	}
}

func workerPollInterval(status string) time.Duration {
	if status == "starting" || status == "working" {
		return 200 * time.Millisecond
	}
	// A waiting human prompt can be large; do not refetch it at the active
	// rate. A successful action explicitly wakes the monitor instead.
	return time.Second
}
func validFailureCode(code string) bool {
	if len(code) == 0 || len(code) > 160 {
		return false
	}
	for _, r := range code {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '$') {
			return false
		}
	}
	return code == "adapter_failure" || strings.HasPrefix(code, "adapter_failure_") || strings.HasPrefix(code, "unsupported_gui_") || strings.HasPrefix(code, "unsupported_choice_")
}
func (s *Service) view(w http.ResponseWriter, v *session) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.lastSeen = s.now()
	deadline := v.created.Add(s.cfg.Maximum)
	if idle := v.lastAction.Add(s.cfg.Idle); idle.Before(deadline) {
		deadline = idle
	}
	writeJSON(w, 200, View{Protocol: Protocol, Forge: Pin, Build: s.cfg.Build, ID: v.id, Status: v.status, State: v.state, Error: v.message, ExpiresAt: deadline.UTC().Format(time.RFC3339), ReconnectSeconds: int(s.cfg.Disconnect.Seconds())})
}
func (s *Service) action(w http.ResponseWriter, v *session, a Action) {
	v.actionMu.Lock()
	defer v.actionMu.Unlock()
	// Do not hold the session mutex during the worker request: a synchronous Forge
	// choice must have a reply path independent of engine dispatch and state polling.
	raw, _ := json.Marshal(a)
	hash := sha256.Sum256(raw)
	v.mu.Lock()
	if saved, ok := v.receipts[a.Request]; ok {
		v.mu.Unlock()
		if saved.Hash != hash {
			serviceError(w, 409, "request identifier reused")
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(saved.Status)
			w.Write(saved.Body)
		}
		return
	}
	if terminal(v.status) {
		v.mu.Unlock()
		serviceError(w, 410, "session has ended")
		return
	}
	v.mu.Unlock()
	code, data, err := s.worker(v, "POST", "/action", raw)
	if err != nil {
		serviceError(w, 503, "Engine reply unavailable. Reconnect before retrying; never assume an action succeeded.")
		return
	}
	v.mu.Lock()
	if code == 200 {
		v.lastAction = s.now()
	}
	v.lastSeen = s.now()
	v.receipts[a.Request] = receipt{hash, code, append([]byte(nil), data...)}
	v.receiptOrder = append(v.receiptOrder, a.Request)
	for len(v.receiptOrder) > 128 {
		delete(v.receipts, v.receiptOrder[0])
		v.receiptOrder = v.receiptOrder[1:]
	}
	v.mu.Unlock()
	if code == http.StatusOK {
		select {
		case v.poll <- struct{}{}:
		default:
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	w.Write(data)
}
func (s *Service) end(v *session, status, message string) {
	v.mu.Lock()
	if v.ending || terminal(v.status) && isDone(v.done) {
		v.mu.Unlock()
		return
	}
	v.ending = true
	v.status = status
	v.message = message
	pid := v.command.Process.Pid
	v.mu.Unlock()
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	select {
	case <-v.done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-v.done
	}
}
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.draining = true
			s.stopping = true
			all := make([]*session, 0, len(s.sessions))
			for _, v := range s.sessions {
				all = append(all, v)
			}
			s.mu.Unlock()
			for _, v := range all {
				if !isDone(v.done) {
					s.end(v, "cancelled", "Engine service restarting")
				}
			}
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, v := range s.sessions {
				if isDone(v.done) && s.now().Sub(v.created) > 3*time.Hour {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}
