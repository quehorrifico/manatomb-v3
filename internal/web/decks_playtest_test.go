package web

import "testing"

func TestNormalizeWorkbenchDraftSeedCommander(t *testing.T) {
	in := workbenchPlaytestPayload{
		CommanderName: "Atraxa, Grand Unifier",
		Format:        "Commander",
		Name:          "Ignored",
		Description:   "Test deck",
		Tags:          "Ramp, Midrange",
		Cards: []workbenchPlaytestCard{
			{Name: "Atraxa, Grand Unifier", Qty: 1},
			{Name: "Sol Ring", Qty: 1},
			{Name: "Island", Qty: 3},
		},
		MaybeCards: []workbenchPlaytestCard{
			{Name: "Arcane Signet", Qty: 1},
		},
		CommanderCandidates: []string{"Atraxa, Grand Unifier", "The Goose Mother"},
		CardMeta: map[string]workbenchPlaytestCardMeta{
			"Atraxa, Grand Unifier": {
				Name:                 "Atraxa, Grand Unifier",
				ManaCost:             "{3}{G}{W}{U}",
				IsCommanderCandidate: true,
			},
		},
	}

	got := normalizeWorkbenchDraftSeed(in)

	if got.Name != "New Guest Deck" {
		t.Fatalf("expected guest deck name to be fixed, got %q", got.Name)
	}
	if got.Format != "Commander" {
		t.Fatalf("expected Commander format, got %q", got.Format)
	}
	if got.CommanderName != "Atraxa, Grand Unifier" {
		t.Fatalf("expected commander preserved, got %q", got.CommanderName)
	}
	if got.Cards["Island"] != 3 {
		t.Fatalf("expected Island count 3, got %d", got.Cards["Island"])
	}
	if got.MaybeCards["Arcane Signet"] != 1 {
		t.Fatalf("expected Arcane Signet maybe count 1, got %d", got.MaybeCards["Arcane Signet"])
	}
	if len(got.CommanderCandidates) != 2 {
		t.Fatalf("expected commander candidates to be preserved, got %v", got.CommanderCandidates)
	}
	if !got.CardMeta["Atraxa, Grand Unifier"].IsCommanderCandidate {
		t.Fatalf("expected commander metadata to be preserved")
	}
	if got.CardMeta["Atraxa, Grand Unifier"].ManaCost != "{3}{G}{W}{U}" {
		t.Fatalf("expected mana cost to be preserved, got %q", got.CardMeta["Atraxa, Grand Unifier"].ManaCost)
	}
}

func TestNormalizeWorkbenchDraftSeedSandboxClearsCommander(t *testing.T) {
	in := workbenchPlaytestPayload{
		CommanderName: "Atraxa, Grand Unifier",
		Format:        "Sandbox",
		Sandbox:       true,
		Cards: []workbenchPlaytestCard{
			{Name: "Sol Ring", Qty: 1},
		},
	}

	got := normalizeWorkbenchDraftSeed(in)

	if got.Format != "Sandbox" {
		t.Fatalf("expected Sandbox format, got %q", got.Format)
	}
	if got.CommanderName != "" {
		t.Fatalf("expected commander to be cleared for sandbox, got %q", got.CommanderName)
	}
	if got.Sandbox != true {
		t.Fatalf("expected sandbox flag to be preserved")
	}
}
