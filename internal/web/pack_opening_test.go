package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestPackOpeningConfiguredSlotsMatchCardCount(t *testing.T) {
	for setCode, setConfig := range packOpeningSetConfigs {
		for packType, packConfig := range setConfig.Packs {
			if got, want := len(packConfig.Slots), packConfig.CardCount; got != want {
				t.Fatalf("%s %s has %d slots, want %d", setCode, packType, got, want)
			}
		}
	}
}

func TestPackOpeningSetIconSVGURIUsesStableScryfallCDNPath(t *testing.T) {
	if got, want := packOpeningSetIconSVGURI(" ECL "), "https://svgs.scryfall.io/sets/ecl.svg"; got != want {
		t.Fatalf("set icon URI = %q, want %q", got, want)
	}
	for _, invalid := range []string{"", "../ecl", "ecl?raw=1", "ecl.svg"} {
		if got := packOpeningSetIconSVGURI(invalid); got != "" {
			t.Fatalf("unsafe set code %q produced icon URI %q", invalid, got)
		}
	}
}

// This opt-in preflight checks the exact locally synced card data used by the
// simulator. Run it before exposing another configured product:
// PACK_OPENING_PREFLIGHT_DATABASE_URL=postgres://... go test ./internal/web -run PackOpeningDatabasePreflight -v
func TestPackOpeningDatabasePreflight(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("PACK_OPENING_PREFLIGHT_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set PACK_OPENING_PREFLIGHT_DATABASE_URL to audit configured products against local card data")
	}
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	setCodes := make([]string, 0, len(packOpeningSetConfigs))
	for setCode := range packOpeningSetConfigs {
		setCodes = append(setCodes, setCode)
	}
	sort.Strings(setCodes)
	for _, setCode := range setCodes {
		config := packOpeningSetConfigs[setCode]
		candidates, _, loadErr := loadPackOpeningCandidates(ctx, database, setCode, packOpeningConfigSetCodes(setCode, config))
		if loadErr != nil {
			t.Logf("%s: candidates unavailable: %v", setCode, loadErr)
			continue
		}
		packIDs := make([]string, 0, len(config.Packs))
		for packID := range config.Packs {
			packIDs = append(packIDs, packID)
		}
		sort.Strings(packIDs)
		for _, packID := range packIDs {
			packConfig := config.Packs[packID]
			_, published := packOpeningPublishedProductFor(setCode, packID)
			if published {
				packConfig = packOpeningPublishedPackConfig(setCode, packID, packConfig)
			}
			available := packOpeningPackConfigAvailable(candidates, packConfig)
			t.Logf("%s/%s: candidates=%d mechanically_available=%t launch_enabled=%t", setCode, packID, len(candidates), available, published)
			if published && !available {
				t.Errorf("launch product %s/%s does not pass its local-data availability checks", setCode, packID)
				continue
			}
			if published {
				for trial := 0; trial < 100; trial++ {
					picker := newPackOpeningPicker(candidates, rand.New(rand.NewSource(int64(trial+1))))
					seen := map[string]bool{}
					for _, slot := range packConfig.Slots {
						card, ok := picker.pickSlot(slot)
						if !ok {
							t.Fatalf("launch product %s/%s failed to generate trial %d at slot %q", setCode, packID, trial, slot.Label)
						}
						key := packOpeningCardFinishKey(card)
						if seen[key] {
							t.Fatalf("launch product %s/%s repeated %q in trial %d", setCode, packID, key, trial)
						}
						seen[key] = true
					}
				}
			}
		}
	}
}

func TestPackOpeningRecentSetsHavePlayAndCollectorBoosters(t *testing.T) {
	for _, setCode := range []string{"hob", "msh", "sos", "tmt", "ecl", "tla", "spm", "inr", "fdn", "mh3", "mkm"} {
		config, ok := packOpeningSetConfigs[setCode]
		if !ok {
			t.Fatalf("missing recent set config %s", setCode)
		}
		for _, packType := range []string{"play", "collector"} {
			pack, ok := config.Packs[packType]
			if !ok {
				t.Fatalf("%s missing %s booster config", setCode, packType)
			}
			if pack.SourceURL == "" {
				t.Fatalf("%s %s booster must have an official source", setCode, packType)
			}
		}
	}
}

func TestPackOpeningPrePlayBoosterSetsUseSourcedPackTypes(t *testing.T) {
	expected := map[string][]string{
		"lci": {"draft", "set", "collector"},
		"woe": {"draft", "set", "collector"},
		"cmm": {"draft", "set", "collector"},
		"ltr": {"draft", "set", "collector"},
		"mom": {"draft", "set", "collector"},
		"one": {"draft", "set", "collector"},
		"bro": {"draft", "set", "collector"},
	}
	for setCode, packTypes := range expected {
		config, ok := packOpeningSetConfigs[setCode]
		if !ok {
			t.Fatalf("missing pre-Play-Booster set config %s", setCode)
		}
		for _, packType := range packTypes {
			pack, ok := config.Packs[packType]
			if !ok {
				t.Fatalf("%s missing %s booster config", setCode, packType)
			}
			if pack.SourceURL == "" {
				t.Fatalf("%s %s booster must have an official source", setCode, packType)
			}
		}
	}
}

func TestPackOpeningPickerBuildsLayoutAndRetroBuckets(t *testing.T) {
	transform := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "inr-transform-common", SetCode: "inr", Rarity: "common"},
		Layout:          "transform",
		TypeLine:        "Creature",
	}
	retro := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "inr-retro-rare", SetCode: "inr", Rarity: "rare"},
		Layout:          "normal",
		TypeLine:        "Creature",
		FrameEffects:    []string{"retro"},
	}
	picker := newPackOpeningPicker([]packOpeningCandidate{transform, retro}, rand.New(rand.NewSource(1)))

	card, ok := picker.pick("set:inr:dfc:common")
	if !ok || card.ID != transform.ID {
		t.Fatalf("expected double-faced common, got %q ok=%v", card.ID, ok)
	}
	card, ok = picker.pick("set:inr:retro")
	if !ok || card.ID != retro.ID {
		t.Fatalf("expected retro-frame card, got %q ok=%v", card.ID, ok)
	}
}

func TestPackOpeningPickerBuildsBattleBuckets(t *testing.T) {
	battle := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "mom-battle", SetCode: "mom", Rarity: "uncommon"},
		Layout:          "transform",
		TypeLine:        "Battle - Siege",
	}
	creature := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "mom-dfc-creature", SetCode: "mom", Rarity: "uncommon"},
		Layout:          "transform",
		TypeLine:        "Creature",
	}
	picker := newPackOpeningPicker([]packOpeningCandidate{battle, creature}, rand.New(rand.NewSource(1)))

	card, ok := picker.pick("set:mom:battle")
	if !ok || card.ID != battle.ID {
		t.Fatalf("expected battle, got %q ok=%v", card.ID, ok)
	}
	card, ok = picker.pick("set:mom:nonbattle_dfc")
	if !ok || card.ID != creature.ID {
		t.Fatalf("expected non-battle double-faced card, got %q ok=%v", card.ID, ok)
	}
}

func TestPackOpeningPickerBuildsLegendaryAndReleaseBuckets(t *testing.T) {
	legend := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "cmm-legend", SetCode: "cmm", Rarity: "rare"},
		TypeLine:        "Legendary Creature",
	}
	guest := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "spg-lci", SetCode: "spg", Rarity: "rare"},
		TypeLine:        "Artifact",
		ReleasedAt:      "2023-11-17",
	}
	picker := newPackOpeningPicker([]packOpeningCandidate{legend, guest}, rand.New(rand.NewSource(1)))

	card, ok := picker.pick("set:cmm:legendary:rare")
	if !ok || card.ID != legend.ID {
		t.Fatalf("expected legendary rare, got %q ok=%v", card.ID, ok)
	}
	card, ok = picker.pick("set:spg:release:2023-11-17:rare")
	if !ok || card.ID != guest.ID {
		t.Fatalf("expected release-specific guest, got %q ok=%v", card.ID, ok)
	}
}

func TestPackOpeningPackConfigAvailableRequiresRelatedBuckets(t *testing.T) {
	msh := packOpeningSetConfigs["msh"]
	play := msh.Packs["play"]
	collector := msh.Packs["collector"]
	candidates := packOpeningTestCandidates("msh", 90, 100, 40, 20, 20)

	if packOpeningPackConfigAvailable(candidates, play) {
		t.Fatal("Marvel Super Heroes play booster should require its complete release-scoped source-material pool")
	}
	if packOpeningPackConfigAvailable(candidates, collector) {
		t.Fatal("Marvel Super Heroes collector booster should require related MSC/MAR candidates")
	}
	for i := 0; i < 60; i++ {
		candidates = append(candidates, packOpeningCandidate{
			packOpeningCard: packOpeningCard{ID: fmt.Sprintf("mar-%d", i), SetCode: "mar", Rarity: "rare"},
			ReleasedAt:      "2026-06-26",
			Finishes:        []string{"nonfoil", "foil"},
		})
	}
	if !packOpeningPackConfigAvailable(candidates, play) {
		t.Fatal("Marvel Super Heroes play booster should become available with its complete source-material pool")
	}
}

func TestPackOpeningTreatmentBucketsNeverFallBack(t *testing.T) {
	specific := packOpeningCandidate{
		packOpeningCard: packOpeningCard{
			ID:      "fin-foil-default-rare",
			Name:    "Foil Default Rare",
			SetCode: "fin",
			Rarity:  "rare",
		},
		TypeLine: "Creature",
		Finishes: []string{"foil"},
	}
	picker := newPackOpeningPicker([]packOpeningCandidate{specific}, rand.New(rand.NewSource(1)))
	card, ok := picker.pick("set:fin:foil_default:rare")
	if !ok || card.ID != specific.ID {
		t.Fatalf("expected treatment-specific card, got %q ok=%v", card.ID, ok)
	}

	fallbacks := packOpeningTestCandidates("fin", 0, 0, 1, 0, 0)
	fallbacks[0].Finishes = []string{"nonfoil"}
	picker = newPackOpeningPicker(fallbacks, rand.New(rand.NewSource(1)))
	if card, ok = picker.pick("set:fin:foil_default:rare"); ok {
		t.Fatalf("treatment pool silently fell back to %q rarity=%q", card.ID, card.Rarity)
	}
}

func TestPackOpeningPickerBuildsSetWildcardBuckets(t *testing.T) {
	candidates := packOpeningTestCandidates("bro", 1, 1, 1, 1, 0)
	picker := newPackOpeningPicker(candidates, rand.New(rand.NewSource(1)))
	if picked, ok := picker.pick("set:bro:wildcard"); !ok || picked.SetCode != "bro" {
		t.Fatalf("expected set wildcard card, got %q ok=%v", picked.ID, ok)
	}
	picker = newPackOpeningPicker(candidates, rand.New(rand.NewSource(1)))
	if picked, ok := picker.pick("set:bro:foil:wildcard"); !ok || picked.SetCode != "bro" {
		t.Fatalf("expected treatment wildcard card, got %q ok=%v", picked.ID, ok)
	}
}

func TestPackOpeningCatalogOnlyUsesPublishedSets(t *testing.T) {
	for _, setCode := range []string{"hob", "ecl", "tla", "spm", "eoe", "fin", "fdn", "dsk", "blb", "woe", "one", "bro"} {
		if _, ok := packOpeningConfigForSet(setCode, "expansion", "2026-01-01", 300); !ok {
			t.Fatalf("published set %s should be available", setCode)
		}
	}
	for _, setCode := range []string{"released", "msh", "sos", "tmt", "inr", "tdm", "dft", "lci"} {
		if _, ok := packOpeningConfigForSet(setCode, "expansion", "2026-01-01", 300); ok {
			t.Fatalf("unpublished set %s must not be auto-added", setCode)
		}
	}
}

func TestPackOpeningLaunchProductsAreAuditedIndividually(t *testing.T) {
	for _, test := range []struct {
		setCode    string
		packTypeID string
		accuracy   string
	}{
		{setCode: "spm", packTypeID: "play", accuracy: packOpeningAccuracySourced},
		{setCode: "spm", packTypeID: "collector", accuracy: packOpeningAccuracyStructure},
		{setCode: "hob", packTypeID: "play", accuracy: packOpeningAccuracySourced},
		{setCode: "hob", packTypeID: "collector", accuracy: packOpeningAccuracyStructure},
	} {
		publication, ok := packOpeningPublishedProductFor(test.setCode, test.packTypeID)
		if !ok || publication.Accuracy != test.accuracy || publication.BasicRarity {
			t.Fatalf("%s/%s publication = %#v, want audited %s recipe", test.setCode, test.packTypeID, publication, test.accuracy)
		}
	}
	if _, ok := packOpeningPublishedProductFor("fin", "play"); !ok {
		t.Fatal("Final Fantasy Play Booster should be launch-ready")
	}
	if _, ok := packOpeningPublishedProductFor("fin", "collector"); ok {
		t.Fatal("Final Fantasy Collector Booster must remain hidden until its conditional foil and Through the Ages collation is modeled safely")
	}
	if publication, ok := packOpeningPublishedProductFor("eoe", "play"); !ok || !publication.BasicRarity {
		t.Fatal("Edge of Eternities must use the safe basic-rarity recipe until its Special Guests pool is release-scoped")
	}
	if _, ok := packOpeningPublishedProductFor("tmt", "play"); ok {
		t.Fatal("Teenage Mutant Ninja Turtles must remain hidden until every non-booster-marked specialty pool is audited")
	}
	for _, setCode := range []string{"woe", "one", "bro"} {
		publication, ok := packOpeningPublishedProductFor(setCode, "collector")
		if !ok || publication.Accuracy != packOpeningAccuracyStructure || !publication.HardenCollectorPools {
			t.Fatalf("%s Collector Booster should use the hardened structure approximation: %#v", setCode, publication)
		}
	}
}

func TestPackOpeningPublishedCollectorPoolsExcludeBroadFoilAndLandBuckets(t *testing.T) {
	landLabels := map[string]string{
		"woe": "Traditional Foil Full-art Basic Land",
		"one": "Traditional Foil Panorama or Phyrexianized Land",
		"bro": "Traditional Foil Mech Land",
	}
	for setCode, landLabel := range landLabels {
		config, ok := packOpeningConfigForSet(setCode, "expansion", "2023-01-01", 300)
		if !ok {
			t.Fatalf("published set %s is unavailable", setCode)
		}
		pack := config.Packs["collector"]
		seenLand := false
		for _, slot := range pack.Slots {
			switch slot.Label {
			case "Traditional Foil Common":
				if slot.Bucket != "set:"+setCode+":foil_default:common" {
					t.Fatalf("%s common slot uses %q", setCode, slot.Bucket)
				}
			case "Traditional Foil Uncommon":
				if slot.Bucket != "set:"+setCode+":foil_default:uncommon" {
					t.Fatalf("%s uncommon slot uses %q", setCode, slot.Bucket)
				}
			case landLabel:
				seenLand = true
				if slot.Bucket != "set:"+setCode+":fullart:common" {
					t.Fatalf("%s dedicated land slot uses %q", setCode, slot.Bucket)
				}
			}
		}
		if !seenLand {
			t.Fatalf("%s Collector Booster is missing its dedicated land slot", setCode)
		}
	}
}

func TestPackOpeningDefaultRarityBucketsExcludeLands(t *testing.T) {
	spell := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "spell", OracleID: "spell", SetCode: "tmt", Rarity: "common"},
		TypeLine:        "Creature",
		Finishes:        []string{"nonfoil"},
	}
	land := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "land", OracleID: "land", SetCode: "tmt", Rarity: "common"},
		TypeLine:        "Basic Land — Island",
		Finishes:        []string{"nonfoil"},
	}
	picker := newPackOpeningPicker([]packOpeningCandidate{spell, land}, rand.New(rand.NewSource(2)))
	items := picker.buckets["set:tmt:default:common"]
	if len(items) != 1 || items[0].ID != spell.ID {
		t.Fatalf("default common pool = %#v, want only the nonland spell", items)
	}
}

func TestPackOpeningDefaultBucketsExcludeSupplementalPromos(t *testing.T) {
	promo := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "bundle", OracleID: "bundle", SetCode: "tmt", Rarity: "rare"},
		TypeLine:        "Legendary Creature — Mutant",
		Finishes:        []string{"foil"},
		PromoTypes:      []string{"bundle"},
	}
	picker := newPackOpeningPicker([]packOpeningCandidate{promo}, rand.New(rand.NewSource(2)))
	if items := picker.buckets["set:tmt:default:rare"]; len(items) != 0 {
		t.Fatalf("default rare pool admitted bundle promo: %#v", items)
	}
}

func TestPackOpeningCompoundTreatmentBucketsAreNotOrdinary(t *testing.T) {
	for _, bucket := range []string{
		"set:fin:foil_boosterfun:rare",
		"set:fin:nonfoil_boosterfun:mythic",
		"set:fin:surgefoil_boosterfun:rare_or_mythic",
	} {
		if packOpeningOrdinaryBucket(bucket) {
			t.Fatalf("compound treatment bucket %q was classified as ordinary", bucket)
		}
	}
	if !packOpeningOrdinaryBucket("set:fin:foil_default:rare") {
		t.Fatal("foil default rarity bucket should remain an ordinary booster pool")
	}
}

func TestPackOpeningWeightedSlotRequiresEveryPublishedBranch(t *testing.T) {
	picker := newPackOpeningPicker(packOpeningTestCandidates("tst", 1, 0, 1, 0, 0), rand.New(rand.NewSource(2)))
	if _, ok := picker.pickWeighted(wb("set:tst:common", 900, "set:tst:mythic", 100)); ok {
		t.Fatal("weighted slot must not renormalize around a missing mythic pool")
	}
}

func TestPackOpeningSlotsDoNotRepeatSamePrintingAndFinish(t *testing.T) {
	candidates := []packOpeningCandidate{
		{packOpeningCard: packOpeningCard{ID: "a", OracleID: "oracle-a", Name: "Card A", SetCode: "tst", Rarity: "common"}, Finishes: []string{"nonfoil", "foil"}},
		{packOpeningCard: packOpeningCard{ID: "b", OracleID: "oracle-b", Name: "Card B", SetCode: "tst", Rarity: "common"}, Finishes: []string{"nonfoil", "foil"}},
	}
	picker := newPackOpeningPicker(candidates, rand.New(rand.NewSource(7)))
	first, firstOK := picker.pickSlot(packSlot{Label: "Common", Bucket: "set:tst:common"})
	second, secondOK := picker.pickSlot(packSlot{Label: "Common", Bucket: "set:tst:common"})
	if !firstOK || !secondOK {
		t.Fatalf("nonfoil slots failed: first=%v second=%v", firstOK, secondOK)
	}
	if first.ID == second.ID && first.Finish == second.Finish {
		t.Fatalf("same printing and finish repeated: %s/%s", first.ID, first.Finish)
	}

	// A physical booster may legitimately contain the same printing twice when
	// one copy is non-foil and the other occupies the dedicated foil slot.
	foil, foilOK := picker.pickSlot(packSlot{Label: "Traditional Foil", Bucket: "set:tst:common"})
	if !foilOK || foil.Finish != "foil" {
		t.Fatalf("foil slot = %#v ok=%v", foil.packOpeningCard, foilOK)
	}
}

func TestPackOpeningDuplicatePreventionUsesOracleIdentity(t *testing.T) {
	candidates := []packOpeningCandidate{
		{packOpeningCard: packOpeningCard{ID: "default-print", OracleID: "same-oracle", Name: "Same Card", SetCode: "tst", Rarity: "rare"}, Finishes: []string{"nonfoil"}},
		{packOpeningCard: packOpeningCard{ID: "showcase-print", OracleID: "same-oracle", Name: "Same Card", SetCode: "tst", Rarity: "rare"}, Finishes: []string{"nonfoil"}, FrameEffects: []string{"showcase"}},
	}
	picker := newPackOpeningPicker(candidates, rand.New(rand.NewSource(11)))
	if _, ok := picker.pickSlot(packSlot{Label: "Rare", Bucket: "set:tst:rare"}); !ok {
		t.Fatal("first oracle printing should be available")
	}
	if card, ok := picker.pickSlot(packSlot{Label: "Showcase Rare", Bucket: "set:tst:showcase:rare"}); ok {
		t.Fatalf("second treatment repeated the same oracle and finish: %#v", card.packOpeningCard)
	}
}

func TestPackOpeningDuplicatePreventionFailsClosedWhenPoolIsExhausted(t *testing.T) {
	candidate := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "only", SetCode: "tst", Rarity: "common"},
		Finishes:        []string{"nonfoil"},
	}
	picker := newPackOpeningPicker([]packOpeningCandidate{candidate}, rand.New(rand.NewSource(7)))
	if _, ok := picker.pickSlot(packSlot{Label: "Common", Bucket: "set:tst:common"}); !ok {
		t.Fatal("first slot should consume the only printing")
	}
	if card, ok := picker.pickSlot(packSlot{Label: "Common", Bucket: "set:tst:common"}); ok {
		t.Fatalf("exhausted slot repeated %#v instead of failing closed", card.packOpeningCard)
	}
}

func TestPackOpeningPickerExcludesDigitalAndVariationCandidates(t *testing.T) {
	valid := packOpeningCandidate{packOpeningCard: packOpeningCard{ID: "valid", SetCode: "tst", Rarity: "rare"}, Finishes: []string{"nonfoil"}}
	digital := packOpeningCandidate{packOpeningCard: packOpeningCard{ID: "digital", SetCode: "tst", Rarity: "rare"}, Digital: true, Finishes: []string{"nonfoil"}}
	variation := packOpeningCandidate{packOpeningCard: packOpeningCard{ID: "variation", SetCode: "tst", Rarity: "rare"}, Variation: true, Finishes: []string{"nonfoil"}}
	picker := newPackOpeningPicker([]packOpeningCandidate{valid, digital, variation}, rand.New(rand.NewSource(3)))
	if got := len(picker.buckets["set:tst:rare"]); got != 1 || picker.buckets["set:tst:rare"][0].ID != valid.ID {
		t.Fatalf("ordinary rare pool = %#v, want only the physical non-variation printing", picker.buckets["set:tst:rare"])
	}
}

func TestPackOpeningFinishControlsDisplayedPrice(t *testing.T) {
	candidate := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "finish", SetCode: "tst", Rarity: "rare", PriceUSD: "1.00"},
		Finishes:        []string{"nonfoil", "foil", "etched"},
		PriceUSDNonfoil: "1.00",
		PriceUSDFoil:    "4.25",
		PriceUSDEtched:  "8.50",
	}
	foil, ok := packOpeningApplyFinish(candidate, "Traditional Foil Rare", "set:tst:rare")
	if !ok || foil.Finish != "foil" || foil.PriceUSD != "4.25" {
		t.Fatalf("foil card = %#v, want foil price", foil.packOpeningCard)
	}
	noFoil, ok := packOpeningApplyFinish(candidate, "Premium", "set:tst:nonfoil:rare")
	if !ok || noFoil.Finish != "nonfoil" || noFoil.PriceUSD != "1.00" {
		t.Fatalf("nonfoil card = %#v, want nonfoil price", noFoil.packOpeningCard)
	}
	legacy := candidate
	legacy.PriceUSDNonfoil = ""
	legacy.PriceUSD = "1.00"
	legacyNonfoil, ok := packOpeningApplyFinish(legacy, "Common", "set:tst:common")
	if !ok || legacyNonfoil.PriceUSD != "1.00" {
		t.Fatalf("legacy nonfoil card = %#v, want pre-migration price fallback", legacyNonfoil.packOpeningCard)
	}
}

func TestPackOpeningSourceMaterialFinishOdds(t *testing.T) {
	const trials = 20000
	picker := &packOpeningPicker{rng: rand.New(rand.NewSource(42))}
	foil := 0
	for i := 0; i < trials; i++ {
		finish, ok := picker.pickFinish(wf("nonfoil", 75, "foil", 25))
		if !ok {
			t.Fatal("source-material finish roll failed")
		}
		if finish == "foil" {
			foil++
		}
	}
	rate := float64(foil) / trials
	if rate < .24 || rate > .26 {
		t.Fatalf("foil rate = %.4f, want approximately .25", rate)
	}
	for _, setCode := range []string{"msh", "tmt", "tla", "spm"} {
		collector := packOpeningSetConfigs[setCode].Packs["collector"]
		found := false
		for _, slot := range collector.Slots {
			if slot.Label == "Source Material" {
				found = len(slot.FinishWeighted) == 2
			}
		}
		if !found {
			t.Fatalf("%s collector source-material slot lacks the published 75/25 finish recipe", setCode)
		}
	}
}

func TestPackOpeningPublishedReplacementRatesRemainWired(t *testing.T) {
	tests := []struct {
		setCode    string
		slotLabel  string
		targetPart string
		wantRate   float64
	}{
		{setCode: "ecl", slotLabel: "Common / Special Guest", targetPart: ":spg:", wantRate: 0.018},
		{setCode: "tla", slotLabel: "Common / Source Material", targetPart: ":tle:", wantRate: 1.0 / 26.0},
		{setCode: "fin", slotLabel: "Common / Through the Ages", targetPart: ":fca:", wantRate: 1.0 / 3.0},
		{setCode: "spm", slotLabel: "Common / Source Material", targetPart: ":mar:", wantRate: 1.0 / 24.0},
	}

	const trials = 50000
	for _, test := range tests {
		t.Run(test.setCode, func(t *testing.T) {
			var slot packSlot
			for _, candidate := range packOpeningSetConfigs[test.setCode].Packs["play"].Slots {
				if candidate.Label == test.slotLabel {
					slot = candidate
					break
				}
			}
			if len(slot.Weighted) == 0 {
				t.Fatalf("slot %q has no published weighted recipe", test.slotLabel)
			}

			picker := &packOpeningPicker{
				rng:     rand.New(rand.NewSource(47)),
				buckets: map[string][]packOpeningCandidate{},
				used:    map[string]bool{},
			}
			for index, weighted := range slot.Weighted {
				picker.buckets[weighted.bucket] = []packOpeningCandidate{{
					packOpeningCard: packOpeningCard{
						ID:       fmt.Sprintf("candidate-%d", index),
						OracleID: fmt.Sprintf("oracle-%d", index),
						Rarity:   "common",
					},
					Finishes: []string{"nonfoil"},
				}}
			}

			hits := 0
			for range trials {
				bucket, ok := picker.pickWeightedSlotBucket(slot, "")
				if !ok {
					t.Fatal("published replacement roll failed")
				}
				if strings.Contains(bucket, test.targetPart) {
					hits++
				}
			}
			gotRate := float64(hits) / trials
			if difference := gotRate - test.wantRate; difference < -0.004 || difference > 0.004 {
				t.Fatalf("replacement rate = %.4f, want approximately %.4f", gotRate, test.wantRate)
			}
		})
	}
}

func TestPackOpeningPackTypeUsesTruthfulAccuracyLabel(t *testing.T) {
	config, ok := packOpeningConfigForSet("dsk", "expansion", "2024-09-27", 300)
	if !ok {
		t.Fatal("published Duskmourn config is unavailable")
	}
	packType, ok := packOpeningConfiguredPackType("play", config.Packs["play"])
	if !ok || packType.Accuracy != packOpeningAccuracyStructure || packType.AccuracyLabel != packOpeningAccuracyStructureLabel {
		t.Fatalf("pack type accuracy = %q / %q", packType.Accuracy, packType.AccuracyLabel)
	}
	if packType.AccuracySummary == "" || len(packType.SlotRecipe) == 0 || len(packType.Limitations) == 0 {
		t.Fatalf("simulation details are incomplete: %#v", packType)
	}
}

func TestPackOpeningAccuracyTiersExposeStableJSONContract(t *testing.T) {
	tests := []struct {
		setCode    string
		packTypeID string
		accuracy   string
		label      string
	}{
		{setCode: "ecl", packTypeID: "play", accuracy: packOpeningAccuracySourced, label: packOpeningAccuracySourcedLabel},
		{setCode: "dsk", packTypeID: "play", accuracy: packOpeningAccuracyStructure, label: packOpeningAccuracyStructureLabel},
		{setCode: "woe", packTypeID: "collector", accuracy: packOpeningAccuracyStructure, label: packOpeningAccuracyStructureLabel},
		{setCode: "eoe", packTypeID: "play", accuracy: packOpeningAccuracyBasicRarity, label: packOpeningAccuracyBasicRarityLabel},
		{setCode: "spm", packTypeID: "play", accuracy: packOpeningAccuracySourced, label: packOpeningAccuracySourcedLabel},
		{setCode: "spm", packTypeID: "collector", accuracy: packOpeningAccuracyStructure, label: packOpeningAccuracyStructureLabel},
		{setCode: "hob", packTypeID: "play", accuracy: packOpeningAccuracySourced, label: packOpeningAccuracySourcedLabel},
		{setCode: "hob", packTypeID: "collector", accuracy: packOpeningAccuracyStructure, label: packOpeningAccuracyStructureLabel},
	}
	for _, test := range tests {
		t.Run(test.setCode+"/"+test.packTypeID, func(t *testing.T) {
			config, ok := packOpeningConfigForSet(test.setCode, "expansion", "2026-01-01", 300)
			if !ok {
				t.Fatal("published config is unavailable")
			}
			packType, ok := packOpeningConfiguredPackType(test.packTypeID, config.Packs[test.packTypeID])
			if !ok {
				t.Fatal("published pack type is unavailable")
			}
			if packType.Accuracy != test.accuracy || packType.AccuracyLabel != test.label {
				t.Fatalf("accuracy = %q / %q, want %q / %q", packType.Accuracy, packType.AccuracyLabel, test.accuracy, test.label)
			}
			encoded, err := json.Marshal(packType)
			if err != nil {
				t.Fatal(err)
			}
			var contract map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &contract); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"accuracy", "accuracy_label", "accuracy_summary", "slot_recipe", "limitations"} {
				if _, exists := contract[field]; !exists {
					t.Fatalf("JSON contract is missing %q: %s", field, encoded)
				}
			}
		})
	}
}

func TestPackOpeningSlotRecipeAggregatesRepeatedLabels(t *testing.T) {
	slots := []packSlot{{Label: "Common"}, {Label: "Rare"}, {Label: "Common"}, {Label: "Land"}}
	got := packOpeningSlotRecipe(slots)
	want := []string{"2× Common", "Rare", "Land"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("slot recipe = %#v, want %#v", got, want)
	}
}

func TestPackOpeningBasicRarityPublicationDropsUnsafeSpecialtyPools(t *testing.T) {
	config, ok := packOpeningConfigForSet("eoe", "expansion", "2025-08-01", 300)
	if !ok {
		t.Fatal("Edge of Eternities basic-rarity publication is unavailable")
	}
	pack := config.Packs["play"]
	if pack.Accuracy != packOpeningAccuracyBasicRarity || len(pack.Slots) != 14 {
		t.Fatalf("basic pack = accuracy %q slots %d", pack.Accuracy, len(pack.Slots))
	}
	for _, slot := range pack.Slots {
		buckets := []string{slot.Bucket}
		for _, weighted := range slot.Weighted {
			buckets = append(buckets, weighted.bucket)
		}
		for _, bucket := range buckets {
			if bucket == "" {
				continue
			}
			if strings.Contains(bucket, "set:spg") || strings.Contains(bucket, "set:eos") || strings.Contains(bucket, "boosterfun") || strings.Contains(bucket, "fullart") || strings.Contains(bucket, ":land") {
				t.Fatalf("basic rarity slot admitted specialty pool %q", bucket)
			}
		}
	}
}

func TestPackOpeningTMPlayLandPoolsAreCollectorNumberScoped(t *testing.T) {
	candidates := []packOpeningCandidate{
		{packOpeningCard: packOpeningCard{ID: "dual", SetCode: "tmt", CollectorNumber: "183", Rarity: "common"}, TypeLine: "Land", Finishes: []string{"nonfoil", "foil"}},
		{packOpeningCard: packOpeningCard{ID: "rooftop", SetCode: "tmt", CollectorNumber: "191", Rarity: "common"}, TypeLine: "Basic Land — Plains", FullArt: true, Finishes: []string{"nonfoil", "foil"}},
		{packOpeningCard: packOpeningCard{ID: "rare-land", SetCode: "tmt", CollectorNumber: "188", Rarity: "rare"}, TypeLine: "Land", Finishes: []string{"nonfoil", "foil"}},
		{packOpeningCard: packOpeningCard{ID: "pizza", SetCode: "tmt", CollectorNumber: "253", Rarity: "common"}, TypeLine: "Basic Land — Plains", FullArt: true, Finishes: []string{"nonfoil", "foil"}},
		{packOpeningCard: packOpeningCard{ID: "premium", SetCode: "tmt", CollectorNumber: "305", Rarity: "common"}, TypeLine: "Basic Land — Plains", FullArt: true, Finishes: []string{"foil"}},
	}
	picker := newPackOpeningPicker(candidates, rand.New(rand.NewSource(8)))
	for _, bucket := range []string{
		"set:tmt:nonfoil_play_dual:land",
		"set:tmt:foil_play_dual:land",
		"set:tmt:nonfoil_rooftop:land",
		"set:tmt:foil_rooftop:land",
	} {
		items := picker.buckets[bucket]
		if len(items) != 1 || (items[0].ID != "dual" && items[0].ID != "rooftop") {
			t.Fatalf("TMT Play land bucket %q = %#v", bucket, items)
		}
	}

	var landSlot packSlot
	for _, slot := range packOpeningSetConfigs["tmt"].Packs["play"].Slots {
		if slot.Label == "Land" {
			landSlot = slot
			break
		}
	}
	if len(landSlot.Weighted) != 4 {
		t.Fatalf("TMT land recipe = %#v", landSlot.Weighted)
	}
	weights := map[string]int{}
	for _, weighted := range landSlot.Weighted {
		weights[weighted.bucket] = weighted.weight
	}
	if weights["set:tmt:nonfoil_play_dual:land"] != 48 || weights["set:tmt:foil_play_dual:land"] != 12 || weights["set:tmt:nonfoil_rooftop:land"] != 32 || weights["set:tmt:foil_rooftop:land"] != 8 {
		t.Fatalf("TMT land weights = %#v", weights)
	}
}

func TestPackOpeningSpiderManAndHobbitPoolsAreCollectorNumberScoped(t *testing.T) {
	candidates := []packOpeningCandidate{
		packOpeningNumberedCandidate("spm-main", "spm", "1", "common", "Creature", true, "nonfoil", "foil"),
		packOpeningNumberedCandidate("spm-dual", "spm", "181", "common", "Land", true, "nonfoil", "foil"),
		packOpeningNumberedCandidate("spm-scene", "spm", "199", "uncommon", "Creature", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("spm-web", "spm", "208", "mythic", "Creature", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("spm-panel", "spm", "218", "rare", "Enchantment", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("spm-classic", "spm", "232", "mythic", "Creature", false, "foil"),
		packOpeningNumberedCandidate("spm-costume", "spm", "235", "rare", "Creature", false, "foil"),
		packOpeningNumberedCandidate("spm-gauntlet", "spm", "243", "mythic", "Artifact", false, "foil"),
		packOpeningNumberedCandidate("spm-extended", "spm", "244", "mythic", "Creature", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-main", "hob", "187", "rare", "Land", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-dual", "hob", "182", "common", "Land", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-default-basic", "hob", "189", "common", "Basic Land", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-journey", "hob", "194", "common", "Basic Land", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-scene", "hob", "199", "uncommon", "Creature", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-dragon", "hob", "214", "rare", "Creature", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-book", "hob", "239", "mythic", "Enchantment", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-surge-dragon", "hob", "250", "rare", "Creature", false, "foil"),
		packOpeningNumberedCandidate("hob-surge-book", "hob", "275", "mythic", "Creature", false, "foil"),
		packOpeningNumberedCandidate("hob-extended", "hob", "285", "rare", "Creature", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hob-seasonal", "hob", "313", "common", "Basic Land", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hoc-scene", "hoc", "1", "rare", "Creature", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hoc-classic", "hoc", "13", "mythic", "Creature", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hoc-surge", "hoc", "53", "mythic", "Creature", false, "foil"),
		packOpeningNumberedCandidate("hoc-dwarvish", "hoc", "93", "mythic", "Artifact", false, "nonfoil", "foil"),
		packOpeningNumberedCandidate("hoc-extended", "hoc", "98", "mythic", "Creature", false, "nonfoil"),
	}
	picker := newPackOpeningPicker(candidates, rand.New(rand.NewSource(21)))

	wantSingle := map[string]string{
		"set:spm:main:common":                    "spm-main",
		"set:spm:play_dual:land":                 "spm-dual",
		"set:spm:scene_boosterfun:uncommon":      "spm-scene",
		"set:spm:webslinger_boosterfun:mythic":   "spm-web",
		"set:spm:panel_boosterfun:rare":          "spm-panel",
		"set:spm:classiccomic_boosterfun:mythic": "spm-classic",
		"set:spm:costume_boosterfun:rare":        "spm-costume",
		"set:spm:gauntlet_boosterfun:mythic":     "spm-gauntlet",
		"set:spm:extendedart_main:mythic":        "spm-extended",
		"set:hob:main:rare":                      "hob-main",
		"set:hob:play_dual:land":                 "hob-dual",
		"set:hob:default_basic:land":             "hob-default-basic",
		"set:hob:journey:land":                   "hob-journey",
		"set:hob:scene:uncommon":                 "hob-scene",
		"set:hob:dragonhoard:rare":               "hob-dragon",
		"set:hob:bookcover:mythic":               "hob-book",
		"set:hob:surgefoil_dragonhoard:rare":     "hob-surge-dragon",
		"set:hob:surgefoil_bookcover:mythic":     "hob-surge-book",
		"set:hob:extendedart_main:rare":          "hob-extended",
		"set:hoc:scene:rare":                     "hoc-scene",
		"set:hoc:classicartist:mythic":           "hoc-classic",
		"set:hoc:surgefoil_classicartist:mythic": "hoc-surge",
		"set:hoc:dwarvish:mythic":                "hoc-dwarvish",
		"set:hoc:extendedart_main:mythic":        "hoc-extended",
	}
	for bucket, wantID := range wantSingle {
		items := picker.buckets[bucket]
		if len(items) != 1 || items[0].ID != wantID {
			t.Fatalf("bucket %q = %#v, want only %q", bucket, items, wantID)
		}
	}
	for _, bucket := range []string{"set:hob:main:common", "set:hob:journey:land", "set:hob:default_basic:land"} {
		for _, card := range picker.buckets[bucket] {
			if card.ID == "hob-seasonal" {
				t.Fatalf("seasonal Bundle/Prerelease land leaked into %q", bucket)
			}
		}
	}
}

func TestPackOpeningNewPublishedLandAndPremiumRecipes(t *testing.T) {
	spider := packOpeningSetConfigs["spm"]
	var spiderLand packSlot
	for _, slot := range spider.Packs["play"].Slots {
		if slot.Label == "Land" {
			spiderLand = slot
			break
		}
	}
	if len(spiderLand.Weighted) != 3 || len(spiderLand.FinishWeighted) != 2 {
		t.Fatalf("Spider-Man land slot = %#v", spiderLand)
	}
	spiderLandWeights := map[string]int{}
	for _, weighted := range spiderLand.Weighted {
		spiderLandWeights[weighted.bucket] = weighted.weight
	}
	if spiderLandWeights["set:spm:play_dual:land"] != 500 || spiderLandWeights["set:spm:spiderweb:land"] != 250 || spiderLandWeights["set:spm:default_basic:land"] != 250 {
		t.Fatalf("Spider-Man land weights = %#v", spiderLandWeights)
	}

	hobbit := packOpeningSetConfigs["hob"]
	var hobbitLand packSlot
	for _, slot := range hobbit.Packs["play"].Slots {
		if slot.Label == "Land" {
			hobbitLand = slot
			break
		}
	}
	if got := len(hobbitLand.Weighted); got != 6 {
		t.Fatalf("Hobbit land slot has %d branches, want 6", got)
	}
	weights := map[string]int{}
	for _, weighted := range hobbitLand.Weighted {
		weights[weighted.bucket] = weighted.weight
	}
	for bucket, want := range map[string]int{
		"set:hob:nonfoil_default_basic:land": 267,
		"set:hob:foil_default_basic:land":    67,
		"set:hob:nonfoil_journey:land":       133,
		"set:hob:foil_journey:land":          33,
		"set:hob:nonfoil_play_dual:land":     400,
		"set:hob:foil_play_dual:land":        100,
	} {
		if weights[bucket] != want {
			t.Fatalf("Hobbit land weight %q = %d, want %d", bucket, weights[bucket], want)
		}
	}
	if _, exposed := hobbit.Packs["collector"].RequiredPools["set:hob:headliner:mythic"]; exposed {
		t.Fatal("Hobbit headliner must remain outside the simulated recipe without a published per-pack rate")
	}
}

func packOpeningNumberedCandidate(id, setCode, collectorNumber, rarity, typeLine string, booster bool, finishes ...string) packOpeningCandidate {
	return packOpeningCandidate{
		packOpeningCard: packOpeningCard{
			ID:              id,
			OracleID:        "oracle-" + id,
			Name:            id,
			SetCode:         setCode,
			CollectorNumber: collectorNumber,
			Rarity:          rarity,
		},
		TypeLine: typeLine,
		Booster:  booster,
		Finishes: finishes,
	}
}

func packOpeningTestCandidates(setCode string, commons int, uncommons int, rares int, mythics int, lands int) []packOpeningCandidate {
	var out []packOpeningCandidate
	out = append(out, packOpeningTestRarityCandidates(setCode, "common", commons, "Creature")...)
	out = append(out, packOpeningTestRarityCandidates(setCode, "uncommon", uncommons, "Creature")...)
	out = append(out, packOpeningTestRarityCandidates(setCode, "rare", rares, "Creature")...)
	out = append(out, packOpeningTestRarityCandidates(setCode, "mythic", mythics, "Creature")...)
	out = append(out, packOpeningTestRarityCandidates(setCode, "common", lands, "Basic Land")...)
	return out
}

func packOpeningTestRarityCandidates(setCode string, rarity string, count int, typeLine string) []packOpeningCandidate {
	out := make([]packOpeningCandidate, 0, count)
	for i := 0; i < count; i++ {
		id := strings.ToLower(fmt.Sprintf("%s-%s-%s-%03d", setCode, rarity, strings.ReplaceAll(typeLine, " ", "-"), i))
		out = append(out, packOpeningCandidate{
			packOpeningCard: packOpeningCard{
				ID:      id,
				Name:    id,
				SetCode: setCode,
				Rarity:  rarity,
			},
			TypeLine: typeLine,
			Finishes: []string{"nonfoil", "foil"},
		})
	}
	return out
}
