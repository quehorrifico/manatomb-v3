package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartCommanderDeckGuestRedirectResetsWorkbench(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodPost, "/decks/new/commander", nil)
	rec := httptest.NewRecorder()

	app.startCommanderDeck(rec, req, "Atraxa, Grand Unifier")

	res := rec.Result()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("startCommanderDeck() status = %d, want %d", res.StatusCode, http.StatusSeeOther)
	}

	want := "/decks/new/workbench?commander_name=Atraxa%2C+Grand+Unifier&format=Commander&reset=1"
	if got := res.Header.Get("Location"); got != want {
		t.Fatalf("startCommanderDeck() redirect = %q, want %q", got, want)
	}
}
