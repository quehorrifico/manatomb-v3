package quickbuild

import "testing"

func TestBuildStatsDoesNotCountWipesAsInteraction(t *testing.T) {
	commander := classifyCard(CandidateCard{
		OracleID:             "commander",
		Name:                 "Talrand, Sky Summoner",
		ManaCost:             "{2}{U}{U}",
		TypeLine:             "Legendary Creature - Merfolk Wizard",
		OracleText:           "Whenever you cast an instant or sorcery spell, create a 2/2 blue Drake creature token with flying.",
		ColorIdentity:        []string{"U"},
		CMC:                  4,
		CommanderLegal:       true,
		IsCommanderCandidate: true,
	})

	builder := newBuilder(commander, Profile{Strategy: "Control"}, nil, 123)

	wrath := classifyCard(CandidateCard{
		OracleID:       "wrath",
		Name:           "Wrath of God",
		ManaCost:       "{2}{W}{W}",
		TypeLine:       "Sorcery",
		OracleText:     "Destroy all creatures. They can't be regenerated.",
		ColorIdentity:  []string{"W"},
		CMC:            4,
		CommanderLegal: true,
	})
	counterspell := classifyCard(CandidateCard{
		OracleID:       "counterspell",
		Name:           "Counterspell",
		ManaCost:       "{U}{U}",
		TypeLine:       "Instant",
		OracleText:     "Counter target spell.",
		ColorIdentity:  []string{"U"},
		CMC:            2,
		CommanderLegal: true,
	})

	builder.addCard(wrath, "wipes", 1)
	builder.addCard(counterspell, "interaction", 1)

	stats := builder.buildStats()
	if stats.WipeCount != 1 {
		t.Fatalf("expected one wipe, got %d", stats.WipeCount)
	}
	if stats.InteractionCount != 1 {
		t.Fatalf("expected wipes to stay separate from interaction count, got %d", stats.InteractionCount)
	}
}
