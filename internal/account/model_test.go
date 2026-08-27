package account

import "testing"

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()
	if got := normalizeEmail("  Player.Name+Decks@Example.COM  "); got != "player.name+decks@example.com" {
		t.Fatalf("normalizeEmail() = %q", got)
	}
}
