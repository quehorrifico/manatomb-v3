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
}

type deckAnalyticsExtra struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type deckAnalyticsCardInput struct {
	Name       string
	TypeLine   string
	OracleText string
	CMC        float64
	Qty        int
}

type deckAnalyticsRequest struct {
	DeckID        int64  `json:"deck_id"`
	CommanderName string `json:"commander_name"`
	Cards         []struct {
		Name string `json:"name"`
		Qty  int    `json:"qty"`
	} `json:"cards"`
}

func emptyDeckAnalytics(commanderName string) deckAnalyticsData {
	return computeDeckAnalytics(commanderName, nil)
}

func computeDeckAnalyticsFromDeckCards(commanderName string, deckCards []decks.DeckCard) deckAnalyticsData {
	rows := make([]deckAnalyticsCardInput, 0, len(deckCards))
	for _, dc := range deckCards {
		rows = append(rows, deckAnalyticsCardInput{
			Name:       strings.TrimSpace(dc.CardName),
			TypeLine:   strings.TrimSpace(dc.TypeLine),
			OracleText: strings.TrimSpace(dc.OracleText),
			CMC:        dc.CMC,
			Qty:        dc.Quantity,
		})
	}
	return computeDeckAnalytics(commanderName, rows)
}

func computeDeckAnalytics(commanderName string, rows []deckAnalyticsCardInput) deckAnalyticsData {
	commanderName = strings.TrimSpace(commanderName)
	commanderSet := commanderName != ""

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
		collectDeckExtras(extraCounts, oracle, qty)
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

func collectDeckExtras(counts map[string]int, oracle string, qty int) {
	if qty <= 0 || oracle == "" {
		return
	}

	add := func(name string, condition bool) {
		if condition {
			counts[name] += qty
		}
	}

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

func (a *App) buildGuestDeckAnalytics(r *http.Request, commanderName string, requestCards []struct {
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
			CMC:        dbCard.CMC,
			Qty:        item.Qty,
		})
	}

	return computeDeckAnalytics(commanderName, rows), nil
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

		analytics = computeDeckAnalyticsFromDeckCards(d.CommanderName, deckCards)
	} else {
		var err error
		analytics, err = a.buildGuestDeckAnalytics(r, req.CommanderName, req.Cards)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(analytics)
}
