package quickbuild

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
)

var manaTokenPattern = regexp.MustCompile(`\{([^}]+)\}`)

var supportedCommanderThemes = []string{
	"Aggro",
	"Midrange",
	"Control",
	"Combo",
	"Aristocrats",
	"Spellslinger",
	"Tokens",
	"Reanimator",
	"Voltron",
	"Tribal",
	"Artifacts",
	"Enchantments",
	"Blink",
	"Counters",
	"Lands",
	"Lifegain",
	"Graveyard",
}

var supportedTribes = map[string]struct{}{
	"angel": {}, "artifact": {}, "assassin": {}, "bear": {}, "cat": {}, "cleric": {}, "demon": {}, "dinosaur": {},
	"dragon": {}, "drake": {}, "druid": {}, "elf": {}, "faerie": {}, "goblin": {}, "human": {}, "hydra": {},
	"knight": {}, "merfolk": {}, "pirate": {}, "rogue": {}, "samurai": {}, "shaman": {}, "sliver": {}, "soldier": {},
	"spirit": {}, "vampire": {}, "warrior": {}, "wizard": {}, "wolf": {}, "zombie": {},
}

func classifyCard(card CandidateCard) CandidateCard {
	oracle := strings.ToLower(strings.TrimSpace(card.OracleText))
	typeLine := strings.ToLower(strings.TrimSpace(card.TypeLine))
	name := strings.TrimSpace(card.Name)
	nameLower := strings.ToLower(name)

	roleSet := newOrderedSet()
	themeSet := newOrderedSet()
	strategySet := newOrderedSet()
	landTagSet := newOrderedSet()
	manaTagSet := newOrderedSet()
	scoreFlags := map[string]bool{}

	colorPips := parseManaPips(card.ManaCost)
	curveBucket := manaCurveBucket(card.CMC)

	if card.CMC <= 2 {
		manaTagSet.Add("cheap")
	} else if card.CMC >= 6 {
		manaTagSet.Add("top_end")
	} else {
		manaTagSet.Add("midgame")
	}
	for color, qty := range colorPips {
		if qty > 0 {
			manaTagSet.Add("cost:" + color)
		}
	}

	if strings.Contains(typeLine, "creature") {
		scoreFlags["creature"] = true
	}
	if strings.Contains(typeLine, "artifact") {
		scoreFlags["artifact"] = true
	}
	if strings.Contains(typeLine, "enchantment") {
		scoreFlags["enchantment"] = true
	}
	if strings.Contains(typeLine, "instant") {
		scoreFlags["instant"] = true
	}
	if strings.Contains(typeLine, "sorcery") {
		scoreFlags["sorcery"] = true
	}
	if strings.Contains(typeLine, "legendary") && strings.Contains(typeLine, "creature") {
		scoreFlags["legendary_creature"] = true
	}
	if strings.Contains(oracle, "enters the battlefield tapped") {
		scoreFlags["etb_tapped"] = true
	}
	if strings.Contains(oracle, "counter target") {
		scoreFlags["counterspell"] = true
	}
	if isTutorText(oracle) {
		scoreFlags["tutor"] = true
	}
	if isFastManaText(typeLine, oracle, card.CMC) {
		scoreFlags["fast_mana"] = true
	}
	if strings.Contains(oracle, "gain life") || strings.Contains(oracle, "gains life") || strings.Contains(oracle, "whenever you gain life") {
		scoreFlags["lifegain"] = true
		themeSet.Add("Lifegain")
	}
	if strings.Contains(oracle, "enters the battlefield") && strings.Contains(typeLine, "creature") {
		scoreFlags["etb_value"] = true
	}
	if strings.Contains(oracle, "search your library for") && strings.Contains(oracle, "land") {
		scoreFlags["land_ramp"] = true
	}

	if isBasicLandCard(name, typeLine) {
		roleSet.Add("land_basic")
		landTagSet.Add("basic")
	}

	if strings.Contains(typeLine, "land") {
		isFixing := false
		if strings.Contains(oracle, "any color") || strings.Contains(oracle, "one mana of any type") {
			isFixing = true
			landTagSet.Add("multi_fix")
			manaTagSet.Add("produces:any")
		}
		for _, color := range extractProducedColors(oracle) {
			manaTagSet.Add("produces:" + color)
			isFixing = true
		}
		if strings.Contains(oracle, "search your library for a basic land") || strings.Contains(oracle, "search your library for up to") && strings.Contains(oracle, "basic land") {
			isFixing = true
			landTagSet.Add("basic_fetch")
		}
		if strings.Contains(oracle, "enters the battlefield tapped") {
			landTagSet.Add("tapped")
		}
		if strings.Contains(oracle, "add {c}") {
			landTagSet.Add("colorless")
			scoreFlags["produces_colorless"] = true
		}
		if strings.Contains(oracle, "add one mana of any color in your commander's color identity") {
			landTagSet.Add("commander_fixing")
			isFixing = true
			manaTagSet.Add("produces:any")
		}
		if strings.Contains(oracle, "shares a creature type") || strings.Contains(oracle, "choose a creature type") {
			landTagSet.Add("tribal")
			themeSet.Add("Tribal")
		}
		if isFixing {
			roleSet.Add("land_fixing")
		}
		if !roleSet.Has("land_basic") && (!isFixing || strings.Contains(oracle, "sacrifice") || strings.Contains(oracle, "draw a card") || strings.Contains(oracle, "create ")) {
			roleSet.Add("land_utility")
		}
	}

	if isRampText(typeLine, oracle) {
		roleSet.Add("ramp")
	}
	if isCardDrawText(oracle) {
		roleSet.Add("draw")
	}
	if isSpotRemovalText(oracle) {
		roleSet.Add("spot_removal")
	}
	if isBoardWipeText(oracle) {
		roleSet.Add("wipe")
	}
	if hasProtectionText(oracle) {
		roleSet.Add("protection")
	}
	if isRecursionText(oracle) {
		roleSet.Add("recursion")
	}
	if isUtilityText(typeLine, oracle) {
		roleSet.Add("utility")
	}
	if isTokenMakerText(oracle, card.AllPartsJSON) {
		roleSet.Add("token_maker")
		themeSet.Add("Tokens")
	}
	if isTokenPayoffText(oracle) {
		roleSet.Add("token_payoff")
		themeSet.Add("Tokens")
	}
	if isSacOutletText(oracle) {
		roleSet.Add("sac_outlet")
		themeSet.Add("Aristocrats")
	}
	if isSacPayoffText(oracle) {
		roleSet.Add("sac_payoff")
		themeSet.Add("Aristocrats")
	}
	if isGraveyardEnablerText(oracle) {
		roleSet.Add("graveyard_enabler")
		themeSet.Add("Graveyard")
	}
	if isGraveyardPayoffText(oracle) {
		roleSet.Add("graveyard_payoff")
		themeSet.Add("Graveyard")
		themeSet.Add("Reanimator")
	}
	if isSpellslingerPayoffText(typeLine, oracle) {
		roleSet.Add("spellslinger_payoff")
		themeSet.Add("Spellslinger")
	}
	if isArtifactPayoffText(typeLine, oracle) {
		roleSet.Add("artifact_payoff")
		themeSet.Add("Artifacts")
	}
	if isEnchantmentPayoffText(typeLine, oracle) {
		roleSet.Add("enchantment_payoff")
		themeSet.Add("Enchantments")
	}
	if isBlinkPayoffText(typeLine, oracle) {
		roleSet.Add("blink_payoff")
		themeSet.Add("Blink")
	}
	if isCountersPayoffText(oracle) {
		roleSet.Add("counters_payoff")
		themeSet.Add("Counters")
	}
	if isTribalPayoffText(typeLine, oracle) {
		roleSet.Add("tribal_payoff")
		themeSet.Add("Tribal")
	}
	if isVoltronText(typeLine, oracle) {
		roleSet.Add("voltron_piece")
		themeSet.Add("Voltron")
	}
	if isFinisherText(typeLine, oracle, card.CMC) {
		roleSet.Add("finisher")
	}
	if isLandThemeText(oracle) {
		themeSet.Add("Lands")
	}

	for _, subtype := range parseSubtypes(typeLine) {
		if subtype == "" {
			continue
		}
		if _, ok := supportedTribes[subtype]; ok {
			strategySet.Add("tribe:" + subtype)
		}
	}

	if scoreFlags["counterspell"] || roleSet.Has("wipe") {
		strategySet.Add("Control")
	}
	if roleSet.Has("spellslinger_payoff") || scoreFlags["instant"] || scoreFlags["sorcery"] {
		strategySet.Add("Control")
	}
	if roleSet.Has("voltron_piece") {
		strategySet.Add("Aggro")
	}
	if roleSet.Has("sac_outlet") || roleSet.Has("sac_payoff") || roleSet.Has("graveyard_payoff") {
		strategySet.Add("Midrange")
	}
	if len(strategySet.Values()) == 0 {
		strategySet.Add("Midrange")
	}

	card.Roles = roleSet.Values()
	card.Themes = filterSupportedThemes(themeSet.Values())
	card.StrategyTags = strategySet.Values()
	card.LandTags = landTagSet.Values()
	card.ManaTags = manaTagSet.Values()
	card.CurveBucket = curveBucket
	card.ColorPips = colorPips
	card.ScoreFlags = scoreFlags

	if strings.Contains(nameLower, "forest") && roleSet.Has("land_basic") {
		card.ScoreFlags["basic_forest"] = true
	}
	return card
}

func inferProfile(commander CandidateCard, override CommanderOverride) Profile {
	themeScores := make(map[string]int)
	for _, theme := range commander.Themes {
		themeScores[theme] += 3
	}

	oracle := strings.ToLower(strings.TrimSpace(commander.OracleText))
	typeLine := strings.ToLower(strings.TrimSpace(commander.TypeLine))
	if isTokenMakerText(oracle, commander.AllPartsJSON) {
		themeScores["Tokens"] += 4
	}
	if isSacOutletText(oracle) || isSacPayoffText(oracle) {
		themeScores["Aristocrats"] += 4
	}
	if isSpellslingerPayoffText(typeLine, oracle) {
		themeScores["Spellslinger"] += 4
	}
	if isVoltronText(typeLine, oracle) {
		themeScores["Voltron"] += 4
	}
	if isGraveyardPayoffText(oracle) || isGraveyardEnablerText(oracle) {
		themeScores["Reanimator"] += 4
		themeScores["Graveyard"] += 3
	}
	if isArtifactPayoffText(typeLine, oracle) {
		themeScores["Artifacts"] += 4
	}
	if isEnchantmentPayoffText(typeLine, oracle) {
		themeScores["Enchantments"] += 4
	}
	if isBlinkPayoffText(typeLine, oracle) {
		themeScores["Blink"] += 4
	}
	if isCountersPayoffText(oracle) {
		themeScores["Counters"] += 4
	}
	if isLandThemeText(oracle) {
		themeScores["Lands"] += 4
	}
	if strings.Contains(oracle, "gain life") || strings.Contains(oracle, "whenever you gain life") {
		themeScores["Lifegain"] += 4
	}

	tribe := detectCommanderTribe(typeLine, oracle)
	if tribe != "" {
		themeScores["Tribal"] += 4
	}

	if override.Enabled {
		for _, theme := range override.ForceThemes {
			theme = normalizeLabel(theme)
			if theme == "" {
				continue
			}
			themeScores[theme] += 100
		}
	}

	primaryTheme, themes := selectProfileThemes(themeScores, 3)
	strategy := inferPrimaryStrategy(primaryTheme, themes, commander)
	if override.Enabled && strings.TrimSpace(override.ForceStrategy) != "" {
		strategy = strings.TrimSpace(override.ForceStrategy)
	}

	profile := baseProfileForStrategy(strategy)
	profile.PrimaryTheme = primaryTheme
	profile.Themes = themes
	profile.Tribe = tribe
	profile.Strategy = strategy

	applyThemeAdjustments(&profile)

	if override.Enabled {
		for bucket, value := range override.BucketOverrides {
			if value <= 0 {
				continue
			}
			switch bucket {
			case "lands":
				profile.LandCount = value
			case "ramp":
				profile.RampCount = value
			case "draw":
				profile.DrawCount = value
			case "interaction":
				profile.InteractionCount = value
			case "wipes":
				profile.WipeCount = value
			case "protection":
				profile.ProtectionCount = value
			case "utility":
				profile.UtilityCount = value
			case "synergy":
				profile.SynergyCount = value
			}
		}
	}

	profile.Explanation = buildProfileExplanation(profile)
	return profile
}

func baseProfileForStrategy(strategy string) Profile {
	switch strategy {
	case "Aggro":
		return Profile{
			Strategy:         "Aggro",
			LandCount:        36,
			RampCount:        9,
			DrawCount:        8,
			InteractionCount: 7,
			WipeCount:        1,
			ProtectionCount:  5,
			UtilityCount:     6,
			SynergyCount:     27,
		}
	case "Control":
		return Profile{
			Strategy:         "Control",
			LandCount:        37,
			RampCount:        10,
			DrawCount:        11,
			InteractionCount: 10,
			WipeCount:        3,
			ProtectionCount:  2,
			UtilityCount:     2,
			SynergyCount:     24,
		}
	default:
		return Profile{
			Strategy:         "Midrange",
			LandCount:        37,
			RampCount:        10,
			DrawCount:        10,
			InteractionCount: 8,
			WipeCount:        2,
			ProtectionCount:  3,
			UtilityCount:     4,
			SynergyCount:     25,
		}
	}
}

func buildProfileExplanation(profile Profile) string {
	if profile.PrimaryTheme == "" {
		return "Built a playtest-ready " + profile.Strategy + " starter shell around your commander."
	}
	if len(profile.Themes) == 1 {
		return "Built a " + profile.Strategy + " starter shell leaning on " + profile.PrimaryTheme + "."
	}
	return "Built a " + profile.Strategy + " starter shell leaning on " + strings.Join(profile.Themes[:minInt(3, len(profile.Themes))], ", ") + "."
}

func inferPrimaryStrategy(primaryTheme string, themes []string, commander CandidateCard) string {
	if primaryTheme == "Voltron" || hasString(themes, "Aggro") {
		return "Aggro"
	}
	if primaryTheme == "Spellslinger" {
		return "Control"
	}
	if commander.ScoreFlags["counterspell"] || strings.Contains(strings.ToLower(commander.OracleText), "counter target") {
		return "Control"
	}
	return "Midrange"
}

func selectProfileThemes(scores map[string]int, limit int) (string, []string) {
	type pair struct {
		Name  string
		Score int
	}

	pairs := make([]pair, 0, len(scores))
	for _, name := range supportedCommanderThemes {
		if scores[name] <= 0 {
			continue
		}
		pairs = append(pairs, pair{Name: name, Score: scores[name]})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Score == pairs[j].Score {
			return pairs[i].Name < pairs[j].Name
		}
		return pairs[i].Score > pairs[j].Score
	})
	if len(pairs) == 0 || pairs[0].Score < 4 {
		return "", nil
	}

	primaryTheme := pairs[0].Name
	primaryScore := pairs[0].Score
	themes := []string{primaryTheme}
	for i := 1; i < len(pairs) && len(themes) < limit; i++ {
		if pairs[i].Score < 3 {
			continue
		}
		if pairs[i].Score+2 < primaryScore {
			continue
		}
		themes = append(themes, pairs[i].Name)
	}
	return primaryTheme, themes
}

func topThemes(scores map[string]int, limit int) []string {
	type pair struct {
		Name  string
		Score int
	}

	pairs := make([]pair, 0, len(scores))
	for _, name := range supportedCommanderThemes {
		if scores[name] <= 0 {
			continue
		}
		pairs = append(pairs, pair{Name: name, Score: scores[name]})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Score == pairs[j].Score {
			return pairs[i].Name < pairs[j].Name
		}
		return pairs[i].Score > pairs[j].Score
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, pair.Name)
	}
	return out
}

func detectCommanderTribe(typeLine, oracle string) string {
	if !strings.Contains(typeLine, "creature") {
		return ""
	}
	subtypes := parseSubtypes(typeLine)
	candidates := make([]string, 0, len(subtypes))
	for _, subtype := range subtypes {
		if _, ok := supportedTribes[subtype]; ok {
			candidates = append(candidates, subtype)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	if strings.Contains(oracle, "choose a creature type") || strings.Contains(oracle, "of the chosen type") || strings.Contains(oracle, "shares a creature type") {
		if len(candidates) == 1 {
			return candidates[0]
		}
	}
	for _, tribe := range candidates {
		if oracleMentionsTribe(oracle, tribe) {
			return tribe
		}
	}
	return ""
}

func parseManaPips(manaCost string) map[string]int {
	out := map[string]int{}
	matches := manaTokenPattern.FindAllStringSubmatch(strings.TrimSpace(manaCost), -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		token := strings.ToUpper(strings.TrimSpace(match[1]))
		if token == "" {
			continue
		}
		for _, color := range []string{"W", "U", "B", "R", "G"} {
			if strings.Contains(token, color) {
				out[color]++
			}
		}
	}
	return out
}

func extractProducedColors(oracle string) []string {
	set := newOrderedSet()
	if strings.Contains(oracle, "any color") {
		set.Add("W")
		set.Add("U")
		set.Add("B")
		set.Add("R")
		set.Add("G")
	}
	for _, color := range []string{"W", "U", "B", "R", "G"} {
		if strings.Contains(oracle, "add {"+strings.ToLower(color)+"}") || strings.Contains(oracle, "add {"+color+"}") {
			set.Add(color)
		}
	}
	return set.Values()
}

func parseSubtypes(typeLine string) []string {
	parts := strings.Split(typeLine, "—")
	if len(parts) < 2 {
		parts = strings.Split(typeLine, "-")
	}
	if len(parts) < 2 {
		return nil
	}

	raw := strings.Fields(strings.TrimSpace(parts[len(parts)-1]))
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		token = strings.ToLower(strings.Trim(strings.TrimSpace(token), ","))
		if token == "" {
			continue
		}
		out = append(out, token)
	}
	return out
}

func filterSupportedThemes(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if hasString(supportedCommanderThemes, value) {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func manaCurveBucket(cmc float64) int {
	if cmc < 0 {
		return 0
	}
	bucket := int(math.Floor(cmc + 1e-9))
	if bucket >= 7 {
		return 7
	}
	return bucket
}

func isTutorText(oracle string) bool {
	return strings.Contains(oracle, "search your library for") && strings.Contains(oracle, "card")
}

func isCardDrawText(oracle string) bool {
	return strings.Contains(oracle, "draw ") ||
		strings.Contains(oracle, "investigate") ||
		strings.Contains(oracle, "look at the top") && strings.Contains(oracle, "put") && strings.Contains(oracle, "into your hand")
}

func isRampText(typeLine, oracle string) bool {
	if strings.Contains(typeLine, "land") {
		return false
	}
	return strings.Contains(oracle, "add {") ||
		strings.Contains(oracle, "add one mana of any color") ||
		strings.Contains(oracle, "add one mana of any type") ||
		(strings.Contains(oracle, "search your library for") && strings.Contains(oracle, "land")) ||
		strings.Contains(oracle, "treasure token") ||
		strings.Contains(oracle, "create a treasure")
}

func isFastManaText(typeLine, oracle string, cmc float64) bool {
	if strings.Contains(typeLine, "land") || cmc > 2 {
		return false
	}
	return strings.Contains(oracle, "add {") || strings.Contains(oracle, "treasure token")
}

func isSpotRemovalText(oracle string) bool {
	return strings.Contains(oracle, "destroy target") ||
		strings.Contains(oracle, "exile target") ||
		strings.Contains(oracle, "counter target") ||
		strings.Contains(oracle, "return target") ||
		strings.Contains(oracle, "target creature gets -") ||
		strings.Contains(oracle, "target permanent's owner puts it") ||
		(strings.Contains(oracle, "deals ") && strings.Contains(oracle, "damage to target")) ||
		(strings.Contains(oracle, "deals ") && strings.Contains(oracle, "damage to any target")) ||
		strings.Contains(oracle, "fight target") ||
		strings.Contains(oracle, "fights target creature") ||
		strings.Contains(oracle, "target opponent sacrifices") ||
		strings.Contains(oracle, "each opponent sacrifices")
}

func isBoardWipeText(oracle string) bool {
	return strings.Contains(oracle, "destroy all") ||
		strings.Contains(oracle, "exile all") ||
		strings.Contains(oracle, "each creature") ||
		strings.Contains(oracle, "all creatures get -")
}

func hasProtectionText(oracle string) bool {
	return strings.Contains(oracle, "hexproof") ||
		strings.Contains(oracle, "indestructible") ||
		strings.Contains(oracle, "protection from") ||
		strings.Contains(oracle, "ward {") ||
		strings.Contains(oracle, "can't be countered") ||
		strings.Contains(oracle, "phase out")
}

func isRecursionText(oracle string) bool {
	return strings.Contains(oracle, "return target") && strings.Contains(oracle, "graveyard") ||
		strings.Contains(oracle, "cast") && strings.Contains(oracle, "from your graveyard") ||
		strings.Contains(oracle, "from your graveyard to your hand")
}

func isUtilityText(typeLine, oracle string) bool {
	return isTutorText(oracle) ||
		strings.Contains(oracle, "scry ") ||
		strings.Contains(oracle, "surveil ") ||
		strings.Contains(oracle, "choose one") ||
		(strings.Contains(typeLine, "artifact") && strings.Contains(oracle, "draw")) ||
		strings.Contains(oracle, "untap target")
}

func isTokenMakerText(oracle, allParts string) bool {
	if strings.Contains(oracle, "create") && strings.Contains(oracle, " token") {
		return true
	}
	raw := strings.TrimSpace(allParts)
	if raw == "" || raw == "[]" {
		return false
	}
	var parts []struct {
		Component string `json:"component"`
	}
	if err := json.Unmarshal([]byte(raw), &parts); err != nil {
		return false
	}
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part.Component), "token") {
			return true
		}
	}
	return false
}

func isTokenPayoffText(oracle string) bool {
	if !strings.Contains(oracle, "token") {
		return false
	}

	return strings.Contains(oracle, "creatures you control get +") ||
		strings.Contains(oracle, "whenever one or more tokens") ||
		strings.Contains(oracle, "populate") ||
		strings.Contains(oracle, "create twice") ||
		strings.Contains(oracle, "double the number of tokens")
}

func isSacOutletText(oracle string) bool {
	return strings.Contains(oracle, "sacrifice another") ||
		strings.Contains(oracle, "sacrifice a creature:") ||
		strings.Contains(oracle, "sacrifice an artifact:")
}

func isSacPayoffText(oracle string) bool {
	return strings.Contains(oracle, "whenever") && strings.Contains(oracle, "dies") ||
		strings.Contains(oracle, "whenever") && strings.Contains(oracle, "sacrifice")
}

func isGraveyardEnablerText(oracle string) bool {
	return strings.Contains(oracle, "mill ") ||
		strings.Contains(oracle, "discard") ||
		strings.Contains(oracle, "put the top") && strings.Contains(oracle, "into your graveyard")
}

func isGraveyardPayoffText(oracle string) bool {
	return strings.Contains(oracle, "from your graveyard") ||
		strings.Contains(oracle, "in your graveyard") ||
		strings.Contains(oracle, "creature card from a graveyard")
}

func isSpellslingerPayoffText(typeLine, oracle string) bool {
	return strings.Contains(oracle, "instant or sorcery") ||
		strings.Contains(oracle, "whenever you cast a noncreature spell") ||
		strings.Contains(oracle, "whenever you cast an instant") ||
		strings.Contains(oracle, "whenever you cast a sorcery") ||
		strings.Contains(oracle, "whenever you cast or copy") ||
		strings.Contains(oracle, "magecraft") ||
		strings.Contains(oracle, "copy target instant") ||
		strings.Contains(oracle, "storm")
}

func isArtifactPayoffText(typeLine, oracle string) bool {
	return strings.Contains(oracle, "artifact spell") ||
		strings.Contains(oracle, "whenever an artifact") ||
		strings.Contains(oracle, "artifacts you control") ||
		strings.Contains(oracle, "for each artifact") ||
		strings.Contains(oracle, "affinity for artifacts") ||
		strings.Contains(oracle, "metalcraft")
}

func isEnchantmentPayoffText(typeLine, oracle string) bool {
	return strings.Contains(oracle, "enchantment spell") ||
		strings.Contains(oracle, "whenever an enchantment") ||
		strings.Contains(oracle, "constellation") ||
		strings.Contains(oracle, "for each enchantment")
}

func isBlinkPayoffText(typeLine, oracle string) bool {
	return strings.Contains(oracle, "exile another target") && strings.Contains(oracle, "return it") ||
		strings.Contains(oracle, "exile up to one target") && strings.Contains(oracle, "return it")
}

func isCountersPayoffText(oracle string) bool {
	return strings.Contains(oracle, "+1/+1 counter") ||
		strings.Contains(oracle, "proliferate") ||
		strings.Contains(oracle, "counter on")
}

func isTribalPayoffText(typeLine, oracle string) bool {
	return strings.Contains(oracle, "creature type") ||
		strings.Contains(oracle, "of the chosen type") ||
		strings.Contains(oracle, "shares a creature type") ||
		strings.Contains(typeLine, "kindred")
}

func isVoltronText(typeLine, oracle string) bool {
	return strings.Contains(typeLine, "equipment") ||
		strings.Contains(typeLine, "aura") ||
		strings.Contains(oracle, "equipped creature") ||
		strings.Contains(oracle, "enchanted creature") ||
		strings.Contains(oracle, "attach to")
}

func isFinisherText(typeLine, oracle string, cmc float64) bool {
	if cmc < 5 {
		return false
	}
	return strings.Contains(oracle, "creatures you control get +") ||
		strings.Contains(oracle, "extra combat") ||
		strings.Contains(oracle, "each opponent loses") ||
		strings.Contains(oracle, "whenever this creature attacks")
}

func isLandThemeText(oracle string) bool {
	return strings.Contains(oracle, "landfall") ||
		strings.Contains(oracle, "whenever a land") ||
		(strings.Contains(oracle, "search your library for") && strings.Contains(oracle, "land"))
}

func isBasicLandCard(name, typeLine string) bool {
	typeLine = strings.ToLower(strings.TrimSpace(typeLine))
	if strings.Contains(typeLine, "basic") && strings.Contains(typeLine, "land") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plains", "island", "swamp", "mountain", "forest", "wastes":
		return true
	default:
		return false
	}
}

func strongTheme(themes []string, values ...string) bool {
	for _, value := range values {
		if hasString(themes, value) {
			return true
		}
	}
	return false
}

func applyThemeAdjustments(profile *Profile) {
	if profile == nil {
		return
	}

	switch profile.PrimaryTheme {
	case "Tokens":
		profile.UtilityCount = maxInt(profile.UtilityCount-2, 2)
		profile.SynergyCount += 3
	case "Aristocrats":
		profile.LandCount = 36
		profile.ProtectionCount = maxInt(profile.ProtectionCount-1, 2)
		profile.UtilityCount = maxInt(profile.UtilityCount-3, 1)
		profile.SynergyCount = 99 - profile.LandCount - profile.RampCount - profile.DrawCount - profile.InteractionCount - profile.WipeCount - profile.ProtectionCount - profile.UtilityCount
	case "Reanimator":
		profile.DrawCount = maxInt(profile.DrawCount-1, 9)
		profile.ProtectionCount = maxInt(profile.ProtectionCount-1, 2)
		profile.UtilityCount = maxInt(profile.UtilityCount-3, 1)
		profile.SynergyCount = 99 - profile.LandCount - profile.RampCount - profile.DrawCount - profile.InteractionCount - profile.WipeCount - profile.ProtectionCount - profile.UtilityCount
	case "Voltron":
		profile.LandCount = 36
		profile.ProtectionCount += 1
		profile.UtilityCount = maxInt(profile.UtilityCount-2, 4)
		profile.SynergyCount = 99 - profile.LandCount - profile.RampCount - profile.DrawCount - profile.InteractionCount - profile.WipeCount - profile.ProtectionCount - profile.UtilityCount
	case "Spellslinger":
		profile.LandCount = 36
		profile.DrawCount += 1
		profile.UtilityCount = maxInt(profile.UtilityCount-1, 1)
		profile.SynergyCount = 99 - profile.LandCount - profile.RampCount - profile.DrawCount - profile.InteractionCount - profile.WipeCount - profile.ProtectionCount - profile.UtilityCount
	case "Tribal":
		profile.DrawCount = maxInt(profile.DrawCount-1, 9)
		profile.UtilityCount = maxInt(profile.UtilityCount-3, 1)
		profile.SynergyCount = 99 - profile.LandCount - profile.RampCount - profile.DrawCount - profile.InteractionCount - profile.WipeCount - profile.ProtectionCount - profile.UtilityCount
	}
}

func oracleMentionsTribe(oracle, tribe string) bool {
	tribe = strings.TrimSpace(strings.ToLower(tribe))
	if tribe == "" {
		return false
	}
	return strings.Contains(oracle, tribe) || strings.Contains(oracle, tribe+"s")
}

func hasString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type orderedSet struct {
	order []string
	seen  map[string]struct{}
}

func newOrderedSet() *orderedSet {
	return &orderedSet{seen: map[string]struct{}{}}
}

func (s *orderedSet) Add(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if _, ok := s.seen[value]; ok {
		return
	}
	s.seen[value] = struct{}{}
	s.order = append(s.order, value)
}

func (s *orderedSet) Has(value string) bool {
	_, ok := s.seen[value]
	return ok
}

func (s *orderedSet) Values() []string {
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}
