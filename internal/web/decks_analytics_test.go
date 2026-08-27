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

func TestComputeDeckAnalytics_StatCardsAndCategories(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Forest", TypeLine: "Basic Land — Forest", Qty: 2},
		{Name: "Rampant Growth", TypeLine: "Sorcery", OracleText: "Search your library for a basic land card, put that card onto the battlefield tapped, then shuffle.", Qty: 1, CMC: 2},
		{Name: "Harmonize", TypeLine: "Sorcery", OracleText: "Draw three cards.", Qty: 1, CMC: 4},
		{Name: "Swords to Plowshares", TypeLine: "Instant", OracleText: "Exile target creature. Its controller gains life equal to its power.", Qty: 1, CMC: 1},
		{Name: "Eerie Interlude", TypeLine: "Instant", OracleText: "Exile any number of target creatures you control. Return those cards to the battlefield under their owner's control.", Qty: 1, CMC: 3},
	}

	analytics := computeDeckAnalytics("Commander", "Atraxa, Praetors' Voice", rows)
	assertStatCard(t, analytics, "ramp", "Rampant Growth")
	assertStatCard(t, analytics, "draw", "Harmonize")
	assertStatCard(t, analytics, "interaction", "Swords to Plowshares")
	assertCategory(t, analytics, "Ramp", 1)
	assertCategory(t, analytics, "Draw", 1)
	assertCategory(t, analytics, "Removal", 1)
	assertCategory(t, analytics, "Blink", 1)
	if got := analytics.CategoryBreakdown[len(analytics.CategoryBreakdown)-1].Label; got != "Lands" {
		t.Fatalf("expected lands category to sort last, got %q", got)
	}
}

func TestComputeDeckAnalytics_ManaPipsAndSources(t *testing.T) {
	t.Parallel()

	rows := []deckAnalyticsCardInput{
		{Name: "Kitchen Finks", ManaCost: "{1}{G/W}{G/W}", TypeLine: "Creature", Qty: 1},
		{Name: "Llanowar Elves", ManaCost: "{G}", TypeLine: "Creature", OracleText: "{T}: Add {G}.", Qty: 2},
		{Name: "Azorius Signet", ManaCost: "{2}", TypeLine: "Artifact", OracleText: "{1}, {T}: Add {W}{U}.", Qty: 2},
		{Name: "Watery Grave", TypeLine: "Land — Island Swamp", Qty: 1},
		{Name: "Wastes", TypeLine: "Basic Land", OracleText: "{T}: Add {C}.", Qty: 1},
		{Name: "Kozilek's Pathfinder", ManaCost: "{6}{C}{C}", TypeLine: "Creature", Qty: 1},
		{Name: "Mana Confluence", TypeLine: "Land", OracleText: "{T}: Add one mana of any color.", Qty: 1},
	}

	analytics := computeDeckAnalytics("Sandbox", "", rows)
	if got := manaColorStatCount(analytics.ManaPips, "W"); got != 2 {
		t.Fatalf("white mana pips = %d, want 2", got)
	}
	if got := manaColorStatCount(analytics.ManaPips, "G"); got != 4 {
		t.Fatalf("green mana pips = %d, want 4", got)
	}
	if got := manaColorStatCount(analytics.ManaPips, "C"); got != 2 {
		t.Fatalf("colorless mana pips = %d, want 2", got)
	}
	if got := manaColorStatCount(analytics.ManaSources, "W"); got != 3 {
		t.Fatalf("white mana sources = %d, want 3", got)
	}
	if got := manaColorStatCount(analytics.ManaSources, "U"); got != 4 {
		t.Fatalf("blue mana sources = %d, want 4", got)
	}
	if got := manaColorStatCount(analytics.ManaSources, "B"); got != 2 {
		t.Fatalf("black mana sources = %d, want 2", got)
	}
	if got := manaColorStatCount(analytics.ManaSources, "C"); got != 1 {
		t.Fatalf("colorless mana sources = %d, want 1", got)
	}
	assertManaColorCard(t, analytics.ManaPips, "G", "Kitchen Finks")
	assertManaColorCard(t, analytics.ManaSources, "U", "Watery Grave")
}

func TestComputeDeckAnalytics_ManaProfileExcludesCommanderSlot(t *testing.T) {
	t.Parallel()

	analytics := computeDeckAnalytics("Commander", "Omnath, Locus of Creation", []deckAnalyticsCardInput{
		{Name: "Omnath, Locus of Creation", ManaCost: "{R}{G}{W}{U}", TypeLine: "Legendary Creature", ColorID: "W,U,R,G", Qty: 1},
		{Name: "Command Tower", TypeLine: "Land", OracleText: "{T}: Add one mana of any color in your commander's color identity.", Qty: 1},
	})

	for _, symbol := range []string{"W", "U", "R", "G"} {
		if got := manaColorStatCount(analytics.ManaPips, symbol); got != 0 {
			t.Fatalf("commander %s pip leaked into mainboard count: %d", symbol, got)
		}
		if got := manaColorStatCount(analytics.ManaSources, symbol); got != 1 {
			t.Fatalf("Command Tower %s source count = %d, want 1", symbol, got)
		}
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
	if len(analytics.PowerEstimate.Signals) != 1 {
		t.Fatalf("expected only the driving power signal, got %d", len(analytics.PowerEstimate.Signals))
	}
	signal := analytics.PowerEstimate.Signals[0]
	if signal.Label != "Game Changers" {
		t.Fatalf("expected Game Changers signal, got %q", signal.Label)
	}
	assertStringPresent(t, signal.Cards, "Rhystic Study")
	assertStringPresent(t, signal.Cards, "Demonic Tutor")
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
	assertStringPresent(t, analytics.PowerEstimate.CompactCombos, "Thassa's Oracle + Demonic Consultation")
	if len(analytics.PowerEstimate.Signals) != 1 {
		t.Fatalf("expected only compact combo signal, got %d", len(analytics.PowerEstimate.Signals))
	}
	if analytics.PowerEstimate.Signals[0].Label != "Compact Wins" {
		t.Fatalf("expected Compact Wins signal, got %q", analytics.PowerEstimate.Signals[0].Label)
	}
	assertStringPresent(t, analytics.PowerEstimate.Signals[0].Cards, "Thassa's Oracle + Demonic Consultation")
}

func assertStatCard(t *testing.T, analytics deckAnalyticsData, key, wantName string) {
	t.Helper()
	for _, card := range analytics.StatCards[key] {
		if card.Name == wantName {
			return
		}
	}
	t.Fatalf("expected %q in stat_cards[%q], got %#v", wantName, key, analytics.StatCards[key])
}

func assertCategory(t *testing.T, analytics deckAnalyticsData, wantLabel string, wantCount int) {
	t.Helper()
	for _, category := range analytics.CategoryBreakdown {
		if category.Label == wantLabel {
			if category.Count != wantCount {
				t.Fatalf("expected %s count %d, got %d", wantLabel, wantCount, category.Count)
			}
			return
		}
	}
	t.Fatalf("expected category %q in %#v", wantLabel, analytics.CategoryBreakdown)
}

func manaColorStatCount(stats []deckManaColorStat, symbol string) int {
	for _, stat := range stats {
		if stat.Symbol == symbol {
			return stat.Count
		}
	}
	return 0
}

func assertManaColorCard(t *testing.T, stats []deckManaColorStat, symbol, cardName string) {
	t.Helper()
	for _, stat := range stats {
		if stat.Symbol != symbol {
			continue
		}
		for _, card := range stat.Cards {
			if card.Name == cardName {
				return
			}
		}
	}
	t.Fatalf("expected %q in mana color %q: %#v", cardName, symbol, stats)
}

func assertStringPresent(t *testing.T, got []string, want string) {
	t.Helper()
	for _, value := range got {
		if value == want {
			return
		}
	}
	t.Fatalf("expected %q in %#v", want, got)
}
