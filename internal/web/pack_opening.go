package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"
)

type packOpeningSetOption struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	SetType    string `json:"set_type,omitempty"`
	ReleasedAt string `json:"released_at"`
	CardCount  int    `json:"card_count"`
	Label      string `json:"label"`
	IconSVGURI string `json:"icon_svg_uri,omitempty"`
}

type packOpeningPackType struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	CardCount       int      `json:"card_count"`
	PackArtURI      string   `json:"pack_art_uri,omitempty"`
	PackArtSize     string   `json:"pack_art_size,omitempty"`
	PackArtPosition string   `json:"pack_art_position,omitempty"`
	SourceURL       string   `json:"source_url,omitempty"`
	Accuracy        string   `json:"accuracy"`
	AccuracyLabel   string   `json:"accuracy_label"`
	AccuracySummary string   `json:"accuracy_summary"`
	SlotRecipe      []string `json:"slot_recipe"`
	Limitations     []string `json:"limitations"`
}

type packOpeningPageData struct {
	Sets            []packOpeningSetOption
	PackTypes       []packOpeningPackType
	PackTypeOptions map[string][]packOpeningPackType
	DefaultSetCode  string
}

type packOpeningCatalogSnapshot struct {
	ExpiresAt       time.Time
	Sets            []packOpeningSetOption
	PackTypes       []packOpeningPackType
	PackTypeOptions map[string][]packOpeningPackType
}

type packOpeningCard struct {
	ID              string `json:"id"`
	OracleID        string `json:"oracle_id"`
	Name            string `json:"name"`
	ImageURI        string `json:"image_uri"`
	DetailPath      string `json:"detail_path"`
	SetName         string `json:"set_name"`
	SetCode         string `json:"set_code"`
	Rarity          string `json:"rarity"`
	CollectorNumber string `json:"collector_number"`
	PriceUSD        string `json:"price_usd"`
	Finish          string `json:"finish"`
	SlotLabel       string `json:"slot_label"`
}

type packOpeningCandidate struct {
	packOpeningCard
	Layout          string
	TypeLine        string
	ReleasedAt      string
	Finishes        []string
	FrameEffects    []string
	PromoTypes      []string
	BorderColor     string
	Frame           string
	SecurityStamp   string
	FullArt         bool
	Textless        bool
	Booster         bool
	Digital         bool
	Variation       bool
	PriceUSDNonfoil string
	PriceUSDFoil    string
	PriceUSDEtched  string
}

type packOpeningResponse struct {
	OK       bool                 `json:"ok"`
	Message  string               `json:"message,omitempty"`
	Set      packOpeningSetOption `json:"set"`
	PackType packOpeningPackType  `json:"pack_type"`
	Cards    []packOpeningCard    `json:"cards"`
}

type packSlot struct {
	Label          string
	Bucket         string
	Weighted       []weightedPackBucket
	FinishWeighted []weightedPackFinish
}

type weightedPackFinish struct {
	finish string
	weight int
}

type packOpeningSetConfig struct {
	SourceURL       string
	RelatedSetCodes []string
	Packs           map[string]packOpeningConfiguredPack
}

type packOpeningConfiguredPack struct {
	PackArtURI      string
	PackArtSize     string
	PackArtPosition string
	SourceURL       string
	CardCount       int
	Slots           []packSlot
	RequiredPools   map[string]int
	ExpectedPools   map[string]int
	Accuracy        string
	AccuracySummary string
	Limitations     []string
}

const (
	packOpeningAccuracySourced          = "sourced-recipe"
	packOpeningAccuracySourcedLabel     = "Published slot-rate approximation"
	packOpeningAccuracyStructure        = "structure-approximation"
	packOpeningAccuracyStructureLabel   = "Published structure approximation"
	packOpeningAccuracyBasicRarity      = "basic-rarity"
	packOpeningAccuracyBasicRarityLabel = "Basic rarity approximation"
)

type packOpeningPublishedProduct struct {
	Accuracy             string
	AccuracySummary      string
	Limitations          []string
	BasicRarity          bool
	HardenCollectorPools bool
}

// A set can have multiple configured recipes while only a subset has completed
// the stricter launch audit. Keep exposure product-specific so adding a Play
// Booster never accidentally publishes its Collector Booster recipe.
var packOpeningLaunchProducts = map[string]map[string]packOpeningPublishedProduct{
	"hob": {
		"play": {
			Accuracy:        packOpeningAccuracySourced,
			AccuracySummary: "Uses Wizards' published Play Booster slot rates with collector-number-scoped main-set and Booster Fun pools.",
			Limitations: []string{
				"Rates Wizards publishes only as less than 1% use the remaining published probability, distributed by the number of eligible cards.",
				"Factory print-sheet sequencing and sealed-product correlations are not modeled.",
			},
		},
		"collector": {
			Accuracy:        packOpeningAccuracyStructure,
			AccuracySummary: "Uses Wizards' published Collector Booster slots and appearance rates with collector-number-scoped treatment pools.",
			Limitations: []string{
				"The approximately 500-copy gleaming gold headliner is omitted because Wizards does not publish the Collector Booster print run needed to derive a per-pack rate.",
				"Factory print-sheet sequencing and sealed-product correlations are not modeled.",
			},
		},
	},
	"ecl": {"play": packOpeningSourcedPublication()},
	"tla": {"play": packOpeningSourcedPublication()},
	"fin": {
		"play": packOpeningSourcedPublication(),
	},
	"spm": {
		"play": {
			Accuracy:        packOpeningAccuracySourced,
			AccuracySummary: "Uses Wizards' published Play Booster slot rates with release- and collector-number-scoped specialty pools.",
			Limitations: []string{
				"Rates Wizards publishes only as less than 1% use the remaining published probability, distributed conservatively across the listed treatments.",
				"Source-material slots currently draw from the 38 English MAR printings supplied by the bulk dataset; Wizards lists 40 possible cards.",
				"Factory print-sheet sequencing and sealed-product correlations are not modeled.",
			},
		},
		"collector": {
			Accuracy:        packOpeningAccuracyStructure,
			AccuracySummary: "Uses Wizards' published Collector Booster structure and known appearance rates with collector-number-scoped treatment pools.",
			Limitations: []string{
				"The cosmic foil headliner is omitted because Wizards describes it only as extremely rare and does not publish a per-pack rate.",
				"Sub-1% costume, Gauntlet, and scene-mythic rates use the remaining published probability after the disclosed exact rates.",
				"Source-material slots currently draw from the 38 English MAR printings supplied by the bulk dataset; Wizards lists 40 possible cards.",
				"Factory print-sheet sequencing and sealed-product correlations are not modeled.",
			},
		},
	},
	"eoe": {"play": packOpeningBasicRarityPublication()},
	"fdn": {"play": packOpeningBasicRarityPublication()},
	"dsk": {
		"play": {
			Accuracy:        packOpeningAccuracyStructure,
			AccuracySummary: "Uses the published Play Booster structure and eligible Duskmourn treatment pools, with rounded appearance rates.",
			Limitations: []string{
				"Rounded treatment rates are modeled independently rather than from factory print sheets.",
			},
		},
	},
	"blb": {"play": packOpeningBasicRarityPublication()},
	"woe": {"collector": packOpeningCollectorStructurePublication()},
	"one": {"collector": packOpeningCollectorStructurePublication()},
	"bro": {"collector": packOpeningCollectorStructurePublication()},
}

var packOpeningPackTypes = []packOpeningPackType{
	{
		ID:          "play",
		Name:        "Play Booster",
		Description: "Sourced modern booster structure with commons, uncommons, wildcard, rare slot, foil slot, and land.",
		CardCount:   14,
	},
	{
		ID:          "draft",
		Name:        "Draft Booster",
		Description: "Classic 15-card draft-style spread with commons, uncommons, a rare slot, and land.",
		CardCount:   15,
	},
	{
		ID:          "set",
		Name:        "Set Booster",
		Description: "A swingier opening with more wildcard moments and multiple rare chances.",
		CardCount:   12,
	},
	{
		ID:          "collector",
		Name:        "Collector Booster",
		Description: "A premium-feeling pack with foil-style slots, wildcard rares, and splashy pulls.",
		CardCount:   15,
	},
}

func packOpeningBasicRarityPublication() packOpeningPublishedProduct {
	return packOpeningPublishedProduct{
		Accuracy:        packOpeningAccuracyBasicRarity,
		AccuracySummary: "Uses a deliberately simplified rarity spread drawn only from the set's ordinary booster-eligible card pools.",
		Limitations: []string{
			"Does not reproduce foil, treatment, replacement, or supplemental-set slots.",
			"The dedicated land slot is represented by an additional ordinary common to avoid admitting product-exclusive or nonbasic lands.",
			"Rarity weights are representative rather than product-specific.",
		},
		BasicRarity: true,
	}
}

func packOpeningSourcedPublication() packOpeningPublishedProduct {
	return packOpeningPublishedProduct{
		Accuracy:        packOpeningAccuracySourced,
		AccuracySummary: "Uses an official Wizards product breakdown for slot structure and published appearance rates.",
		Limitations: []string{
			"Factory print-sheet sequencing and sealed-product correlations are not modeled.",
		},
	}
}

func packOpeningCollectorStructurePublication() packOpeningPublishedProduct {
	return packOpeningPublishedProduct{
		Accuracy:        packOpeningAccuracyStructure,
		AccuracySummary: "Uses the published Collector Booster slot structure with conservative, treatment-specific eligible pools.",
		Limitations: []string{
			"Factory print-sheet sequencing and conditional treatment correlations are not modeled.",
			"Language-specific collation is represented by the English card pool.",
		},
		HardenCollectorPools: true,
	}
}

func packOpeningAccuracyLabel(accuracy string) string {
	switch strings.ToLower(strings.TrimSpace(accuracy)) {
	case packOpeningAccuracyStructure:
		return packOpeningAccuracyStructureLabel
	case packOpeningAccuracyBasicRarity:
		return packOpeningAccuracyBasicRarityLabel
	default:
		return packOpeningAccuracySourcedLabel
	}
}

func packOpeningAccuracySummary(accuracy string) string {
	switch strings.ToLower(strings.TrimSpace(accuracy)) {
	case packOpeningAccuracyStructure:
		return "Uses the published product structure, with some appearance rates simplified where exact collation is unavailable."
	case packOpeningAccuracyBasicRarity:
		return "Uses a deliberately simplified rarity spread drawn only from the set's ordinary booster-eligible card pools."
	default:
		return "Uses an official product breakdown for slot structure and published appearance rates."
	}
}

func packOpeningBasicRarityPack(base packOpeningConfiguredPack, setCode, packTypeID string) packOpeningConfiguredPack {
	setCode = strings.ToLower(strings.TrimSpace(setCode))
	common := "set:" + setCode + ":default:common"
	uncommon := "set:" + setCode + ":default:uncommon"
	rareOrMythic := wb("set:"+setCode+":default:rare", 875, "set:"+setCode+":default:mythic", 125)
	wildcard := wb(common, 400, uncommon, 350, "set:"+setCode+":default:rare", 200, "set:"+setCode+":default:mythic", 50)
	var slots []packSlot
	switch strings.ToLower(strings.TrimSpace(packTypeID)) {
	case "draft":
		slots = appendSlots(nil,
			repeatPackSlot(10, "Common", common),
			repeatPackSlot(3, "Uncommon", uncommon),
			[]packSlot{
				{Label: "Rare or Mythic", Weighted: rareOrMythic},
				{Label: "Additional Common", Bucket: common},
			},
		)
	case "set":
		slots = appendSlots(nil,
			repeatPackSlot(4, "Common", common),
			repeatPackSlot(3, "Uncommon", uncommon),
			repeatPackSlot(3, "Rarity Wildcard", ""),
			[]packSlot{
				{Label: "Rare or Mythic", Weighted: rareOrMythic},
				{Label: "Additional Common", Bucket: common},
			},
		)
		for index := range slots {
			if slots[index].Label == "Rarity Wildcard" {
				slots[index].Weighted = wildcard
			}
		}
	default:
		slots = appendSlots(nil,
			repeatPackSlot(7, "Common", common),
			repeatPackSlot(3, "Uncommon", uncommon),
			[]packSlot{
				{Label: "Rarity Wildcard", Weighted: wildcard},
				{Label: "Rarity Wildcard", Weighted: wildcard},
				{Label: "Rare or Mythic", Weighted: rareOrMythic},
				{Label: "Additional Common", Bucket: common},
			},
		)
	}

	base.CardCount = len(slots)
	base.Slots = slots
	base.RequiredPools = nil
	base.ExpectedPools = nil
	return base
}

func packOpeningPublishedProductFor(setCode, packTypeID string) (packOpeningPublishedProduct, bool) {
	products := packOpeningLaunchProducts[strings.ToLower(strings.TrimSpace(setCode))]
	publication, ok := products[strings.ToLower(strings.TrimSpace(packTypeID))]
	return publication, ok
}

func packOpeningPublishedPackConfig(setCode, packTypeID string, base packOpeningConfiguredPack) packOpeningConfiguredPack {
	publication, ok := packOpeningPublishedProductFor(setCode, packTypeID)
	if !ok {
		return base
	}
	if publication.BasicRarity {
		base = packOpeningBasicRarityPack(base, setCode, packTypeID)
	}
	if publication.HardenCollectorPools {
		base = packOpeningConservativeCollectorPack(base, setCode)
	}
	base.Accuracy = publication.Accuracy
	base.AccuracySummary = publication.AccuracySummary
	base.Limitations = append([]string{}, publication.Limitations...)
	return base
}

func packOpeningConservativeCollectorPack(base packOpeningConfiguredPack, setCode string) packOpeningConfiguredPack {
	setCode = strings.ToLower(strings.TrimSpace(setCode))
	landLabel := map[string]string{
		"woe": "Traditional Foil Full-art Basic Land",
		"one": "Traditional Foil Panorama or Phyrexianized Land",
		"bro": "Traditional Foil Mech Land",
	}[setCode]
	for index := range base.Slots {
		slot := &base.Slots[index]
		switch slot.Label {
		case "Traditional Foil Common":
			slot.Bucket = "set:" + setCode + ":foil_default:common"
			slot.Weighted = nil
		case "Traditional Foil Uncommon":
			slot.Bucket = "set:" + setCode + ":foil_default:uncommon"
			slot.Weighted = nil
		default:
			if landLabel != "" && slot.Label == landLabel {
				slot.Bucket = "set:" + setCode + ":fullart:common"
				slot.Weighted = nil
			}
		}
	}
	return base
}

const (
	packOpeningSourceHobbit            = "https://magic.wizards.com/en/news/feature/collecting-the-hobbit"
	packOpeningSourceMarvelSuperHeroes = "https://magic.wizards.com/en/news/feature/collecting-marvel-super-heroes"
	packOpeningSourceSecretsStrixhaven = "https://magic.wizards.com/en/news/feature/collecting-secrets-of-strixhaven"
	packOpeningSourceTMNT              = "https://magic.wizards.com/en/news/feature/collecting-teenage-mutant-ninja-turtles"
	packOpeningSourceLorwynEclipsed    = "https://magic.wizards.com/en/news/feature/collecting-lorwyn-eclipsed"
	packOpeningSourceAvatar            = "https://magic.wizards.com/en/news/feature/collecting-avatar-the-last-airbender"
	packOpeningSourceSpiderMan         = "https://magic.wizards.com/en/news/feature/collecting-marvels-spider-man"
	packOpeningSourceEdgeOfEternities  = "https://magic.wizards.com/en/news/feature/collecting-edge-of-eternities"
	packOpeningSourceFinalFantasy      = "https://magic.wizards.com/en/news/feature/collecting-final-fantasy"
	packOpeningSourceInnistradRemaster = "https://magic.wizards.com/en/news/feature/collecting-innistrad-remastered"
	packOpeningSourceFoundations       = "https://magic.wizards.com/en/news/feature/collecting-foundations"
	packOpeningSourceTarkirDragonstorm = "https://magic.wizards.com/en/news/feature/collecting-tarkir-dragonstorm"
	packOpeningSourceAetherdrift       = "https://magic.wizards.com/en/news/feature/collecting-aetherdrift"
	packOpeningSourceDuskmourn         = "https://magic.wizards.com/en/news/feature/collecting-duskmourn"
	packOpeningSourceBloomburrow       = "https://magic.wizards.com/en/news/feature/collecting-bloomburrow"
	packOpeningSourceOutlaws           = "https://magic.wizards.com/en/news/feature/collecting-outlaws-of-thunder-junction"
	packOpeningSourceKarlovManor       = "https://magic.wizards.com/en/news/feature/collecting-murders-at-karlov-manor"
	packOpeningSourceModernHorizons3   = "https://magic.wizards.com/en/news/feature/collecting-modern-horizons-3"
	packOpeningSourceLostCaverns       = "https://magic.wizards.com/en/news/feature/collecting-the-lost-caverns-of-ixalan"
	packOpeningSourceWildsEldraine     = "https://magic.wizards.com/en/news/feature/collecting-wilds-of-eldraine"
	packOpeningSourceCommanderMasters  = "https://magic.wizards.com/en/news/feature/collecting-commander-masters"
	packOpeningSourceLordOfTheRings    = "https://magic.wizards.com/en/news/feature/collecting-the-lord-of-the-rings-tales-of-middle-earth"
	packOpeningSourceMarchMachine      = "https://magic.wizards.com/en/news/feature/collecting-march-of-the-machine"
	packOpeningSourceAllWillBeOne      = "https://magic.wizards.com/en/news/feature/collecting-phyrexia-all-will-be-one"
	packOpeningSourceBrothersWar       = "https://magic.wizards.com/en/news/feature/whats-inside-the-brothers-war-boosters"
)

func packOpeningModernPlayBoosterConfig(sourceURL, packArtURI, setCode string, commonReplacement []weightedPackBucket, wildcard []weightedPackBucket, rareSlot []weightedPackBucket, foilSlot []weightedPackBucket, landSlot []weightedPackBucket) packOpeningConfiguredPack {
	commonBucket := "set:" + setCode + ":default:common"
	uncommonBucket := "set:" + setCode + ":default:uncommon"
	commonReplacementLabel := "Common / Guest"
	if len(commonReplacement) == 0 {
		commonReplacement = wb(commonBucket, 1000)
		commonReplacementLabel = "Common"
	}
	if len(wildcard) == 0 {
		wildcard = wb(commonBucket, 417, uncommonBucket, 333, "set:"+setCode+":default:rare", 200, "set:"+setCode+":default:mythic", 50)
	}
	if len(rareSlot) == 0 {
		rareSlot = wb("set:"+setCode+":default:rare", 875, "set:"+setCode+":default:mythic", 125)
	}
	if len(foilSlot) == 0 {
		foilSlot = wb("set:"+setCode+":foil_default:common", 580, "set:"+setCode+":foil_default:uncommon", 320, "set:"+setCode+":foil_default:rare", 80, "set:"+setCode+":foil_default:mythic", 20)
	}
	if len(landSlot) == 0 {
		landSlot = wb("set:"+setCode+":default:land", 800, "set:"+setCode+":fullart:land", 200)
	}
	return packOpeningConfiguredPack{
		SourceURL:       sourceURL,
		PackArtURI:      packArtURI,
		PackArtSize:     "cover",
		PackArtPosition: "center",
		CardCount:       14,
		Slots: appendSlots(nil,
			repeatPackSlot(6, "Common", commonBucket),
			[]packSlot{{Label: commonReplacementLabel, Weighted: commonReplacement}},
			repeatPackSlot(3, "Uncommon", uncommonBucket),
			[]packSlot{
				{Label: "Wildcard", Weighted: wildcard},
				{Label: "Rare or Mythic", Weighted: rareSlot},
				{Label: "Traditional Foil", Weighted: foilSlot},
				{Label: "Land", Weighted: landSlot},
			},
		),
	}
}

func packOpeningCollectorBoosterConfig(sourceURL, packArtURI, setCode string, commonCount int, uncommonCount int, slots []packSlot) packOpeningConfiguredPack {
	commonBucket := "set:" + setCode + ":foil:common"
	uncommonBucket := "set:" + setCode + ":foil:uncommon"
	return packOpeningConfiguredPack{
		SourceURL:       sourceURL,
		PackArtURI:      packArtURI,
		PackArtSize:     "cover",
		PackArtPosition: "center",
		CardCount:       commonCount + uncommonCount + len(slots),
		Slots: appendSlots(nil,
			repeatPackSlot(commonCount, "Traditional Foil Common", commonBucket),
			repeatPackSlot(uncommonCount, "Traditional Foil Uncommon", uncommonBucket),
			slots,
		),
	}
}

func packOpeningWithRequiredPools(config packOpeningConfiguredPack, required map[string]int) packOpeningConfiguredPack {
	config.RequiredPools = required
	return config
}

func packOpeningConfigForSet(setCode, setType, releasedAt string, cardCount int) (packOpeningSetConfig, bool) {
	setCode = strings.ToLower(strings.TrimSpace(setCode))
	if len(packOpeningLaunchProducts[setCode]) == 0 {
		return packOpeningSetConfig{}, false
	}
	config, ok := packOpeningSetConfigs[setCode]
	if !ok {
		return packOpeningSetConfig{}, false
	}
	config.Packs = clonePackOpeningPacks(config.Packs)
	for packTypeID := range packOpeningLaunchProducts[setCode] {
		base, exists := config.Packs[packTypeID]
		if !exists {
			continue
		}
		config.Packs[packTypeID] = packOpeningPublishedPackConfig(setCode, packTypeID, base)
	}
	return config, true
}

func clonePackOpeningPacks(source map[string]packOpeningConfiguredPack) map[string]packOpeningConfiguredPack {
	cloned := make(map[string]packOpeningConfiguredPack, len(source))
	for id, config := range source {
		config.Slots = append([]packSlot{}, config.Slots...)
		config.Limitations = append([]string{}, config.Limitations...)
		cloned[id] = config
	}
	return cloned
}

var packOpeningSetConfigs = map[string]packOpeningSetConfig{
	"hob": {
		SourceURL:       packOpeningSourceHobbit,
		RelatedSetCodes: []string{"hoc"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:       packOpeningSourceHobbit,
				PackArtURI:      "https://media.wizards.com/2026/images/daily/8Bmxd4HQE3/en_0dIZenqjm9.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       14,
				Slots: appendSlots(nil,
					repeatWeightedPackSlot(7, "Common", wb("set:hob:main:common", 922, "set:hob:scene:common", 78)),
					repeatWeightedPackSlot(3, "Uncommon", wb("set:hob:main:uncommon", 836, "set:hob:scene:uncommon", 55, "set:hob:dragonhoard:uncommon", 109)),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb(
							"set:hob:main:common", 74170,
							"set:hob:main:uncommon", 3900,
							"set:hob:main:rare", 17300,
							"set:hob:main:mythic", 2300,
							"set:hob:scene:common", 93,
							"set:hob:scene:uncommon", 186,
							"set:hob:scene:rare", 419,
							"set:hob:dragonhoard:uncommon", 280,
							"set:hob:dragonhoard:rare", 513,
							"set:hob:dragonhoard:mythic", 373,
							"set:hob:bookcover:rare", 186,
							"set:hob:bookcover:mythic", 280,
						)},
						{Label: "Rare or Mythic", Weighted: wb(
							"set:hob:main:rare", 8290,
							"set:hob:main:mythic", 1110,
							"set:hob:scene:rare", 180,
							"set:hob:dragonhoard:rare", 230,
							"set:hob:dragonhoard:mythic", 84,
							"set:hob:bookcover:rare", 42,
							"set:hob:bookcover:mythic", 64,
						)},
						{Label: "Traditional Foil", Weighted: wb(
							"set:hob:main:common", 5980,
							"set:hob:main:uncommon", 2930,
							"set:hob:main:rare", 710,
							"set:hob:main:mythic", 100,
							"set:hob:scene:common", 11,
							"set:hob:scene:uncommon", 22,
							"set:hob:scene:rare", 50,
							"set:hob:dragonhoard:uncommon", 34,
							"set:hob:dragonhoard:rare", 62,
							"set:hob:dragonhoard:mythic", 45,
							"set:hob:bookcover:rare", 22,
							"set:hob:bookcover:mythic", 34,
						)},
						{Label: "Land", Weighted: wb(
							"set:hob:nonfoil_default_basic:land", 267,
							"set:hob:foil_default_basic:land", 67,
							"set:hob:nonfoil_journey:land", 133,
							"set:hob:foil_journey:land", 33,
							"set:hob:nonfoil_play_dual:land", 400,
							"set:hob:foil_play_dual:land", 100,
						)},
					},
				),
				ExpectedPools: map[string]int{
					"set:hob:main:common":          60,
					"set:hob:main:uncommon":        55,
					"set:hob:main:rare":            53,
					"set:hob:main:mythic":          15,
					"set:hob:scene:common":         2,
					"set:hob:scene:uncommon":       4,
					"set:hob:scene:rare":           9,
					"set:hob:dragonhoard:uncommon": 6,
					"set:hob:dragonhoard:rare":     11,
					"set:hob:dragonhoard:mythic":   8,
					"set:hob:bookcover:rare":       4,
					"set:hob:bookcover:mythic":     6,
					"set:hob:play_dual:land":       5,
					"set:hob:default_basic:land":   5,
					"set:hob:journey:land":         5,
				},
			},
			"collector": {
				SourceURL:       packOpeningSourceHobbit,
				PackArtURI:      "https://media.wizards.com/2026/images/daily/8Bmxd4HQE3/en_GG8zCyKg86.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatWeightedPackSlot(5, "Traditional Foil Common", wb("set:hob:main:common", 896, "set:hob:play_dual:land", 75, "set:hob:scene:common", 30)),
					[]packSlot{{Label: "Foil Uncommon", Weighted: wb("set:hob:main:uncommon", 761, "set:hob:scene:uncommon", 56, "set:hob:dragonhoard:uncommon", 83, "set:hob:surgefoil_dragonhoard:uncommon", 100)}},
					repeatWeightedPackSlot(3, "Foil Uncommon", wb("set:hob:main:uncommon", 846, "set:hob:scene:uncommon", 62, "set:hob:dragonhoard:uncommon", 92)),
					[]packSlot{
						{Label: "Traditional Foil Middle-earth Journey Basic Land", Bucket: "set:hob:journey:land"},
					},
					repeatWeightedPackSlot(2, "Traditional Foil Rare or Mythic", wb("set:hob:main:rare", 876, "set:hob:main:mythic", 124)),
					repeatWeightedPackSlot(2, "Non-foil Booster Fun", wb(
						"set:hob:scene:rare", 101,
						"set:hoc:scene:rare", 142,
						"set:hob:dragonhoard:rare", 131,
						"set:hob:dragonhoard:mythic", 45,
						"set:hob:bookcover:rare", 42,
						"set:hob:bookcover:mythic", 33,
						"set:hoc:classicartist:mythic", 119,
						"set:hoc:dwarvish:mythic", 15,
						"set:hob:extendedart_main:rare", 309,
						"set:hob:extendedart_main:mythic", 12,
						"set:hoc:extendedart_main:mythic", 53,
					)),
					[]packSlot{{Label: "Foil Booster Fun", Weighted: wb(
						"set:hob:scene:rare", 110,
						"set:hob:dragonhoard:rare", 143,
						"set:hob:dragonhoard:mythic", 49,
						"set:hob:surgefoil_dragonhoard:rare", 65,
						"set:hob:surgefoil_dragonhoard:mythic", 24,
						"set:hob:bookcover:rare", 46,
						"set:hob:bookcover:mythic", 36,
						"set:hob:surgefoil_bookcover:rare", 24,
						"set:hob:surgefoil_bookcover:mythic", 18,
						"set:hoc:surgefoil_classicartist:mythic", 119,
						"set:hoc:dwarvish:mythic", 16,
						"set:hob:extendedart_main:rare", 338,
						"set:hob:extendedart_main:mythic", 13,
					)}},
				),
				ExpectedPools: map[string]int{
					"set:hob:main:common":                    60,
					"set:hob:main:uncommon":                  55,
					"set:hob:main:rare":                      53,
					"set:hob:main:mythic":                    15,
					"set:hob:scene:common":                   2,
					"set:hob:scene:uncommon":                 4,
					"set:hob:scene:rare":                     9,
					"set:hob:dragonhoard:uncommon":           6,
					"set:hob:dragonhoard:rare":               11,
					"set:hob:dragonhoard:mythic":             8,
					"set:hob:surgefoil_dragonhoard:uncommon": 6,
					"set:hob:surgefoil_dragonhoard:rare":     11,
					"set:hob:surgefoil_dragonhoard:mythic":   8,
					"set:hob:bookcover:rare":                 4,
					"set:hob:bookcover:mythic":               6,
					"set:hob:surgefoil_bookcover:rare":       4,
					"set:hob:surgefoil_bookcover:mythic":     6,
					"set:hob:play_dual:land":                 5,
					"set:hob:journey:land":                   5,
					"set:hob:extendedart_main:rare":          26,
					"set:hob:extendedart_main:mythic":        2,
					"set:hoc:scene:rare":                     12,
					"set:hoc:classicartist:mythic":           40,
					"set:hoc:surgefoil_classicartist:mythic": 40,
					"set:hoc:dwarvish:mythic":                5,
					"set:hoc:extendedart_main:mythic":        9,
				},
			},
		},
	},
	"msh": {
		SourceURL:       packOpeningSourceMarvelSuperHeroes,
		RelatedSetCodes: []string{"mar", "msc"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:  packOpeningSourceMarvelSuperHeroes,
				PackArtURI: "https://media.wizards.com/2026/images/daily/MjzNgmIVwR/en_ogkuoLmJyc.webp",
				CardCount:  14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:msh:common"),
					[]packSlot{{Label: "Common / Source Material", Weighted: wb("set:msh:common", 2300, "set:mar:release:2026-06-26", 100)}},
					repeatPackSlot(3, "Uncommon", "set:msh:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb("set:msh:common", 124, "set:msh:uncommon", 667, "set:msh:rare", 188, "set:msh:mythic", 21)},
						{Label: "Rare or Mythic", Weighted: wb("set:msh:rare", 814, "set:msh:mythic", 186)},
						{Label: "Traditional Foil", Weighted: wb("set:msh:common", 599, "set:msh:uncommon", 308, "set:msh:rare", 80, "set:msh:mythic", 13)},
						{Label: "Land", Bucket: "set:msh:land"},
					},
				),
				RequiredPools: map[string]int{"set:mar:release:2026-06-26": 60},
			},
			"collector": {
				SourceURL:  packOpeningSourceMarvelSuperHeroes,
				PackArtURI: "https://media.wizards.com/2026/images/daily/MjzNgmIVwR/en_72Lv6DECMM.webp",
				CardCount:  15,
				Slots: appendSlots(nil,
					repeatPackSlot(3, "Traditional Foil Common", "set:msh:common"),
					repeatPackSlot(2, "Traditional Foil Uncommon", "set:msh:uncommon"),
					repeatPackSlot(2, "Traditional Foil MSC Common", "set:msc:common"),
					[]packSlot{
						{Label: "Traditional Foil MSC Uncommon", Bucket: "set:msc:uncommon"},
						{Label: "Foil Scene Common / Uncommon", Weighted: wb("set:msh:common", 200, "set:msh:uncommon", 800)},
						{Label: "Foil City Land", Bucket: "set:msh:land"},
						{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:msh:rare", 450, "set:msh:mythic", 94, "set:msc:rare", 368, "set:msc:mythic", 60)},
						{Label: "Commander Booster Fun", Weighted: wb("set:msc:rare", 936, "set:msc:mythic", 64)},
						{Label: "Non-foil Booster Fun", Weighted: wb("set:msh:rare", 811, "set:msh:mythic", 189)},
						{Label: "Source Material", Bucket: "set:mar:release:2026-06-26", FinishWeighted: wf("nonfoil", 75, "foil", 25)},
						{Label: "Foil Booster Fun", Weighted: wb("set:msh:rare", 901, "set:msh:mythic", 99)},
					},
				),
				RequiredPools: map[string]int{"set:mar:release:2026-06-26": 60},
			},
		},
	},
	"sos": {
		SourceURL:       packOpeningSourceSecretsStrixhaven,
		RelatedSetCodes: []string{"soa", "soc", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:       packOpeningSourceSecretsStrixhaven,
				PackArtURI:      "https://media.wizards.com/2026/images/daily/jUGJyjn33x/en_vpHqsMAObL.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       14,
				Slots: appendSlots(nil,
					repeatPackSlot(5, "Common", "set:sos:default:common"),
					[]packSlot{{Label: "Common / Special Guest", Weighted: wb("set:sos:default:common", 982, "set:spg:release:2026-04-24:mythic", 18)}},
					repeatPackSlot(3, "Uncommon", "set:sos:default:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb("set:sos:default:common", 391, "set:sos:default:uncommon", 391, "set:sos:default:rare", 195, "set:sos:default:mythic", 19, "set:sos:boosterfun:rare_or_mythic", 4)},
						{Label: "Rare or Mythic", Weighted: wb("set:sos:default:rare", 825, "set:sos:default:mythic", 141, "set:sos:boosterfun:rare", 24, "set:sos:boosterfun:mythic", 10)},
						{Label: "Mystical Archive", Weighted: wb("set:soa:uncommon", 875, "set:soa:rare", 96, "set:soa:mythic", 29)},
						{Label: "Traditional Foil", Weighted: wb("set:sos:foil_default:common", 544, "set:sos:foil_default:uncommon", 336, "set:sos:foil_default:rare", 67, "set:sos:foil_default:mythic", 11, "set:sos:foil_boosterfun:rare_or_mythic", 14, "set:soa:foil:uncommon", 28)},
						{Label: "Land", Bucket: "set:sos:land"},
					},
				),
				RequiredPools: map[string]int{"set:spg:release:2026-04-24:mythic": 10},
			},
			"collector": {
				SourceURL:       packOpeningSourceSecretsStrixhaven,
				PackArtURI:      "https://media.wizards.com/2026/images/daily/jUGJyjn33x/en_x3WCBdzNAo.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(4, "Traditional Foil Common", "set:sos:foil:common"),
					repeatPackSlot(3, "Traditional Foil Uncommon", "set:sos:foil:uncommon"),
					repeatPackSlot(2, "Uncommon Mystical Archive", "set:soa:uncommon"),
					[]packSlot{
						{Label: "Traditional Foil Spellcraft Land", Bucket: "set:sos:foil_boosterfun:land"},
						{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:sos:foil_default:rare", 857, "set:sos:foil_default:mythic", 143)},
						{Label: "Secrets of Strixhaven Commander", Weighted: wb("set:soc:extendedart:rare", 910, "set:soc:borderless:mythic", 90)},
						{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:sos:extendedart:rare", 700, "set:sos:extendedart:mythic", 50, "set:sos:borderless:rare", 71, "set:sos:borderless:mythic", 50, "set:sos:showcase:rare", 86, "set:sos:showcase:mythic", 43)},
						{Label: "Rare or Mythic Mystical Archive", Weighted: wb("set:soa:rare", 768, "set:soa:mythic", 232)},
						{Label: "Foil Booster Fun Rare or Mythic", Weighted: wb("set:sos:foil_boosterfun:rare", 733, "set:sos:foil_boosterfun:mythic", 222, "set:spg:release:2026-04-24:mythic", 45)},
					},
				),
				RequiredPools: map[string]int{"set:spg:release:2026-04-24:mythic": 10},
			},
		},
	},
	"tmt": {
		SourceURL:       packOpeningSourceTMNT,
		RelatedSetCodes: []string{"tmc", "pza"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:       packOpeningSourceTMNT,
				PackArtURI:      "https://media.wizards.com/2025/images/daily/rJCJQBLkFD/en_Pj04YTRZxP.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:tmt:default:common"),
					[]packSlot{{Label: "Common / Source Material", Weighted: wb("set:tmt:default:common", 27, "set:pza", 1)}},
					repeatPackSlot(2, "Uncommon", "set:tmt:default:uncommon"),
					[]packSlot{
						{Label: "Legendary Turtle", Weighted: wb("set:tmt:default:turtle:common", 641, "set:tmt:default:turtle:uncommon", 196, "set:tmt:default:turtle:rare", 79, "set:tmt:default:turtle:mythic", 34, "set:tmt:boosterfun:turtle", 50)},
						{Label: "Wildcard", Weighted: wb("set:tmt:default:common", 249, "set:tmt:default:uncommon", 621, "set:tmt:default:rare", 76, "set:tmt:default:mythic", 8, "set:tmt:boosterfun:common_or_uncommon", 39, "set:tmt:boosterfun:rare_or_mythic", 7)},
						{Label: "Rare or Mythic", Weighted: wb("set:tmt:default:rare", 849, "set:tmt:default:mythic", 92, "set:tmt:boosterfun:rare", 49, "set:tmt:boosterfun:mythic", 10)},
						{Label: "Traditional Foil", Weighted: wb("set:tmt:foil_default:common", 615, "set:tmt:foil_default:uncommon", 277, "set:tmt:foil_default:rare", 67, "set:tmt:foil_default:mythic", 8, "set:tmt:foil_boosterfun:common_or_uncommon", 23, "set:tmt:foil_boosterfun:rare_or_mythic", 10)},
						{Label: "Land", Weighted: wb(
							"set:tmt:nonfoil_play_dual:land", 48,
							"set:tmt:foil_play_dual:land", 12,
							"set:tmt:nonfoil_rooftop:land", 32,
							"set:tmt:foil_rooftop:land", 8,
						)},
					},
				),
				RequiredPools: map[string]int{"set:pza": 20},
			},
			"collector": {
				SourceURL:       packOpeningSourceTMNT,
				PackArtURI:      "https://media.wizards.com/2025/images/daily/rJCJQBLkFD/en_0AtH45UfuN.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(5, "Traditional Foil Common", "set:tmt:foil:common"),
					repeatPackSlot(3, "Traditional Foil Uncommon", "set:tmt:foil:uncommon"),
					[]packSlot{
						{Label: "Turtle Power! Reprint", Weighted: wb("set:tmc:nonfoil:common", 519, "set:tmc:surgefoil:common", 231, "set:tmc:nonfoil:uncommon", 173, "set:tmc:surgefoil:uncommon", 77)},
						{Label: "Traditional Foil or Surge Foil Land", Weighted: wb("set:tmt:foil:land", 667, "set:tmt:surgefoil:land", 333)},
						{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:tmt:foil_default:rare", 876, "set:tmt:foil_default:mythic", 124)},
						{Label: "Booster Fun or Turtle Power!", Weighted: wb("set:tmt:nonfoil_boosterfun:rare", 354, "set:tmt:nonfoil_boosterfun:mythic", 44, "set:tmt:extendedart:rare", 161, "set:tmc:nonfoil:rare", 452, "set:tmc:nonfoil:mythic", 22, "set:tmc:surgefoil:rare_or_mythic", 67)},
						{Label: "Booster Fun or Turtle Power!", Weighted: wb("set:tmt:nonfoil_boosterfun:rare", 354, "set:tmt:nonfoil_boosterfun:mythic", 44, "set:tmt:extendedart:rare", 161, "set:tmc:nonfoil:rare", 452, "set:tmc:nonfoil:mythic", 22, "set:tmc:surgefoil:rare_or_mythic", 67)},
						{Label: "Source Material", Bucket: "set:pza", FinishWeighted: wf("nonfoil", 75, "foil", 25)},
						{Label: "Foil Booster Fun Rare or Mythic", Weighted: wb("set:tmt:foil_boosterfun:rare", 792, "set:tmt:foil_boosterfun:mythic", 99, "set:tmt:fracturefoil:mythic", 9, "set:tmt:japanshowcase:rare_or_mythic", 90)},
					},
				),
				RequiredPools: map[string]int{"set:pza": 20},
			},
		},
	},
	"ecl": {
		SourceURL:       packOpeningSourceLorwynEclipsed,
		RelatedSetCodes: []string{"ecc", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:       packOpeningSourceLorwynEclipsed,
				PackArtURI:      "https://media.wizards.com/2025/images/daily/en_YaHTKsL20v.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:ecl:default:common"),
					[]packSlot{{Label: "Common / Special Guest", Weighted: wb("set:ecl:default:common", 982, "set:spg:release:2026-01-23:mythic", 18)}},
					repeatPackSlot(3, "Uncommon", "set:ecl:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb("set:ecl:default:common", 180, "set:ecl:default:uncommon", 580, "set:ecl:default:rare", 190, "set:ecl:default:mythic", 20, "set:ecl:boosterfun:uncommon", 20, "set:ecl:boosterfun:rare_or_mythic", 10)},
						{Label: "Rare or Mythic", Weighted: wb("set:ecl:default:rare", 782, "set:ecl:default:mythic", 136, "set:ecl:boosterfun:rare", 62, "set:ecl:boosterfun:mythic", 20)},
						{Label: "Traditional Foil", Weighted: wb("set:ecl:foil_default:common", 604, "set:ecl:foil_default:uncommon", 298, "set:ecl:foil_default:rare", 65, "set:ecl:foil_default:mythic", 11, "set:ecl:foil_boosterfun:uncommon", 10, "set:ecl:foil_boosterfun:rare_or_mythic", 12)},
						{Label: "Land", Weighted: wb("set:ecl:default:land", 500, "set:ecl:fullart:land", 500)},
					},
				),
				RequiredPools: map[string]int{"set:spg:release:2026-01-23:mythic": 20},
			},
			"collector": packOpeningWithRequiredPools(packOpeningCollectorBoosterConfig(
				packOpeningSourceLorwynEclipsed,
				"https://media.wizards.com/2025/images/daily/en_7uAapZkktd.webp",
				"ecl",
				5,
				4,
				[]packSlot{
					{Label: "Traditional Foil Full-art Basic Land", Bucket: "set:ecl:foil:land"},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:ecl:foil_default:rare", 855, "set:ecl:foil_default:mythic", 145)},
					{Label: "Lorwyn Eclipsed Commander", Weighted: wb("set:ecc:extendedart:rare", 910, "set:ecc:borderless:mythic", 90)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:ecl:extendedart:rare", 377, "set:ecl:showcase:rare", 337, "set:ecl:showcase:mythic", 91, "set:ecl:borderless:rare", 130, "set:ecl:borderless:mythic", 45, "set:ecl:borderless:land", 20)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:ecl:extendedart:rare", 377, "set:ecl:showcase:rare", 337, "set:ecl:showcase:mythic", 91, "set:ecl:borderless:rare", 130, "set:ecl:borderless:mythic", 45, "set:ecl:borderless:land", 20)},
					{Label: "Foil Booster Fun Rare or Mythic", Weighted: wb("set:ecl:foil_boosterfun:rare", 676, "set:ecl:foil_boosterfun:mythic", 167, "set:spg:release:2026-01-23:mythic", 105, "set:ecl:japanshowcase:rare_or_mythic", 90, "set:ecl:fracturefoil:rare_or_mythic", 10)},
				},
			), map[string]int{"set:spg:release:2026-01-23:mythic": 20}),
		},
	},
	"tla": {
		SourceURL:       packOpeningSourceAvatar,
		RelatedSetCodes: []string{"tle"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:       packOpeningSourceAvatar,
				PackArtURI:      "https://media.wizards.com/2025/images/daily/en_GGlVjEQHwR.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:tla:default:common"),
					[]packSlot{{Label: "Common / Source Material", Weighted: wb("set:tla:default:common", 25, "set:tle:sourcematerial", 1)}},
					repeatPackSlot(3, "Uncommon", "set:tla:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb("set:tla:default:common", 42, "set:tla:default:uncommon", 741, "set:tla:default:rare", 167, "set:tla:default:mythic", 26, "set:tla:boosterfun:rare_or_mythic", 24)},
						{Label: "Rare or Mythic", Weighted: wb("set:tla:default:rare", 800, "set:tla:default:mythic", 126, "set:tla:boosterfun:rare", 54, "set:tla:boosterfun:mythic", 20)},
						{Label: "Traditional Foil", Weighted: wb("set:tla:foil_default:common", 539, "set:tla:foil_default:uncommon", 367, "set:tla:foil_default:rare", 67, "set:tla:foil_default:mythic", 12, "set:tla:foil_boosterfun:rare_or_mythic", 15)},
						{Label: "Land", Weighted: wb("set:tla:default:land", 650, "set:tla:fullart:land", 350)},
					},
				),
				RequiredPools: map[string]int{"set:tle:sourcematerial": 61},
			},
			"collector": {
				SourceURL:       packOpeningSourceAvatar,
				PackArtURI:      "https://media.wizards.com/2025/images/daily/en_NsQoFYo4ht.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(3, "Traditional Foil Common", "set:tla:foil:common"),
					repeatPackSlot(3, "Traditional Foil Uncommon", "set:tla:foil:uncommon"),
					repeatPackSlot(2, "Traditional Foil TLE Common", "set:tle:foil:common"),
					[]packSlot{
						{Label: "Traditional Foil TLE or Booster Fun Uncommon", Weighted: wb("set:tle:foil:uncommon", 920, "set:tla:foil_boosterfun:uncommon", 80)},
						{Label: "Traditional Foil Full-art Land", Bucket: "set:tla:foil:land"},
						{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:tla:foil_default:rare", 857, "set:tla:foil_default:mythic", 143)},
						{Label: "TLE Rare or Mythic", Weighted: wb("set:tle:rare", 485, "set:tle:mythic", 191, "set:tle:extendedart:rare", 242, "set:tle:extendedart:mythic", 82)},
						{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:tla:nonfoil_boosterfun:rare", 806, "set:tla:nonfoil_boosterfun:mythic", 194)},
						{Label: "Source Material", Bucket: "set:tle:sourcematerial", FinishWeighted: wf("nonfoil", 75, "foil", 25)},
						{Label: "Foil Booster Fun Rare or Mythic", Weighted: wb("set:tla:foil_boosterfun:rare", 756, "set:tla:foil_boosterfun:mythic", 228)},
					},
				),
				RequiredPools: map[string]int{"set:tle:sourcematerial": 61},
			},
		},
	},
	"spm": {
		SourceURL:       packOpeningSourceSpiderMan,
		RelatedSetCodes: []string{"spe", "mar"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:       packOpeningSourceSpiderMan,
				PackArtURI:      "https://media.wizards.com/2025/images/daily/en_EC9qVSPee7.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:spm:main:common"),
					[]packSlot{{Label: "Common / Source Material", Weighted: wb("set:spm:main:common", 23, "set:mar:release:2025-09-26", 1)}},
					repeatPackSlot(3, "Uncommon", "set:spm:play_uncommon_with_boosterfun"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb(
							"set:spm:main:common", 708,
							"set:spm:main:uncommon", 41,
							"set:spm:main:rare", 209,
							"set:spm:main:mythic", 29,
							"set:spm:webslinger_boosterfun:rare", 3,
							"set:spm:webslinger_boosterfun:mythic", 1,
							"set:spm:panel_boosterfun:rare", 4,
							"set:spm:panel_boosterfun:mythic", 1,
							"set:spm:scene_boosterfun:uncommon", 1,
							"set:spm:scene_boosterfun:rare", 2,
							"set:spm:scene_boosterfun:mythic", 1,
						)},
						{Label: "Rare or Mythic", Weighted: wb(
							"set:spm:main:rare", 837,
							"set:spm:main:mythic", 117,
							"set:spm:webslinger_boosterfun:rare", 12,
							"set:spm:webslinger_boosterfun:mythic", 1,
							"set:spm:panel_boosterfun:rare", 21,
							"set:spm:panel_boosterfun:mythic", 1,
							"set:spm:scene_boosterfun:rare", 10,
							"set:spm:scene_boosterfun:mythic", 1,
						)},
						{Label: "Traditional Foil", Weighted: wb(
							"set:spm:main:common", 658,
							"set:spm:main:uncommon", 241,
							"set:spm:main:rare", 78,
							"set:spm:main:mythic", 11,
							"set:spm:webslinger_boosterfun:rare", 3,
							"set:spm:webslinger_boosterfun:mythic", 1,
							"set:spm:panel_boosterfun:rare", 4,
							"set:spm:panel_boosterfun:mythic", 1,
							"set:spm:scene_boosterfun:uncommon", 1,
							"set:spm:scene_boosterfun:rare", 1,
							"set:spm:scene_boosterfun:mythic", 1,
						)},
						{Label: "Land", Weighted: wb("set:spm:play_dual:land", 500, "set:spm:spiderweb:land", 250, "set:spm:default_basic:land", 250), FinishWeighted: wf("nonfoil", 80, "foil", 20)},
					},
				),
				ExpectedPools: map[string]int{
					"set:spm:main:common":                   60,
					"set:spm:main:uncommon":                 55,
					"set:spm:main:rare":                     53,
					"set:spm:main:mythic":                   15,
					"set:spm:play_uncommon_with_boosterfun": 58,
					"set:spm:scene_boosterfun:uncommon":     3,
					"set:spm:scene_boosterfun:rare":         4,
					"set:spm:scene_boosterfun:mythic":       2,
					"set:spm:webslinger_boosterfun:rare":    7,
					"set:spm:webslinger_boosterfun:mythic":  3,
					"set:spm:panel_boosterfun:rare":         10,
					"set:spm:panel_boosterfun:mythic":       4,
					"set:spm:play_dual:land":                5,
					"set:spm:spiderweb:land":                5,
					"set:spm:default_basic:land":            5,
					"set:mar:release:2025-09-26":            38,
				},
			},
			"collector": {
				SourceURL:       packOpeningSourceSpiderMan,
				PackArtURI:      "https://media.wizards.com/2025/images/daily/en_lFtxMgGFNV.webp",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatWeightedPackSlot(5, "Traditional Foil Common", wb("set:spm:main:common", 858, "set:spm:play_dual:land", 71, "set:spe:welcome:common", 71)),
					repeatWeightedPackSlot(4, "Traditional Foil Uncommon", wb("set:spm:main:uncommon", 873, "set:spe:welcome:uncommon", 79, "set:spm:scene_boosterfun:uncommon", 48)),
					[]packSlot{
						{Label: "Traditional Foil Spiderweb Basic Land", Bucket: "set:spm:spiderweb:land"},
						{Label: "Traditional Foil Default Rare or Mythic", Weighted: wb("set:spm:main:rare", 779, "set:spm:main:mythic", 110, "set:spe:welcome:rare", 74, "set:spe:welcome:mythic", 37)},
					},
					repeatWeightedPackSlot(2, "Non-foil Booster Fun", wb(
						"set:spm:extendedart_main:rare", 508,
						"set:spm:extendedart_main:mythic", 54,
						"set:spm:webslinger_boosterfun:rare", 92,
						"set:spm:webslinger_boosterfun:mythic", 15,
						"set:spm:panel_boosterfun:rare", 154,
						"set:spm:panel_boosterfun:mythic", 31,
						"set:spm:scene_boosterfun:rare", 46,
						"set:spm:scene_boosterfun:mythic", 8,
						"set:spe:scene_box_boosterfun:rare", 92,
					)),
					[]packSlot{
						{Label: "Source Material", Bucket: "set:mar:release:2025-09-26", FinishWeighted: wf("nonfoil", 75, "foil", 25)},
						{Label: "Foil Booster Fun", Weighted: wb(
							"set:spm:extendedart_main:rare", 540,
							"set:spm:extendedart_main:mythic", 57,
							"set:spm:webslinger_boosterfun:rare", 98,
							"set:spm:webslinger_boosterfun:mythic", 16,
							"set:spm:panel_boosterfun:rare", 164,
							"set:spm:panel_boosterfun:mythic", 32,
							"set:spm:scene_boosterfun:rare", 49,
							"set:spm:scene_boosterfun:mythic", 7,
							"set:spm:classiccomic_boosterfun:mythic", 25,
							"set:spm:costume_boosterfun:rare", 7,
							"set:spm:gauntlet_boosterfun:mythic", 5,
						)},
					},
				),
				ExpectedPools: map[string]int{
					"set:spm:main:common":                    60,
					"set:spm:main:uncommon":                  55,
					"set:spm:main:rare":                      53,
					"set:spm:main:mythic":                    15,
					"set:spm:play_dual:land":                 5,
					"set:spm:spiderweb:land":                 5,
					"set:spm:scene_boosterfun:uncommon":      3,
					"set:spm:scene_boosterfun:rare":          4,
					"set:spm:scene_boosterfun:mythic":        2,
					"set:spm:webslinger_boosterfun:rare":     7,
					"set:spm:webslinger_boosterfun:mythic":   3,
					"set:spm:panel_boosterfun:rare":          10,
					"set:spm:panel_boosterfun:mythic":        4,
					"set:spm:extendedart_main:rare":          33,
					"set:spm:extendedart_main:mythic":        7,
					"set:spm:classiccomic_boosterfun:mythic": 3,
					"set:spm:costume_boosterfun:rare":        7,
					"set:spm:gauntlet_boosterfun:mythic":     1,
					"set:spe:welcome:common":                 5,
					"set:spe:welcome:uncommon":               5,
					"set:spe:welcome:rare":                   5,
					"set:spe:welcome:mythic":                 5,
					"set:spe:scene_box_boosterfun:rare":      6,
					"set:mar:release:2025-09-26":             38,
				},
			},
		},
	},
	"eoe": {
		SourceURL:       packOpeningSourceEdgeOfEternities,
		RelatedSetCodes: []string{"eos", "eoc", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:  packOpeningSourceEdgeOfEternities,
				PackArtURI: "https://media.wizards.com/2025/images/daily/en_zUzgBbcHaS.webp",
				CardCount:  14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:eoe:common"),
					[]packSlot{{Label: "Common / Special Guest", Weighted: wb("set:eoe:common", 982, "set:spg", 18)}},
					repeatPackSlot(3, "Uncommon", "set:eoe:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb("set:eoe:common", 125, "set:eoe:uncommon", 625, "set:eoe:rare", 116, "set:eoe:mythic", 9, "set:eos:rare", 110, "set:eos:mythic", 25)},
						{Label: "Rare or Mythic", Weighted: wb("set:eoe:rare", 844, "set:eoe:mythic", 156)},
						{Label: "Traditional Foil", Weighted: wb("set:eoe:common", 580, "set:eoe:uncommon", 320, "set:eoe:rare", 74, "set:eoe:mythic", 11, "set:eos:rare", 10, "set:eos:mythic", 5)},
						{Label: "Land", Weighted: wb("set:eoe:land", 800, "set:eos:land", 200)},
					},
				),
			},
			"collector": {
				SourceURL:  packOpeningSourceEdgeOfEternities,
				PackArtURI: "https://media.wizards.com/2025/images/daily/en_1VEQ1D6AZC.webp",
				CardCount:  15,
				Slots: appendSlots(nil,
					repeatPackSlot(5, "Traditional Foil Common", "set:eoe:common"),
					repeatPackSlot(4, "Traditional Foil Uncommon", "set:eoe:uncommon"),
					[]packSlot{
						{Label: "Celestial Basic Land", Weighted: wb("set:eoe:land", 667, "set:eos:land", 333)},
						{Label: "Default Rare or Mythic", Weighted: wb("set:eoe:rare", 860, "set:eoe:mythic", 140)},
						{Label: "Commander Card", Weighted: wb("set:eoc:rare", 910, "set:eoc:mythic", 90)},
						{Label: "Non-foil Booster Fun", Weighted: wb("set:eoe:rare", 820, "set:eoe:mythic", 180)},
						{Label: "Stellar Sights Land", Weighted: wb("set:eos:rare", 810, "set:eos:mythic", 190)},
						{Label: "Foil Booster Fun", Weighted: wb("set:eoe:rare", 800, "set:eoe:mythic", 140, "set:spg", 60)},
					},
				),
			},
		},
	},
	"fin": {
		SourceURL:       packOpeningSourceFinalFantasy,
		RelatedSetCodes: []string{"fca", "fic"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:  packOpeningSourceFinalFantasy,
				PackArtURI: "https://media.wizards.com/2025/images/daily/en_ZcV8b56jf0.webp",
				CardCount:  14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:fin:default:common"),
					[]packSlot{{Label: "Common / Through the Ages", Weighted: wb("set:fin:default:common", 2000, "set:fca:uncommon", 632, "set:fca:rare", 298, "set:fca:mythic", 70)}},
					repeatPackSlot(3, "Uncommon", "set:fin:default:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb("set:fin:default:common", 193, "set:fin:default:uncommon", 640, "set:fin:default:rare", 148, "set:fin:default:mythic", 19)},
						{Label: "Rare or Mythic", Weighted: wb("set:fin:default:rare", 800, "set:fin:default:mythic", 100, "set:fin:borderless:rare", 80, "set:fin:borderless:mythic", 10, "set:fin:boosterfun:rare", 5, "set:fin:boosterfun:mythic", 5)},
						{Label: "Traditional Foil", Weighted: wb("set:fin:foil_default:common", 5575, "set:fin:foil_default:uncommon", 3590, "set:fin:foil_default:rare", 550, "set:fin:foil_default:mythic", 75, "set:fin:foil_boosterfun:common", 10, "set:fin:foil_boosterfun:uncommon", 50, "set:fin:foil_boosterfun:rare", 100, "set:fin:foil_boosterfun:mythic", 25, "set:fin:boosterfun:uncommon", 25)},
						{Label: "Land", Bucket: "set:fin:land"},
					},
				),
			},
			"collector": {
				SourceURL:  packOpeningSourceFinalFantasy,
				PackArtURI: "https://media.wizards.com/2025/images/daily/en_A6mqWUmFzm.webp",
				CardCount:  15,
				Slots: appendSlots(nil,
					repeatPackSlot(3, "Traditional Foil Common", "set:fin:foil_default:common"),
					repeatPackSlot(3, "Traditional Foil Uncommon", "set:fin:foil_default:uncommon"),
					[]packSlot{
						{Label: "Booster Fun Common / Uncommon", Weighted: wb("set:fin:nonfoil_boosterfun:common", 50, "set:fin:nonfoil_boosterfun:uncommon", 950)},
						{Label: "Foil Booster Fun Common / Uncommon", Weighted: wb("set:fin:foil_boosterfun:common", 138, "set:fin:foil_boosterfun:uncommon", 862)},
						{Label: "Traditional Foil Basic Land", Bucket: "set:fin:foil:land"},
						{Label: "Default Rare or Mythic", Weighted: wb("set:fin:foil_default:rare", 878, "set:fin:foil_default:mythic", 122)},
						{Label: "Booster Fun Rare or Mythic", Weighted: wb("set:fin:nonfoil_boosterfun:rare", 821, "set:fin:nonfoil_boosterfun:mythic", 179, "set:fic:nonfoil_boosterfun:rare", 460, "set:fic:nonfoil_boosterfun:mythic", 60)},
						{Label: "Booster Fun Rare or Mythic", Weighted: wb("set:fin:nonfoil_boosterfun:rare", 821, "set:fin:nonfoil_boosterfun:mythic", 179, "set:fic:nonfoil_boosterfun:rare", 460, "set:fic:nonfoil_boosterfun:mythic", 60)},
						{Label: "Booster Fun Rare or Mythic", Weighted: wb("set:fin:nonfoil_boosterfun:rare", 821, "set:fin:nonfoil_boosterfun:mythic", 179, "set:fic:nonfoil_boosterfun:rare", 460, "set:fic:nonfoil_boosterfun:mythic", 60)},
						{Label: "Through the Ages", Weighted: wb("set:fca:uncommon", 683, "set:fca:rare", 257, "set:fca:mythic", 60)},
						{Label: "Foil / Surge Rare or Mythic", Weighted: wb("set:fin:foil_boosterfun:rare", 850, "set:fin:foil_boosterfun:mythic", 150, "set:fin:surgefoil_boosterfun:rare", 70, "set:fin:surgefoil_boosterfun:mythic", 30)},
					},
				),
			},
		},
	},
	"inr": {
		SourceURL: packOpeningSourceInnistradRemaster,
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:       packOpeningSourceInnistradRemaster,
				PackArtURI:      "https://media.wizards.com/2024/images/daily/EN_TPJjOHUmFl.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       14,
				Slots: appendSlots(nil,
					repeatPackSlot(5, "Single-faced Common", "set:inr:singleface:common"),
					[]packSlot{{Label: "Double-faced Common", Bucket: "set:inr:dfc:common"}},
					repeatPackSlot(3, "Uncommon", "set:inr:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb("set:inr:common", 400, "set:inr:uncommon", 350, "set:inr:rare", 200, "set:inr:mythic", 50)},
						{Label: "Rare or Mythic", Weighted: wb("set:inr:rare", 857, "set:inr:mythic", 143)},
						{Label: "Retro Frame", Bucket: "set:inr:retro"},
						{Label: "Traditional Foil", Weighted: wb("set:inr:foil:common", 580, "set:inr:foil:uncommon", 320, "set:inr:foil:rare", 80, "set:inr:foil:mythic", 20)},
						{Label: "Land", Bucket: "set:inr:land"},
					},
				),
			},
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceInnistradRemaster,
				"https://media.wizards.com/2024/images/daily/en_R5fKYQYFlS.png",
				"inr",
				4,
				3,
				[]packSlot{
					{Label: "Traditional Foil Retro-frame Land", Bucket: "set:inr:foil:land"},
					{Label: "Non-foil Booster Fun Common or Uncommon", Bucket: "set:inr:nonfoil_boosterfun:common_or_uncommon"},
					{Label: "Non-foil Booster Fun Common or Uncommon", Bucket: "set:inr:nonfoil_boosterfun:common_or_uncommon"},
					{Label: "Traditional Foil Booster Fun Common or Uncommon", Bucket: "set:inr:foil_boosterfun:common_or_uncommon"},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:inr:foil_default:rare", 857, "set:inr:foil_default:mythic", 143)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:inr:nonfoil_boosterfun:rare", 857, "set:inr:nonfoil_boosterfun:mythic", 143)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:inr:nonfoil_boosterfun:rare", 857, "set:inr:nonfoil_boosterfun:mythic", 143)},
					{Label: "Traditional Foil Booster Fun Rare or Mythic", Weighted: wb("set:inr:foil_boosterfun:rare", 849, "set:inr:foil_boosterfun:mythic", 150, "set:inr:serialized:mythic", 1)},
				},
			),
		},
	},
	"fdn": {
		SourceURL:       packOpeningSourceFoundations,
		RelatedSetCodes: []string{"spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": packOpeningModernPlayBoosterConfig(
				packOpeningSourceFoundations,
				"https://media.wizards.com/2024/images/daily/en_ewZYCzE0Ag.png",
				"fdn",
				wb("set:fdn:default:common", 982, "set:spg", 18),
				wb("set:fdn:default:common", 417, "set:fdn:default:uncommon", 333, "set:fdn:default:rare", 200, "set:fdn:default:mythic", 42, "set:fdn:borderless:mythic", 8),
				wb("set:fdn:default:rare", 875, "set:fdn:default:mythic", 117, "set:fdn:borderless:mythic", 8),
				wb("set:fdn:foil_default:common", 580, "set:fdn:foil_default:uncommon", 320, "set:fdn:foil_default:rare", 80, "set:fdn:foil_default:mythic", 12, "set:fdn:foil_boosterfun:mythic", 8),
				wb("set:fdn:default:land", 800, "set:fdn:fullart:land", 200),
			),
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceFoundations,
				"https://media.wizards.com/2024/images/daily/en_jGI91ABfYx.png",
				"fdn",
				5,
				4,
				[]packSlot{
					{Label: "Traditional Foil Full-art Basic Land", Bucket: "set:fdn:foil:land"},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:fdn:foil_default:rare", 857, "set:fdn:foil_default:mythic", 143)},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:fdn:foil_default:rare", 857, "set:fdn:foil_default:mythic", 143)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:fdn:nonfoil_boosterfun:rare", 857, "set:fdn:nonfoil_boosterfun:mythic", 143)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:fdn:nonfoil_boosterfun:rare", 857, "set:fdn:nonfoil_boosterfun:mythic", 143)},
					{Label: "Foil Booster Fun Rare or Mythic", Weighted: wb("set:fdn:foil_boosterfun:rare", 845, "set:fdn:foil_boosterfun:mythic", 145, "set:fdn:fracturefoil:mythic", 10)},
				},
			),
		},
	},
	"mh3": {
		SourceURL:       packOpeningSourceModernHorizons3,
		RelatedSetCodes: []string{"m3c", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:       packOpeningSourceModernHorizons3,
				PackArtURI:      "https://media.wizards.com/2024/images/daily/en_jhpBKHHkxxXH.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:mh3:default:common"),
					repeatPackSlot(3, "Uncommon", "set:mh3:default:uncommon"),
					[]packSlot{
						{Label: "New-to-Modern Reprint", Bucket: "set:mh3:retro"},
						{Label: "Non-foil Wildcard", Weighted: wb("set:mh3:default:common", 400, "set:mh3:default:uncommon", 350, "set:mh3:default:rare", 200, "set:mh3:default:mythic", 50)},
						{Label: "Rare or Mythic", Weighted: wb("set:mh3:default:rare", 857, "set:mh3:default:mythic", 143)},
						{Label: "Traditional Foil Wildcard", Weighted: wb("set:mh3:foil:common", 580, "set:mh3:foil:uncommon", 320, "set:mh3:foil:rare", 80, "set:mh3:foil:mythic", 20)},
						{Label: "Basic Land or Common", Weighted: wb("set:mh3:land", 500, "set:mh3:common", 500)},
					},
				),
			},
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceModernHorizons3,
				"https://media.wizards.com/2024/images/daily/en_dQGCweJwMSFX.png",
				"mh3",
				4,
				3,
				[]packSlot{
					{Label: "Traditional Foil Full-art Basic Land", Bucket: "set:mh3:foil:land"},
					{Label: "Non-foil Retro-frame Common or Uncommon", Bucket: "set:mh3:retro:common_or_uncommon"},
					{Label: "Traditional Foil Retro-frame Common or Uncommon", Bucket: "set:mh3:foil_boosterfun:common_or_uncommon"},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:mh3:foil_default:rare", 857, "set:mh3:foil_default:mythic", 143)},
					{Label: "Modern Horizons 3 Commander", Weighted: wb("set:m3c:extendedart:rare", 910, "set:m3c:borderless:mythic", 90)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:mh3:nonfoil_boosterfun:rare", 857, "set:mh3:nonfoil_boosterfun:mythic", 143)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:mh3:nonfoil_boosterfun:rare", 857, "set:mh3:nonfoil_boosterfun:mythic", 143)},
					{Label: "Traditional Foil Booster Fun Rare or Mythic", Weighted: wb("set:mh3:foil_boosterfun:rare", 849, "set:mh3:foil_boosterfun:mythic", 150, "set:mh3:serialized:mythic", 1)},
				},
			),
		},
	},
	"mkm": {
		SourceURL:       packOpeningSourceKarlovManor,
		RelatedSetCodes: []string{"mkc", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": packOpeningModernPlayBoosterConfig(
				packOpeningSourceKarlovManor,
				"https://media.wizards.com/2024/images/daily/en_CXzIRxpmXBDV.png",
				"mkm",
				wb("set:mkm:default:common", 982, "set:spg", 18),
				wb("set:mkm:default:common", 417, "set:mkm:default:uncommon", 333, "set:mkm:default:rare", 200, "set:mkm:default:mythic", 42, "set:mkm:borderless:mythic", 8),
				wb("set:mkm:default:rare", 875, "set:mkm:default:mythic", 117, "set:mkm:borderless:mythic", 8),
				wb("set:mkm:foil_default:common", 580, "set:mkm:foil_default:uncommon", 320, "set:mkm:foil_default:rare", 80, "set:mkm:foil_default:mythic", 12, "set:mkm:foil_boosterfun:mythic", 8),
				wb("set:mkm:default:land", 800, "set:mkm:fullart:land", 200),
			),
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceKarlovManor,
				"https://media.wizards.com/2024/images/daily/en_C6zTqHqoMVj5.png",
				"mkm",
				4,
				3,
				[]packSlot{
					{Label: "Traditional Foil Full-art Basic Land", Bucket: "set:mkm:foil:land"},
					{Label: "Non-foil Showcase Dossier Common or Uncommon", Bucket: "set:mkm:nonfoil_boosterfun:common_or_uncommon"},
					{Label: "Traditional Foil Showcase Dossier Common or Uncommon", Bucket: "set:mkm:foil_boosterfun:common_or_uncommon"},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:mkm:foil_default:rare", 857, "set:mkm:foil_default:mythic", 143)},
					{Label: "Non-foil Extended-art Rare", Bucket: "set:mkm:extendedart:rare"},
					{Label: "Murders at Karlov Manor Commander", Weighted: wb("set:mkc:extendedart:rare", 910, "set:mkc:borderless:mythic", 90)},
					{Label: "Non-foil Showcase or Borderless Rare or Mythic", Weighted: wb("set:mkm:nonfoil_boosterfun:rare", 857, "set:mkm:nonfoil_boosterfun:mythic", 143)},
					{Label: "Traditional Foil Showcase or Borderless Rare or Mythic", Weighted: wb("set:mkm:foil_boosterfun:rare", 849, "set:mkm:foil_boosterfun:mythic", 150, "set:mkm:serialized:mythic", 1)},
				},
			),
		},
	},
	"lci": {
		SourceURL:       packOpeningSourceLostCaverns,
		RelatedSetCodes: []string{"lcc", "rex", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"draft": {
				SourceURL:       packOpeningSourceLostCaverns,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_BPwC9oRSH3n.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(8, "Common", "set:lci:common"),
					[]packSlot{{Label: "Common or Traditional Foil", Weighted: wb("set:lci:common", 667, "set:lci:foil:wildcard", 333)}},
					[]packSlot{{Label: "Double-faced Common or Uncommon", Bucket: "set:lci:dfc:common_or_uncommon"}},
					repeatPackSlot(3, "Uncommon", "set:lci:uncommon"),
					[]packSlot{
						{Label: "Rare or Mythic", Weighted: wb("set:lci:rare", 857, "set:lci:mythic", 143)},
						{Label: "Cave or Core Land", Bucket: "set:lci:land"},
					},
				),
			},
			"set": {
				SourceURL:       packOpeningSourceLostCaverns,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_5jX19Z8sVhp.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       12,
				Slots: appendSlots(nil,
					repeatPackSlot(3, "Common", "set:lci:common"),
					repeatPackSlot(3, "Uncommon", "set:lci:uncommon"),
					[]packSlot{
						{Label: "Double-faced Common or Uncommon", Bucket: "set:lci:dfc:common_or_uncommon"},
						{Label: "Wildcard", Weighted: wb("set:lci:wildcard", 917, "set:rex", 83)},
						{Label: "Wildcard", Weighted: wb("set:lci:wildcard", 917, "set:rex", 83)},
						{Label: "Rare or Mythic", Weighted: wb("set:lci:rare", 857, "set:lci:mythic", 143)},
						{Label: "Traditional Foil", Bucket: "set:lci:foil:wildcard"},
						{Label: "Cave or Core Land", Bucket: "set:lci:land"},
					},
				),
			},
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceLostCaverns,
				"https://media.wizards.com/2023/images/daily/en_FxA6F3AuJBP.png",
				"lci",
				4,
				3,
				[]packSlot{
					{Label: "Traditional Foil Full-art Basic Land", Bucket: "set:lci:foil:land"},
					{Label: "Traditional Foil Showcase or Borderless Uncommon", Weighted: wb("set:lci:foil_boosterfun:uncommon", 970, "set:spg:release:2023-11-17:uncommon", 30)},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:lci:foil_default:rare", 853, "set:lci:foil_default:mythic", 147)},
					{Label: "Non-foil Extended-art Rare or Mythic", Weighted: wb("set:lci:extendedart:rare", 974, "set:lci:extendedart:mythic", 26)},
					{Label: "The Lost Caverns of Ixalan Commander", Weighted: wb("set:lcc:extendedart:rare", 875, "set:lcc:extendedart:mythic", 125)},
					{Label: "Non-foil Showcase or Borderless Rare or Mythic", Weighted: wb("set:lci:nonfoil_boosterfun:rare", 730, "set:lci:nonfoil_boosterfun:mythic", 270)},
					{Label: "Jurassic World Collection", Bucket: "set:rex"},
					{Label: "Traditional Foil Booster Fun Rare or Mythic", Weighted: wb("set:lci:foil_boosterfun:rare", 760, "set:lci:foil_boosterfun:mythic", 159, "set:spg:release:2023-11-17:rare_or_mythic", 80, "set:lci:neonink:rare_or_mythic", 1)},
				},
			),
		},
	},
	"woe": {
		SourceURL:       packOpeningSourceWildsEldraine,
		RelatedSetCodes: []string{"woc", "wot"},
		Packs: map[string]packOpeningConfiguredPack{
			"draft": {
				SourceURL:       packOpeningSourceWildsEldraine,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_dQVFR4srSbx0.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(8, "Common", "set:woe:common"),
					[]packSlot{{Label: "Common or Traditional Foil", Weighted: wb("set:woe:common", 667, "set:woe:foil:wildcard", 333)}},
					repeatPackSlot(3, "Uncommon", "set:woe:uncommon"),
					[]packSlot{
						{Label: "Enchanting Tales", Weighted: wb("set:wot:uncommon", 667, "set:wot:rare", 222, "set:wot:mythic", 111)},
						{Label: "Rare or Mythic", Weighted: wb("set:woe:rare", 857, "set:woe:mythic", 143)},
						{Label: "Land", Weighted: wb("set:woe:default:land", 667, "set:woe:fullart:land", 333)},
					},
				),
			},
			"set": {
				SourceURL:       packOpeningSourceWildsEldraine,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_pJB7oTWrJrOU.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       12,
				Slots: appendSlots(nil,
					repeatPackSlot(3, "Common", "set:woe:common"),
					repeatPackSlot(2, "Uncommon", "set:woe:uncommon"),
					[]packSlot{
						{Label: "Enchanting Tales", Weighted: wb("set:wot:uncommon", 667, "set:wot:rare", 222, "set:wot:mythic", 111)},
						{Label: "Wildcard", Bucket: "set:woe:wildcard"},
						{Label: "Wildcard", Weighted: wb("set:woe:wildcard", 850, "set:woc:rare_or_mythic", 150)},
						{Label: "Rare or Mythic", Weighted: wb("set:woe:rare", 857, "set:woe:mythic", 143)},
						{Label: "Traditional Foil", Bucket: "set:woe:foil:wildcard"},
						{Label: "Land", Weighted: wb("set:woe:default:land", 667, "set:woe:fullart:land", 333)},
						{Label: "Additional Common or Uncommon", Bucket: "set:woe:common_or_uncommon"},
					},
				),
			},
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceWildsEldraine,
				"https://media.wizards.com/2023/images/daily/en_kk0f7AcxFWlU.png",
				"woe",
				4,
				3,
				[]packSlot{
					{Label: "Traditional Foil Full-art Basic Land", Bucket: "set:woe:foil:land"},
					{Label: "Non-foil Enchanting Tales Uncommon", Bucket: "set:wot:nonfoil:uncommon"},
					{Label: "Traditional Foil Enchanting Tales Uncommon", Bucket: "set:wot:foil:uncommon"},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:woe:foil_default:rare", 857, "set:woe:foil_default:mythic", 143)},
					{Label: "Extended-art Rare or Mythic", Weighted: wb("set:woe:extendedart:rare_or_mythic", 750, "set:woc:extendedart:rare_or_mythic", 250)},
					{Label: "Non-foil Showcase or Borderless Rare or Mythic", Bucket: "set:woe:nonfoil_boosterfun:rare_or_mythic"},
					{Label: "Non-foil Rare or Mythic Enchanting Tales", Weighted: wb("set:wot:nonfoil:rare", 733, "set:wot:nonfoil:mythic", 267)},
					{Label: "Traditional or Confetti Foil Booster Fun", Weighted: wb("set:woe:foil_boosterfun:rare_or_mythic", 600, "set:wot:foil:rare_or_mythic", 372, "set:wot:confettifoil:rare_or_mythic", 28)},
				},
			),
		},
	},
	"cmm": {
		SourceURL: packOpeningSourceCommanderMasters,
		Packs: map[string]packOpeningConfiguredPack{
			"draft": {
				SourceURL:       packOpeningSourceCommanderMasters,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_ktVTnfSSKrIM.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       20,
				Slots: appendSlots(nil,
					repeatPackSlot(11, "Common", "set:cmm:common"),
					repeatPackSlot(3, "Nonlegendary Uncommon", "set:cmm:nonlegendary:uncommon"),
					repeatPackSlot(2, "Legendary Uncommon", "set:cmm:legendary:uncommon"),
					[]packSlot{
						{Label: "Legendary Rare or Mythic", Weighted: wb("set:cmm:legendary:rare", 857, "set:cmm:legendary:mythic", 143)},
						{Label: "Nonlegendary Rare or Mythic", Weighted: wb("set:cmm:nonlegendary:rare", 857, "set:cmm:nonlegendary:mythic", 143)},
						{Label: "Nonlegendary Uncommon or Rare or Mythic", Weighted: wb("set:cmm:nonlegendary:uncommon", 667, "set:cmm:nonlegendary:rare", 286, "set:cmm:nonlegendary:mythic", 47)},
						{Label: "Traditional Foil", Bucket: "set:cmm:foil:wildcard"},
					},
				),
			},
			"set": {
				SourceURL:       packOpeningSourceCommanderMasters,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_t1UnJeGrSmzp.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(4, "Common", "set:cmm:common"),
					repeatPackSlot(2, "Nonlegendary Uncommon", "set:cmm:nonlegendary:uncommon"),
					[]packSlot{
						{Label: "Retro-frame Basic Land", Bucket: "set:cmm:retro:land"},
						{Label: "Borderless Common or Uncommon", Bucket: "set:cmm:borderless:common_or_uncommon"},
						{Label: "Legendary Uncommon", Bucket: "set:cmm:legendary:uncommon"},
						{Label: "Legendary Uncommon or Nonlegendary Rare or Mythic", Weighted: wb("set:cmm:legendary:uncommon", 500, "set:cmm:nonlegendary:rare", 429, "set:cmm:nonlegendary:mythic", 71)},
						{Label: "Wildcard", Bucket: "set:cmm:wildcard"},
						{Label: "Wildcard", Bucket: "set:cmm:wildcard"},
						{Label: "Legendary Rare or Mythic", Weighted: wb("set:cmm:legendary:rare", 857, "set:cmm:legendary:mythic", 143)},
						{Label: "Nonlegendary Rare or Mythic", Weighted: wb("set:cmm:nonlegendary:rare", 857, "set:cmm:nonlegendary:mythic", 143)},
						{Label: "Traditional Foil", Bucket: "set:cmm:foil:wildcard"},
					},
				),
			},
			"collector": {
				SourceURL:       packOpeningSourceCommanderMasters,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_Mv2qPqwTVVBZ.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(4, "Traditional Foil Common", "set:cmm:foil:common"),
					repeatPackSlot(2, "Traditional Foil Uncommon", "set:cmm:foil:uncommon"),
					[]packSlot{
						{Label: "Traditional Foil Retro-frame Basic Land", Bucket: "set:cmm:foil:land"},
						{Label: "Non-foil Borderless Common or Uncommon", Bucket: "set:cmm:nonfoil_boosterfun:common_or_uncommon"},
						{Label: "Non-foil Borderless Common or Uncommon", Bucket: "set:cmm:nonfoil_boosterfun:common_or_uncommon"},
						{Label: "Traditional Foil Borderless Common or Uncommon", Bucket: "set:cmm:foil_boosterfun:common_or_uncommon"},
						{Label: "Traditional Foil Rare or Mythic", Bucket: "set:cmm:foil:rare_or_mythic"},
						{Label: "Foil-etched Rare or Mythic", Bucket: "set:cmm:etched:rare_or_mythic"},
						{Label: "Extended-art Commander Rare or Mythic", Bucket: "set:cmm:extendedart:rare_or_mythic"},
						{Label: "Non-foil Borderless Rare or Mythic", Bucket: "set:cmm:nonfoil_boosterfun:rare_or_mythic"},
						{Label: "Traditional or Textured Foil Rare or Mythic", Weighted: wb("set:cmm:foil_boosterfun:rare_or_mythic", 960, "set:cmm:texturedfoil:rare_or_mythic", 40)},
					},
				),
			},
		},
	},
	"ltr": {
		SourceURL:       packOpeningSourceLordOfTheRings,
		RelatedSetCodes: []string{"ltc"},
		Packs: map[string]packOpeningConfiguredPack{
			"draft": {
				SourceURL:       packOpeningSourceLordOfTheRings,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_Zwfg7jACuSjG.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(10, "Common or Traditional Foil", "set:ltr:common"),
					repeatPackSlot(3, "Uncommon", "set:ltr:uncommon"),
					[]packSlot{
						{Label: "Rare or Mythic", Weighted: wb("set:ltr:rare", 857, "set:ltr:mythic", 143)},
						{Label: "Middle-earth Map Land", Bucket: "set:ltr:land"},
					},
				),
			},
			"set": {
				SourceURL:       packOpeningSourceLordOfTheRings,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_1sZDzTl46Sgb.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       12,
				Slots: appendSlots(nil,
					repeatPackSlot(4, "Common", "set:ltr:common"),
					repeatPackSlot(3, "Uncommon", "set:ltr:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Bucket: "set:ltr:wildcard"},
						{Label: "Wildcard", Weighted: wb("set:ltr:wildcard", 850, "set:ltc:rare_or_mythic", 150)},
						{Label: "Rare or Mythic", Weighted: wb("set:ltr:rare", 857, "set:ltr:mythic", 143)},
						{Label: "Traditional Foil", Bucket: "set:ltr:foil:wildcard"},
						{Label: "Middle-earth Map Land", Bucket: "set:ltr:land"},
					},
				),
			},
			"collector": {
				SourceURL:       packOpeningSourceLordOfTheRings,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_bDwmHxDoZBBB.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(3, "Traditional Foil Common", "set:ltr:foil:common"),
					[]packSlot{{Label: "Traditional Foil Common or Serialized Ring", Weighted: wb("set:ltr:foil:common", 999, "set:ltr:serialized:mythic", 1)}},
					repeatPackSlot(2, "Traditional Foil Uncommon", "set:ltr:foil:uncommon"),
					[]packSlot{
						{Label: "Traditional Foil Middle-earth Map Land", Bucket: "set:ltr:foil:land"},
						{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:ltr:foil_default:rare", 857, "set:ltr:foil_default:mythic", 143)},
						{Label: "Extended-art Main-set Rare or Mythic", Bucket: "set:ltr:extendedart:rare_or_mythic"},
						{Label: "Extended-art Commander Rare or Mythic", Bucket: "set:ltc:extendedart:rare_or_mythic"},
						{Label: "Showcase Ring Uncommon or Nazgul", Bucket: "set:ltr:nonfoil_boosterfun:uncommon"},
						{Label: "Non-foil Booster Fun Rare or Mythic", Bucket: "set:ltr:nonfoil_boosterfun:rare_or_mythic"},
						{Label: "Borderless Scene Card", Bucket: "set:ltr:borderless"},
						{Label: "Traditional Foil Booster Fun Common or Uncommon", Bucket: "set:ltr:foil_boosterfun:common_or_uncommon"},
						{Label: "Traditional or Surge Foil Booster Fun Rare or Mythic", Weighted: wb("set:ltr:foil_boosterfun:rare_or_mythic", 992, "set:ltr:surgefoil:mythic", 8)},
					},
				),
			},
		},
	},
	"mom": {
		SourceURL:       packOpeningSourceMarchMachine,
		RelatedSetCodes: []string{"moc", "mul"},
		Packs: map[string]packOpeningConfiguredPack{
			"draft": {
				SourceURL:       packOpeningSourceMarchMachine,
				PackArtURI:      "https://media.wizards.com/2022/images/daily/en_kcwuw79dkj6m.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(7, "Common", "set:mom:common"),
					repeatPackSlot(2, "Uncommon", "set:mom:uncommon"),
					[]packSlot{
						{Label: "Multiverse Legends", Weighted: wb("set:mul:uncommon", 667, "set:mul:rare", 222, "set:mul:mythic", 111)},
						{Label: "Battle", Bucket: "set:mom:battle"},
						{Label: "Non-battle Double-faced Card", Bucket: "set:mom:nonbattle_dfc"},
						{Label: "Rare or Mythic", Weighted: wb("set:mom:rare", 857, "set:mom:mythic", 143)},
						{Label: "Common or Traditional Foil", Weighted: wb("set:mom:common", 667, "set:mom:foil:wildcard", 333)},
						{Label: "Basic or Full-art Land", Weighted: wb("set:mom:default:land", 667, "set:mom:fullart:land", 333)},
					},
				),
			},
			"set": {
				SourceURL:       packOpeningSourceMarchMachine,
				PackArtURI:      "https://media.wizards.com/2022/images/daily/en_bks5d4k9prjm.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       12,
				Slots: appendSlots(nil,
					repeatPackSlot(2, "Connected Common", "set:mom:common"),
					repeatPackSlot(2, "Connected Uncommon", "set:mom:uncommon"),
					[]packSlot{
						{Label: "Multiverse Legends", Weighted: wb("set:mul:uncommon", 667, "set:mul:rare", 222, "set:mul:mythic", 111)},
						{Label: "Battle", Bucket: "set:mom:battle"},
						{Label: "Double-faced Common or Uncommon", Bucket: "set:mom:dfc:common_or_uncommon"},
						{Label: "Wildcard", Weighted: wb("set:mom:wildcard", 850, "set:moc:rare_or_mythic", 100, "set:mul:rare_or_mythic", 50)},
						{Label: "Wildcard", Weighted: wb("set:mom:wildcard", 850, "set:moc:rare_or_mythic", 100, "set:mul:rare_or_mythic", 50)},
						{Label: "Rare or Mythic", Weighted: wb("set:mom:rare", 857, "set:mom:mythic", 143)},
						{Label: "Traditional Foil or Foil-etched", Weighted: wb("set:mom:foil:common", 400, "set:mom:foil:uncommon", 250, "set:mom:foil:rare", 120, "set:mom:foil:mythic", 30, "set:mul:etched:uncommon", 130, "set:mul:etched:rare", 55, "set:mul:etched:mythic", 15)},
						{Label: "Basic or Full-art Land", Weighted: wb("set:mom:default:land", 667, "set:mom:fullart:land", 333)},
					},
				),
			},
			"collector": {
				SourceURL:       packOpeningSourceMarchMachine,
				PackArtURI:      "https://media.wizards.com/2023/images/daily/en_jfHlPWfUmtHv_2.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(5, "Traditional Foil Common", "set:mom:foil:common"),
					repeatPackSlot(2, "Traditional Foil Uncommon", "set:mom:foil:uncommon"),
					[]packSlot{
						{Label: "Traditional Foil Multiverse Legends Uncommon", Bucket: "set:mul:foil:uncommon"},
						{Label: "Traditional Foil Multiverse Legends Uncommon", Bucket: "set:mul:foil:uncommon"},
						{Label: "Traditional Foil Full-art Basic Land", Bucket: "set:mom:foil:land"},
						{Label: "Traditional Foil Rare or Mythic", Bucket: "set:mom:foil:rare_or_mythic"},
						{Label: "Non-foil Extended-art Commander Rare or Mythic", Bucket: "set:moc:extendedart:rare_or_mythic"},
						{Label: "Non-foil Booster Fun Rare or Mythic", Bucket: "set:mom:nonfoil_boosterfun:rare_or_mythic"},
						{Label: "Traditional Foil Booster Fun Rare or Mythic", Bucket: "set:mom:foil_boosterfun:rare_or_mythic"},
						{Label: "Traditional, Etched, Halo, or Serialized Multiverse Legends Rare or Mythic", Weighted: wb("set:mul:foil:rare_or_mythic", 750, "set:mul:etched:rare_or_mythic", 140, "set:mul:halofoil:rare_or_mythic", 100, "set:mul:serialized:rare_or_mythic", 10)},
					},
				),
			},
		},
	},
	"one": {
		SourceURL:       packOpeningSourceAllWillBeOne,
		RelatedSetCodes: []string{"onc"},
		Packs: map[string]packOpeningConfiguredPack{
			"draft": {
				SourceURL:       packOpeningSourceAllWillBeOne,
				PackArtURI:      "https://media.wizards.com/2022/images/daily/en_2sjDm3uasd.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(10, "Common or Traditional Foil", "set:one:common"),
					repeatPackSlot(3, "Uncommon", "set:one:uncommon"),
					[]packSlot{
						{Label: "Rare or Mythic", Weighted: wb("set:one:rare", 857, "set:one:mythic", 143)},
						{Label: "Panorama or Phyrexianized Land", Bucket: "set:one:land"},
					},
				),
			},
			"set": {
				SourceURL:       packOpeningSourceAllWillBeOne,
				PackArtURI:      "https://media.wizards.com/2022/images/daily/en_6SAzjyg6SW.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       12,
				Slots: appendSlots(nil,
					repeatPackSlot(4, "Common", "set:one:common"),
					repeatPackSlot(3, "Uncommon", "set:one:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Bucket: "set:one:wildcard"},
						{Label: "Wildcard", Weighted: wb("set:one:wildcard", 850, "set:onc:rare_or_mythic", 150)},
						{Label: "Rare or Mythic", Weighted: wb("set:one:rare", 857, "set:one:mythic", 143)},
						{Label: "Traditional Foil", Bucket: "set:one:foil:wildcard"},
						{Label: "Panorama or Phyrexianized Land", Bucket: "set:one:land"},
					},
				),
			},
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceAllWillBeOne,
				"https://media.wizards.com/2022/images/daily/en_UQlGGg3rBD.png",
				"one",
				4,
				2,
				[]packSlot{
					{Label: "Traditional Foil Panorama or Phyrexianized Land", Bucket: "set:one:foil:land"},
					{Label: "Traditional Foil Rare or Mythic", Bucket: "set:one:foil_default:rare_or_mythic"},
					{Label: "Extended-art Rare", Bucket: "set:one:extendedart:rare"},
					{Label: "Extended-art Commander Rare or Mythic", Bucket: "set:onc:extendedart:rare_or_mythic"},
					{Label: "Non-foil Ichor Common or Uncommon", Bucket: "set:one:nonfoil_boosterfun:common_or_uncommon"},
					{Label: "Traditional Foil Ichor Common or Uncommon", Bucket: "set:one:foil_boosterfun:common_or_uncommon"},
					{Label: "Step-and-compleat Foil", Bucket: "set:one:stepandcompleat"},
					{Label: "Non-foil Booster Fun Rare or Mythic", Bucket: "set:one:nonfoil_boosterfun:rare_or_mythic"},
					{Label: "Traditional Foil Booster Fun or Extended-art Rare or Mythic", Bucket: "set:one:foil_boosterfun:rare_or_mythic"},
				},
			),
		},
	},
	"bro": {
		SourceURL:       packOpeningSourceBrothersWar,
		RelatedSetCodes: []string{"brc", "brr", "bot"},
		Packs: map[string]packOpeningConfiguredPack{
			"draft": {
				SourceURL:       packOpeningSourceBrothersWar,
				PackArtURI:      "https://media.wizards.com/2022/images/daily/en_oiHEvPYXO4.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       15,
				Slots: appendSlots(nil,
					repeatPackSlot(9, "Common or Traditional Foil", "set:bro:common"),
					repeatPackSlot(3, "Uncommon", "set:bro:uncommon"),
					[]packSlot{
						{Label: "Retro or Schematic Artifact", Weighted: wb("set:brr:uncommon", 667, "set:brr:rare", 222, "set:brr:mythic", 111)},
						{Label: "Rare or Mythic", Weighted: wb("set:bro:rare", 857, "set:bro:mythic", 143)},
						{Label: "Basic or Mech Land", Bucket: "set:bro:land"},
					},
				),
			},
			"set": {
				SourceURL:       packOpeningSourceBrothersWar,
				PackArtURI:      "https://media.wizards.com/2022/images/daily/en_QSF4L03KY4.png",
				PackArtSize:     "cover",
				PackArtPosition: "center",
				CardCount:       12,
				Slots: appendSlots(nil,
					repeatPackSlot(3, "Connected Common", "set:bro:common"),
					repeatPackSlot(3, "Connected Uncommon", "set:bro:uncommon"),
					[]packSlot{
						{Label: "Retro or Schematic Artifact", Weighted: wb("set:brr:uncommon", 667, "set:brr:rare", 222, "set:brr:mythic", 111)},
						{Label: "Wildcard", Weighted: wb("set:bro:wildcard", 760, "set:brc:rare_or_mythic", 100, "set:bot:mythic", 70, "set:brr:rare_or_mythic", 70)},
						{Label: "Wildcard", Weighted: wb("set:bro:wildcard", 760, "set:brc:rare_or_mythic", 100, "set:bot:mythic", 70, "set:brr:rare_or_mythic", 70)},
						{Label: "Rare or Mythic", Weighted: wb("set:bro:rare", 857, "set:bro:mythic", 143)},
						{Label: "Traditional Foil", Bucket: "set:bro:foil_wildcard"},
						{Label: "Basic or Mech Land", Bucket: "set:bro:land"},
					},
				),
			},
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceBrothersWar,
				"https://media.wizards.com/2022/images/daily/en_u3og1lEhwS.png",
				"bro",
				4,
				2,
				[]packSlot{
					{Label: "Traditional Foil Mech Land", Bucket: "set:bro:foil:land"},
					{Label: "Foil Alternate-frame or Serialized Rare or Mythic", Weighted: wb("set:bro:foil_boosterfun:rare_or_mythic", 849, "set:brr:foil:rare_or_mythic", 150, "set:brr:serialized:rare_or_mythic", 1)},
					{Label: "Non-foil Retro Artifact", Bucket: "set:brr:nonfoil"},
					{Label: "Non-foil Schematic Artifact", Bucket: "set:brr:nonfoil"},
					{Label: "Traditional Foil Retro or Schematic Uncommon", Bucket: "set:brr:foil:uncommon"},
					{Label: "Transformers", Bucket: "set:bot"},
					{Label: "Traditional Foil Rare or Mythic", Bucket: "set:bro:foil_default:rare_or_mythic"},
					{Label: "Non-foil Borderless or Extended-art Rare or Mythic", Bucket: "set:bro:nonfoil_boosterfun:rare_or_mythic"},
					{Label: "Extended-art Commander Rare or Mythic", Bucket: "set:brc:extendedart:rare_or_mythic"},
				},
			),
		},
	},
	"tdm": {
		SourceURL:       packOpeningSourceTarkirDragonstorm,
		RelatedSetCodes: []string{"tdc", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": {
				SourceURL:  packOpeningSourceTarkirDragonstorm,
				PackArtURI: "https://media.wizards.com/2025/images/daily/en_fUtHYDmcSH.png",
				CardCount:  14,
				Slots: appendSlots(nil,
					repeatPackSlot(6, "Common", "set:tdm:common"),
					[]packSlot{{Label: "Common / Special Guest", Weighted: wb("set:tdm:common", 985, "set:spg", 15)}},
					repeatPackSlot(3, "Uncommon", "set:tdm:uncommon"),
					[]packSlot{
						{Label: "Wildcard", Weighted: wb("set:tdm:common", 171, "set:tdm:uncommon", 621, "set:tdm:rare", 179, "set:tdm:mythic", 30)},
						{Label: "Rare or Mythic", Weighted: wb("set:tdm:rare", 857, "set:tdm:mythic", 153)},
						{Label: "Traditional Foil", Weighted: wb("set:tdm:common", 581, "set:tdm:uncommon", 334, "set:tdm:rare", 72, "set:tdm:mythic", 13)},
						{Label: "Land", Bucket: "set:tdm:land"},
					},
				),
			},
			"collector": {
				SourceURL:  packOpeningSourceTarkirDragonstorm,
				PackArtURI: "https://media.wizards.com/2025/images/daily/en_WZ4ZDjOLeP.webp",
				CardCount:  15,
				Slots: appendSlots(nil,
					repeatPackSlot(4, "Traditional Foil Common", "set:tdm:common"),
					repeatPackSlot(3, "Traditional Foil Uncommon", "set:tdm:uncommon"),
					[]packSlot{
						{Label: "Draconic Frame", Weighted: wb("set:tdm:common", 546, "set:tdm:uncommon", 454)},
						{Label: "Foil Draconic Frame", Weighted: wb("set:tdm:common", 546, "set:tdm:uncommon", 454)},
						{Label: "Booster Fun Basic Land", Bucket: "set:tdm:land"},
						{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:tdm:rare", 857, "set:tdm:mythic", 143)},
						{Label: "Commander Card", Weighted: wb("set:tdc:rare", 889, "set:tdc:mythic", 111)},
						{Label: "Non-foil Booster Fun", Weighted: wb("set:tdm:rare", 857, "set:tdm:mythic", 143)},
						{Label: "Non-foil Booster Fun", Weighted: wb("set:tdm:rare", 857, "set:tdm:mythic", 143)},
						{Label: "Foil Booster Fun", Weighted: wb("set:tdm:rare", 830, "set:tdm:mythic", 100, "set:spg", 70)},
					},
				),
			},
		},
	},
	"dft": {
		SourceURL:       packOpeningSourceAetherdrift,
		RelatedSetCodes: []string{"drc", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": packOpeningModernPlayBoosterConfig(
				packOpeningSourceAetherdrift,
				"https://media.wizards.com/2025/images/daily/en_lUT8Ezy7OD.webp",
				"dft",
				wb("set:dft:default:common", 985, "set:spg", 15),
				wb("set:dft:default:common", 83, "set:dft:default:uncommon", 625, "set:dft:default:rare", 174, "set:dft:default:mythic", 29, "set:dft:borderless:common", 40, "set:dft:borderless:uncommon", 43),
				wb("set:dft:default:rare", 780, "set:dft:default:mythic", 130, "set:dft:borderless:rare", 80, "set:dft:borderless:mythic", 10),
				wb("set:dft:foil_default:common", 605, "set:dft:foil_default:uncommon", 300, "set:dft:foil_default:rare", 64, "set:dft:foil_default:mythic", 11, "set:dft:foil_boosterfun:common", 5, "set:dft:foil_boosterfun:uncommon", 5, "set:dft:foil_boosterfun:rare", 9, "set:dft:foil_boosterfun:mythic", 1),
				wb("set:dft:land", 875, "set:dft:fullart:land", 125),
			),
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceAetherdrift,
				"https://media.wizards.com/2025/images/daily/en_UVtjGItEUR.webp",
				"dft",
				4,
				3,
				[]packSlot{
					{Label: "Traditional Foil Driver's Seat Land", Bucket: "set:dft:fullart:land"},
					{Label: "Non-foil Revved Up", Weighted: wb("set:dft:nonfoil_boosterfun:common", 469, "set:dft:nonfoil_boosterfun:uncommon", 531)},
					{Label: "Traditional Foil Revved Up", Weighted: wb("set:dft:foil_boosterfun:common", 469, "set:dft:foil_boosterfun:uncommon", 531)},
					{Label: "Traditional Foil Default Rare or Mythic", Weighted: wb("set:dft:foil_default:rare", 857, "set:dft:foil_default:mythic", 143)},
					{Label: "Aetherdrift Commander", Weighted: wb("set:drc:extendedart:rare", 8425, "set:drc:borderless:mythic", 1050, "set:drc:foil_boosterfun:mythic", 525)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:dft:nonfoil_boosterfun:rare", 870, "set:dft:nonfoil_boosterfun:mythic", 130)},
					{Label: "Non-foil Booster Fun Rare or Mythic", Weighted: wb("set:dft:nonfoil_boosterfun:rare", 870, "set:dft:nonfoil_boosterfun:mythic", 130)},
					{Label: "Traditional Foil Booster Fun Rare or Mythic", Weighted: wb("set:dft:foil_boosterfun:rare", 746, "set:dft:foil_boosterfun:mythic", 112, "set:dft:japanshowcase:rare_or_mythic", 100, "set:spg:foil:mythic", 42)},
				},
			),
		},
	},
	"dsk": {
		SourceURL:       packOpeningSourceDuskmourn,
		RelatedSetCodes: []string{"dsc", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": packOpeningModernPlayBoosterConfig(
				packOpeningSourceDuskmourn,
				"https://media.wizards.com/2024/images/daily/en_2cd5365ea0.png",
				"dsk",
				nil,
				wb("set:dsk:default:common", 125, "set:dsk:default:uncommon", 625, "set:dsk:default:rare", 219, "set:dsk:default:mythic", 31),
				wb("set:dsk:default:rare", 844, "set:dsk:default:mythic", 136, "set:dsk:boosterfun:rare", 18, "set:dsk:boosterfun:mythic", 2),
				wb("set:dsk:foil_default:common", 580, "set:dsk:foil_default:uncommon", 320, "set:dsk:foil_default:rare", 80, "set:dsk:foil_default:mythic", 20),
				wb("set:dsk:default:land", 733, "set:dsk:fullart:land", 267),
			),
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceDuskmourn,
				"https://media.wizards.com/2024/images/daily/en_b1a729aa13.png",
				"dsk",
				5,
				4,
				[]packSlot{
					{Label: "Traditional Foil Full-art Manor Land", Bucket: "set:dsk:fullart:land"},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:dsk:foil_default:rare", 769, "set:dsk:foil_default:mythic", 128, "set:dsk:foil_boosterfun:rare", 90, "set:dsk:foil_boosterfun:mythic", 13)},
					{Label: "Duskmourn Commander", Weighted: wb("set:dsc:foil_boosterfun:mythic", 60, "set:dsc:nonfoil_boosterfun:mythic", 121, "set:dsc:extendedart:rare", 819)},
					{Label: "Non-foil Booster Fun or Extended Art", Weighted: wb("set:dsk:extendedart:rare", 188, "set:dsk:extendedart:mythic", 36, "set:dsk:borderless:rare", 144, "set:dsk:borderless:mythic", 40, "set:dsk:showcase:rare", 522, "set:dsk:showcase:mythic", 70)},
					{Label: "Non-foil Booster Fun or Extended Art", Weighted: wb("set:dsk:extendedart:rare", 188, "set:dsk:extendedart:mythic", 36, "set:dsk:borderless:rare", 144, "set:dsk:borderless:mythic", 40, "set:dsk:showcase:rare", 522, "set:dsk:showcase:mythic", 70)},
					{Label: "Traditional Foil Showcase Finish", Weighted: wb("set:dsk:foil_boosterfun:rare", 859, "set:dsk:foil_boosterfun:mythic", 141, "set:spg:foil:mythic", 31, "set:dsk:texturedfoil:mythic", 10, "set:dsk:fracturefoil:mythic", 10)},
				},
			),
		},
	},
	"blb": {
		SourceURL:       packOpeningSourceBloomburrow,
		RelatedSetCodes: []string{"blc", "spg"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": packOpeningModernPlayBoosterConfig(
				packOpeningSourceBloomburrow,
				"https://media.wizards.com/2024/images/daily/en_62561802ed.png",
				"blb",
				wb("set:blb:default:common", 985, "set:spg", 15),
				wb("set:blb:default:common", 125, "set:blb:default:uncommon", 625, "set:blb:default:rare", 220, "set:blb:default:mythic", 30),
				wb("set:blb:default:rare", 857, "set:blb:default:mythic", 144),
				wb("set:blb:foil_default:common", 580, "set:blb:foil_default:uncommon", 320, "set:blb:foil_default:rare", 80, "set:blb:foil_default:mythic", 20),
				wb("set:blb:fullart:land", 1000),
			),
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceBloomburrow,
				"https://media.wizards.com/2024/images/daily/en_049d3361dd.png",
				"blb",
				5,
				4,
				[]packSlot{
					{Label: "Traditional Foil Seasonal Land", Bucket: "set:blb:fullart:land"},
					{Label: "Traditional Foil Rare or Mythic", Weighted: wb("set:blb:foil_default:rare", 857, "set:blb:foil_default:mythic", 144)},
					{Label: "Bloomburrow Commander", Weighted: wb("set:blc:borderless:rare", 282, "set:blc:borderless:mythic", 1549, "set:blc:extendedart:rare", 7890, "set:blc:foil_boosterfun:mythic", 280)},
					{Label: "Non-foil Alternate-border Rare or Mythic", Weighted: wb("set:blb:nonfoil_boosterfun:rare", 870, "set:blb:nonfoil_boosterfun:mythic", 130)},
					{Label: "Non-foil Alternate-border Rare or Mythic", Weighted: wb("set:blb:nonfoil_boosterfun:rare", 870, "set:blb:nonfoil_boosterfun:mythic", 130)},
					{Label: "Traditional Foil Alternate-border Rare or Mythic", Weighted: wb("set:blb:foil_boosterfun:rare", 820, "set:blb:foil_boosterfun:mythic", 170, "set:blb:raisedfoil:mythic", 10)},
				},
			),
		},
	},
	"otj": {
		SourceURL:       packOpeningSourceOutlaws,
		RelatedSetCodes: []string{"otp", "big", "spg", "otc"},
		Packs: map[string]packOpeningConfiguredPack{
			"play": packOpeningModernPlayBoosterConfig(
				packOpeningSourceOutlaws,
				"https://media.wizards.com/2024/images/daily/en_immfozjk36yn.png",
				"otj",
				wb("set:otj:default:common", 800, "set:big", 156, "set:spg", 44),
				wb("set:otj:default:common", 367, "set:otj:default:uncommon", 333, "set:otj:default:rare", 150, "set:otj:default:mythic", 25, "set:otj:boosterfun:rare", 100, "set:otj:boosterfun:mythic", 25),
				wb("set:otj:default:rare", 812, "set:otj:default:mythic", 163, "set:otj:boosterfun:rare", 21, "set:otj:boosterfun:mythic", 4),
				wb("set:otj:foil:common", 580, "set:otj:foil:uncommon", 320, "set:otj:foil:rare", 80, "set:otj:foil:mythic", 20, "set:otp:foil:uncommon", 50, "set:otp:foil:rare_or_mythic", 25),
				wb("set:otj:land", 1000),
			),
			"collector": packOpeningCollectorBoosterConfig(
				packOpeningSourceOutlaws,
				"https://media.wizards.com/2024/images/daily/en_d246onezcgrt.png",
				"otj",
				4,
				3,
				[]packSlot{
					{Label: "Traditional Foil Western Landscape Land", Bucket: "set:otj:fullart:land"},
					{Label: "Non-foil Breaking News Uncommon", Bucket: "set:otp:nonfoil:uncommon"},
					{Label: "Traditional Foil Breaking News Uncommon", Bucket: "set:otp:foil:uncommon"},
					{Label: "Traditional Foil OTJ or BIG Rare or Mythic", Weighted: wb("set:otj:foil:rare", 700, "set:otj:foil:mythic", 125, "set:big:foil:mythic", 175)},
					{Label: "Non-foil Booster Fun or Extended Art", Weighted: wb("set:otj:nonfoil_boosterfun:rare", 860, "set:otj:nonfoil_boosterfun:mythic", 140, "set:big:nonfoil_boosterfun:mythic", 100)},
					{Label: "Outlaws Commander", Weighted: wb("set:otc:borderless:mythic", 89, "set:otc:extendedart:rare", 871, "set:otc:foil_boosterfun:mythic", 40)},
					{Label: "Non-foil Breaking News Rare or Mythic", Weighted: wb("set:otp:nonfoil:rare", 667, "set:otp:nonfoil:mythic", 333)},
					{Label: "Traditional Foil Splashy Pull", Weighted: wb("set:otj:foil_boosterfun:rare_or_mythic", 520, "set:otp:foil:rare_or_mythic", 290, "set:big:foil_boosterfun:mythic", 100, "set:spg:foil:mythic", 30, "set:otp:texturedfoil:mythic", 10, "set:big:raisedfoil:mythic", 5)},
				},
			),
		},
	},
}

func (a *App) HandlePackOpeningShow(w http.ResponseWriter, r *http.Request) {
	catalog, err := a.packOpeningCatalog(r.Context())
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	sets := catalog.Sets
	if len(sets) == 0 {
		setFlash(w, "No pack-ready sets are available yet.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	packTypes := catalog.PackTypes
	packTypeOptions := catalog.PackTypeOptions
	if len(packTypes) == 0 {
		setFlash(w, "No sourced pack configurations are available yet.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	a.Renderer.Render(w, "pack_opening", TemplateData{
		CurrentUser: CurrentUser(r),
		Data: packOpeningPageData{
			Sets:            sets,
			PackTypes:       packTypes,
			PackTypeOptions: packTypeOptions,
			DefaultSetCode:  sets[0].Code,
		},
		Flash: readFlash(w, r),
	})
}

func (a *App) HandlePackOpeningPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writePackOpeningError(w, http.StatusBadRequest, "Choose a set and pack type.")
		return
	}

	setCode := strings.TrimSpace(r.Form.Get("set_code"))
	packTypeID := strings.TrimSpace(r.Form.Get("pack_type"))
	cards, setOption, packType, err := generatePackOpening(r.Context(), a.DB, setCode, packTypeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writePackOpeningError(w, http.StatusNotFound, "That pack is not available.")
			return
		}
		if errors.Is(err, errPackOpeningNotEnoughCards) {
			writePackOpeningError(w, http.StatusBadRequest, "That set does not have enough pack-ready cards for this pack type.")
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(packOpeningResponse{
		OK:       true,
		Set:      setOption,
		PackType: packType,
		Cards:    cards,
	})
}

var errPackOpeningNotEnoughCards = errors.New("not enough cards for pack")

func (a *App) packOpeningCatalog(ctx context.Context) (*packOpeningCatalogSnapshot, error) {
	a.packCatalogMu.Lock()
	defer a.packCatalogMu.Unlock()

	if a.packCatalog != nil && time.Now().Before(a.packCatalog.ExpiresAt) {
		return a.packCatalog, nil
	}
	sets, err := listPackOpeningSets(ctx, a.DB)
	if err != nil {
		return nil, err
	}
	packTypes, packTypeOptions, err := packOpeningPackTypeOptions(ctx, a.DB, sets)
	if err != nil {
		return nil, err
	}
	availableSets := make([]packOpeningSetOption, 0, len(sets))
	for _, setOption := range sets {
		if len(packTypeOptions[strings.ToLower(setOption.Code)]) > 0 {
			availableSets = append(availableSets, setOption)
		}
	}
	a.packCatalog = &packOpeningCatalogSnapshot{
		ExpiresAt:       time.Now().Add(15 * time.Minute),
		Sets:            availableSets,
		PackTypes:       packTypes,
		PackTypeOptions: packTypeOptions,
	}
	return a.packCatalog, nil
}

func listPackOpeningSets(ctx context.Context, db *sql.DB) ([]packOpeningSetOption, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			lower(cp.set_code) AS set_code,
			COALESCE(NULLIF(max(cp.set_name), ''), upper(max(cp.set_code))) AS set_name,
			lower(COALESCE(NULLIF(max(cp.set_type), ''), '')) AS set_type,
			COALESCE(to_char(min(cp.released_at), 'YYYY-MM-DD'), '') AS released_at,
			count(*) AS card_count
		FROM card_prints cp
		JOIN oracle_cards oc
		  ON oc.oracle_id = cp.oracle_id
		WHERE cp.lang = 'en'
		  AND COALESCE(cp.image_uri, '') <> ''
		  AND NOT COALESCE(cp.digital, false)
		  AND NOT COALESCE(cp.variation, false)
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%promo%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%secret lair%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '% commander'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE 'commander 20%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE 'commander anthology%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE 'duel deck%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE 'world championship%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE 'premium deck%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%guild kit%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%game night%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%welcome deck%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%starter commander%'
		  AND lower(COALESCE(cp.set_name, '')) <> 'the list'
		  AND lower(COALESCE(cp.set_name, '')) <> 'unknown event'
		  AND lower(COALESCE(cp.rarity, '')) IN ('common', 'uncommon', 'rare', 'mythic')
		  AND lower(btrim(COALESCE(oc.type_line, ''))) <> 'card'
		  AND lower(COALESCE(oc.layout, '')) NOT IN ('token', 'double_faced_token')
		GROUP BY lower(cp.set_code)
		HAVING count(*) >= 14
		   AND count(*) FILTER (WHERE lower(COALESCE(cp.rarity, '')) = 'common') >= 6
		   AND count(*) FILTER (WHERE lower(COALESCE(cp.rarity, '')) = 'uncommon') >= 3
		   AND count(*) FILTER (WHERE lower(COALESCE(cp.rarity, '')) IN ('rare', 'mythic')) >= 1
		ORDER BY min(cp.released_at) DESC NULLS LAST, max(cp.set_name) ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []packOpeningSetOption
	for rows.Next() {
		var item packOpeningSetOption
		if err := rows.Scan(&item.Code, &item.Name, &item.SetType, &item.ReleasedAt, &item.CardCount); err != nil {
			return nil, err
		}
		if _, ok := packOpeningConfigForSet(item.Code, item.SetType, item.ReleasedAt, item.CardCount); !ok {
			continue
		}
		item.Label = item.Name
		if year := packOpeningYear(item.ReleasedAt); year != "" {
			item.Label += " (" + year + ")"
		}
		item.IconSVGURI = packOpeningSetIconSVGURI(item.Code)
		out = append(out, item)
	}
	return out, rows.Err()
}

func generatePackOpening(ctx context.Context, db *sql.DB, setCode string, packTypeID string) ([]packOpeningCard, packOpeningSetOption, packOpeningPackType, error) {
	setCode = strings.ToLower(strings.TrimSpace(setCode))
	if setCode == "" {
		return nil, packOpeningSetOption{}, packOpeningPackType{}, sql.ErrNoRows
	}
	setConfig, ok := packOpeningConfigForSetCode(ctx, db, setCode)
	if !ok {
		return nil, packOpeningSetOption{}, packOpeningPackType{}, sql.ErrNoRows
	}
	requestedPackType := strings.ToLower(strings.TrimSpace(packTypeID))
	if _, published := packOpeningPublishedProductFor(setCode, requestedPackType); requestedPackType != "" && !published {
		return nil, packOpeningSetOption{}, packOpeningPackType{}, sql.ErrNoRows
	}
	packConfig, packTypeID, ok := packOpeningConfiguredPackByID(setConfig, packTypeID)
	if _, published := packOpeningPublishedProductFor(setCode, packTypeID); !ok || !published {
		return nil, packOpeningSetOption{}, packOpeningPackType{}, sql.ErrNoRows
	}
	packType, ok := packOpeningConfiguredPackType(packTypeID, packConfig)
	if !ok {
		return nil, packOpeningSetOption{}, packOpeningPackType{}, sql.ErrNoRows
	}

	candidates, setOption, err := loadPackOpeningCandidates(ctx, db, setCode, packOpeningConfigSetCodes(setCode, setConfig))
	if err != nil {
		return nil, packOpeningSetOption{}, packOpeningPackType{}, err
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	picker := newPackOpeningPicker(candidates, rng)
	slots := packConfig.Slots
	cards := make([]packOpeningCard, 0, len(slots))
	for _, slot := range slots {
		card, ok := picker.pickSlot(slot)
		if !ok {
			return nil, packOpeningSetOption{}, packOpeningPackType{}, errPackOpeningNotEnoughCards
		}
		card.SlotLabel = slot.Label
		cards = append(cards, card.packOpeningCard)
	}

	return cards, setOption, packType, nil
}

func packOpeningPackTypeByID(id string) (packOpeningPackType, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, packType := range packOpeningPackTypes {
		if packType.ID == id {
			return packType, true
		}
	}
	return packOpeningPackType{}, false
}

func packOpeningPackTypeOptions(ctx context.Context, db *sql.DB, sets []packOpeningSetOption) ([]packOpeningPackType, map[string][]packOpeningPackType, error) {
	options := map[string][]packOpeningPackType{}
	seen := map[string]bool{}
	var pageTypes []packOpeningPackType
	for _, setOption := range sets {
		setCode := strings.ToLower(setOption.Code)
		config, ok := packOpeningConfigForSet(setCode, setOption.SetType, setOption.ReleasedAt, setOption.CardCount)
		if !ok {
			continue
		}
		candidates, _, err := loadPackOpeningCandidates(ctx, db, setCode, packOpeningConfigSetCodes(setCode, config))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, nil, err
		}
		for _, baseType := range packOpeningPackTypes {
			if _, published := packOpeningPublishedProductFor(setCode, baseType.ID); !published {
				continue
			}
			packConfig, ok := config.Packs[baseType.ID]
			if !ok {
				continue
			}
			if !packOpeningPackConfigAvailable(candidates, packConfig) {
				continue
			}
			packType, ok := packOpeningConfiguredPackType(baseType.ID, packConfig)
			if !ok {
				continue
			}
			options[setCode] = append(options[setCode], packType)
			if !seen[packType.ID] {
				pageTypes = append(pageTypes, packType)
				seen[packType.ID] = true
			}
		}
	}
	return pageTypes, options, nil
}

func packOpeningPackConfigAvailable(candidates []packOpeningCandidate, packConfig packOpeningConfiguredPack) bool {
	rng := rand.New(rand.NewSource(1))
	picker := newPackOpeningPicker(candidates, rng)
	for bucket, minimum := range packConfig.RequiredPools {
		if minimum <= 0 || len(picker.buckets[strings.ToLower(strings.TrimSpace(bucket))]) < minimum {
			return false
		}
	}
	for bucket, expected := range packConfig.ExpectedPools {
		if expected <= 0 || len(picker.buckets[strings.ToLower(strings.TrimSpace(bucket))]) != expected {
			return false
		}
	}
	for _, slot := range packConfig.Slots {
		if _, ok := picker.pickSlot(slot); !ok {
			return false
		}
	}
	return true
}

func packOpeningConfiguredPackByID(config packOpeningSetConfig, id string) (packOpeningConfiguredPack, string, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id != "" {
		packConfig, ok := config.Packs[id]
		return packConfig, id, ok
	}
	for _, baseType := range packOpeningPackTypes {
		if packConfig, ok := config.Packs[baseType.ID]; ok {
			return packConfig, baseType.ID, true
		}
	}
	return packOpeningConfiguredPack{}, "", false
}

func packOpeningConfiguredPackType(id string, packConfig packOpeningConfiguredPack) (packOpeningPackType, bool) {
	packType, ok := packOpeningPackTypeByID(id)
	if !ok {
		return packOpeningPackType{}, false
	}
	packType.PackArtURI = packConfig.PackArtURI
	packType.PackArtSize = packConfig.PackArtSize
	packType.PackArtPosition = packConfig.PackArtPosition
	packType.SourceURL = packConfig.SourceURL
	packType.Accuracy = strings.ToLower(strings.TrimSpace(packConfig.Accuracy))
	if packType.Accuracy == "" {
		packType.Accuracy = packOpeningAccuracySourced
	}
	packType.AccuracyLabel = packOpeningAccuracyLabel(packType.Accuracy)
	packType.AccuracySummary = strings.TrimSpace(packConfig.AccuracySummary)
	if packType.AccuracySummary == "" {
		packType.AccuracySummary = packOpeningAccuracySummary(packType.Accuracy)
	}
	packType.SlotRecipe = packOpeningSlotRecipe(packConfig.Slots)
	packType.Limitations = append([]string{}, packConfig.Limitations...)
	if len(packType.Limitations) == 0 && packType.Accuracy == packOpeningAccuracySourced {
		packType.Limitations = []string{"Factory print-sheet sequencing and sealed-product correlations are not modeled."}
	} else if packType.Limitations == nil {
		packType.Limitations = []string{}
	}
	if packConfig.CardCount > 0 {
		packType.CardCount = packConfig.CardCount
	} else if len(packConfig.Slots) > 0 {
		packType.CardCount = len(packConfig.Slots)
	}
	return packType, true
}

func packOpeningSlotRecipe(slots []packSlot) []string {
	counts := map[string]int{}
	order := make([]string, 0, len(slots))
	for _, slot := range slots {
		label := strings.TrimSpace(slot.Label)
		if label == "" {
			label = "Card"
		}
		if counts[label] == 0 {
			order = append(order, label)
		}
		counts[label]++
	}
	recipe := make([]string, 0, len(order))
	for _, label := range order {
		if counts[label] == 1 {
			recipe = append(recipe, label)
			continue
		}
		recipe = append(recipe, fmt.Sprintf("%d× %s", counts[label], label))
	}
	return recipe
}

func packOpeningConfigForSetCode(ctx context.Context, db *sql.DB, setCode string) (packOpeningSetConfig, bool) {
	setCode = strings.ToLower(strings.TrimSpace(setCode))
	if config, ok := packOpeningConfigForSet(setCode, "", "", 0); ok {
		return config, true
	}
	return packOpeningSetConfig{}, false
}

func packOpeningConfigSetCodes(primarySetCode string, config packOpeningSetConfig) []string {
	seen := map[string]bool{}
	var out []string
	for _, code := range append([]string{primarySetCode}, config.RelatedSetCodes...) {
		code = strings.ToLower(strings.TrimSpace(code))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out
}

func loadPackOpeningCandidates(ctx context.Context, db *sql.DB, primarySetCode string, setCodes []string) ([]packOpeningCandidate, packOpeningSetOption, error) {
	primarySetCode = strings.ToLower(strings.TrimSpace(primarySetCode))
	setCodes = packOpeningConfigSetCodes(primarySetCode, packOpeningSetConfig{RelatedSetCodes: setCodes})
	if len(setCodes) == 0 {
		return nil, packOpeningSetOption{}, sql.ErrNoRows
	}
	placeholders := make([]string, 0, len(setCodes))
	args := make([]any, 0, len(setCodes))
	for i, code := range setCodes {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, code)
	}

	query := `
		SELECT
			cp.scryfall_id::text,
			oc.oracle_id::text,
			cp.name,
			COALESCE(cp.image_uri, ''),
			COALESCE(cp.set_name, ''),
			cp.set_code,
			COALESCE(cp.collector_number, ''),
			lower(COALESCE(cp.rarity, '')),
			COALESCE(cp.price_usd, ''),
			COALESCE(NULLIF(cp.price_usd_nonfoil, ''), cp.price_usd, ''),
			COALESCE(cp.price_usd_foil, ''),
			COALESCE(cp.price_usd_etched, ''),
			lower(COALESCE(oc.layout, '')),
			COALESCE(oc.type_line, ''),
			COALESCE(cp.finishes_json, '[]'::jsonb)::text,
			COALESCE(cp.frame_effects_json, '[]'::jsonb)::text,
			COALESCE(cp.promo_types_json, '[]'::jsonb)::text,
			lower(COALESCE(cp.border_color, '')),
			lower(COALESCE(cp.frame, '')),
			lower(COALESCE(cp.security_stamp, '')),
			COALESCE(cp.full_art, false),
			COALESCE(cp.textless, false),
			COALESCE(cp.booster, false),
			COALESCE(cp.digital, false),
			COALESCE(cp.variation, false),
			COALESCE(to_char(cp.released_at, 'YYYY-MM-DD'), '')
		FROM card_prints cp
		JOIN oracle_cards oc
		  ON oc.oracle_id = cp.oracle_id
		WHERE cp.set_code IN (` + strings.Join(placeholders, ",") + `)
		  AND (
			cp.lang = 'en'
			OR (
				cp.lang = 'dw'
				AND cp.set_code = 'hoc'
				AND cp.collector_number IN ('93', '94', '95', '96', '97')
			)
		  )
		  AND COALESCE(cp.image_uri, '') <> ''
		  AND NOT COALESCE(cp.digital, false)
		  AND NOT COALESCE(cp.variation, false)
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%promo%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%secret lair%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE 'duel deck%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE 'world championship%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE 'premium deck%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%guild kit%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%game night%'
		  AND lower(COALESCE(cp.set_name, '')) NOT LIKE '%welcome deck%'
		  AND lower(COALESCE(cp.set_name, '')) <> 'the list'
		  AND lower(COALESCE(cp.set_name, '')) <> 'unknown event'
		  AND lower(COALESCE(cp.rarity, '')) IN ('common', 'uncommon', 'rare', 'mythic')
		  AND lower(btrim(COALESCE(oc.type_line, ''))) <> 'card'
		  AND lower(COALESCE(oc.layout, '')) NOT IN ('token', 'double_faced_token')
		ORDER BY cp.set_code = $1 DESC, cp.set_code ASC, cp.collector_number ASC, cp.name ASC
	`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, packOpeningSetOption{}, err
	}
	defer rows.Close()

	var candidates []packOpeningCandidate
	var setOption packOpeningSetOption
	primaryCount := 0
	for rows.Next() {
		var card packOpeningCandidate
		var releasedAt string
		var finishesJSON string
		var frameEffectsJSON string
		var promoTypesJSON string
		if err := rows.Scan(
			&card.ID,
			&card.OracleID,
			&card.Name,
			&card.ImageURI,
			&card.SetName,
			&card.SetCode,
			&card.CollectorNumber,
			&card.Rarity,
			&card.PriceUSD,
			&card.PriceUSDNonfoil,
			&card.PriceUSDFoil,
			&card.PriceUSDEtched,
			&card.Layout,
			&card.TypeLine,
			&finishesJSON,
			&frameEffectsJSON,
			&promoTypesJSON,
			&card.BorderColor,
			&card.Frame,
			&card.SecurityStamp,
			&card.FullArt,
			&card.Textless,
			&card.Booster,
			&card.Digital,
			&card.Variation,
			&releasedAt,
		); err != nil {
			return nil, packOpeningSetOption{}, err
		}
		card.Finishes = packOpeningStringListJSON(finishesJSON)
		card.FrameEffects = packOpeningStringListJSON(frameEffectsJSON)
		card.PromoTypes = packOpeningStringListJSON(promoTypesJSON)
		card.ReleasedAt = releasedAt
		card.DetailPath = cardPrintingDetailPath(card.OracleID, card.ID)
		candidates = append(candidates, card)
		if card.SetCode == primarySetCode {
			primaryCount++
		}
		if card.SetCode == primarySetCode {
			if setOption.Code == "" {
				setOption = packOpeningSetOption{
					Code: card.SetCode,
					Name: card.SetName,
				}
			}
			if releasedAt != "" && (setOption.ReleasedAt == "" || releasedAt < setOption.ReleasedAt) {
				setOption.ReleasedAt = releasedAt
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, packOpeningSetOption{}, err
	}
	if len(candidates) == 0 {
		return nil, packOpeningSetOption{}, sql.ErrNoRows
	}
	if primaryCount == 0 || setOption.Code == "" {
		return nil, packOpeningSetOption{}, sql.ErrNoRows
	}
	setOption.CardCount = primaryCount
	setOption.Label = setOption.Name
	if year := packOpeningYear(setOption.ReleasedAt); year != "" {
		setOption.Label += " (" + year + ")"
	}
	setOption.IconSVGURI = packOpeningSetIconSVGURI(setOption.Code)
	return candidates, setOption, nil
}

type packOpeningPicker struct {
	rng     *rand.Rand
	buckets map[string][]packOpeningCandidate
	used    map[string]bool
}

func newPackOpeningPicker(candidates []packOpeningCandidate, rng *rand.Rand) *packOpeningPicker {
	picker := &packOpeningPicker{
		rng:     rng,
		buckets: map[string][]packOpeningCandidate{},
		used:    map[string]bool{},
	}
	setHasBoosterCards := map[string]bool{}
	for _, card := range candidates {
		if card.Digital || card.Variation || !card.Booster {
			continue
		}
		setHasBoosterCards[strings.ToLower(strings.TrimSpace(card.SetCode))] = true
	}
	for _, card := range candidates {
		if card.Digital || card.Variation {
			continue
		}
		rarity := strings.ToLower(strings.TrimSpace(card.Rarity))
		setCode := strings.ToLower(strings.TrimSpace(card.SetCode))
		ordinaryEligible := card.Booster || !setHasBoosterCards[setCode]
		isRareOrMythic := rarity == "rare" || rarity == "mythic"
		isCommonOrUncommon := rarity == "common" || rarity == "uncommon"
		typeLine := strings.ToLower(card.TypeLine)
		isLand := strings.Contains(typeLine, "land")
		isTurtle := strings.Contains(typeLine, "turtle")
		isLegendary := strings.Contains(typeLine, "legendary")
		isBattle := strings.Contains(typeLine, "battle")
		isDFC := card.Layout == "transform" || card.Layout == "modal_dfc" || card.Layout == "reversible_card"
		addBucket := func(bucket string) {
			bucket = strings.ToLower(strings.TrimSpace(bucket))
			if bucket == "" || (!ordinaryEligible && packOpeningOrdinaryBucket(bucket)) {
				return
			}
			picker.buckets[bucket] = append(picker.buckets[bucket], card)
		}
		packOpeningAddProductBuckets(card, setCode, rarity, addBucket)

		addBucket(rarity)
		addBucket("wildcard")
		if setCode != "" {
			addBucket("set:" + setCode)
			addBucket("set:" + setCode + ":" + rarity)
			addBucket("set:" + setCode + ":wildcard")
			if card.ReleasedAt != "" {
				addBucket("set:" + setCode + ":release:" + card.ReleasedAt)
				addBucket("set:" + setCode + ":release:" + card.ReleasedAt + ":" + rarity)
			}
		}
		if isRareOrMythic {
			addBucket("rare_or_mythic")
			if setCode != "" {
				addBucket("set:" + setCode + ":rare_or_mythic")
			}
		}
		if isCommonOrUncommon {
			addBucket("common_or_uncommon")
			if setCode != "" {
				addBucket("set:" + setCode + ":common_or_uncommon")
			}
		}
		if isLand {
			addBucket("land")
			if setCode != "" {
				addBucket("set:" + setCode + ":land")
			}
		}
		if isTurtle {
			addBucket("turtle")
			addBucket("turtle:" + rarity)
			if setCode != "" {
				addBucket("set:" + setCode + ":turtle")
				addBucket("set:" + setCode + ":turtle:" + rarity)
			}
		}
		if isLegendary {
			addBucket("legendary")
			addBucket("legendary:" + rarity)
			if setCode != "" {
				addBucket("set:" + setCode + ":legendary")
				addBucket("set:" + setCode + ":legendary:" + rarity)
			}
		} else {
			addBucket("nonlegendary")
			addBucket("nonlegendary:" + rarity)
			if setCode != "" {
				addBucket("set:" + setCode + ":nonlegendary")
				addBucket("set:" + setCode + ":nonlegendary:" + rarity)
			}
		}
		if isBattle {
			addBucket("battle")
			addBucket("battle:" + rarity)
			if setCode != "" {
				addBucket("set:" + setCode + ":battle")
				addBucket("set:" + setCode + ":battle:" + rarity)
			}
		} else {
			addBucket("nonbattle")
			addBucket("nonbattle:" + rarity)
			if setCode != "" {
				addBucket("set:" + setCode + ":nonbattle")
				addBucket("set:" + setCode + ":nonbattle:" + rarity)
			}
		}
		if isDFC {
			addBucket("dfc")
			addBucket("dfc:" + rarity)
			if isCommonOrUncommon {
				addBucket("dfc:common_or_uncommon")
			}
			if setCode != "" {
				addBucket("set:" + setCode + ":dfc")
				addBucket("set:" + setCode + ":dfc:" + rarity)
				if isCommonOrUncommon {
					addBucket("set:" + setCode + ":dfc:common_or_uncommon")
				}
			}
			if !isBattle {
				addBucket("nonbattle_dfc")
				addBucket("nonbattle_dfc:" + rarity)
				if isCommonOrUncommon {
					addBucket("nonbattle_dfc:common_or_uncommon")
				}
				if setCode != "" {
					addBucket("set:" + setCode + ":nonbattle_dfc")
					addBucket("set:" + setCode + ":nonbattle_dfc:" + rarity)
					if isCommonOrUncommon {
						addBucket("set:" + setCode + ":nonbattle_dfc:common_or_uncommon")
					}
				}
			}
		} else {
			addBucket("singleface")
			addBucket("singleface:" + rarity)
			if isCommonOrUncommon {
				addBucket("singleface:common_or_uncommon")
			}
			if setCode != "" {
				addBucket("set:" + setCode + ":singleface")
				addBucket("set:" + setCode + ":singleface:" + rarity)
				if isCommonOrUncommon {
					addBucket("set:" + setCode + ":singleface:common_or_uncommon")
				}
			}
		}
		if setCode != "" {
			for _, treatment := range packOpeningTreatments(card) {
				prefix := "set:" + setCode + ":" + treatment + ":"
				addBucket("set:" + setCode + ":" + treatment)
				// A default-frame rarity slot represents a spell slot, not a
				// basic/dual-land slot. Scryfall marks some new sets' entire
				// physical run booster=false, so this semantic guard is needed
				// even when the booster flag cannot distinguish the print sheet.
				defaultTreatment := treatment == "default" || treatment == "nonfoil_default" || treatment == "foil_default"
				if !isLand || !defaultTreatment {
					addBucket(prefix + "wildcard")
					addBucket(prefix + rarity)
					if isRareOrMythic {
						addBucket(prefix + "rare_or_mythic")
					}
					if isCommonOrUncommon {
						addBucket(prefix + "common_or_uncommon")
					}
					if isTurtle {
						addBucket(prefix + "turtle")
						addBucket(prefix + "turtle:" + rarity)
					}
				}
				if isLand {
					addBucket(prefix + "land")
				}
			}
		}
	}
	for key := range picker.buckets {
		picker.rng.Shuffle(len(picker.buckets[key]), func(i, j int) {
			picker.buckets[key][i], picker.buckets[key][j] = picker.buckets[key][j], picker.buckets[key][i]
		})
	}
	return picker
}

func packOpeningStringListJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.ReplaceAll(value, "-", "")
		value = strings.ReplaceAll(value, "_", "")
		value = strings.ReplaceAll(value, " ", "")
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func packOpeningOrdinaryBucket(bucket string) bool {
	bucket = strings.ToLower(strings.TrimSpace(bucket))
	for _, treatment := range []string{
		"boosterfun", "extendedart", "borderless", "showcase", "serialized",
		"surgefoil", "raisedfoil", "texturedfoil", "fracturefoil", "japanshowcase",
		"retro", "poster", "manafoil", "invisibleink", "neonink", "confettifoil",
		"halofoil", "stepandcompleat", "fullart", "textless", "etched", "sourcematerial",
		"play_dual", "rooftop", "spiderweb", "default_basic", "journey",
		"scene", "dragonhoard", "bookcover", "classicartist", "dwarvish",
	} {
		// Treatments can be compound bucket segments such as
		// "foil_boosterfun" or "nonfoil_boosterfun". Matching only a
		// colon-prefixed treatment misclassifies those as ordinary pools and
		// drops their non-booster-marked printings.
		if strings.Contains(bucket, treatment) {
			return false
		}
	}
	return true
}

func packOpeningTreatments(card packOpeningCandidate) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(treatment string) {
		treatment = strings.ToLower(strings.TrimSpace(treatment))
		if treatment == "" || seen[treatment] {
			return
		}
		seen[treatment] = true
		out = append(out, treatment)
	}

	boosterFun := packOpeningIsBoosterFun(card)
	supplementalPromo := packOpeningHas(card.PromoTypes, "bundle", "buyabox", "prerelease", "headliner", "promopack")
	if !boosterFun && !supplementalPromo {
		add("default")
		if packOpeningHas(card.Finishes, "nonfoil") {
			add("nonfoil_default")
		}
		if packOpeningHas(card.Finishes, "foil") {
			add("foil_default")
		}
	}
	if card.Booster {
		add("booster")
	}
	if packOpeningHas(card.Finishes, "nonfoil") {
		add("nonfoil")
	}
	if packOpeningHas(card.Finishes, "foil") {
		add("foil")
	}
	if packOpeningHas(card.Finishes, "etched") {
		add("etched")
	}
	if boosterFun {
		add("boosterfun")
		if packOpeningHas(card.Finishes, "nonfoil") {
			add("nonfoil_boosterfun")
		}
		if packOpeningHas(card.Finishes, "foil") {
			add("foil_boosterfun")
		}
		if packOpeningHas(card.Finishes, "etched") {
			add("etched_boosterfun")
		}
	}
	if card.FullArt {
		add("fullart")
	}
	if card.Textless {
		add("textless")
	}
	if packOpeningHasTreatment(card, "borderless") {
		add("borderless")
	}
	if packOpeningHasTreatment(card, "showcase") {
		add("showcase")
	}
	if packOpeningHasTreatment(card, "extendedart", "extended") {
		add("extendedart")
	}
	if packOpeningHasTreatment(card, "serialized") {
		add("serialized")
	}
	if packOpeningHasTreatment(card, "surgefoil", "surge") {
		add("surgefoil")
		if boosterFun {
			add("surgefoil_boosterfun")
		}
	}
	if packOpeningHasTreatment(card, "raisedfoil", "raised") {
		add("raisedfoil")
	}
	if packOpeningHasTreatment(card, "texturedfoil", "textured") {
		add("texturedfoil")
	}
	if packOpeningHasTreatment(card, "fracturefoil", "fracture") {
		add("fracturefoil")
	}
	if packOpeningHasTreatment(card, "japanshowcase") {
		add("japanshowcase")
	}
	if packOpeningHasTreatment(card, "retro") {
		add("retro")
	}
	if packOpeningHasTreatment(card, "poster", "movieposter") {
		add("poster")
	}
	if packOpeningHasTreatment(card, "manafoil") {
		add("manafoil")
	}
	if packOpeningHasTreatment(card, "invisibleink") {
		add("invisibleink")
	}
	if packOpeningHasTreatment(card, "neonink") {
		add("neonink")
	}
	if packOpeningHasTreatment(card, "confettifoil", "confetti") {
		add("confettifoil")
	}
	if packOpeningHasTreatment(card, "halofoil", "halo") {
		add("halofoil")
	}
	if packOpeningHasTreatment(card, "stepandcompleat") {
		add("stepandcompleat")
	}
	if packOpeningHasTreatment(card, "sourcematerial") {
		add("sourcematerial")
	}
	return out
}

func packOpeningIsBoosterFun(card packOpeningCandidate) bool {
	if card.FullArt || card.Textless || card.Variation {
		return true
	}
	return packOpeningHasTreatment(card,
		"boosterfun",
		"borderless",
		"showcase",
		"extendedart",
		"extended",
		"retro",
		"serialized",
		"surge",
		"surgefoil",
		"raised",
		"raisedfoil",
		"textured",
		"texturedfoil",
		"fracture",
		"fracturefoil",
		"japanshowcase",
		"neonink",
		"poster",
		"stepandcompleat",
		"ripplefoil",
	)
}

func packOpeningHasTreatment(card packOpeningCandidate, needles ...string) bool {
	if packOpeningHas(card.FrameEffects, needles...) || packOpeningHas(card.PromoTypes, needles...) || packOpeningHas([]string{card.SecurityStamp, card.BorderColor, card.Frame}, needles...) {
		return true
	}
	return false
}

func packOpeningHas(values []string, needles ...string) bool {
	if len(values) == 0 || len(needles) == 0 {
		return false
	}
	needleSet := map[string]bool{}
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		needle = strings.ReplaceAll(needle, "-", "")
		needle = strings.ReplaceAll(needle, "_", "")
		needle = strings.ReplaceAll(needle, " ", "")
		if needle != "" {
			needleSet[needle] = true
		}
	}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.ReplaceAll(value, "-", "")
		value = strings.ReplaceAll(value, "_", "")
		value = strings.ReplaceAll(value, " ", "")
		if needleSet[value] {
			return true
		}
	}
	return false
}

func (p *packOpeningPicker) pickSlot(slot packSlot) (packOpeningCandidate, bool) {
	selectedFinish := ""
	if len(slot.FinishWeighted) > 0 {
		finish, ok := p.pickFinish(slot.FinishWeighted)
		if !ok {
			return packOpeningCandidate{}, false
		}
		selectedFinish = finish
	}

	var bucket string
	var ok bool
	if len(slot.Weighted) > 0 {
		bucket, ok = p.pickWeightedSlotBucket(slot, selectedFinish)
	} else {
		bucket, ok = p.resolveSlotBucket(slot, selectedFinish)
	}
	if !ok {
		return packOpeningCandidate{}, false
	}

	eligible := make([]packOpeningCandidate, 0, len(p.available(bucket)))
	for _, candidate := range p.available(bucket) {
		finished, finishOK := p.finishSlotCandidate(candidate, slot, bucket, selectedFinish)
		if !finishOK || p.used[packOpeningCardFinishKey(finished)] {
			continue
		}
		eligible = append(eligible, finished)
	}
	if len(eligible) == 0 {
		return packOpeningCandidate{}, false
	}
	card := eligible[p.rng.Intn(len(eligible))]
	p.used[packOpeningCardFinishKey(card)] = true
	return card, true
}

func (p *packOpeningPicker) finishSlotCandidate(card packOpeningCandidate, slot packSlot, bucket string, selectedFinish string) (packOpeningCandidate, bool) {
	if selectedFinish != "" {
		return packOpeningApplySelectedFinish(card, selectedFinish)
	}
	return packOpeningApplyFinish(card, slot.Label, bucket)
}

func (p *packOpeningPicker) pickWeightedSlotBucket(slot packSlot, selectedFinish string) (string, bool) {
	total := 0
	available := make([]weightedPackBucket, 0, len(slot.Weighted))
	for _, weighted := range slot.Weighted {
		if weighted.weight <= 0 || !p.slotBucketPresent(weighted.bucket, slot, selectedFinish) {
			// Never silently renormalize a published recipe around a pool that
			// is absent from the synced data.
			return "", false
		}
		if !p.slotBucketAvailable(weighted.bucket, slot, selectedFinish) {
			// Duplicate prevention can exhaust a tiny treatment family after an
			// earlier repeated slot selected it. Condition the remaining draw on
			// the still-eligible branches instead of failing the whole pack.
			continue
		}
		available = append(available, weighted)
		total += weighted.weight
	}
	if total <= 0 || len(available) == 0 {
		return "", false
	}
	roll := p.rng.Intn(total)
	for _, weighted := range available {
		if roll < weighted.weight {
			return weighted.bucket, true
		}
		roll -= weighted.weight
	}
	return available[len(available)-1].bucket, true
}

func (p *packOpeningPicker) resolveSlotBucket(slot packSlot, selectedFinish string) (string, bool) {
	bucket := strings.ToLower(strings.TrimSpace(slot.Bucket))
	prefix := packOpeningBucketPrefix(bucket)
	switch {
	case bucket == "rare_or_mythic" || strings.HasSuffix(bucket, ":rare_or_mythic"):
		mythicBucket := prefix + "mythic"
		rareBucket := prefix + "rare"
		if p.rng.Intn(8) == 0 && p.slotBucketAvailable(mythicBucket, slot, selectedFinish) {
			return mythicBucket, true
		}
		if p.slotBucketAvailable(rareBucket, slot, selectedFinish) {
			return rareBucket, true
		}
		return "", false
	case bucket == "wildcard" || strings.HasSuffix(bucket, ":wildcard"):
		wildcard := slot
		wildcard.Weighted = []weightedPackBucket{
			{bucket: prefix + "common", weight: 40},
			{bucket: prefix + "uncommon", weight: 35},
			{bucket: prefix + "rare", weight: 20},
			{bucket: prefix + "mythic", weight: 5},
		}
		return p.pickWeightedSlotBucket(wildcard, selectedFinish)
	case bucket == "foil_wildcard" || strings.HasSuffix(bucket, ":foil_wildcard"):
		wildcard := slot
		wildcard.Weighted = []weightedPackBucket{
			{bucket: prefix + "common", weight: 48},
			{bucket: prefix + "uncommon", weight: 32},
			{bucket: prefix + "rare", weight: 16},
			{bucket: prefix + "mythic", weight: 4},
		}
		return p.pickWeightedSlotBucket(wildcard, selectedFinish)
	default:
		if p.slotBucketAvailable(bucket, slot, selectedFinish) {
			return bucket, true
		}
		return "", false
	}
}

func (p *packOpeningPicker) slotBucketAvailable(bucket string, slot packSlot, selectedFinish string) bool {
	for _, candidate := range p.available(bucket) {
		finished, ok := p.finishSlotCandidate(candidate, slot, bucket, selectedFinish)
		if ok && !p.used[packOpeningCardFinishKey(finished)] {
			return true
		}
	}
	return false
}

func (p *packOpeningPicker) slotBucketPresent(bucket string, slot packSlot, selectedFinish string) bool {
	for _, candidate := range p.available(bucket) {
		if _, ok := p.finishSlotCandidate(candidate, slot, bucket, selectedFinish); ok {
			return true
		}
	}
	return false
}

func packOpeningCardFinishKey(card packOpeningCandidate) string {
	identity := strings.ToLower(strings.TrimSpace(card.OracleID))
	if identity == "" {
		identity = strings.ToLower(strings.TrimSpace(card.Name))
	}
	if identity == "" {
		identity = strings.ToLower(strings.TrimSpace(card.ID))
	}
	return identity + "\x00" + strings.ToLower(strings.TrimSpace(card.Finish))
}

func (p *packOpeningPicker) pickFinish(finishes []weightedPackFinish) (string, bool) {
	total := 0
	for _, finish := range finishes {
		if finish.weight <= 0 {
			return "", false
		}
		total += finish.weight
	}
	if total <= 0 {
		return "", false
	}
	roll := p.rng.Intn(total)
	for _, finish := range finishes {
		if roll < finish.weight {
			return finish.finish, true
		}
		roll -= finish.weight
	}
	return finishes[len(finishes)-1].finish, true
}

func packOpeningApplyFinish(card packOpeningCandidate, slotLabel string, bucket string) (packOpeningCandidate, bool) {
	label := strings.ToLower(strings.TrimSpace(slotLabel))
	bucket = strings.ToLower(strings.TrimSpace(bucket))
	finish := ""
	switch {
	case strings.Contains(label, "non-foil") || strings.Contains(label, "nonfoil"):
		finish = "nonfoil"
	case strings.Contains(label, "etched"):
		finish = "etched"
	case strings.Contains(label, "foil"):
		finish = "foil"
	case strings.Contains(bucket, ":nonfoil"):
		finish = "nonfoil"
	case strings.Contains(bucket, ":etched"):
		finish = "etched"
	case strings.Contains(bucket, "foil"):
		finish = "foil"
	case packOpeningHas(card.Finishes, "nonfoil"):
		finish = "nonfoil"
	case packOpeningHas(card.Finishes, "foil"):
		finish = "foil"
	case packOpeningHas(card.Finishes, "etched"):
		finish = "etched"
	default:
		return packOpeningCandidate{}, false
	}
	return packOpeningApplySelectedFinish(card, finish)
}

func packOpeningApplySelectedFinish(card packOpeningCandidate, finish string) (packOpeningCandidate, bool) {
	finish = strings.ToLower(strings.TrimSpace(finish))
	if !packOpeningHas(card.Finishes, finish) {
		return packOpeningCandidate{}, false
	}
	card.Finish = finish
	switch finish {
	case "foil":
		card.PriceUSD = strings.TrimSpace(card.PriceUSDFoil)
	case "etched":
		card.PriceUSD = strings.TrimSpace(card.PriceUSDEtched)
	default:
		price := strings.TrimSpace(card.PriceUSDNonfoil)
		if price == "" {
			// price_usd was the pre-finish schema's nonfoil price. Keep existing
			// installations useful while sync version 6 backfills the split fields.
			price = strings.TrimSpace(card.PriceUSD)
		}
		card.PriceUSD = price
	}
	return card, true
}

func (p *packOpeningPicker) pick(bucket string) (packOpeningCandidate, bool) {
	bucket = strings.ToLower(strings.TrimSpace(bucket))
	prefix := packOpeningBucketPrefix(bucket)
	switch {
	case bucket == "rare_or_mythic" || strings.HasSuffix(bucket, ":rare_or_mythic"):
		if p.rng.Intn(8) == 0 {
			if card, ok := p.pickDirect(prefix + "mythic"); ok {
				return card, true
			}
		}
		return p.pickDirect(prefix + "rare")
	case bucket == "wildcard" || strings.HasSuffix(bucket, ":wildcard"):
		return p.pickWeighted([]weightedPackBucket{
			{bucket: prefix + "common", weight: 40},
			{bucket: prefix + "uncommon", weight: 35},
			{bucket: prefix + "rare", weight: 20},
			{bucket: prefix + "mythic", weight: 5},
		})
	case bucket == "foil_wildcard" || strings.HasSuffix(bucket, ":foil_wildcard"):
		return p.pickWeighted([]weightedPackBucket{
			{bucket: prefix + "common", weight: 48},
			{bucket: prefix + "uncommon", weight: 32},
			{bucket: prefix + "rare", weight: 16},
			{bucket: prefix + "mythic", weight: 4},
		})
	case bucket == "land" || strings.HasSuffix(bucket, ":land"):
		return p.pickDirect(prefix + "land")
	default:
		return p.pickDirect(bucket)
	}
}

func packOpeningBucketPrefix(bucket string) string {
	for _, suffix := range []string{"rare_or_mythic", "foil_wildcard", "wildcard", "common_or_uncommon", "common", "uncommon", "rare", "mythic", "land"} {
		if strings.HasSuffix(bucket, ":"+suffix) {
			return strings.TrimSuffix(bucket, suffix)
		}
		if bucket == suffix {
			return ""
		}
	}
	return ""
}

type weightedPackBucket struct {
	bucket string
	weight int
}

func wb(values ...any) []weightedPackBucket {
	if len(values)%2 != 0 {
		panic("pack opening bucket weights must be bucket/weight pairs")
	}
	out := make([]weightedPackBucket, 0, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		bucket, ok := values[i].(string)
		if !ok || strings.TrimSpace(bucket) == "" {
			panic(fmt.Sprintf("pack opening bucket weight %d has an invalid bucket", i/2+1))
		}
		weight, ok := values[i+1].(int)
		if !ok || weight <= 0 {
			panic(fmt.Sprintf("pack opening bucket %q has an invalid weight", bucket))
		}
		out = append(out, weightedPackBucket{bucket: strings.ToLower(strings.TrimSpace(bucket)), weight: weight})
	}
	return out
}

func wf(values ...any) []weightedPackFinish {
	if len(values)%2 != 0 {
		panic("pack opening finish weights must be finish/weight pairs")
	}
	out := make([]weightedPackFinish, 0, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		finish, ok := values[i].(string)
		if !ok || strings.TrimSpace(finish) == "" {
			panic(fmt.Sprintf("pack opening finish weight %d has an invalid finish", i/2+1))
		}
		weight, ok := values[i+1].(int)
		if !ok || weight <= 0 {
			panic(fmt.Sprintf("pack opening finish %q has an invalid weight", finish))
		}
		out = append(out, weightedPackFinish{finish: strings.ToLower(strings.TrimSpace(finish)), weight: weight})
	}
	return out
}

func (p *packOpeningPicker) pickWeighted(buckets []weightedPackBucket) (packOpeningCandidate, bool) {
	card, _, ok := p.pickWeightedBucket(buckets)
	return card, ok
}

func (p *packOpeningPicker) pickWeightedBucket(buckets []weightedPackBucket) (packOpeningCandidate, string, bool) {
	total := 0
	for _, bucket := range buckets {
		if len(p.available(bucket.bucket)) == 0 {
			return packOpeningCandidate{}, "", false
		}
		total += bucket.weight
	}
	if total <= 0 {
		return packOpeningCandidate{}, "", false
	}
	roll := p.rng.Intn(total)
	for _, bucket := range buckets {
		if roll < bucket.weight {
			card, ok := p.pickDirect(bucket.bucket)
			return card, bucket.bucket, ok
		}
		roll -= bucket.weight
	}
	bucket := buckets[len(buckets)-1].bucket
	card, ok := p.pickDirect(bucket)
	return card, bucket, ok
}

func (p *packOpeningPicker) pickDirect(bucket string) (packOpeningCandidate, bool) {
	items := p.available(bucket)
	if len(items) == 0 {
		return packOpeningCandidate{}, false
	}
	return items[p.rng.Intn(len(items))], true
}

func (p *packOpeningPicker) available(bucket string) []packOpeningCandidate {
	bucket = strings.ToLower(strings.TrimSpace(bucket))
	return p.buckets[bucket]
}

func repeatPackSlot(count int, label string, bucket string) []packSlot {
	out := make([]packSlot, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, packSlot{Label: label, Bucket: bucket})
	}
	return out
}

func repeatWeightedPackSlot(count int, label string, weighted []weightedPackBucket) []packSlot {
	out := make([]packSlot, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, packSlot{Label: label, Weighted: weighted})
	}
	return out
}

func appendSlots(base []packSlot, groups ...[]packSlot) []packSlot {
	for _, group := range groups {
		base = append(base, group...)
	}
	return base
}

func writePackOpeningError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(packOpeningResponse{
		OK:      false,
		Message: message,
	})
}

func packOpeningYear(releasedAt string) string {
	releasedAt = strings.TrimSpace(releasedAt)
	if len(releasedAt) < 4 {
		return ""
	}
	return releasedAt[:4]
}

func packOpeningSetIconSVGURI(setCode string) string {
	setCode = strings.ToLower(strings.TrimSpace(setCode))
	if setCode == "" {
		return ""
	}
	for _, value := range setCode {
		if (value < 'a' || value > 'z') && (value < '0' || value > '9') {
			return ""
		}
	}
	return "https://svgs.scryfall.io/sets/" + setCode + ".svg"
}

func raritySortKey(rarity string) int {
	switch strings.ToLower(strings.TrimSpace(rarity)) {
	case "mythic":
		return 4
	case "rare":
		return 3
	case "uncommon":
		return 2
	case "common":
		return 1
	default:
		return 0
	}
}

func sortPackOpeningCardsByRarity(cards []packOpeningCard) {
	sort.SliceStable(cards, func(i, j int) bool {
		left := raritySortKey(cards[i].Rarity)
		right := raritySortKey(cards[j].Rarity)
		if left == right {
			return cards[i].Name < cards[j].Name
		}
		return left > right
	})
}
