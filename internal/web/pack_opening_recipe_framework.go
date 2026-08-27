package web

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Pack recipe extension guide: ../../docs/crack-a-pack-recipes.md
//
// Collector-number rules are intentionally data rather than product-specific
// switch statements. Wizards product articles commonly define treatments by
// collector-number range, while Scryfall's treatment metadata is not always
// precise enough to distinguish those print sheets. Keeping the ranges here
// gives a future recipe author one auditable place to translate the article's
// card gallery into simulator pools.
type packOpeningCollectorNumberSpan struct {
	First int
	Last  int
}

type packOpeningCollectorPoolTarget struct {
	Bucket       string
	WithRarity   bool
	OnlyRarities []string
}

type packOpeningCollectorPoolRule struct {
	Numbers      []packOpeningCollectorNumberSpan
	ExactNumbers []string
	TypeContains string
	Targets      []packOpeningCollectorPoolTarget
}

var packOpeningCollectorPoolRules = map[string][]packOpeningCollectorPoolRule{
	"tmt": {
		{
			Numbers:      []packOpeningCollectorNumberSpan{{First: 183, Last: 187}, {First: 189, Last: 189}},
			TypeContains: "land",
			Targets: []packOpeningCollectorPoolTarget{
				{Bucket: "set:tmt:nonfoil_play_dual:land"},
				{Bucket: "set:tmt:foil_play_dual:land"},
			},
		},
		{
			Numbers:      []packOpeningCollectorNumberSpan{{First: 191, Last: 195}},
			TypeContains: "land",
			Targets: []packOpeningCollectorPoolTarget{
				{Bucket: "set:tmt:nonfoil_rooftop:land"},
				{Bucket: "set:tmt:foil_rooftop:land"},
			},
		},
	},
	"spm": {
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 1, Last: 180}, {First: 182, Last: 182}, {First: 187, Last: 188}},
			Targets: []packOpeningCollectorPoolTarget{
				{Bucket: "set:spm:main", WithRarity: true},
				{Bucket: "set:spm:play_uncommon_with_boosterfun", OnlyRarities: []string{"uncommon"}},
			},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 181, Last: 181}, {First: 183, Last: 186}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:play_dual:land"}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 189, Last: 193}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:spiderweb:land"}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 194, Last: 198}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:default_basic:land"}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 199, Last: 207}},
			Targets: []packOpeningCollectorPoolTarget{
				{Bucket: "set:spm:scene_boosterfun", WithRarity: true},
				{Bucket: "set:spm:play_uncommon_with_boosterfun", OnlyRarities: []string{"uncommon"}},
			},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 208, Last: 217}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:webslinger_boosterfun", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 218, Last: 231}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:panel_boosterfun", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 232, Last: 234}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:classiccomic_boosterfun", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 235, Last: 241}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:costume_boosterfun", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 242, Last: 242}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:cosmic_boosterfun", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 243, Last: 243}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:gauntlet_boosterfun", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 244, Last: 283}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spm:extendedart_main", WithRarity: true}},
		},
	},
	"spe": {
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 1, Last: 20}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spe:welcome", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 21, Last: 26}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:spe:scene_box_boosterfun", WithRarity: true}},
		},
	},
	"hob": {
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 1, Last: 181}, {First: 187, Last: 188}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hob:main", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 182, Last: 186}},
			Targets: []packOpeningCollectorPoolTarget{
				{Bucket: "set:hob:play_dual:land"},
				{Bucket: "set:hob:nonfoil_play_dual:land"},
				{Bucket: "set:hob:foil_play_dual:land"},
			},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 189, Last: 193}},
			Targets: []packOpeningCollectorPoolTarget{
				{Bucket: "set:hob:default_basic:land"},
				{Bucket: "set:hob:nonfoil_default_basic:land"},
				{Bucket: "set:hob:foil_default_basic:land"},
			},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 194, Last: 198}},
			Targets: []packOpeningCollectorPoolTarget{
				{Bucket: "set:hob:journey:land"},
				{Bucket: "set:hob:nonfoil_journey:land"},
				{Bucket: "set:hob:foil_journey:land"},
			},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 199, Last: 213}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hob:scene", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 214, Last: 238}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hob:dragonhoard", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 239, Last: 248}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hob:bookcover", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 249, Last: 249}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hob:headliner", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 250, Last: 274}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hob:surgefoil_dragonhoard", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 275, Last: 284}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hob:surgefoil_bookcover", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 285, Last: 312}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hob:extendedart_main", WithRarity: true}},
		},
	},
	"hoc": {
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 1, Last: 12}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hoc:scene", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 13, Last: 52}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hoc:classicartist", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 53, Last: 92}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hoc:surgefoil_classicartist", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 93, Last: 97}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hoc:dwarvish", WithRarity: true}},
		},
		{
			Numbers: []packOpeningCollectorNumberSpan{{First: 98, Last: 106}},
			Targets: []packOpeningCollectorPoolTarget{{Bucket: "set:hoc:extendedart_main", WithRarity: true}},
		},
	},
}

func packOpeningAddProductBuckets(card packOpeningCandidate, setCode, rarity string, addBucket func(string)) {
	collectorNumberRaw := strings.ToLower(strings.TrimSpace(card.CollectorNumber))
	if collectorNumberRaw == "" {
		return
	}
	collectorNumber, numericCollectorNumberErr := strconv.Atoi(collectorNumberRaw)
	setCode = strings.ToLower(strings.TrimSpace(setCode))
	rarity = strings.ToLower(strings.TrimSpace(rarity))
	typeLine := strings.ToLower(strings.TrimSpace(card.TypeLine))
	seen := map[string]bool{}
	add := func(bucket string) {
		bucket = strings.ToLower(strings.TrimSpace(bucket))
		if bucket == "" || seen[bucket] {
			return
		}
		seen[bucket] = true
		addBucket(bucket)
	}

	for _, rule := range packOpeningCollectorPoolRules[setCode] {
		if !packOpeningCollectorNumberMatches(collectorNumberRaw, collectorNumber, numericCollectorNumberErr == nil && collectorNumber > 0, rule) {
			continue
		}
		if required := strings.ToLower(strings.TrimSpace(rule.TypeContains)); required != "" && !strings.Contains(typeLine, required) {
			continue
		}
		for _, target := range rule.Targets {
			if !packOpeningRarityAllowed(rarity, target.OnlyRarities) {
				continue
			}
			add(target.Bucket)
			if target.WithRarity && rarity != "" {
				add(target.Bucket + ":" + rarity)
			}
		}
	}
}

func packOpeningCollectorNumberMatches(raw string, number int, numeric bool, rule packOpeningCollectorPoolRule) bool {
	for _, exact := range rule.ExactNumbers {
		if raw == exact {
			return true
		}
	}
	if numeric {
		for _, span := range rule.Numbers {
			if number >= span.First && number <= span.Last {
				return true
			}
		}
	}
	return false
}

func packOpeningRarityAllowed(rarity string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if rarity == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

// packOpeningValidateRecipeLibrary is deliberately callable from a fast unit
// test. It catches the most common copy/paste mistakes before a recipe can be
// published; the database preflight remains responsible for proving that the
// locally synced Scryfall pools actually contain enough eligible cards.
func packOpeningValidateRecipeLibrary() error {
	var problems []string
	packTypeIDs := map[string]bool{}
	for _, packType := range packOpeningPackTypes {
		packTypeIDs[packType.ID] = true
	}

	launchSetCodes := make([]string, 0, len(packOpeningLaunchProducts))
	for setCode := range packOpeningLaunchProducts {
		launchSetCodes = append(launchSetCodes, setCode)
	}
	sort.Strings(launchSetCodes)
	for _, setCode := range launchSetCodes {
		if setCode != strings.ToLower(strings.TrimSpace(setCode)) || setCode == "" {
			problems = append(problems, fmt.Sprintf("launch set code %q is not canonical lowercase", setCode))
		}
		setConfig, ok := packOpeningSetConfigs[setCode]
		if !ok {
			problems = append(problems, fmt.Sprintf("launch set %s has no set configuration", setCode))
			continue
		}
		if !packOpeningOfficialSourceURL(setConfig.SourceURL) {
			problems = append(problems, fmt.Sprintf("%s has non-official source URL %q", setCode, setConfig.SourceURL))
		}

		productIDs := make([]string, 0, len(packOpeningLaunchProducts[setCode]))
		for productID := range packOpeningLaunchProducts[setCode] {
			productIDs = append(productIDs, productID)
		}
		sort.Strings(productIDs)
		for _, productID := range productIDs {
			prefix := setCode + "/" + productID
			publication := packOpeningLaunchProducts[setCode][productID]
			if !packTypeIDs[productID] {
				problems = append(problems, prefix+" uses an unknown pack type")
			}
			base, ok := setConfig.Packs[productID]
			if !ok {
				problems = append(problems, prefix+" is published without a configured recipe")
				continue
			}
			if !packOpeningKnownAccuracy(publication.Accuracy) {
				problems = append(problems, fmt.Sprintf("%s has unknown accuracy tier %q", prefix, publication.Accuracy))
			}
			if strings.TrimSpace(publication.AccuracySummary) == "" || len(publication.Limitations) == 0 {
				problems = append(problems, prefix+" publication must explicitly declare an accuracy summary and limitations")
			}
			if publication.BasicRarity && publication.Accuracy != packOpeningAccuracyBasicRarity {
				problems = append(problems, prefix+" enables BasicRarity without the basic-rarity accuracy tier")
			}
			effective := packOpeningPublishedPackConfig(setCode, productID, base)
			if !packOpeningOfficialSourceURL(effective.SourceURL) {
				problems = append(problems, fmt.Sprintf("%s has non-official product source URL %q", prefix, effective.SourceURL))
			}
			if err := packOpeningValidateConfiguredPack(prefix, effective); err != nil {
				problems = append(problems, err.Error())
			}
			packType, ok := packOpeningConfiguredPackType(productID, effective)
			if !ok || strings.TrimSpace(packType.AccuracySummary) == "" || len(packType.SlotRecipe) == 0 || len(packType.Limitations) == 0 {
				problems = append(problems, prefix+" is missing effective accuracy summary, slot recipe, or limitations metadata")
			}
		}
	}

	knownSetCodes := map[string]bool{}
	for setCode, config := range packOpeningSetConfigs {
		knownSetCodes[setCode] = true
		for _, related := range config.RelatedSetCodes {
			knownSetCodes[strings.ToLower(strings.TrimSpace(related))] = true
		}
	}
	for setCode, rules := range packOpeningCollectorPoolRules {
		if !knownSetCodes[setCode] {
			problems = append(problems, fmt.Sprintf("collector-number rules reference unknown set %s", setCode))
		}
		if err := packOpeningValidateCollectorPoolRules(setCode, rules); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid pack-opening recipe library:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func packOpeningOfficialSourceURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return parsed.Scheme == "https" && strings.EqualFold(parsed.Hostname(), "magic.wizards.com") && strings.HasPrefix(parsed.EscapedPath(), "/en/news/")
}

func packOpeningKnownAccuracy(accuracy string) bool {
	switch strings.ToLower(strings.TrimSpace(accuracy)) {
	case packOpeningAccuracySourced, packOpeningAccuracyStructure, packOpeningAccuracyBasicRarity:
		return true
	default:
		return false
	}
}

func packOpeningValidateConfiguredPack(prefix string, pack packOpeningConfiguredPack) error {
	var problems []string
	if pack.CardCount <= 0 || len(pack.Slots) != pack.CardCount {
		problems = append(problems, fmt.Sprintf("declares %d cards but defines %d slots", pack.CardCount, len(pack.Slots)))
	}
	for index, slot := range pack.Slots {
		slotPrefix := fmt.Sprintf("slot %d", index+1)
		if strings.TrimSpace(slot.Label) == "" {
			problems = append(problems, slotPrefix+" has no label")
		}
		hasBucket := strings.TrimSpace(slot.Bucket) != ""
		hasWeighted := len(slot.Weighted) > 0
		if hasBucket == hasWeighted {
			problems = append(problems, slotPrefix+" must define exactly one direct bucket or weighted bucket list")
		}
		if hasBucket && slot.Bucket != strings.ToLower(strings.TrimSpace(slot.Bucket)) {
			problems = append(problems, slotPrefix+" direct bucket is not canonical lowercase")
		}
		seenBuckets := map[string]bool{}
		for _, weighted := range slot.Weighted {
			bucket := strings.TrimSpace(weighted.bucket)
			if bucket == "" || weighted.weight <= 0 {
				problems = append(problems, slotPrefix+" has an empty bucket or non-positive weight")
				continue
			}
			if bucket != strings.ToLower(bucket) {
				problems = append(problems, slotPrefix+" weighted bucket is not canonical lowercase")
			}
			if seenBuckets[bucket] {
				problems = append(problems, slotPrefix+" repeats weighted bucket "+bucket)
			}
			seenBuckets[bucket] = true
		}
		seenFinishes := map[string]bool{}
		for _, weighted := range slot.FinishWeighted {
			finish := strings.ToLower(strings.TrimSpace(weighted.finish))
			if weighted.weight <= 0 || (finish != "nonfoil" && finish != "foil" && finish != "etched") {
				problems = append(problems, slotPrefix+" has an invalid finish weight")
			}
			if seenFinishes[finish] {
				problems = append(problems, slotPrefix+" repeats finish "+finish)
			}
			seenFinishes[finish] = true
		}
	}
	for bucket, minimum := range pack.RequiredPools {
		if bucket == "" || bucket != strings.ToLower(strings.TrimSpace(bucket)) || minimum <= 0 {
			problems = append(problems, fmt.Sprintf("required pool %q has an invalid name or minimum %d", bucket, minimum))
		}
		if _, exact := pack.ExpectedPools[bucket]; exact {
			problems = append(problems, fmt.Sprintf("pool %q cannot be both minimum-required and exact-sized", bucket))
		}
	}
	for bucket, expected := range pack.ExpectedPools {
		if bucket == "" || bucket != strings.ToLower(strings.TrimSpace(bucket)) || expected <= 0 {
			problems = append(problems, fmt.Sprintf("expected pool %q has an invalid name or exact size %d", bucket, expected))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", prefix, strings.Join(problems, "; "))
}

func packOpeningValidateCollectorPoolRules(setCode string, rules []packOpeningCollectorPoolRule) error {
	var problems []string
	covered := map[int]bool{}
	coveredExact := map[string]bool{}
	for ruleIndex, rule := range rules {
		prefix := fmt.Sprintf("%s collector rule %d", setCode, ruleIndex+1)
		if (len(rule.Numbers) == 0 && len(rule.ExactNumbers) == 0) || len(rule.Targets) == 0 {
			problems = append(problems, prefix+" needs at least one number selector and target")
		}
		for _, span := range rule.Numbers {
			if span.First <= 0 || span.Last < span.First {
				problems = append(problems, fmt.Sprintf("%s has invalid collector-number span %d-%d", prefix, span.First, span.Last))
				continue
			}
			for number := span.First; number <= span.Last; number++ {
				if covered[number] {
					problems = append(problems, fmt.Sprintf("%s overlaps collector number %d", prefix, number))
					break
				}
				covered[number] = true
			}
		}
		for _, rawExact := range rule.ExactNumbers {
			exact := strings.ToLower(strings.TrimSpace(rawExact))
			if exact == "" || rawExact != exact {
				problems = append(problems, prefix+" has an empty or non-canonical exact collector number")
				continue
			}
			if coveredExact[exact] {
				problems = append(problems, prefix+" repeats exact collector number "+exact)
			}
			coveredExact[exact] = true
			if numeric, err := strconv.Atoi(exact); err == nil && numeric > 0 {
				if covered[numeric] {
					problems = append(problems, fmt.Sprintf("%s exact collector number %s overlaps a numeric span", prefix, exact))
				}
				covered[numeric] = true
			}
		}
		if rule.TypeContains != strings.ToLower(strings.TrimSpace(rule.TypeContains)) {
			problems = append(problems, prefix+" type selector is not canonical lowercase")
		}
		seenTargets := map[string]bool{}
		for _, target := range rule.Targets {
			bucket := strings.TrimSpace(target.Bucket)
			if bucket == "" || bucket != strings.ToLower(bucket) || !strings.HasPrefix(bucket, "set:"+setCode+":") {
				problems = append(problems, fmt.Sprintf("%s has invalid target bucket %q", prefix, target.Bucket))
			}
			key := bucket + fmt.Sprintf("/%t/%s", target.WithRarity, strings.Join(target.OnlyRarities, ","))
			if seenTargets[key] {
				problems = append(problems, prefix+" repeats target "+bucket)
			}
			seenTargets[key] = true
			for _, rarity := range target.OnlyRarities {
				rarity = strings.ToLower(strings.TrimSpace(rarity))
				if rarity != "common" && rarity != "uncommon" && rarity != "rare" && rarity != "mythic" {
					problems = append(problems, fmt.Sprintf("%s target %s has invalid rarity %q", prefix, bucket, rarity))
				}
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "; "))
}
