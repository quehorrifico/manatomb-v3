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
