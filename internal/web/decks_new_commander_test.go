package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"manatomb/app/internal/account"
)

func TestHandleDeckNewCommanderMoreRejectsInvalidCursorState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "invalid seed",
			url:  "/decks/new/commander/more?seed=not-a-seed",
		},
		{
			name: "invalid cursor",
			url:  "/decks/new/commander/more?seed=aa759df7-c324-4a72-a0a5-c86fcfe5d8d0&after=not-a-cursor",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			(&App{}).HandleDeckNewCommanderMore(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("HandleDeckNewCommanderMore() status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
				t.Fatalf("HandleDeckNewCommanderMore() content type = %q, want JSON", got)
			}
		})
	}
}

func TestHandleDeckNewCommanderMoreRejectsUnsupportedMethod(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/decks/new/commander/more", nil)
	rec := httptest.NewRecorder()

	(&App{}).HandleDeckNewCommanderMore(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("HandleDeckNewCommanderMore() status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestStartCommanderDeckGuestRedirectResetsWorkbench(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodPost, "/decks/new/commander", nil)
	rec := httptest.NewRecorder()

	app.startCommanderDeck(
		rec,
		req,
		"Atraxa, Grand Unifier",
		"223e4567-e89b-12d3-a456-426614174000",
	)

	res := rec.Result()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("startCommanderDeck() status = %d, want %d", res.StatusCode, http.StatusSeeOther)
	}

	want := "/decks/new/workbench?commander_name=Atraxa%2C+Grand+Unifier&commander_print_id=223e4567-e89b-12d3-a456-426614174000&format=Commander&reset=1"
	if got := res.Header.Get("Location"); got != want {
		t.Fatalf("startCommanderDeck() redirect = %q, want %q", got, want)
	}
}

func TestStartCommanderDeckSignedInUsesDraftWorkbench(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodPost, "/decks/new/commander", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUser, &account.User{ID: 42}))
	rec := httptest.NewRecorder()

	app.startCommanderDeck(
		rec,
		req,
		"Atraxa, Grand Unifier",
		"223e4567-e89b-12d3-a456-426614174000",
	)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("startCommanderDeck() status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := "/decks/new/workbench?commander_name=Atraxa%2C+Grand+Unifier&commander_print_id=223e4567-e89b-12d3-a456-426614174000&format=Commander&reset=1"
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("startCommanderDeck() redirect = %q, want %q", got, want)
	}
}

func TestHandleDeckSandboxRedirectSignedInUsesDraftWorkbench(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/decks/new/sandbox", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUser, &account.User{ID: 42}))
	rec := httptest.NewRecorder()

	(&App{}).HandleDeckSandboxRedirect(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("HandleDeckSandboxRedirect() status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := "/decks/new/workbench?format=Sandbox&reset=1&sandbox=1"
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("HandleDeckSandboxRedirect() redirect = %q, want %q", got, want)
	}
}

func TestHandleDeckCommanderSelectGuestPreservesPrinting(t *testing.T) {
	t.Parallel()

	form := url.Values{
		"commander_name":     {"Atraxa, Grand Unifier"},
		"commander_print_id": {"223e4567-e89b-12d3-a456-426614174000"},
	}
	req := httptest.NewRequest(http.MethodPost, "/decks/new/commander", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	(&App{}).HandleDeckCommanderSelect(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("HandleDeckCommanderSelect() status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := "/decks/new/workbench?commander_name=Atraxa%2C+Grand+Unifier&commander_print_id=223e4567-e89b-12d3-a456-426614174000&format=Commander&reset=1"
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("HandleDeckCommanderSelect() redirect = %q, want %q", got, want)
	}
}
