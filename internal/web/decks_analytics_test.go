package web

import "testing"

func TestComputeDeckAnalytics_CommanderWarnings(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Sol Ring", TypeLine: "Artifact", Qty: 99},
	}

	analytics := computeDeckAnalytics("Commander", "", rows)
	if len(analytics.ValidationWarnings) == 0 {
		t.Fatal("expected commander validation warnings")
	}
}

func TestComputeDeckAnalytics_ConstructedCopyLimitWarnings(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Lightning Bolt", TypeLine: "Instant", Qty: 5},
	}

	analytics := computeDeckAnalytics("Modern", "", rows)
	if len(analytics.ValidationWarnings) == 0 {
		t.Fatal("expected Modern copy-limit warning")
	}
}

func TestComputeDeckAnalytics_LimitedSizeWarnings(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Grizzly Bears", TypeLine: "Creature", Qty: 37},
	}

	analytics := computeDeckAnalytics("Draft", "", rows)
	if len(analytics.ValidationWarnings) == 0 {
		t.Fatal("expected Draft size warning")
	}
}

func TestComputeDeckAnalytics_GuideChecksPresent(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Forest", TypeLine: "Basic Land — Forest", Qty: 36},
		{Name: "Rampant Growth", TypeLine: "Sorcery", OracleText: "Search your library for a basic land card, put that card onto the battlefield tapped, then shuffle.", Qty: 8, CMC: 2},
		{Name: "Harmonize", TypeLine: "Sorcery", OracleText: "Draw three cards.", Qty: 8, CMC: 4},
		{Name: "Beast Within", TypeLine: "Instant", OracleText: "Destroy target permanent.", Qty: 8, CMC: 3},
		{Name: "Elvish Mystic", TypeLine: "Creature", Qty: 39, CMC: 1},
	}

	analytics := computeDeckAnalytics("Commander", "Atraxa, Praetors' Voice", rows)
	if len(analytics.GuideChecks) == 0 {
		t.Fatal("expected guide checks")
	}
	if analytics.GuideChecks[0].Label != "Deck Size" {
		t.Fatalf("expected first guide check to be deck size, got %q", analytics.GuideChecks[0].Label)
	}
}

func TestComputeDeckAnalytics_DeckSizeGuideAlert(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Island", TypeLine: "Basic Land — Island", Qty: 24},
	}

	analytics := computeDeckAnalytics("Modern", "", rows)
	if len(analytics.GuideChecks) == 0 {
		t.Fatal("expected guide checks")
	}
	if analytics.GuideChecks[0].Tone != "alert" {
		t.Fatalf("expected alert tone for undersized deck, got %q", analytics.GuideChecks[0].Tone)
	}
}

func TestComputeDeckAnalytics_CommanderPowerGameChangers(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Island", TypeLine: "Basic Land — Island", Qty: 36},
		{Name: "Rhystic Study", TypeLine: "Enchantment", OracleText: "Whenever an opponent casts a spell, you may draw a card unless that player pays {1}.", Qty: 1, CMC: 3},
		{Name: "Demonic Tutor", TypeLine: "Sorcery", OracleText: "Search your library for a card, put that card into your hand, then shuffle.", Qty: 1, CMC: 2},
		{Name: "Mystical Tutor", TypeLine: "Instant", OracleText: "Search your library for an instant or sorcery card, reveal it, then shuffle and put that card on top.", Qty: 1, CMC: 1},
		{Name: "Smothering Tithe", TypeLine: "Enchantment", OracleText: "Whenever an opponent draws a card, that player may pay {2}. If the player doesn't, you create a Treasure token.", Qty: 1, CMC: 4},
		{Name: "Grizzly Bears", TypeLine: "Creature", Qty: 59, CMC: 2},
	}

	analytics := computeDeckAnalytics("Commander", "Atraxa, Praetors' Voice", rows)
	if !analytics.PowerEstimate.Available {
		t.Fatal("expected commander power estimate")
	}
	if analytics.PowerEstimate.GameChangerCount != 4 {
		t.Fatalf("expected 4 game changers, got %d", analytics.PowerEstimate.GameChangerCount)
	}
	if analytics.PowerEstimate.Bracket < 4 {
		t.Fatalf("expected bracket 4+ for more than three game changers, got %d", analytics.PowerEstimate.Bracket)
	}
}

func TestComputeDeckAnalytics_CommanderPowerCompactCombo(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Island", TypeLine: "Basic Land — Island", Qty: 36},
		{Name: "Thassa's Oracle", TypeLine: "Creature", OracleText: "When Thassa's Oracle enters, look at the top X cards of your library. If X is greater than or equal to the number of cards in your library, you win the game.", Qty: 1, CMC: 2},
		{Name: "Demonic Consultation", TypeLine: "Instant", OracleText: "Choose a card name. Exile the top six cards of your library, then reveal cards from the top of your library until you reveal the named card.", Qty: 1, CMC: 1},
		{Name: "Grizzly Bears", TypeLine: "Creature", Qty: 61, CMC: 2},
	}

	analytics := computeDeckAnalytics("Commander", "Atraxa, Praetors' Voice", rows)
	if analytics.PowerEstimate.CompactComboCount != 1 {
		t.Fatalf("expected one compact combo, got %d", analytics.PowerEstimate.CompactComboCount)
	}
	if analytics.PowerEstimate.Bracket < 4 {
		t.Fatalf("expected bracket 4+ for compact combo, got %d", analytics.PowerEstimate.Bracket)
	}
}
