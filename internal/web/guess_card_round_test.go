package web

import "testing"

func TestParseGuessCardGameID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want int64
		ok   bool
	}{
		{raw: "42", want: 42, ok: true},
		{raw: " 7 ", want: 7, ok: true},
		{raw: "", ok: false},
		{raw: "0", ok: false},
		{raw: "-2", ok: false},
		{raw: "not-a-round", ok: false},
	}

	for _, tt := range tests {
		got, ok := parseGuessCardGameID(tt.raw)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("parseGuessCardGameID(%q) = (%d, %t), want (%d, %t)", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}

func TestGuessCardRoundPath(t *testing.T) {
	t.Parallel()

	if got := guessCardRoundPath(42); got != "/games/guess-card?game_id=42" {
		t.Fatalf("guessCardRoundPath() = %q", got)
	}
}

func TestGuessCardRoundRefreshPathKeepsValidSubmittedRound(t *testing.T) {
	t.Parallel()

	if got := guessCardRoundRefreshPath("42"); got != "/games/guess-card?game_id=42" {
		t.Fatalf("valid refresh path = %q", got)
	}
	if got := guessCardRoundRefreshPath("not-a-game"); got != "/games/guess-card" {
		t.Fatalf("invalid refresh path = %q", got)
	}
}
