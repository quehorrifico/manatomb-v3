package web

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type deckAnalyticsData struct {
	TotalCards            int                  `json:"total_cards"`
	MainboardCards        int                  `json:"mainboard_cards"`
	CommanderSet          bool                 `json:"commander_set"`
	LandCount             int                  `json:"land_count"`
	NonLandCount          int                  `json:"nonland_count"`
	AverageCMC            float64              `json:"average_cmc"`
	AverageCMCDisplay     string               `json:"average_cmc_display"`
	RampCount             int                  `json:"ramp_count"`
	FastManaCount         int                  `json:"fast_mana_count"`
	CardDrawCount         int                  `json:"card_draw_count"`
	TutorCount            int                  `json:"tutor_count"`
	InteractionCount      int                  `json:"interaction_count"`
	CheapInteractionCount int                  `json:"cheap_interaction_count"`
	CounterspellCount     int                  `json:"counterspell_count"`
	RemovalCount          int                  `json:"removal_count"`
	BoardWipeCount        int                  `json:"board_wipe_count"`
	ProtectionCount       int                  `json:"protection_count"`
	LowCMCNonLandCount    int                  `json:"low_cmc_nonland_count"`
	CreatureCount         int                  `json:"creature_count"`
	ArtifactCount         int                  `json:"artifact_count"`
	EnchantmentCount      int                  `json:"enchantment_count"`
	InstantCount          int                  `json:"instant_count"`
	SorceryCount          int                  `json:"sorcery_count"`
	PlaneswalkerCount     int                  `json:"planeswalker_count"`
	BattleCount           int                  `json:"battle_count"`
	Curve0Count           int                  `json:"curve_0_count"`
	Curve1Count           int                  `json:"curve_1_count"`
	Curve2Count           int                  `json:"curve_2_count"`
	Curve3Count           int                  `json:"curve_3_count"`
	Curve4Count           int                  `json:"curve_4_count"`
	Curve5Count           int                  `json:"curve_5_count"`
	Curve6Count           int                  `json:"curve_6_count"`
	Curve7PlusCount       int                  `json:"curve_7_plus_count"`
	DeckExtras            []deckAnalyticsExtra `json:"deck_extras"`
	GuideChecks           []deckGuideCheck     `json:"guide_checks,omitempty"`
	ValidationWarnings    []string             `json:"validation_warnings,omitempty"`
}

type deckAnalyticsExtra struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type deckGuideCheck struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Target string `json:"target,omitempty"`
	Hint   string `json:"hint,omitempty"`
	Tone   string `json:"tone"`
}

type deckAnalyticsCardInput struct {
	Name       string
	TypeLine   string
	OracleText string
	AllParts   string
	ColorID    string
	CMC        float64
	Qty        int
}

type deckAnalyticsRequest struct {
	DeckID        int64  `json:"deck_id"`
	Format        string `json:"format"`
	CommanderName string `json:"commander_name"`
	Cards         []struct {
		Name string `json:"name"`
		Qty  int    `json:"qty"`
	} `json:"cards"`
}

func emptyDeckAnalytics(format, commanderName string) deckAnalyticsData {
	return computeDeckAnalytics(format, commanderName, nil)
}

func computeDeckAnalyticsFromDeckCards(format, commanderName string, deckCards []decks.DeckCard) deckAnalyticsData {
	rows := make([]deckAnalyticsCardInput, 0, len(deckCards))
	for _, dc := range deckCards {
		rows = append(rows, deckAnalyticsCardInput{
			Name:       strings.TrimSpace(dc.CardName),
			TypeLine:   strings.TrimSpace(dc.TypeLine),
			OracleText: strings.TrimSpace(dc.OracleText),
			AllParts:   strings.TrimSpace(dc.AllPartsJSON),
			ColorID:    strings.TrimSpace(dc.ColorIdentity),
			CMC:        dc.CMC,
			Qty:        dc.Quantity,
		})
	}
	return computeDeckAnalytics(format, commanderName, rows)
}

func computeDeckAnalytics(format, commanderName string, rows []deckAnalyticsCardInput) deckAnalyticsData {
	format = decks.NormalizeFormat(format)
	commanderName = strings.TrimSpace(commanderName)
	commanderSet := decks.FormatRequiresCommander(format) && commanderName != ""

	out := deckAnalyticsData{
		CommanderSet:      commanderSet,
		AverageCMCDisplay: "0.00",
	}

	var nonLandCMCSum float64
	extraCounts := make(map[string]int)

	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		qty := row.Qty
		if name == "" || qty <= 0 {
			continue
		}

		if commanderSet && strings.EqualFold(name, commanderName) {
			// Commander occupies its own slot, not the mainboard.
			qty -= 1
		}
		if qty <= 0 {
			continue
		}

		out.MainboardCards += qty

		typeLine := strings.ToLower(strings.TrimSpace(row.TypeLine))
		isLand := strings.Contains(typeLine, "land")
		if isLand {
			out.LandCount += qty
		} else {
			out.NonLandCount += qty
			nonLandCMCSum += row.CMC * float64(qty)
			if row.CMC <= 2 {
				out.LowCMCNonLandCount += qty
			}

			cmc := row.CMC
			if cmc < 0 {
				cmc = 0
			}
			switch manaCurveBucket(cmc) {
			case 0:
				out.Curve0Count += qty
			case 1:
				out.Curve1Count += qty
			case 2:
				out.Curve2Count += qty
			case 3:
				out.Curve3Count += qty
			case 4:
				out.Curve4Count += qty
			case 5:
				out.Curve5Count += qty
			case 6:
				out.Curve6Count += qty
			default:
				out.Curve7PlusCount += qty
			}
		}

		if strings.Contains(typeLine, "creature") {
			out.CreatureCount += qty
		}
		if strings.Contains(typeLine, "artifact") {
			out.ArtifactCount += qty
		}
		if strings.Contains(typeLine, "enchantment") {
			out.EnchantmentCount += qty
		}
		if strings.Contains(typeLine, "instant") {
			out.InstantCount += qty
		}
		if strings.Contains(typeLine, "sorcery") {
			out.SorceryCount += qty
		}
		if strings.Contains(typeLine, "planeswalker") {
			out.PlaneswalkerCount += qty
		}
		if strings.Contains(typeLine, "battle") {
			out.BattleCount += qty
		}

		oracle := strings.ToLower(strings.TrimSpace(row.OracleText))
		collectDeckExtras(extraCounts, oracle, row.AllParts, qty)
		isCounterspell := strings.Contains(oracle, "counter target")
		isRemoval := hasAny(oracle,
			"destroy target",
			"exile target",
			"return target",
			"target creature gets -",
			"target permanent's owner puts it",
		)
		isBoardWipe := hasAny(oracle,
			"destroy all",
			"exile all",
			"each creature",
			"all creatures get -",
		)
		isInteraction := isCounterspell || isRemoval || isBoardWipe
		if isInteraction {
			out.InteractionCount += qty
			if row.CMC <= 2 {
				out.CheapInteractionCount += qty
			}
		}
		if isCounterspell {
			out.CounterspellCount += qty
		}
		if isRemoval {
			out.RemovalCount += qty
		}
		if isBoardWipe {
			out.BoardWipeCount += qty
		}
		if isTutorCard(oracle) {
			out.TutorCount += qty
		}
		if isCardDrawCard(oracle) {
			out.CardDrawCount += qty
		}
		if isRampCard(typeLine, oracle) {
			out.RampCount += qty
		}
		if isFastManaCard(typeLine, oracle, row.CMC) {
			out.FastManaCount += qty
		}
		if hasProtectionText(oracle) {
			out.ProtectionCount += qty
		}
	}

	if out.CommanderSet {
		out.TotalCards = out.MainboardCards + 1
	} else {
		out.TotalCards = out.MainboardCards
	}

	if out.NonLandCount > 0 {
		out.AverageCMC = nonLandCMCSum / float64(out.NonLandCount)
		out.AverageCMCDisplay = fmt.Sprintf("%.2f", out.AverageCMC)
	}
	out.DeckExtras = orderedDeckExtras(extraCounts)
	out.GuideChecks = buildDeckGuideChecks(format, out)
	out.ValidationWarnings = buildDeckValidationWarnings(format, commanderName, rows, out)

	return out
}

func manaCurveBucket(cmc float64) int {
	if cmc < 0 {
		return 0
	}
	b := int(math.Floor(cmc + 1e-9))
	if b >= 7 {
		return 7
	}
	return b
}

func hasAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func isTutorCard(oracle string) bool {
	return strings.Contains(oracle, "search your library for") && strings.Contains(oracle, "card")
}

func isCardDrawCard(oracle string) bool {
	return strings.Contains(oracle, "draw ")
}

func isRampCard(typeLine, oracle string) bool {
	if strings.Contains(typeLine, "land") {
		return false
	}
	if strings.Contains(oracle, "add {") {
		return true
	}
	if strings.Contains(oracle, "search your library for") && strings.Contains(oracle, "land") {
		return true
	}
	if strings.Contains(oracle, "treasure token") {
		return true
	}
	return false
}

func isFastManaCard(typeLine, oracle string, cmc float64) bool {
	if strings.Contains(typeLine, "land") || cmc > 2 {
		return false
	}
	return strings.Contains(oracle, "add {") || strings.Contains(oracle, "treasure token")
}

func hasProtectionText(oracle string) bool {
	return hasAny(oracle,
		"hexproof",
		"indestructible",
		"protection from",
		"ward {",
		"can't be countered",
	)
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

func buildDeckValidationWarnings(format, commanderName string, rows []deckAnalyticsCardInput, analytics deckAnalyticsData) []string {
	warnings := make([]string, 0, 4)
	targetSize := decks.FormatTargetMainboardSize(format)
	copyLimit := decks.FormatCopyLimit(format)
	requiresCommander := decks.FormatRequiresCommander(format)

	if requiresCommander && strings.TrimSpace(commanderName) == "" {
		warnings = append(warnings, format+" decks need a commander card set.")
	}

	if targetSize > 0 {
		currentCount := analytics.MainboardCards
		label := "mainboard"
		if requiresCommander {
			currentCount = analytics.TotalCards
			label = "deck"
		}
		if currentCount < targetSize {
			warnings = append(warnings, fmt.Sprintf("%s usually wants %d cards. This %s has %d.", format, targetSize, label, currentCount))
		} else if requiresCommander && currentCount > targetSize {
			warnings = append(warnings, fmt.Sprintf("%s decks should be %d cards. This deck has %d.", format, targetSize, currentCount))
		}
	}

	if copyLimit > 0 {
		for _, row := range rows {
			qty := row.Qty
			if requiresCommander && strings.EqualFold(strings.TrimSpace(row.Name), commanderName) {
				qty -= 1
			}
			if qty <= copyLimit || isBasicLandCard(row.Name, row.TypeLine) {
				continue
			}
			if copyLimit == 1 {
				warnings = append(warnings, fmt.Sprintf("%s has %d copies of %s. %s is a singleton format outside basic lands.", format, qty, row.Name, format))
			} else {
				warnings = append(warnings, fmt.Sprintf("%s has %d copies of %s. Most %s decks cap non-basic cards at %d copies.", format, qty, row.Name, format, copyLimit))
			}
			if len(warnings) >= 6 {
				break
			}
		}
	}

	return warnings
}

func buildDeckGuideChecks(format string, analytics deckAnalyticsData) []deckGuideCheck {
	format = decks.NormalizeFormat(format)
	targetSize := decks.FormatTargetMainboardSize(format)
	requiresCommander := decks.FormatRequiresCommander(format)

	checks := []deckGuideCheck{
		buildDeckSizeGuideCheck(format, analytics, targetSize, requiresCommander),
	}

	sampleFloor := 12
	if targetSize > 0 {
		sampleFloor = maxInt(sampleFloor, targetSize/2)
	}

	if analytics.MainboardCards < sampleFloor {
		checks = append(checks,
			deckGuideCheck{
				Label:  "Mana Base",
				Value:  fmt.Sprintf("%d", analytics.LandCount),
				Target: defaultLandGuideTarget(format),
				Hint:   "Add more cards before mana guidance becomes useful.",
				Tone:   "info",
			},
		)
		if requiresCommander {
			checks = append(checks,
				deckGuideCheck{Label: "Ramp", Value: fmt.Sprintf("%d", analytics.RampCount), Target: "8-12", Hint: "Add more cards to judge acceleration.", Tone: "info"},
				deckGuideCheck{Label: "Card Draw", Value: fmt.Sprintf("%d", analytics.CardDrawCount), Target: "8-12", Hint: "Add more cards to judge sustained draw.", Tone: "info"},
				deckGuideCheck{Label: "Interaction", Value: fmt.Sprintf("%d", analytics.InteractionCount), Target: "8-12", Hint: "Add more cards to judge your answers.", Tone: "info"},
			)
		} else {
			checks = append(checks,
				deckGuideCheck{Label: "Early Plays", Value: fmt.Sprintf("%d", analytics.LowCMCNonLandCount), Target: defaultEarlyPlaysGuideTarget(format), Hint: "Add more cards to judge your early curve.", Tone: "info"},
				deckGuideCheck{Label: "Card Flow", Value: fmt.Sprintf("%d", analytics.CardDrawCount), Target: defaultCardFlowGuideTarget(format), Hint: "Add more cards to judge your draw density.", Tone: "info"},
				deckGuideCheck{Label: "Interaction", Value: fmt.Sprintf("%d", analytics.InteractionCount), Target: defaultInteractionGuideTarget(format), Hint: "Add more cards to judge your answers.", Tone: "info"},
			)
		}
		return checks
	}

	landMin, landMax, landWarnPad := landGuideRange(format)
	checks = append(checks, buildRangeGuideCheck(
		"Mana Base",
		analytics.LandCount,
		fmt.Sprintf("%d-%d", landMin, landMax),
		landMin,
		landMax,
		landWarnPad,
		"Consider a few more lands for smoother draws.",
		"You may have more lands than you need.",
	))

	if requiresCommander {
		checks = append(checks,
			buildRangeGuideCheck("Ramp", analytics.RampCount, "8-12", 8, 12, 2, "A little more ramp should help you deploy faster.", "You already have a lot of acceleration."),
			buildRangeGuideCheck("Card Draw", analytics.CardDrawCount, "8-12", 8, 12, 2, "More card draw will help you keep cards flowing.", "You already have a lot of draw effects."),
			buildRangeGuideCheck("Interaction", analytics.InteractionCount, "8-12", 8, 12, 2, "Add a few more answers so the deck can respond.", "You already have plenty of interaction."),
		)
		return checks
	}

	earlyMin, earlyMax, earlyWarnPad := earlyPlaysGuideRange(format)
	drawMin, drawMax, drawWarnPad := cardFlowGuideRange(format)
	interactionMin, interactionMax, interactionWarnPad := interactionGuideRange(format)
	checks = append(checks,
		buildRangeGuideCheck("Early Plays", analytics.LowCMCNonLandCount, fmt.Sprintf("%d-%d", earlyMin, earlyMax), earlyMin, earlyMax, earlyWarnPad, "More cheap spells can smooth your starts.", "You may be very low to the ground already."),
		buildRangeGuideCheck("Card Flow", analytics.CardDrawCount, fmt.Sprintf("%d-%d", drawMin, drawMax), drawMin, drawMax, drawWarnPad, "A little more draw or selection can improve consistency.", "You already have a lot of card flow."),
		buildRangeGuideCheck("Interaction", analytics.InteractionCount, fmt.Sprintf("%d-%d", interactionMin, interactionMax), interactionMin, interactionMax, interactionWarnPad, "More interaction can help you answer opposing threats.", "You already have a lot of interaction."),
	)

	return checks
}

func buildDeckSizeGuideCheck(format string, analytics deckAnalyticsData, targetSize int, requiresCommander bool) deckGuideCheck {
	currentCount := analytics.MainboardCards
	targetLabel := "Flexible"
	value := fmt.Sprintf("%d", currentCount)
	tone := "info"
	hint := "This format does not have a fixed deck size target here."

	if requiresCommander {
		currentCount = analytics.TotalCards
		value = fmt.Sprintf("%d", currentCount)
	}

	switch {
	case targetSize <= 0:
		return deckGuideCheck{
			Label:  "Deck Size",
			Value:  value,
			Target: targetLabel,
			Hint:   hint,
			Tone:   tone,
		}
	case requiresCommander:
		targetLabel = fmt.Sprintf("%d", targetSize)
		if currentCount == targetSize {
			tone = "good"
			hint = "Right on size. This is ready for refinement and playtesting."
		} else if currentCount < targetSize {
			tone = "alert"
			hint = fmt.Sprintf("Add %d more cards to reach %d.", targetSize-currentCount, targetSize)
		} else {
			tone = "alert"
			hint = fmt.Sprintf("Cut %d cards to get back to %d.", currentCount-targetSize, targetSize)
		}
	default:
		targetLabel = fmt.Sprintf("%d+", targetSize)
		if currentCount >= targetSize {
			tone = "good"
			hint = "Size looks good. Tune the mix next."
		} else {
			tone = "alert"
			hint = fmt.Sprintf("Add at least %d more cards.", targetSize-currentCount)
		}
	}

	return deckGuideCheck{
		Label:  "Deck Size",
		Value:  value,
		Target: targetLabel,
		Hint:   hint,
		Tone:   tone,
	}
}

func buildRangeGuideCheck(label string, value int, target string, idealMin, idealMax, warnPad int, lowHint, highHint string) deckGuideCheck {
	tone := "good"
	hint := "Right in range."

	switch {
	case value < idealMin-warnPad:
		tone = "alert"
		hint = lowHint
	case value < idealMin:
		tone = "warn"
		hint = lowHint
	case value > idealMax+warnPad:
		tone = "alert"
		hint = highHint
	case value > idealMax:
		tone = "warn"
		hint = highHint
	}

	return deckGuideCheck{
		Label:  label,
		Value:  fmt.Sprintf("%d", value),
		Target: target,
		Hint:   hint,
		Tone:   tone,
	}
}

func landGuideRange(format string) (min, max, warnPad int) {
	switch decks.NormalizeFormat(format) {
	case "Commander", "Duel Commander":
		return 35, 40, 2
	case "Brawl", "Historic Brawl", "Oathbreaker":
		return 24, 28, 2
	case "Draft", "Sealed":
		return 16, 18, 1
	default:
		return 22, 27, 2
	}
}

func earlyPlaysGuideRange(format string) (min, max, warnPad int) {
	switch decks.NormalizeFormat(format) {
	case "Draft", "Sealed":
		return 8, 14, 2
	default:
		return 12, 20, 3
	}
}

func cardFlowGuideRange(format string) (min, max, warnPad int) {
	switch decks.NormalizeFormat(format) {
	case "Draft", "Sealed":
		return 2, 5, 1
	default:
		return 4, 8, 2
	}
}

func interactionGuideRange(format string) (min, max, warnPad int) {
	switch decks.NormalizeFormat(format) {
	case "Draft", "Sealed":
		return 4, 8, 1
	default:
		return 5, 10, 2
	}
}

func defaultLandGuideTarget(format string) string {
	min, max, _ := landGuideRange(format)
	return fmt.Sprintf("%d-%d", min, max)
}

func defaultEarlyPlaysGuideTarget(format string) string {
	min, max, _ := earlyPlaysGuideRange(format)
	return fmt.Sprintf("%d-%d", min, max)
}

func defaultCardFlowGuideTarget(format string) string {
	min, max, _ := cardFlowGuideRange(format)
	return fmt.Sprintf("%d-%d", min, max)
}

func defaultInteractionGuideTarget(format string) string {
	min, max, _ := interactionGuideRange(format)
	return fmt.Sprintf("%d-%d", min, max)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type deckAnalyticsRelatedPart struct {
	Component string `json:"component"`
	Name      string `json:"name"`
	TypeLine  string `json:"type_line"`
}

func tokenExtraNameFromPart(name, typeLine string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	line := strings.ToLower(strings.TrimSpace(typeLine))
	combined := base + " " + line

	switch {
	case strings.Contains(combined, "treasure"):
		return "Treasure tokens"
	case strings.Contains(combined, "clue"):
		return "Clue tokens"
	case strings.Contains(combined, "food"):
		return "Food tokens"
	case strings.Contains(combined, "blood"):
		return "Blood tokens"
	case strings.Contains(combined, "map"):
		return "Map tokens"
	case strings.Contains(combined, "powerstone"):
		return "Powerstone tokens"
	case strings.Contains(combined, "incubator"):
		return "Incubator tokens"
	case strings.Contains(combined, "junk"):
		return "Junk tokens"
	case strings.Contains(combined, "role"):
		return "Role tokens"
	case strings.Contains(combined, "gold"):
		return "Gold tokens"
	default:
		return "Other tokens"
	}
}

func collectDeckExtras(counts map[string]int, oracle string, allParts string, qty int) {
	if qty <= 0 {
		return
	}

	add := func(name string, condition bool) {
		if condition {
			counts[name] += qty
		}
	}

	tokenComponentsFound := false
	partsRaw := strings.TrimSpace(allParts)
	if partsRaw != "" && partsRaw != "[]" {
		var parts []deckAnalyticsRelatedPart
		if err := json.Unmarshal([]byte(partsRaw), &parts); err == nil {
			for _, part := range parts {
				if !strings.EqualFold(strings.TrimSpace(part.Component), "token") {
					continue
				}
				name := strings.TrimSpace(part.Name)
				typeLine := strings.TrimSpace(part.TypeLine)
				if name == "" && typeLine == "" {
					continue
				}
				tokenComponentsFound = true
				counts[tokenExtraNameFromPart(name, typeLine)] += qty
			}
		}
	}
	if !tokenComponentsFound {
		hasTreasure := strings.Contains(oracle, "treasure token")
		hasClue := strings.Contains(oracle, "clue token")
		hasFood := strings.Contains(oracle, "food token")
		hasBlood := strings.Contains(oracle, "blood token")
		hasMap := strings.Contains(oracle, "map token")
		hasPowerstone := strings.Contains(oracle, "powerstone token")
		hasIncubator := strings.Contains(oracle, "incubator token")
		hasJunk := strings.Contains(oracle, "junk token")
		hasRole := strings.Contains(oracle, "role token")
		hasGold := strings.Contains(oracle, "gold token")

		add("Treasure tokens", hasTreasure)
		add("Clue tokens", hasClue)
		add("Food tokens", hasFood)
		add("Blood tokens", hasBlood)
		add("Map tokens", hasMap)
		add("Powerstone tokens", hasPowerstone)
		add("Incubator tokens", hasIncubator)
		add("Junk tokens", hasJunk)
		add("Role tokens", hasRole)
		add("Gold tokens", hasGold)

		isSpecificToken := hasTreasure || hasClue || hasFood || hasBlood || hasMap || hasPowerstone || hasIncubator || hasJunk || hasRole || hasGold
		add("Other tokens", strings.Contains(oracle, "create") && strings.Contains(oracle, " token") && !isSpecificToken)
	}

	add("City's blessing", strings.Contains(oracle, "city's blessing"))
	add("The monarch", strings.Contains(oracle, "the monarch"))
	add("The initiative", strings.Contains(oracle, "the initiative"))
	add("Day/Night", hasAny(oracle, "it becomes day", "it becomes night", "daybound", "nightbound"))
	add("Energy counters", hasAny(oracle, "energy counter", "{e}"))
	add("Experience counters", strings.Contains(oracle, "experience counter"))
	add("Poison counters", strings.Contains(oracle, "poison counter"))
	add("Shield counters", strings.Contains(oracle, "shield counter"))
	add("Stun counters", strings.Contains(oracle, "stun counter"))
	add("+1/+1 counters", strings.Contains(oracle, "+1/+1 counter"))
}

func orderedDeckExtras(counts map[string]int) []deckAnalyticsExtra {
	if len(counts) == 0 {
		return nil
	}

	preferred := []string{
		"Treasure tokens",
		"Clue tokens",
		"Food tokens",
		"Blood tokens",
		"Map tokens",
		"Powerstone tokens",
		"Incubator tokens",
		"Junk tokens",
		"Role tokens",
		"Gold tokens",
		"Other tokens",
		"City's blessing",
		"The monarch",
		"The initiative",
		"Day/Night",
		"Energy counters",
		"Experience counters",
		"Poison counters",
		"Shield counters",
		"Stun counters",
		"+1/+1 counters",
	}

	out := make([]deckAnalyticsExtra, 0, len(counts))
	seen := make(map[string]struct{}, len(preferred))
	for _, name := range preferred {
		qty := counts[name]
		if qty <= 0 {
			continue
		}
		out = append(out, deckAnalyticsExtra{Name: name, Count: qty})
		seen[name] = struct{}{}
	}

	rest := make([]string, 0, len(counts))
	for name, qty := range counts {
		if qty <= 0 {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		rest = append(rest, name)
	}
	sort.Strings(rest)
	for _, name := range rest {
		out = append(out, deckAnalyticsExtra{Name: name, Count: counts[name]})
	}

	return out
}

func normalizeAnalyticsRequestCards(raw []struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}) []struct {
	Name string
	Qty  int
} {
	merged := make(map[string]struct {
		Name string
		Qty  int
	}, len(raw))
	order := make([]string, 0, len(raw))

	for _, item := range raw {
		name := strings.TrimSpace(item.Name)
		if name == "" || item.Qty <= 0 {
			continue
		}

		key := strings.ToLower(name)
		if existing, ok := merged[key]; ok {
			existing.Qty += item.Qty
			merged[key] = existing
			continue
		}

		merged[key] = struct {
			Name string
			Qty  int
		}{
			Name: name,
			Qty:  item.Qty,
		}
		order = append(order, key)
	}

	out := make([]struct {
		Name string
		Qty  int
	}, 0, len(order))
	for _, key := range order {
		out = append(out, merged[key])
	}
	return out
}

func (a *App) buildWorkbenchDeckAnalytics(r *http.Request, format, commanderName string, requestCards []struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}) (deckAnalyticsData, error) {
	normalized := normalizeAnalyticsRequestCards(requestCards)
	rows := make([]deckAnalyticsCardInput, 0, len(normalized))

	names := make([]string, 0, len(normalized))
	for _, item := range normalized {
		names = append(names, item.Name)
	}
	byName, err := cards.LookupCardsByNames(r.Context(), a.DB, names)
	if err != nil {
		return deckAnalyticsData{}, err
	}

	for _, item := range normalized {
		key := strings.ToLower(strings.TrimSpace(item.Name))
		dbCard, ok := byName[key]
		if !ok {
			continue
		}

		rows = append(rows, deckAnalyticsCardInput{
			Name:       strings.TrimSpace(dbCard.Name),
			TypeLine:   strings.TrimSpace(dbCard.TypeLine),
			OracleText: strings.TrimSpace(dbCard.OracleText),
			AllParts:   strings.TrimSpace(dbCard.AllPartsJSON),
			ColorID:    strings.TrimSpace(dbCard.ColorIdentity),
			CMC:        dbCard.CMC,
			Qty:        item.Qty,
		})
	}

	return computeDeckAnalytics(format, commanderName, rows), nil
}

func (a *App) HandleDeckAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req deckAnalyticsRequest
	if err := parseJSONBody(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	var analytics deckAnalyticsData

	if req.DeckID > 0 {
		user := CurrentUser(r)
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		d, err := decks.GetDeck(r.Context(), a.DB, req.DeckID, user.ID)
		if err != nil {
			http.Error(w, "deck not found", http.StatusNotFound)
			return
		}

		deckCards, err := decks.ListDeckCards(r.Context(), a.DB, d.ID)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}

		analytics = computeDeckAnalyticsFromDeckCards(d.Format, d.CommanderName, deckCards)
	} else {
		var err error
		analytics, err = a.buildWorkbenchDeckAnalytics(r, req.Format, req.CommanderName, req.Cards)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(analytics)
}
