package web

import (
	"testing"

	"manatomb/app/internal/decks"
	"manatomb/app/internal/quickbuild"
)

func TestGuestRequestEligibleForQuickBuild(t *testing.T) {
	if !guestRequestEligibleForQuickBuild("Atraxa, Praetors' Voice", "Commander", []quickBuildRequestCard{
		{Name: "Atraxa, Praetors' Voice", Qty: 1},
	}, nil, nil) {
		t.Fatalf("expected commander-only request to be eligible")
	}

	if guestRequestEligibleForQuickBuild("Atraxa, Praetors' Voice", "Commander", []quickBuildRequestCard{
		{Name: "Atraxa, Praetors' Voice", Qty: 1},
		{Name: "Sol Ring", Qty: 1},
	}, nil, nil) {
		t.Fatalf("expected non-empty deck request to be rejected")
	}

	if guestRequestEligibleForQuickBuild("Atraxa, Praetors' Voice", "Sandbox", []quickBuildRequestCard{
		{Name: "Atraxa, Praetors' Voice", Qty: 1},
	}, nil, nil) {
		t.Fatalf("expected sandbox request to be rejected")
	}

	if guestRequestEligibleForQuickBuild("Atraxa, Praetors' Voice", "Commander", []quickBuildRequestCard{
		{Name: "Atraxa, Praetors' Voice", Qty: 1},
	}, []quickBuildRequestCard{{Name: "Negate", Qty: 1}}, nil) {
		t.Fatalf("expected request with sideboard cards to be rejected")
	}
}

func TestDeckEligibleForQuickBuild(t *testing.T) {
	deckCards := []decks.DeckCard{
		{CardName: "Atraxa, Praetors' Voice", Quantity: 1},
	}
	if !deckEligibleForQuickBuild("Commander", "Atraxa, Praetors' Voice", deckCards, nil, nil) {
		t.Fatalf("expected commander-only saved deck to be eligible")
	}

	deckCards = append(deckCards, decks.DeckCard{CardName: "Arcane Signet", Quantity: 1})
	if deckEligibleForQuickBuild("Commander", "Atraxa, Praetors' Voice", deckCards, nil, nil) {
		t.Fatalf("expected saved deck with extra mainboard card to be rejected")
	}

	if deckEligibleForQuickBuild("Commander", "Atraxa, Praetors' Voice", []decks.DeckCard{
		{CardName: "Atraxa, Praetors' Voice", Quantity: 1},
	}, []decks.DeckCard{{CardName: "Negate", Quantity: 1}}, nil) {
		t.Fatalf("expected saved deck with sideboard card to be rejected")
	}
}

func TestRecommendedQuickBuildTagsPrefersThemeThenStrategy(t *testing.T) {
	tags := recommendedQuickBuildTags(quickbuild.Summary{
		Strategy:     "Control",
		PrimaryTheme: "Spellslinger",
		Themes:       []string{"Spellslinger", "Tokens"},
	})

	if len(tags) != 2 {
		t.Fatalf("expected two recommended tags, got %v", tags)
	}
	if tags[0] != "Spellslinger" || tags[1] != "Control" {
		t.Fatalf("unexpected recommended tags: %v", tags)
	}
}

func TestRecommendedQuickBuildTagsSkipsMidrangeWhenThemeExists(t *testing.T) {
	tags := recommendedQuickBuildTags(quickbuild.Summary{
		Strategy:     "Midrange",
		PrimaryTheme: "Tokens",
		Themes:       []string{"Tokens"},
	})

	if len(tags) != 1 || tags[0] != "Tokens" {
		t.Fatalf("expected only Tokens tag, got %v", tags)
	}
}

func TestMergeQuickBuildTagsPreservesExistingAndAddsAtMostTwo(t *testing.T) {
	got := mergeQuickBuildTags("Ramp, Control", quickbuild.Summary{
		Strategy:     "Control",
		PrimaryTheme: "Tokens",
		Themes:       []string{"Tokens", "Tribal"},
	})

	want := "Ramp, Control, Tokens"
	if got != want {
		t.Fatalf("mergeQuickBuildTags() = %q, want %q", got, want)
	}
}
