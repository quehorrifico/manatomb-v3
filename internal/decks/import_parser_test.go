package decks

import "testing"

func TestParseCommanderDecklistText_CommanderStyleInput(t *testing.T) {
	t.Parallel()

	commander, cards, err := ParseCommanderDecklistText(`
Commander: Atraxa, Grand Unifier
1 Sol Ring
1 Swords to Plowshares
`)
	if err != nil {
		t.Fatalf("ParseCommanderDecklistText returned error: %v", err)
	}
	if commander != "Atraxa, Grand Unifier" {
		t.Fatalf("commander = %q, want %q", commander, "Atraxa, Grand Unifier")
	}
	if len(cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(cards))
	}
}

func TestParseCommanderDecklistText_GenericDecklistAllowsNoCommander(t *testing.T) {
	t.Parallel()

	commander, cards, err := ParseCommanderDecklistText(`
4 Lightning Bolt
4 Counterspell
20 Island
`)
	if err != nil {
		t.Fatalf("ParseCommanderDecklistText returned error: %v", err)
	}
	if commander != "" {
		t.Fatalf("commander = %q, want empty", commander)
	}
	if len(cards) != 3 {
		t.Fatalf("len(cards) = %d, want 3", len(cards))
	}
}
