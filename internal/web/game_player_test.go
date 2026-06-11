package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"manatomb/app/internal/cards"

	"github.com/google/uuid"
)

func TestGamePlayerCreatesAndReusesGuestCookie(t *testing.T) {
	t.Parallel()

	app := &App{}
	firstRequest := httptest.NewRequest(http.MethodGet, "/games/guess-card", nil)
	firstResponse := httptest.NewRecorder()
	first := app.gamePlayer(firstResponse, firstRequest)
	if !first.IsGuest() {
		t.Fatalf("first player = %#v, want guest", first)
	}
	if _, err := uuid.Parse(first.GuestID); err != nil {
		t.Fatalf("guest id = %q: %v", first.GuestID, err)
	}

	cookies := firstResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != guestGameCookieName || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %#v", cookies)
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/games/spellify", nil)
	secondRequest.AddCookie(cookies[0])
	second := app.gamePlayer(httptest.NewRecorder(), secondRequest)
	if second.GuestID != first.GuestID {
		t.Fatalf("reused guest id = %q, want %q", second.GuestID, first.GuestID)
	}
}

func TestGuestGamesCannotEarnAwards(t *testing.T) {
	t.Parallel()

	card := cards.Card{Name: "Test Card", OracleID: uuid.NewString()}
	guess := buildGuessCardPageData(guessCardGame{GuestID: uuid.NewString(), Status: "active", IsDaily: true}, card)
	if guess.HasAccount || guess.AwardStatus != "Sign in to earn" {
		t.Fatalf("guess guest data = account %t award %q", guess.HasAccount, guess.AwardStatus)
	}

	spellify := buildSpellifyPageData(spellifyGame{GuestID: uuid.NewString(), Status: "active", IsDaily: true}, card)
	if spellify.HasAccount || spellify.AwardStatus != "Sign in to earn" {
		t.Fatalf("spellify guest data = account %t award %q", spellify.HasAccount, spellify.AwardStatus)
	}
}
