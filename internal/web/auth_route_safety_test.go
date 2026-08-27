package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeRejectsUnknownPathsInsteadOfServingDuplicateHomePage(t *testing.T) {
	app := &App{Renderer: NewRenderer()}
	recorder := httptest.NewRecorder()
	app.HandleHome(recorder, httptest.NewRequest(http.MethodGet, "/definitely-not-a-page", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHomeRejectsWrites(t *testing.T) {
	app := &App{Renderer: NewRenderer()}
	recorder := httptest.NewRecorder()
	app.HandleHome(recorder, httptest.NewRequest(http.MethodPost, "/", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("home write response = %d Allow=%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}

func TestLogoutRequiresPost(t *testing.T) {
	app := &App{}
	recorder := httptest.NewRecorder()
	app.HandleLogout(recorder, httptest.NewRequest(http.MethodGet, "/logout", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("logout GET response = %d Allow=%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}
