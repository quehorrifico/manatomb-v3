package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtrasDiscoveryPage(t *testing.T) {
	withRendererRoot(t)
	app := &App{Renderer: NewRenderer()}
	w := httptest.NewRecorder()
	app.HandleExtras(w, httptest.NewRequest(http.MethodGet, "/extras", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("extras returned %d: %s", w.Code, w.Body.String())
	}
	for _, link := range []string{`href="/cpu"`, `href="/games/guess-card"`, `href="/games/spellify"`, `href="/games/pack-opening"`} {
		if !strings.Contains(w.Body.String(), link) {
			t.Errorf("missing activity link %s", link)
		}
	}
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{http.MethodHead, "/extras", http.StatusOK},
		{http.MethodPost, "/extras", http.StatusMethodNotAllowed},
		{http.MethodGet, "/extras/unknown", http.StatusNotFound},
	} {
		w = httptest.NewRecorder()
		app.HandleExtras(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.code {
			t.Errorf("%s %s: got %d, want %d", tc.method, tc.path, w.Code, tc.code)
		}
	}
}
