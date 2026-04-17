package quickbuild

import "testing"

func TestClassifyCardMarksArcaneSignetAsRamp(t *testing.T) {
	card := classifyCard(CandidateCard{
		Name:          "Arcane Signet",
		ManaCost:      "{2}",
		TypeLine:      "Artifact",
		OracleText:    "{T}: Add one mana of any color in your commander's color identity.",
		ColorIdentity: []string{},
		CMC:           2,
	})

	if !hasRole(card, "ramp") {
		t.Fatalf("expected Arcane Signet to be classified as ramp, got roles %v", card.Roles)
	}
}

func TestInferProfileFindsTokenTheme(t *testing.T) {
	commander := classifyCard(CandidateCard{
		Name:                 "Adeline, Resplendent Cathar",
		ManaCost:             "{1}{W}{W}",
		TypeLine:             "Legendary Creature - Human Knight",
		OracleText:           "Whenever you attack, for each opponent, create a 1/1 white Human creature token that's tapped and attacking.",
		AllPartsJSON:         "[]",
		ColorIdentity:        []string{"W"},
		CMC:                  3,
		CommanderLegal:       true,
		IsCommanderCandidate: true,
	})

	profile := inferProfile(commander, CommanderOverride{})
	if !hasString(profile.Themes, "Tokens") {
		t.Fatalf("expected token commander profile to include Tokens theme, got %v", profile.Themes)
	}
	if profile.PrimaryTheme != "Tokens" {
		t.Fatalf("expected token commander primary theme to be Tokens, got %q", profile.PrimaryTheme)
	}
}

func TestClassifyCardAvoidsBroadThemeFalsePositives(t *testing.T) {
	tests := []struct {
		name         string
		card         CandidateCard
		blockedTheme string
	}{
		{
			name: "generic artifact is not artifact synergy",
			card: CandidateCard{
				Name:       "Ornithopter",
				TypeLine:   "Artifact Creature - Thopter",
				OracleText: "Flying",
				CMC:        0,
			},
			blockedTheme: "Artifacts",
		},
		{
			name: "generic enchantment is not enchantment synergy",
			card: CandidateCard{
				Name:       "Pacifism",
				TypeLine:   "Enchantment - Aura",
				OracleText: "Enchant creature\nEnchanted creature can't attack or block.",
				CMC:        2,
			},
			blockedTheme: "Enchantments",
		},
		{
			name: "simple instant is not spellslinger payoff",
			card: CandidateCard{
				Name:       "Lightning Bolt",
				TypeLine:   "Instant",
				OracleText: "Lightning Bolt deals 3 damage to any target.",
				CMC:        1,
			},
			blockedTheme: "Spellslinger",
		},
		{
			name: "generic etb creature is not blink synergy",
			card: CandidateCard{
				Name:       "Elvish Visionary",
				TypeLine:   "Creature - Elf Shaman",
				OracleText: "When Elvish Visionary enters, draw a card.",
				CMC:        2,
			},
			blockedTheme: "Blink",
		},
		{
			name: "tribal body is not tribal payoff by itself",
			card: CandidateCard{
				Name:       "Llanowar Elves",
				TypeLine:   "Creature - Elf Druid",
				OracleText: "{T}: Add {G}.",
				CMC:        1,
			},
			blockedTheme: "Tribal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := classifyCard(tt.card)
			if hasTheme(card, tt.blockedTheme) {
				t.Fatalf("expected %s not to be tagged with theme %s, got themes %v", card.Name, tt.blockedTheme, card.Themes)
			}
		})
	}
}

func TestClassifyCardMarksCommonInteractionPatterns(t *testing.T) {
	tests := []struct {
		name     string
		card     CandidateCard
		wantRole string
		wantFlag string
	}{
		{
			name: "damage spell counts as spot removal",
			card: CandidateCard{
				Name:       "Lightning Bolt",
				TypeLine:   "Instant",
				OracleText: "Lightning Bolt deals 3 damage to any target.",
				CMC:        1,
			},
			wantRole: "spot_removal",
		},
		{
			name: "counterspell stays interaction",
			card: CandidateCard{
				Name:       "Counterspell",
				TypeLine:   "Instant",
				OracleText: "Counter target spell.",
				CMC:        2,
			},
			wantRole: "spot_removal",
			wantFlag: "counterspell",
		},
		{
			name: "edict counts as interaction",
			card: CandidateCard{
				Name:       "Soul Shatter",
				TypeLine:   "Instant",
				OracleText: "Each opponent sacrifices a creature or planeswalker with the highest mana value among creatures and planeswalkers they control.",
				CMC:        3,
			},
			wantRole: "spot_removal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := classifyCard(tt.card)
			if tt.wantRole != "" && !hasRole(card, tt.wantRole) {
				t.Fatalf("expected %s to gain role %s, got roles %v", card.Name, tt.wantRole, card.Roles)
			}
			if tt.wantFlag != "" && !card.ScoreFlags[tt.wantFlag] {
				t.Fatalf("expected %s to gain score flag %s, got flags %v", card.Name, tt.wantFlag, card.ScoreFlags)
			}
		})
	}
}

func TestClassifyCardKeepsTribeStrategyTagWithoutTribalTheme(t *testing.T) {
	card := classifyCard(CandidateCard{
		Name:       "Llanowar Elves",
		TypeLine:   "Creature - Elf Druid",
		OracleText: "{T}: Add {G}.",
		CMC:        1,
	})

	if hasTheme(card, "Tribal") {
		t.Fatalf("expected generic elf to avoid Tribal theme, got %v", card.Themes)
	}
	if !hasStrategyTag(card, "tribe:elf") {
		t.Fatalf("expected generic elf to keep tribe strategy tag, got %v", card.StrategyTags)
	}
}

func TestInferProfileFallsBackWhenThemeConfidenceIsWeak(t *testing.T) {
	commander := classifyCard(CandidateCard{
		Name:                 "Isamaru, Hound of Konda",
		ManaCost:             "{W}",
		TypeLine:             "Legendary Creature - Dog",
		OracleText:           "",
		ColorIdentity:        []string{"W"},
		CMC:                  1,
		CommanderLegal:       true,
		IsCommanderCandidate: true,
	})

	profile := inferProfile(commander, CommanderOverride{})
	if profile.PrimaryTheme != "" {
		t.Fatalf("expected low-signal commander to have no primary theme, got %q", profile.PrimaryTheme)
	}
	if profile.Strategy == "" {
		t.Fatalf("expected fallback strategy to be set")
	}
}

func TestInferProfileKeepsInteractionForThemeShells(t *testing.T) {
	commander := classifyCard(CandidateCard{
		Name:                 "Adeline, Resplendent Cathar",
		ManaCost:             "{1}{W}{W}",
		TypeLine:             "Legendary Creature - Human Knight",
		OracleText:           "Whenever you attack, for each opponent, create a 1/1 white Human creature token that's tapped and attacking.",
		AllPartsJSON:         "[]",
		ColorIdentity:        []string{"W"},
		CMC:                  3,
		CommanderLegal:       true,
		IsCommanderCandidate: true,
	})

	profile := inferProfile(commander, CommanderOverride{})
	if profile.InteractionCount < 8 {
		t.Fatalf("expected token shell to keep at least 8 interaction slots, got %d", profile.InteractionCount)
	}
}
