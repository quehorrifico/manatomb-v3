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
	TotalCards            int                       `json:"total_cards"`
	MainboardCards        int                       `json:"mainboard_cards"`
	CommanderSet          bool                      `json:"commander_set"`
	LandCount             int                       `json:"land_count"`
	NonLandCount          int                       `json:"nonland_count"`
	AverageCMC            float64                   `json:"average_cmc"`
	AverageCMCDisplay     string                    `json:"average_cmc_display"`
	RampCount             int                       `json:"ramp_count"`
	FastManaCount         int                       `json:"fast_mana_count"`
	CardDrawCount         int                       `json:"card_draw_count"`
	TutorCount            int                       `json:"tutor_count"`
	InteractionCount      int                       `json:"interaction_count"`
	CheapInteractionCount int                       `json:"cheap_interaction_count"`
	CounterspellCount     int                       `json:"counterspell_count"`
	RemovalCount          int                       `json:"removal_count"`
	BoardWipeCount        int                       `json:"board_wipe_count"`
	ProtectionCount       int                       `json:"protection_count"`
	LowCMCNonLandCount    int                       `json:"low_cmc_nonland_count"`
	CreatureCount         int                       `json:"creature_count"`
	ArtifactCount         int                       `json:"artifact_count"`
	EnchantmentCount      int                       `json:"enchantment_count"`
	InstantCount          int                       `json:"instant_count"`
	SorceryCount          int                       `json:"sorcery_count"`
	PlaneswalkerCount     int                       `json:"planeswalker_count"`
	BattleCount           int                       `json:"battle_count"`
	Curve0Count           int                       `json:"curve_0_count"`
	Curve1Count           int                       `json:"curve_1_count"`
	Curve2Count           int                       `json:"curve_2_count"`
	Curve3Count           int                       `json:"curve_3_count"`
	Curve4Count           int                       `json:"curve_4_count"`
	Curve5Count           int                       `json:"curve_5_count"`
	Curve6Count           int                       `json:"curve_6_count"`
	Curve7PlusCount       int                       `json:"curve_7_plus_count"`
	DeckExtras            []deckAnalyticsExtra      `json:"deck_extras"`
	CategoryBreakdown     []deckRoleCategory        `json:"category_breakdown,omitempty"`
	StatCards             map[string][]deckRoleCard `json:"stat_cards,omitempty"`
	GuideChecks           []deckGuideCheck          `json:"guide_checks,omitempty"`
	ValidationWarnings    []string                  `json:"validation_warnings,omitempty"`
	PowerEstimate         deckPowerEstimate         `json:"power_estimate"`
}

type deckAnalyticsExtra struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type deckRoleCategory struct {
	Key   string         `json:"key"`
	Label string         `json:"label"`
	Count int            `json:"count"`
	Cards []deckRoleCard `json:"cards,omitempty"`
}

type deckRoleCard struct {
	Name       string   `json:"name"`
	Qty        int      `json:"qty"`
	TypeLine   string   `json:"type_line,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

type deckPowerEstimate struct {
	Available           bool              `json:"available"`
	Bracket             int               `json:"bracket"`
	BracketLabel        string            `json:"bracket_label"`
	Summary             string            `json:"summary"`
	Confidence          string            `json:"confidence"`
	GameChangerCount    int               `json:"game_changer_count"`
	GameChangers        []string          `json:"game_changers,omitempty"`
	MassLandDenialCount int               `json:"mass_land_denial_count"`
	MassLandDenialCards []string          `json:"mass_land_denial_cards,omitempty"`
	ExtraTurnCount      int               `json:"extra_turn_count"`
	ExtraTurnCards      []string          `json:"extra_turn_cards,omitempty"`
	ComboSignalCount    int               `json:"combo_signal_count"`
	ComboSignals        []string          `json:"combo_signals,omitempty"`
	CompactComboCount   int               `json:"compact_combo_count"`
	CompactCombos       []string          `json:"compact_combos,omitempty"`
	FastManaCount       int               `json:"fast_mana_count"`
	FastManaCards       []string          `json:"fast_mana_cards,omitempty"`
	TutorCount          int               `json:"tutor_count"`
	TutorCards          []string          `json:"tutor_cards,omitempty"`
	Signals             []deckPowerSignal `json:"signals,omitempty"`
}

type deckPowerSignal struct {
	Label  string   `json:"label"`
	Value  string   `json:"value"`
	Detail string   `json:"detail,omitempty"`
	Cards  []string `json:"cards,omitempty"`
	Tone   string   `json:"tone"`
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

type deckPowerCardInput struct {
	Name       string
	TypeLine   string
	OracleText string
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
		StatCards:         make(map[string][]deckRoleCard),
	}

	var nonLandCMCSum float64
	extraCounts := make(map[string]int)
	categoryCounts := make(map[string]int)
	categoryCards := make(map[string][]deckRoleCard)
	powerRows := make([]deckPowerCardInput, 0, len(rows))

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

		powerRows = append(powerRows, deckPowerCardInput{
			Name:       name,
			TypeLine:   strings.TrimSpace(row.TypeLine),
			OracleText: strings.TrimSpace(row.OracleText),
			CMC:        row.CMC,
			Qty:        qty,
		})

		out.MainboardCards += qty

		rawTypeLine := strings.TrimSpace(row.TypeLine)
		typeLine := strings.ToLower(rawTypeLine)
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

		rawOracle := strings.TrimSpace(row.OracleText)
		oracle := strings.ToLower(rawOracle)
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
		isTutor := isTutorCard(oracle)
		isCardDraw := isCardDrawCard(oracle)
		isRamp := isRampCard(typeLine, oracle)
		isFastMana := isFastManaCard(typeLine, oracle, row.CMC)
		hasProtection := hasProtectionText(oracle)
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
		if isTutor {
			out.TutorCount += qty
		}
		if isCardDraw {
			out.CardDrawCount += qty
		}
		if isRamp {
			out.RampCount += qty
		}
		if isFastMana {
			out.FastManaCount += qty
		}
		if hasProtection {
			out.ProtectionCount += qty
		}

		roleCategories := classifyDeckRoleCategories(name, typeLine, oracle, row.CMC)
		roleCard := deckRoleCard{
			Name:       name,
			Qty:        qty,
			TypeLine:   rawTypeLine,
			Categories: deckRoleCategoryLabels(roleCategories),
			Reason:     deckRoleCardReason(roleCategories),
		}
		appendDeckStatCard(out.StatCards, "all", roleCard)
		if isLand {
			appendDeckStatCard(out.StatCards, "lands", roleCard)
		} else {
			appendDeckStatCard(out.StatCards, "nonlands", roleCard)
		}
		if isRamp {
			appendDeckStatCard(out.StatCards, "ramp", roleCard)
		}
		if isCardDraw {
			appendDeckStatCard(out.StatCards, "draw", roleCard)
		}
		if isInteraction {
			appendDeckStatCard(out.StatCards, "interaction", roleCard)
		}
		if isRemoval {
			appendDeckStatCard(out.StatCards, "removal", roleCard)
		}
		if isBoardWipe {
			appendDeckStatCard(out.StatCards, "board_wipe", roleCard)
		}
		if isCounterspell {
			appendDeckStatCard(out.StatCards, "counterspell", roleCard)
		}
		if isTutor {
			appendDeckStatCard(out.StatCards, "tutor", roleCard)
		}
		if hasProtection {
			appendDeckStatCard(out.StatCards, "protection", roleCard)
		}

		primaryCategory := primaryDeckRoleCategory(roleCategories, typeLine)
		categoryCounts[primaryCategory] += qty
		categoryCards[primaryCategory] = append(categoryCards[primaryCategory], roleCard)
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
	out.CategoryBreakdown = orderedDeckRoleCategories(categoryCounts, categoryCards)
	sortDeckStatCards(out.StatCards)
	out.GuideChecks = buildDeckGuideChecks(format, out)
	out.ValidationWarnings = buildDeckValidationWarnings(format, commanderName, rows, out)
	out.PowerEstimate = buildDeckPowerEstimate(format, commanderName, powerRows, out)

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

type deckRoleDefinition struct {
	Key   string
	Label string
}

var deckRoleDefinitions = []deckRoleDefinition{
	{Key: "ramp", Label: "Ramp"},
	{Key: "draw", Label: "Draw"},
	{Key: "tutor", Label: "Tutors"},
	{Key: "removal", Label: "Removal"},
	{Key: "board_wipe", Label: "Board Wipes"},
	{Key: "counterspell", Label: "Countermagic"},
	{Key: "protection", Label: "Protection"},
	{Key: "blink", Label: "Blink"},
	{Key: "recursion", Label: "Recursion"},
	{Key: "tokens", Label: "Tokens"},
	{Key: "sacrifice", Label: "Sacrifice"},
	{Key: "graveyard", Label: "Graveyard"},
	{Key: "stax", Label: "Stax"},
	{Key: "combo", Label: "Combo"},
	{Key: "threat", Label: "Threats"},
	{Key: "utility", Label: "Utility"},
	{Key: "lands", Label: "Lands"},
}

var deckRolePrimaryPriority = []string{
	"lands",
	"ramp",
	"draw",
	"tutor",
	"removal",
	"counterspell",
	"board_wipe",
	"protection",
	"blink",
	"recursion",
	"tokens",
	"sacrifice",
	"graveyard",
	"stax",
	"combo",
	"threat",
	"utility",
}

func deckRoleLabel(key string) string {
	for _, def := range deckRoleDefinitions {
		if def.Key == key {
			return def.Label
		}
	}
	return "Utility"
}

func classifyDeckRoleCategories(name, typeLine, oracle string, cmc float64) []string {
	name = strings.TrimSpace(name)
	typeLine = strings.ToLower(strings.TrimSpace(typeLine))
	oracle = strings.ToLower(strings.TrimSpace(oracle))
	out := make([]string, 0, 4)
	add := func(key string, ok bool) {
		if !ok {
			return
		}
		for _, existing := range out {
			if existing == key {
				return
			}
		}
		out = append(out, key)
	}

	isLand := strings.Contains(typeLine, "land")
	add("lands", isLand)
	add("ramp", !isLand && isRampCard(typeLine, oracle))
	add("draw", isCardDrawCard(oracle) || hasAny(oracle, "investigate", "connive", "impulse draw"))
	add("tutor", isTutorCard(oracle))
	add("counterspell", strings.Contains(oracle, "counter target"))
	add("removal", hasAny(oracle,
		"destroy target",
		"exile target",
		"return target",
		"target creature gets -",
		"target permanent's owner puts it",
		"fight target",
		"deals damage to target creature",
		"deals damage to any target",
	))
	add("board_wipe", hasAny(oracle,
		"destroy all",
		"exile all",
		"each creature",
		"all creatures get -",
	))
	add("protection", hasProtectionText(oracle) || hasAny(oracle,
		"phase out",
		"prevent all damage",
		"gain hexproof",
		"gain indestructible",
	))
	add("blink", hasAny(oracle,
		"exile target creature you control, then return",
		"exile another target",
		"exile up to one target",
		"exile any number of target",
		"return it to the battlefield",
		"return them to the battlefield",
		"return those cards to the battlefield",
		"flicker",
	))
	add("recursion", hasAny(oracle,
		"return target card from your graveyard",
		"return target creature card from your graveyard",
		"return a card from your graveyard",
		"from your graveyard to the battlefield",
		"reanimate",
	))
	add("tokens", hasAny(oracle,
		"create a",
		"create two",
		"create three",
		"create x",
		"token",
	))
	add("sacrifice", hasAny(oracle,
		"sacrifice another",
		"sacrifice a creature:",
		"sacrifice a permanent:",
		"whenever you sacrifice",
	))
	add("graveyard", hasAny(oracle,
		"mill ",
		"surveil",
		"delirium",
		"escape",
		"flashback",
		"threshold",
	))
	add("stax", hasAny(oracle,
		"can't cast",
		"can't attack",
		"don't untap",
		"skip",
		"spells cost",
		"players can't",
		"each opponent can't",
		"enters the battlefield tapped",
	))
	add("combo", commanderComboSignal(name, typeLine, oracle) != "" || strings.Contains(oracle, "you win the game"))

	if len(out) == 0 {
		if strings.Contains(typeLine, "creature") || cmc >= 5 {
			out = append(out, "threat")
		} else {
			out = append(out, "utility")
		}
	}
	return out
}

func primaryDeckRoleCategory(categories []string, typeLine string) string {
	if len(categories) == 0 {
		return "utility"
	}
	for _, wanted := range deckRolePrimaryPriority {
		for _, category := range categories {
			if category == wanted {
				return category
			}
		}
	}
	if strings.Contains(strings.ToLower(typeLine), "creature") {
		return "threat"
	}
	return "utility"
}

func deckRoleCategoryLabels(categories []string) []string {
	out := make([]string, 0, len(categories))
	for _, category := range categories {
		out = append(out, deckRoleLabel(category))
	}
	return out
}

func deckRoleCardReason(categories []string) string {
	if len(categories) == 0 {
		return ""
	}
	labels := deckRoleCategoryLabels(categories)
	if len(labels) <= 3 {
		return strings.Join(labels, ", ")
	}
	return strings.Join(labels[:3], ", ") + fmt.Sprintf(" +%d more", len(labels)-3)
}

func appendDeckStatCard(stats map[string][]deckRoleCard, key string, card deckRoleCard) {
	if stats == nil || key == "" || card.Name == "" || card.Qty <= 0 {
		return
	}
	stats[key] = append(stats[key], card)
}

func sortDeckRoleCards(cards []deckRoleCard) {
	sort.SliceStable(cards, func(i, j int) bool {
		return strings.ToLower(cards[i].Name) < strings.ToLower(cards[j].Name)
	})
}

func sortDeckStatCards(stats map[string][]deckRoleCard) {
	for key := range stats {
		sortDeckRoleCards(stats[key])
	}
}

func orderedDeckRoleCategories(counts map[string]int, cards map[string][]deckRoleCard) []deckRoleCategory {
	out := make([]deckRoleCategory, 0, len(counts))
	for _, def := range deckRoleDefinitions {
		count := counts[def.Key]
		if count <= 0 {
			continue
		}
		list := append([]deckRoleCard(nil), cards[def.Key]...)
		sortDeckRoleCards(list)
		out = append(out, deckRoleCategory{
			Key:   def.Key,
			Label: def.Label,
			Count: count,
			Cards: list,
		})
	}
	return out
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

func buildDeckPowerEstimate(format, commanderName string, rows []deckPowerCardInput, analytics deckAnalyticsData) deckPowerEstimate {
	format = decks.NormalizeFormat(format)
	if !decks.FormatRequiresCommander(format) {
		return deckPowerEstimate{}
	}

	out := deckPowerEstimate{
		Available:     true,
		Bracket:       2,
		BracketLabel:  commanderBracketLabel(2),
		Confidence:    commanderPowerConfidence(analytics.TotalCards),
		FastManaCount: analytics.FastManaCount,
		TutorCount:    analytics.TutorCount,
	}

	if strings.TrimSpace(commanderName) == "" {
		out.Bracket = 1
		out.BracketLabel = commanderBracketLabel(out.Bracket)
		out.Confidence = "low"
		out.Summary = "Set a commander for a useful estimate."
		out.Signals = append(out.Signals, deckPowerSignal{
			Label:  "Commander",
			Value:  "Missing",
			Detail: "Power checks are less useful until the command zone is set.",
			Tone:   "warn",
		})
		return out
	}

	seenNames := make(map[string]bool)
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		qty := row.Qty
		if name == "" || qty <= 0 {
			continue
		}

		seenNames[commanderCardKey(name)] = true
		oracle := strings.ToLower(strings.TrimSpace(row.OracleText))
		typeLine := strings.ToLower(strings.TrimSpace(row.TypeLine))

		if isCommanderGameChanger(name) {
			out.GameChangerCount += qty
			out.GameChangers = appendPowerName(out.GameChangers, name)
		}
		if isCommanderMassLandDenialCard(name, oracle) {
			out.MassLandDenialCount += qty
			out.MassLandDenialCards = appendPowerName(out.MassLandDenialCards, name)
		}
		if isCommanderExtraTurnCard(name, oracle) {
			out.ExtraTurnCount += qty
			out.ExtraTurnCards = appendPowerName(out.ExtraTurnCards, name)
		}
		if signal := commanderComboSignal(name, typeLine, oracle); signal != "" {
			out.ComboSignalCount += qty
			out.ComboSignals = appendPowerName(out.ComboSignals, signal)
		}
		if isFastManaCard(typeLine, oracle, row.CMC) {
			out.FastManaCards = appendPowerName(out.FastManaCards, name)
		}
		if isTutorCard(oracle) {
			out.TutorCards = appendPowerName(out.TutorCards, name)
		}
	}

	commanderName = strings.TrimSpace(commanderName)
	if isCommanderGameChanger(commanderName) && !containsFold(out.GameChangers, commanderName) {
		out.GameChangerCount++
		out.GameChangers = appendPowerName(out.GameChangers, commanderName+" (commander)")
	}

	out.CompactCombos = commanderCompactCombos(seenNames)
	out.CompactComboCount = len(out.CompactCombos)
	chainsExtraTurns := out.ExtraTurnCount >= 2 || (seenNames[commanderCardKey("Panoptic Mirror")] && out.ExtraTurnCount > 0)

	switch {
	case isCEDHLean(out, analytics):
		out.Bracket = 5
	case out.GameChangerCount > 3 || out.MassLandDenialCount > 0 || chainsExtraTurns || out.CompactComboCount > 0 || (analytics.FastManaCount >= 5 && analytics.TutorCount >= 4):
		out.Bracket = 4
	case out.GameChangerCount > 0 || out.ComboSignalCount >= 2 || analytics.FastManaCount >= 3 || analytics.TutorCount >= 4 || out.ExtraTurnCount > 0:
		out.Bracket = 3
	case analytics.TotalCards >= 90 && analytics.FastManaCount == 0 && analytics.TutorCount <= 1 && out.ComboSignalCount == 0 && out.ExtraTurnCount == 0:
		out.Bracket = 2
	default:
		out.Bracket = 2
	}

	out.BracketLabel = commanderBracketLabel(out.Bracket)
	out.Summary = commanderPowerSummary(out.Bracket, out.Confidence)
	out.Signals = commanderPowerSignals(out, analytics, chainsExtraTurns)
	return out
}

func commanderBracketLabel(bracket int) string {
	switch bracket {
	case 1:
		return "Exhibition"
	case 2:
		return "Core"
	case 3:
		return "Upgraded"
	case 4:
		return "Optimized"
	case 5:
		return "cEDH"
	default:
		return "Unknown"
	}
}

func commanderPowerSummary(bracket int, confidence string) string {
	prefix := "Estimate"
	if confidence == "low" {
		prefix = "Early estimate"
	}
	switch bracket {
	case 1:
		return prefix + ": commander is missing, so bracket pressure cannot be evaluated."
	case 2:
		return prefix + ": Core-level pressure based on the listed Commander bracket signals."
	case 3:
		return prefix + ": Upgraded pressure from the cards and patterns below."
	case 4:
		return prefix + ": Optimized pressure from unrestricted bracket signals."
	case 5:
		return prefix + ": cEDH-leaning pressure from compact wins and strong consistency."
	default:
		return prefix + ": add more cards to refine this."
	}
}

func commanderPowerConfidence(totalCards int) string {
	switch {
	case totalCards >= 95:
		return "high"
	case totalCards >= 65:
		return "medium"
	default:
		return "low"
	}
}

func commanderPowerSignals(power deckPowerEstimate, analytics deckAnalyticsData, chainsExtraTurns bool) []deckPowerSignal {
	signals := make([]deckPowerSignal, 0, 6)
	add := func(label, value, detail, tone string, cards []string) {
		signals = append(signals, deckPowerSignal{
			Label:  label,
			Value:  value,
			Detail: detail,
			Cards:  append([]string(nil), cards...),
			Tone:   tone,
		})
	}

	if power.Bracket <= 2 {
		add(
			"Bracket Pressure",
			"None detected",
			"No Game Changers, compact two-card combos, mass land denial, or chained extra turns found.",
			"good",
			nil,
		)
		return signals
	}

	if power.Bracket >= 5 {
		if power.CompactComboCount > 0 {
			add("Compact Wins", fmt.Sprintf("%d", power.CompactComboCount), "Compact two-card win package detected.", "alert", power.CompactCombos)
		}
		if power.GameChangerCount >= 6 {
			add("Game Changers", fmt.Sprintf("%d", power.GameChangerCount), "Game Changer density pushes this above casual brackets.", "alert", power.GameChangers)
		}
		if analytics.FastManaCount >= 3 || analytics.TutorCount >= 3 {
			cards := mergePowerNameLists(power.FastManaCards, power.TutorCards)
			add("Consistency", fmt.Sprintf("%d fast mana / %d tutors", analytics.FastManaCount, analytics.TutorCount), "Fast mana and tutor density support cEDH-leaning starts.", "alert", cards)
		}
		return signals
	}

	if power.Bracket >= 4 {
		if power.GameChangerCount > 3 {
			add("Game Changers", fmt.Sprintf("%d", power.GameChangerCount), "More than three Game Changers is an Optimized bracket signal.", "alert", power.GameChangers)
		}
		if power.MassLandDenialCount > 0 {
			add("Mass Land Denial", fmt.Sprintf("%d", power.MassLandDenialCount), "Mass land denial is treated as Bracket 4+ pressure.", "alert", power.MassLandDenialCards)
		}
		if chainsExtraTurns {
			add("Chained Extra Turns", fmt.Sprintf("%d", power.ExtraTurnCount), "Multiple extra-turn effects can take repeated turns.", "alert", power.ExtraTurnCards)
		}
		if power.CompactComboCount > 0 {
			add("Compact Wins", fmt.Sprintf("%d", power.CompactComboCount), "Compact two-card win package detected.", "alert", power.CompactCombos)
		}
		if analytics.FastManaCount >= 5 && analytics.TutorCount >= 4 {
			cards := mergePowerNameLists(power.FastManaCards, power.TutorCards)
			add("Consistency", fmt.Sprintf("%d fast mana / %d tutors", analytics.FastManaCount, analytics.TutorCount), "High fast-mana and tutor density creates optimized consistency.", "alert", cards)
		}
		if len(signals) > 0 {
			return signals
		}
	}

	if power.GameChangerCount > 0 {
		add("Game Changers", fmt.Sprintf("%d", power.GameChangerCount), "Any Game Changer moves the deck above Core.", "warn", power.GameChangers)
	}
	if power.ExtraTurnCount > 0 {
		add("Extra Turns", fmt.Sprintf("%d", power.ExtraTurnCount), "Extra-turn access is an Upgraded bracket signal.", "warn", power.ExtraTurnCards)
	}
	if power.ComboSignalCount >= 2 {
		add("Combo Signals", fmt.Sprintf("%d", power.ComboSignalCount), "Multiple combo pieces point toward compact or infinite wins.", "warn", power.ComboSignals)
	}
	if analytics.FastManaCount >= 3 || analytics.TutorCount >= 4 {
		cards := mergePowerNameLists(power.FastManaCards, power.TutorCards)
		add("Consistency", fmt.Sprintf("%d fast mana / %d tutors", analytics.FastManaCount, analytics.TutorCount), "Fast mana or tutor density raises consistency above Core.", "warn", cards)
	}

	if len(signals) == 0 {
		add("Bracket Pressure", "Upgraded", "Card quality and deck density suggest stronger-than-Core play.", "warn", nil)
	}

	return signals
}

func isCEDHLean(power deckPowerEstimate, analytics deckAnalyticsData) bool {
	if power.CompactComboCount == 0 {
		return false
	}
	if power.GameChangerCount >= 8 && analytics.FastManaCount >= 4 && analytics.TutorCount >= 4 {
		return true
	}
	return power.CompactComboCount >= 2 && power.GameChangerCount >= 6 && analytics.FastManaCount >= 3 && analytics.TutorCount >= 3
}

func isCommanderGameChanger(name string) bool {
	_, ok := commanderGameChangerNames()[commanderCardKey(name)]
	return ok
}

func commanderGameChangerNames() map[string]bool {
	return map[string]bool{
		commanderCardKey("Ad Nauseam"):                      true,
		commanderCardKey("Ancient Tomb"):                    true,
		commanderCardKey("Aura Shards"):                     true,
		commanderCardKey("Biorhythm"):                       true,
		commanderCardKey("Bolas's Citadel"):                 true,
		commanderCardKey("Braids, Cabal Minion"):            true,
		commanderCardKey("Chrome Mox"):                      true,
		commanderCardKey("Coalition Victory"):               true,
		commanderCardKey("Consecrated Sphinx"):              true,
		commanderCardKey("Crop Rotation"):                   true,
		commanderCardKey("Cyclonic Rift"):                   true,
		commanderCardKey("Demonic Tutor"):                   true,
		commanderCardKey("Drannith Magistrate"):             true,
		commanderCardKey("Enlightened Tutor"):               true,
		commanderCardKey("Farewell"):                        true,
		commanderCardKey("Field of the Dead"):               true,
		commanderCardKey("Fierce Guardianship"):             true,
		commanderCardKey("Force of Will"):                   true,
		commanderCardKey("Gaea's Cradle"):                   true,
		commanderCardKey("Gamble"):                          true,
		commanderCardKey("Gifts Ungiven"):                   true,
		commanderCardKey("Glacial Chasm"):                   true,
		commanderCardKey("Grand Arbiter Augustin IV"):       true,
		commanderCardKey("Grim Monolith"):                   true,
		commanderCardKey("Humility"):                        true,
		commanderCardKey("Imperial Seal"):                   true,
		commanderCardKey("Intuition"):                       true,
		commanderCardKey("Jeska's Will"):                    true,
		commanderCardKey("Lion's Eye Diamond"):              true,
		commanderCardKey("Mana Vault"):                      true,
		commanderCardKey("Mishra's Workshop"):               true,
		commanderCardKey("Mox Diamond"):                     true,
		commanderCardKey("Mystical Tutor"):                  true,
		commanderCardKey("Narset, Parter of Veils"):         true,
		commanderCardKey("Natural Order"):                   true,
		commanderCardKey("Necropotence"):                    true,
		commanderCardKey("Notion Thief"):                    true,
		commanderCardKey("Opposition Agent"):                true,
		commanderCardKey("Orcish Bowmasters"):               true,
		commanderCardKey("Panoptic Mirror"):                 true,
		commanderCardKey("Rhystic Study"):                   true,
		commanderCardKey("Seedborn Muse"):                   true,
		commanderCardKey("Serra's Sanctum"):                 true,
		commanderCardKey("Smothering Tithe"):                true,
		commanderCardKey("Survival of the Fittest"):         true,
		commanderCardKey("Teferi's Protection"):             true,
		commanderCardKey("Tergrid, God of Fright"):          true,
		commanderCardKey("Thassa's Oracle"):                 true,
		commanderCardKey("The One Ring"):                    true,
		commanderCardKey("The Tabernacle at Pendrell Vale"): true,
		commanderCardKey("Underworld Breach"):               true,
		commanderCardKey("Vampiric Tutor"):                  true,
		commanderCardKey("Worldly Tutor"):                   true,
	}
}

func isCommanderMassLandDenialCard(name, oracle string) bool {
	switch commanderCardKey(name) {
	case commanderCardKey("Armageddon"),
		commanderCardKey("Ravages of War"),
		commanderCardKey("Cataclysm"),
		commanderCardKey("Catastrophe"),
		commanderCardKey("Jokulhaups"),
		commanderCardKey("Obliterate"),
		commanderCardKey("Decree of Annihilation"),
		commanderCardKey("Devastation"),
		commanderCardKey("Boom // Bust"),
		commanderCardKey("Ruination"),
		commanderCardKey("Blood Moon"),
		commanderCardKey("Magus of the Moon"),
		commanderCardKey("Back to Basics"),
		commanderCardKey("Winter Orb"),
		commanderCardKey("Static Orb"),
		commanderCardKey("Stasis"),
		commanderCardKey("Sunder"),
		commanderCardKey("Wildfire"),
		commanderCardKey("Burning of Xinye"),
		commanderCardKey("Wake of Destruction"):
		return true
	}
	return hasAny(oracle,
		"destroy all lands",
		"destroy all nonbasic lands",
		"each player sacrifices all lands",
		"lands don't untap",
		"nonbasic lands are mountains",
		"lands lose all abilities",
		"players can't untap more than one land",
	)
}

func isCommanderExtraTurnCard(name, oracle string) bool {
	switch commanderCardKey(name) {
	case commanderCardKey("Time Warp"),
		commanderCardKey("Temporal Manipulation"),
		commanderCardKey("Capture of Jingzhou"),
		commanderCardKey("Nexus of Fate"),
		commanderCardKey("Expropriate"),
		commanderCardKey("Time Stretch"),
		commanderCardKey("Karn's Temporal Sundering"),
		commanderCardKey("Alrund's Epiphany"),
		commanderCardKey("Walk the Aeons"),
		commanderCardKey("Temporal Mastery"),
		commanderCardKey("Beacon of Tomorrows"),
		commanderCardKey("Part the Waterveil"),
		commanderCardKey("Savor the Moment"),
		commanderCardKey("Final Fortune"),
		commanderCardKey("Last Chance"),
		commanderCardKey("Warrior's Oath"),
		commanderCardKey("Magistrate's Scepter"),
		commanderCardKey("Time Sieve"):
		return true
	}
	return strings.Contains(oracle, "extra turn")
}

func commanderComboSignal(name, typeLine, oracle string) string {
	key := commanderCardKey(name)
	comboNames := map[string]bool{
		commanderCardKey("Aetherflux Reservoir"):      true,
		commanderCardKey("Basalt Monolith"):           true,
		commanderCardKey("Brain Freeze"):              true,
		commanderCardKey("Demonic Consultation"):      true,
		commanderCardKey("Devoted Druid"):             true,
		commanderCardKey("Dramatic Reversal"):         true,
		commanderCardKey("Dualcaster Mage"):           true,
		commanderCardKey("Food Chain"):                true,
		commanderCardKey("Goblin Bombardment"):        true,
		commanderCardKey("Heliod, Sun-Crowned"):       true,
		commanderCardKey("Isochron Scepter"):          true,
		commanderCardKey("Karmic Guide"):              true,
		commanderCardKey("Kiki-Jiki, Mirror Breaker"): true,
		commanderCardKey("Lion's Eye Diamond"):        true,
		commanderCardKey("Mikaeus, the Unhallowed"):   true,
		commanderCardKey("Power Artifact"):            true,
		commanderCardKey("Reveillark"):                true,
		commanderCardKey("Rings of Brighthearth"):     true,
		commanderCardKey("Sensei's Divining Top"):     true,
		commanderCardKey("Splinter Twin"):             true,
		commanderCardKey("Tainted Pact"):              true,
		commanderCardKey("Thassa's Oracle"):           true,
		commanderCardKey("Triskelion"):                true,
		commanderCardKey("Twinflame"):                 true,
		commanderCardKey("Underworld Breach"):         true,
		commanderCardKey("Vizier of Remedies"):        true,
		commanderCardKey("Walking Ballista"):          true,
		commanderCardKey("Zealous Conscripts"):        true,
	}
	if comboNames[key] {
		return name
	}
	if strings.Contains(oracle, "win the game") || strings.Contains(oracle, "infinite") {
		return name
	}
	if strings.Contains(typeLine, "artifact") && strings.Contains(oracle, "untap") && strings.Contains(oracle, "add {") {
		return name
	}
	return ""
}

func commanderCompactCombos(seenNames map[string]bool) []string {
	pairs := [][2]string{
		{"Thassa's Oracle", "Demonic Consultation"},
		{"Thassa's Oracle", "Tainted Pact"},
		{"Isochron Scepter", "Dramatic Reversal"},
		{"Heliod, Sun-Crowned", "Walking Ballista"},
		{"Kiki-Jiki, Mirror Breaker", "Zealous Conscripts"},
		{"Kiki-Jiki, Mirror Breaker", "Village Bell-Ringer"},
		{"Dualcaster Mage", "Twinflame"},
		{"Dualcaster Mage", "Heat Shimmer"},
		{"Basalt Monolith", "Rings of Brighthearth"},
		{"Grim Monolith", "Power Artifact"},
		{"Mikaeus, the Unhallowed", "Triskelion"},
		{"Devoted Druid", "Vizier of Remedies"},
		{"Underworld Breach", "Brain Freeze"},
		{"Underworld Breach", "Lion's Eye Diamond"},
		{"Sensei's Divining Top", "Bolas's Citadel"},
	}
	out := make([]string, 0, 2)
	for _, pair := range pairs {
		if seenNames[commanderCardKey(pair[0])] && seenNames[commanderCardKey(pair[1])] {
			out = append(out, pair[0]+" + "+pair[1])
		}
	}
	return out
}

func commanderCardKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "’", "'")
	return strings.Join(strings.Fields(name), " ")
}

func appendPowerName(list []string, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" || containsFold(list, name) {
		return list
	}
	return append(list, name)
}

func mergePowerNameLists(lists ...[]string) []string {
	out := make([]string, 0)
	for _, list := range lists {
		for _, name := range list {
			out = appendPowerName(out, name)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func containsFold(list []string, needle string) bool {
	needle = commanderCardKey(needle)
	for _, item := range list {
		if commanderCardKey(strings.TrimSuffix(item, " (commander)")) == needle {
			return true
		}
	}
	return false
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
