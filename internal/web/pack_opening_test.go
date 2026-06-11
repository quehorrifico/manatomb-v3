package web

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
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

func TestPackOpeningRecentSetsHavePlayAndCollectorBoosters(t *testing.T) {
	for _, setCode := range []string{"msh", "sos", "tmt", "ecl", "tla", "spm", "inr", "fdn", "mh3", "mkm"} {
		config, ok := packOpeningSetConfigs[setCode]
		if !ok {
			t.Fatalf("missing recent set config %s", setCode)
		}
		for _, packType := range []string{"play", "collector"} {
			pack, ok := config.Packs[packType]
			if !ok {
				t.Fatalf("%s missing %s booster config", setCode, packType)
			}
			if pack.SourceURL == "" || pack.PackArtURI == "" {
				t.Fatalf("%s %s booster must have an official source and product art", setCode, packType)
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
			if pack.SourceURL == "" || pack.PackArtURI == "" {
				t.Fatalf("%s %s booster must have an official source and product art", setCode, packType)
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

	if !packOpeningPackConfigAvailable(candidates, play) {
		t.Fatal("Marvel Super Heroes play booster should be available with main-set candidates")
	}
	if packOpeningPackConfigAvailable(candidates, collector) {
		t.Fatal("Marvel Super Heroes collector booster should require related MSC/MAR candidates")
	}
}

func TestPackOpeningTreatmentBucketsUseSpecificPrintsThenFallback(t *testing.T) {
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
	picker = newPackOpeningPicker(fallbacks, rand.New(rand.NewSource(1)))
	card, ok = picker.pick("set:fin:foil_default:rare")
	if !ok || card.Rarity != "rare" {
		t.Fatalf("expected fallback rare when treatment metadata is absent, got %q rarity=%q ok=%v", card.ID, card.Rarity, ok)
	}
}

func TestPackOpeningPickerBuildsSetWildcardBuckets(t *testing.T) {
	card := packOpeningCandidate{
		packOpeningCard: packOpeningCard{ID: "bro-common", SetCode: "bro", Rarity: "common"},
		TypeLine:        "Creature",
		Finishes:        []string{"foil"},
	}
	picker := newPackOpeningPicker([]packOpeningCandidate{card}, rand.New(rand.NewSource(1)))
	if picked, ok := picker.pick("set:bro:wildcard"); !ok || picked.ID != card.ID {
		t.Fatalf("expected set wildcard card, got %q ok=%v", picked.ID, ok)
	}
	picker = newPackOpeningPicker([]packOpeningCandidate{card}, rand.New(rand.NewSource(1)))
	if picked, ok := picker.pick("set:bro:foil:wildcard"); !ok || picked.ID != card.ID {
		t.Fatalf("expected treatment wildcard card, got %q ok=%v", picked.ID, ok)
	}
}

func TestPackOpeningAutomaticConfigEligibility(t *testing.T) {
	now := time.Date(2026, time.June, 6, 12, 0, 0, 0, time.UTC)
	if !packOpeningAutomaticSetEligible("expansion", "2026-05-01", 220, now) {
		t.Fatal("released and complete expansion should receive an automatic pack config")
	}
	if packOpeningAutomaticSetEligible("expansion", "2026-08-01", 220, now) {
		t.Fatal("future expansion should not be auto-added")
	}
	if packOpeningAutomaticSetEligible("expansion", "2026-05-01", 80, now) {
		t.Fatal("incomplete expansion should not be auto-added")
	}
	if packOpeningAutomaticSetEligible("promo", "2026-05-01", 220, now) {
		t.Fatal("promo set should not receive an automatic pack config")
	}
	config, ok := packOpeningConfigForSet("released", "expansion", time.Now().UTC().Format("2006-01-02"), 220)
	if !ok || !config.Packs["play"].AutoGenerated || !config.Packs["collector"].AutoGenerated {
		t.Fatal("automatic config should mark Play and Collector packs as generated")
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
		})
	}
	return out
}
