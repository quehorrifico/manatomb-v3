package web

import (
	"math/rand"
	"strings"
	"testing"
)

func TestPackOpeningRecipeLibraryPassesStaticValidation(t *testing.T) {
	if err := packOpeningValidateRecipeLibrary(); err != nil {
		t.Fatal(err)
	}
}

func TestPackOpeningRecipeFrameworkRequiresOfficialWizardsSources(t *testing.T) {
	for _, sourceURL := range []string{
		"https://example.com/en/news/feature/collecting-a-set",
		"http://magic.wizards.com/en/news/feature/collecting-a-set",
		"https://magic.wizards.com.example/en/news/feature/collecting-a-set",
		"https://magic.wizards.com/cards",
		"",
	} {
		if packOpeningOfficialSourceURL(sourceURL) {
			t.Fatalf("non-official source URL %q passed validation", sourceURL)
		}
	}
	if !packOpeningOfficialSourceURL("https://magic.wizards.com/en/news/feature/collecting-a-set") {
		t.Fatal("official Wizards product article failed validation")
	}
}

func TestPackOpeningRecipeFrameworkRejectsMalformedSlotsAndWeights(t *testing.T) {
	pack := packOpeningConfiguredPack{
		CardCount: 2,
		Slots: []packSlot{
			{Label: "No source"},
			{Label: "Two sources", Bucket: "set:tst:common", Weighted: []weightedPackBucket{{bucket: "set:tst:rare", weight: 1}}},
		},
		RequiredPools: map[string]int{"Set:TST:Rare": 0},
	}
	err := packOpeningValidateConfiguredPack("tst/play", pack)
	if err == nil {
		t.Fatal("malformed recipe passed validation")
	}
	for _, want := range []string{"exactly one", "required pool"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("validation error %q does not mention %q", err, want)
		}
	}

	assertPanic := func(name string, run func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s did not fail fast", name)
			}
		}()
		run()
	}
	assertPanic("odd bucket arguments", func() { wb("set:tst:common") })
	assertPanic("invalid bucket weight", func() { wb("set:tst:common", 0) })
	assertPanic("invalid finish weight", func() { wf("foil", "often") })
}

func TestPackOpeningCollectorRulesSupportExactSuffixedNumbers(t *testing.T) {
	rules := []packOpeningCollectorPoolRule{
		{
			ExactNumbers: []string{"12a"},
			Targets:      []packOpeningCollectorPoolTarget{{Bucket: "set:tst:alternate", WithRarity: true}},
		},
	}
	previous, existed := packOpeningCollectorPoolRules["tst"]
	packOpeningCollectorPoolRules["tst"] = rules
	t.Cleanup(func() {
		if existed {
			packOpeningCollectorPoolRules["tst"] = previous
		} else {
			delete(packOpeningCollectorPoolRules, "tst")
		}
	})

	candidate := packOpeningNumberedCandidate("suffixed", "tst", "12A", "rare", "Creature", true, "nonfoil")
	picker := newPackOpeningPicker([]packOpeningCandidate{candidate}, rand.New(rand.NewSource(1)))
	items := picker.buckets["set:tst:alternate:rare"]
	if len(items) != 1 || items[0].ID != candidate.ID {
		t.Fatalf("exact suffixed collector-number pool = %#v", items)
	}
}

func TestPackOpeningCollectorRulesRejectOverlapAndNonCanonicalExactNumbers(t *testing.T) {
	rules := []packOpeningCollectorPoolRule{
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 1, Last: 10}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:tst:main"}},
		},
		{
			Numbers:      []packOpeningCollectorNumberSpan{{First: 10, Last: 20}},
			ExactNumbers: []string{" 21A "},
			Targets:      []packOpeningCollectorPoolTarget{{Bucket: "set:tst:showcase"}},
		},
	}
	err := packOpeningValidateCollectorPoolRules("tst", rules)
	if err == nil || !strings.Contains(err.Error(), "overlaps collector number 10") || !strings.Contains(err.Error(), "non-canonical exact collector number") {
		t.Fatalf("collector-rule validation error = %v", err)
	}
}
