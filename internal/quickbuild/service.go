package quickbuild

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

var (
	ErrCommanderRequired  = errors.New("quick build requires a commander")
	ErrCommanderNotFound  = errors.New("commander not found")
	ErrCommanderIllegal   = errors.New("quick build requires a commander-legal commander card")
	ErrBuildNotPossible   = errors.New("could not build a starter deck for that commander")
	ErrUnsupportedRequest = errors.New("quick build only supports commander decks")
)

type Service struct {
	repo *Repository
}

func NewService(db *sql.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

func (s *Service) Build(ctx context.Context, req Request) (Result, error) {
	commanderName := strings.TrimSpace(req.CommanderName)
	if commanderName == "" {
		return Result{}, ErrCommanderRequired
	}

	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UTC().UnixNano()
	}

	commander, override, err := s.repo.ResolveCommander(ctx, commanderName)
	if err == sql.ErrNoRows {
		return Result{}, ErrCommanderNotFound
	}
	if err != nil {
		return Result{}, err
	}
	if !commander.CommanderLegal || !commander.IsCommanderCandidate {
		return Result{}, ErrCommanderIllegal
	}

	pool, err := s.repo.CandidatePool(ctx, commander)
	if err != nil {
		return Result{}, err
	}
	if len(pool) == 0 {
		return Result{}, ErrBuildNotPossible
	}

	profile := inferProfile(commander, override)
	builder := newBuilder(commander, profile, pool, seed)
	result, err := builder.Build()
	if err != nil {
		return Result{}, err
	}

	cacheCards := make([]CandidateCard, 0, len(result.Cards)+1)
	cacheCards = append(cacheCards, commander)
	for _, built := range result.Cards {
		cacheCards = append(cacheCards, built.Card)
	}
	if err := s.repo.CacheFeatures(ctx, cacheCards); err != nil {
		return Result{}, err
	}

	return result, nil
}

type builder struct {
	commander CandidateCard
	profile   Profile
	pool      []CandidateCard
	seed      int64
	rng       *rand.Rand

	used          map[string]*BuiltCard
	bucketCounts  map[string]int
	tutorCount    int
	fastManaCount int
	repairActions []string
	fallbackNotes []string
}

type builderStats struct {
	TotalSlots       int
	NonLandSlots     int
	LandCount        int
	RampCount        int
	DrawCount        int
	InteractionCount int
	WipeCount        int
	ProtectionCount  int
	CheapPlays       int
	ThemeHits        int
	TappedLands      int
	ColorlessLands   int
	SourceCounts     map[string]int
}

type scoredCandidate struct {
	card   CandidateCard
	score  int
	bucket string
}

func newBuilder(commander CandidateCard, profile Profile, pool []CandidateCard, seed int64) *builder {
	return &builder{
		commander:    commander,
		profile:      profile,
		pool:         pool,
		seed:         seed,
		rng:          rand.New(rand.NewSource(seed)),
		used:         map[string]*BuiltCard{},
		bucketCounts: map[string]int{},
	}
}

func (b *builder) Build() (Result, error) {
	for _, spec := range b.nonLandBuckets() {
		for b.bucketCounts[spec.Name] < spec.Count {
			card, ok := b.selectBucketCard(spec)
			if !ok {
				if spec.Name == "synergy" {
					b.addFallbackNote(fmt.Sprintf("Theme bucket came up short for %s.", b.specLabel(spec)))
				}
				break
			}
			b.addCard(card, spec.Name, 1)
		}
	}

	b.fillMissingNonlands()
	b.buildManaBase()
	b.fillRemainingSlots()
	b.validateAndRepair()

	if b.totalSlots() != 99 {
		return Result{}, ErrBuildNotPossible
	}

	cards := b.sortedBuiltCards()
	summary := Summary{
		Strategy:      b.profile.Strategy,
		PrimaryTheme:  b.profile.PrimaryTheme,
		Themes:        append([]string(nil), b.profile.Themes...),
		BucketCounts:  map[string]int{},
		Explanation:   b.buildExplanation(),
		Seed:          b.seed,
		RepairActions: append([]string(nil), b.repairActions...),
		FallbackNotes: append([]string(nil), b.fallbackNotes...),
		LandMix:       b.landMixSummary(),
	}
	for _, name := range []string{"lands", "ramp", "draw", "interaction", "wipes", "protection", "utility", "synergy"} {
		summary.BucketCounts[name] = b.bucketCounts[name]
	}

	return Result{
		Commander: b.commander,
		Cards:     cards,
		Summary:   summary,
	}, nil
}

func (b *builder) buildExplanation() string {
	explanation := b.profile.Explanation + " This is a playtest-ready first draft."
	if len(b.fallbackNotes) > 0 {
		explanation += " Theme support was thin in some slots, so the build leans on a value shell where needed."
	}
	if len(b.repairActions) > 0 {
		explanation += " Core deck floors and mana sanity were repaired before returning the list."
	}
	return explanation
}

func (b *builder) nonLandBuckets() []BucketSpec {
	specs := []BucketSpec{
		{Name: "ramp", Count: b.profile.RampCount, Roles: []string{"ramp"}, MaxCMC: 4},
		{Name: "draw", Count: b.profile.DrawCount, Roles: []string{"draw"}, MaxCMC: 5},
		{Name: "interaction", Count: b.profile.InteractionCount, Roles: []string{"spot_removal"}, ScoreFlags: []string{"counterspell"}, MaxCMC: 4},
		{Name: "wipes", Count: b.profile.WipeCount, Roles: []string{"wipe"}, MaxCMC: 6},
		{Name: "protection", Count: b.profile.ProtectionCount, Roles: []string{"protection"}, MaxCMC: 4},
		{Name: "utility", Count: b.profile.UtilityCount, Roles: []string{"recursion", "utility", "finisher"}, MaxCMC: 6},
	}
	return append(specs, b.themeBucketSpecs()...)
}

func (b *builder) themeBucketSpecs() []BucketSpec {
	count := b.profile.SynergyCount
	if count <= 0 {
		return nil
	}

	switch b.profile.PrimaryTheme {
	case "Tokens":
		engine := maxInt(1, (count*2)/3)
		return []BucketSpec{
			{Name: "synergy", Count: engine, Roles: []string{"token_maker"}, Themes: []string{"Tokens"}, MaxCMC: 5},
			{Name: "synergy", Count: count - engine, Roles: []string{"token_payoff", "finisher", "protection"}, Themes: []string{"Tokens"}, MaxCMC: 6},
		}
	case "Aristocrats":
		outlets := maxInt(2, count/5)
		payoffs := maxInt(4, count/3)
		return []BucketSpec{
			{Name: "synergy", Count: outlets, Roles: []string{"sac_outlet"}, Themes: []string{"Aristocrats"}, MaxCMC: 4},
			{Name: "synergy", Count: payoffs, Roles: []string{"sac_payoff", "token_maker"}, Themes: []string{"Aristocrats"}, MaxCMC: 5},
			{Name: "synergy", Count: maxInt(0, count-outlets-payoffs), Roles: []string{"recursion", "graveyard_payoff"}, Themes: []string{"Aristocrats", "Graveyard"}, MaxCMC: 5},
		}
	case "Reanimator":
		enablers := maxInt(3, count/3)
		payoffs := maxInt(3, count/3)
		return []BucketSpec{
			{Name: "synergy", Count: enablers, Roles: []string{"graveyard_enabler"}, Themes: []string{"Reanimator", "Graveyard"}, MaxCMC: 4},
			{Name: "synergy", Count: payoffs, Roles: []string{"graveyard_payoff"}, Themes: []string{"Reanimator", "Graveyard"}, MaxCMC: 6},
			{Name: "synergy", Count: maxInt(0, count-enablers-payoffs), Roles: []string{"recursion"}, Themes: []string{"Reanimator", "Graveyard"}, MaxCMC: 5},
		}
	case "Voltron":
		pieces := maxInt(4, (count*2)/3)
		return []BucketSpec{
			{Name: "synergy", Count: pieces, Roles: []string{"voltron_piece"}, Themes: []string{"Voltron"}, MaxCMC: 4},
			{Name: "synergy", Count: count - pieces, Roles: []string{"protection", "draw"}, Themes: []string{"Voltron"}, MaxCMC: 4},
		}
	case "Spellslinger":
		enablers := maxInt(6, (count*2)/3)
		return []BucketSpec{
			{Name: "synergy", Count: enablers, ScoreFlags: []string{"instant", "sorcery"}, MaxCMC: 3},
			{Name: "synergy", Count: count - enablers, Roles: []string{"spellslinger_payoff"}, Themes: []string{"Spellslinger"}, MaxCMC: 5},
		}
	case "Tribal":
		tribeTag := ""
		if b.profile.Tribe != "" {
			tribeTag = "tribe:" + b.profile.Tribe
		}
		bodies := maxInt(6, (count*2)/3)
		return []BucketSpec{
			{Name: "synergy", Count: bodies, StrategyTags: filterNonEmptyStrings([]string{tribeTag}), MaxCMC: 4},
			{Name: "synergy", Count: count - bodies, Roles: []string{"tribal_payoff"}, StrategyTags: filterNonEmptyStrings([]string{tribeTag}), Themes: []string{"Tribal"}, MaxCMC: 5},
		}
	default:
		if len(b.profile.Themes) == 0 {
			return nil
		}
		return []BucketSpec{
			{Name: "synergy", Count: count, Roles: []string{"finisher"}, Themes: append([]string(nil), b.profile.Themes...), MaxCMC: 5},
		}
	}
}

func (b *builder) selectBucketCard(spec BucketSpec) (CandidateCard, bool) {
	scored := make([]scoredCandidate, 0, 32)
	for _, card := range b.pool {
		if !b.canUseCard(card) || b.isLand(card) {
			continue
		}
		score := b.scoreBucketCandidate(card, spec)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredCandidate{card: card, score: score, bucket: spec.Name})
	}
	return b.pickScoredCandidate(scored, b.topKForBucket(spec.Name))
}

func (b *builder) topKForBucket(name string) int {
	switch name {
	case "lands":
		return 6
	case "synergy":
		return 7
	default:
		return 5
	}
}

func (b *builder) fillMissingNonlands() {
	target := 99 - b.profile.LandCount
	for b.nonLandSlots() < target {
		stats := b.buildStats()

		switch {
		case stats.RampCount < b.hardRampFloor():
			if b.tryAddFallbackCard(BucketSpec{Name: "ramp", Count: 1, Roles: []string{"ramp"}, MaxCMC: 4}, "Added extra ramp to meet the floor.") {
				continue
			}
		case stats.DrawCount < b.hardDrawFloor():
			if b.tryAddFallbackCard(BucketSpec{Name: "draw", Count: 1, Roles: []string{"draw"}, MaxCMC: 5}, "Added extra draw to meet the floor.") {
				continue
			}
		case stats.InteractionCount < b.hardInteractionFloor():
			if b.tryAddFallbackCard(BucketSpec{Name: "interaction", Count: 1, Roles: []string{"spot_removal"}, ScoreFlags: []string{"counterspell"}, MaxCMC: 4}, "Added extra interaction to meet the floor.") {
				continue
			}
		case stats.WipeCount < b.hardWipeFloor():
			if b.tryAddFallbackCard(BucketSpec{Name: "wipes", Count: 1, Roles: []string{"wipe"}, MaxCMC: 6}, "Added an extra wipe to meet the floor.") {
				continue
			}
		case b.profile.PrimaryTheme != "" && stats.ThemeHits < b.desiredThemeHits():
			if b.tryAddThemeFallbackCard("Added another on-theme card because the synergy pool was thin.") {
				continue
			}
		case stats.CheapPlays < b.cheapPlayFloor():
			if b.tryAddFallbackCard(BucketSpec{Name: "utility", Count: 1, Roles: []string{"ramp", "draw", "spot_removal", "protection"}, ScoreFlags: []string{"counterspell", "instant", "sorcery"}, MaxCMC: 3}, "Added a cheaper play to smooth the curve.") {
				continue
			}
		}

		card, bucket, ok := b.selectGenericValueCard()
		if !ok {
			b.addFallbackNote("Ran out of nonland candidates before every preferred slot could be filled.")
			break
		}
		b.addCard(card, bucket, 1)
	}
}

func (b *builder) tryAddFallbackCard(spec BucketSpec, note string) bool {
	card, ok := b.selectBucketCard(spec)
	if !ok {
		return false
	}
	b.addCard(card, spec.Name, 1)
	b.addRepairAction(note)
	return true
}

func (b *builder) tryAddThemeFallbackCard(note string) bool {
	card, bucket, ok := b.selectThemeFallbackCard()
	if !ok {
		return false
	}
	b.addCard(card, bucket, 1)
	b.addRepairAction(note)
	return true
}

func (b *builder) selectThemeFallbackCard() (CandidateCard, string, bool) {
	scored := make([]scoredCandidate, 0, 32)
	for _, spec := range b.themeBucketSpecs() {
		for _, card := range b.pool {
			if !b.canUseCard(card) || b.isLand(card) {
				continue
			}
			score := b.scoreBucketCandidate(card, spec)
			if score <= 0 {
				continue
			}
			scored = append(scored, scoredCandidate{card: card, score: score + 10, bucket: spec.Name})
		}
	}
	card, ok := b.pickScoredCandidate(scored, 7)
	if !ok {
		return CandidateCard{}, "", false
	}
	return card, "synergy", true
}

func (b *builder) selectGenericValueCard() (CandidateCard, string, bool) {
	scored := make([]scoredCandidate, 0, 32)
	for _, card := range b.pool {
		if !b.canUseCard(card) || b.isLand(card) {
			continue
		}
		score := b.scoreGenericValue(card)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredCandidate{
			card:   card,
			score:  score,
			bucket: b.genericValueBucket(card),
		})
	}
	card, ok := b.pickScoredCandidate(scored, 8)
	if !ok {
		return CandidateCard{}, "", false
	}
	return card, b.genericValueBucket(card), true
}

func (b *builder) genericValueBucket(card CandidateCard) string {
	switch {
	case hasRole(card, "ramp") && b.bucketCounts["ramp"] < b.profile.RampCount:
		return "ramp"
	case hasRole(card, "draw") && b.bucketCounts["draw"] < b.profile.DrawCount:
		return "draw"
	case hasRole(card, "wipe"):
		return "wipes"
	case hasRole(card, "spot_removal") || card.ScoreFlags["counterspell"]:
		return "interaction"
	case hasRole(card, "utility", "recursion", "protection", "finisher"):
		return "utility"
	case b.matchesPrimaryTheme(card):
		return "synergy"
	default:
		return "utility"
	}
}

func (b *builder) buildManaBase() {
	remaining := b.profile.LandCount - b.bucketCounts["lands"]
	if remaining <= 0 {
		return
	}

	colorDemand := b.colorDemand()
	colorCount := len(b.commander.ColorIdentity)
	minBasics := minimumBasicLandCount(colorCount, b.landRampSpellCount())
	maxNonbasic := maxInt(0, b.profile.LandCount-minBasics)
	nonbasicTarget := minInt(desiredNonbasicLandCount(colorCount), maxNonbasic)
	fixingTarget := minInt(desiredFixingLandCount(colorCount), nonbasicTarget)
	tappedCap := tappedLandCap(colorCount)
	colorlessCap := desiredColorlessLandCap(colorCount)

	currentNonbasic := b.countNonBasicLands()
	currentTapped := b.countTappedLands()
	currentColorless := b.countColorlessLands()
	currentFixing := b.countFixingLands()

	for remaining > 0 && currentFixing < fixingTarget {
		card, ok := b.selectLandCandidate(colorDemand, currentTapped, tappedCap, currentColorless, colorlessCap, true)
		if !ok {
			break
		}
		b.addCard(card, "lands", 1)
		remaining--
		currentNonbasic++
		currentFixing++
		if hasLandTag(card, "tapped") {
			currentTapped++
		}
		if isColorlessLand(card) {
			currentColorless++
		}
	}

	for remaining > 0 && currentNonbasic < nonbasicTarget {
		card, ok := b.selectLandCandidate(colorDemand, currentTapped, tappedCap, currentColorless, colorlessCap, false)
		if !ok {
			break
		}
		b.addCard(card, "lands", 1)
		remaining--
		currentNonbasic++
		if hasRole(card, "land_fixing") {
			currentFixing++
		}
		if hasLandTag(card, "tapped") {
			currentTapped++
		}
		if isColorlessLand(card) {
			currentColorless++
		}
	}

	b.addBasicLands(remaining)
}

func (b *builder) selectLandCandidate(colorDemand map[string]int, tappedCount, tappedCap, colorlessCount, colorlessCap int, requireFixing bool) (CandidateCard, bool) {
	scored := make([]scoredCandidate, 0, 32)
	for _, card := range b.pool {
		if !b.canUseCard(card) || !b.isLand(card) || hasRole(card, "land_basic") {
			continue
		}
		if requireFixing && !hasRole(card, "land_fixing") {
			continue
		}
		if tappedCount >= tappedCap && hasLandTag(card, "tapped") {
			continue
		}
		if colorlessCount >= colorlessCap && isColorlessLand(card) {
			continue
		}
		if len(b.commander.ColorIdentity) == 0 && strings.Contains(strings.ToLower(card.OracleText), "your commander's color identity") {
			continue
		}

		score := b.scoreLandCandidate(card, colorDemand, requireFixing)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredCandidate{card: card, score: score, bucket: "lands"})
	}
	return b.pickScoredCandidate(scored, 6)
}

func (b *builder) fillRemainingSlots() {
	remaining := 99 - b.totalSlots()
	if remaining <= 0 {
		return
	}
	added := b.addBasicLands(remaining)
	remaining -= added
	for remaining > 0 {
		card, bucket, ok := b.selectGenericValueCard()
		if !ok {
			break
		}
		b.addCard(card, bucket, 1)
		remaining--
	}
}

func (b *builder) validateAndRepair() {
	if b.totalSlots() < 99 {
		added := b.addBasicLands(99 - b.totalSlots())
		if added > 0 {
			b.addRepairAction("Added basics to complete the deck size.")
		}
	}

	b.repairBucketFloor("ramp", b.hardRampFloor(), BucketSpec{Name: "ramp", Count: 1, Roles: []string{"ramp"}, MaxCMC: 4})
	b.repairBucketFloor("draw", b.hardDrawFloor(), BucketSpec{Name: "draw", Count: 1, Roles: []string{"draw"}, MaxCMC: 5})
	b.repairBucketFloor("interaction", b.hardInteractionFloor(), BucketSpec{Name: "interaction", Count: 1, Roles: []string{"spot_removal"}, ScoreFlags: []string{"counterspell"}, MaxCMC: 4})
	b.repairBucketFloor("wipes", b.hardWipeFloor(), BucketSpec{Name: "wipes", Count: 1, Roles: []string{"wipe"}, MaxCMC: 6})
	b.repairThemeDensity()
	b.repairCheapCurve()
	b.repairLandSanity()

	if b.totalSlots() < 99 {
		b.fillRemainingSlots()
	}
}

func (b *builder) repairBucketFloor(bucket string, floor int, spec BucketSpec) {
	if floor <= 0 {
		return
	}
	for {
		stats := b.buildStats()
		current := b.bucketStat(stats, bucket)
		if current >= floor {
			return
		}

		card, ok := b.selectBucketCard(spec)
		if !ok {
			b.addFallbackNote(fmt.Sprintf("Could not find enough %s candidates to reach the floor.", bucket))
			return
		}
		if b.totalSlots() < 99 {
			b.addCard(card, bucket, 1)
			b.addRepairAction(fmt.Sprintf("Added %s to improve the %s floor.", card.Name, bucket))
			continue
		}
		cut, ok := b.selectCutCandidate(stats, bucket)
		if !ok {
			b.addFallbackNote(fmt.Sprintf("Could not free a slot to improve the %s floor.", bucket))
			return
		}
		oldName := cut.Card.Name
		b.removeCard(cut.Card, 1)
		b.addCard(card, bucket, 1)
		b.addRepairAction(fmt.Sprintf("Replaced %s with %s to improve %s.", oldName, card.Name, bucket))
	}
}

func (b *builder) repairThemeDensity() {
	if b.profile.PrimaryTheme == "" || b.desiredThemeHits() <= 0 {
		return
	}
	for {
		stats := b.buildStats()
		if stats.ThemeHits >= b.desiredThemeHits() {
			return
		}
		card, bucket, ok := b.selectThemeFallbackCard()
		if !ok {
			b.addFallbackNote(fmt.Sprintf("Theme support for %s is shallow, so the final list keeps a looser value shell.", b.profile.PrimaryTheme))
			return
		}
		if b.totalSlots() < 99 {
			b.addCard(card, bucket, 1)
			b.addRepairAction(fmt.Sprintf("Added %s to improve %s density.", card.Name, b.profile.PrimaryTheme))
			continue
		}
		cut, ok := b.selectCutCandidate(stats, bucket)
		if !ok {
			b.addFallbackNote(fmt.Sprintf("Could not swap in enough %s cards without breaking core floors.", b.profile.PrimaryTheme))
			return
		}
		oldName := cut.Card.Name
		b.removeCard(cut.Card, 1)
		b.addCard(card, bucket, 1)
		b.addRepairAction(fmt.Sprintf("Replaced %s with %s to strengthen %s.", oldName, card.Name, b.profile.PrimaryTheme))
	}
}

func (b *builder) repairCheapCurve() {
	for {
		stats := b.buildStats()
		if stats.CheapPlays >= b.cheapPlayFloor() {
			return
		}
		spec := BucketSpec{
			Name:       "utility",
			Count:      1,
			Roles:      []string{"ramp", "draw", "spot_removal", "protection"},
			ScoreFlags: []string{"counterspell", "instant", "sorcery"},
			MaxCMC:     3,
		}
		card, ok := b.selectBucketCard(spec)
		if !ok {
			return
		}
		if b.totalSlots() < 99 {
			b.addCard(card, "utility", 1)
			b.addRepairAction(fmt.Sprintf("Added %s to smooth the early curve.", card.Name))
			continue
		}
		cut, ok := b.selectCutCandidate(stats, "utility")
		if !ok {
			return
		}
		oldName := cut.Card.Name
		b.removeCard(cut.Card, 1)
		b.addCard(card, "utility", 1)
		b.addRepairAction(fmt.Sprintf("Replaced %s with %s to smooth the curve.", oldName, card.Name))
	}
}

func (b *builder) repairLandSanity() {
	for {
		stats := b.buildStats()
		if stats.TappedLands <= tappedLandCap(len(b.commander.ColorIdentity)) {
			break
		}
		if !b.replaceProblemLand("tapped") {
			break
		}
	}
	for {
		stats := b.buildStats()
		if stats.ColorlessLands <= desiredColorlessLandCap(len(b.commander.ColorIdentity)) {
			break
		}
		if !b.replaceProblemLand("colorless") {
			break
		}
	}
}

func (b *builder) replaceProblemLand(kind string) bool {
	colorDemand := b.colorDemand()
	var candidate *BuiltCard
	for _, built := range b.used {
		if !b.isLand(built.Card) || hasRole(built.Card, "land_basic") {
			continue
		}
		switch kind {
		case "tapped":
			if !hasLandTag(built.Card, "tapped") {
				continue
			}
		case "colorless":
			if !isColorlessLand(built.Card) {
				continue
			}
		default:
			continue
		}
		if candidate == nil || built.Card.Name < candidate.Card.Name {
			copyBuilt := *built
			candidate = &copyBuilt
		}
	}
	if candidate == nil {
		return false
	}

	sourceCounts := b.buildStats().SourceCounts
	basicName := preferredBasicLandName(b.commander.ColorIdentity, colorDemand, sourceCounts)
	basic, ok := b.findNamedCard(basicName)
	if !ok {
		return false
	}
	b.removeCard(candidate.Card, 1)
	b.addCard(basic, "lands", 1)
	b.addRepairAction(fmt.Sprintf("Replaced %s with %s to tighten the mana base.", candidate.Card.Name, basic.Name))
	return true
}

func (b *builder) bucketStat(stats builderStats, bucket string) int {
	switch bucket {
	case "ramp":
		return stats.RampCount
	case "draw":
		return stats.DrawCount
	case "interaction":
		return stats.InteractionCount
	case "wipes":
		return stats.WipeCount
	default:
		return 0
	}
}

func (b *builder) selectCutCandidate(stats builderStats, incomingBucket string) (*BuiltCard, bool) {
	var best *BuiltCard
	bestScore := 1 << 30
	for _, built := range b.used {
		if b.isLand(built.Card) || built.Qty != 1 {
			continue
		}
		if built.Bucket == incomingBucket {
			continue
		}
		if !b.canCutCard(stats, *built) {
			continue
		}
		score := b.cutPriority(*built)
		if score < bestScore || (score == bestScore && built.Card.Name < best.Card.Name) {
			copyBuilt := *built
			best = &copyBuilt
			bestScore = score
		}
	}
	return best, best != nil
}

func (b *builder) canCutCard(stats builderStats, built BuiltCard) bool {
	switch built.Bucket {
	case "ramp":
		return stats.RampCount > b.hardRampFloor()
	case "draw":
		return stats.DrawCount > b.hardDrawFloor()
	case "interaction":
		return stats.InteractionCount > b.hardInteractionFloor()
	case "wipes":
		return stats.WipeCount > b.hardWipeFloor()
	}
	if built.Card.CMC <= 3 && stats.CheapPlays <= b.cheapPlayFloor() {
		return false
	}
	if b.profile.PrimaryTheme != "" && b.matchesPrimaryTheme(built.Card) && stats.ThemeHits <= b.desiredThemeHits() {
		return false
	}
	return true
}

func (b *builder) cutPriority(built BuiltCard) int {
	score := 0
	switch built.Bucket {
	case "utility":
		score += 0
	case "synergy":
		score += 10
	case "protection":
		score += 20
	case "draw":
		score += 30
	case "interaction":
		score += 40
	case "ramp":
		score += 50
	case "wipes":
		score += 60
	default:
		score += 35
	}
	score += b.commanderEngineFit(built.Card) * 5
	if b.matchesPrimaryTheme(built.Card) {
		score += 20
	}
	if built.Card.CMC <= 2 {
		score += 8
	}
	if built.Card.CMC >= 6 {
		score -= 6
	}
	return score
}

func (b *builder) scoreBucketCandidate(card CandidateCard, spec BucketSpec) int {
	if !b.cardMatchesBucket(card, spec) {
		return 0
	}

	score := 0
	score += b.themeFit(card, spec) * 35
	score += b.functionalFit(card, spec) * 25
	score += b.curveFit(card, spec) * 15
	score += b.commanderEngineFit(card) * 15
	score += b.edhrecFit(card) * 10

	if spec.Name == "interaction" {
		if card.ScoreFlags["counterspell"] {
			score += 30
		}
		if card.ScoreFlags["instant"] {
			score += 24
		} else if card.ScoreFlags["sorcery"] {
			score += 12
		}
		if card.CMC <= 2 {
			score += 14
		} else if card.CMC == 3 {
			score += 8
		}
	}
	if spec.Name == "synergy" && b.matchesPrimaryTheme(card) {
		score += 40
	}
	if card.ScoreFlags["tutor"] && b.tutorCount >= 1 {
		score -= 200
	}
	if card.ScoreFlags["fast_mana"] && b.fastManaCount >= 2 {
		score -= 200
	}
	if len(b.commander.ColorIdentity) == 0 && strings.Contains(strings.ToLower(card.OracleText), "your commander's color identity") {
		score = 0
	}
	return score
}

func (b *builder) scoreGenericValue(card CandidateCard) int {
	score := 0
	if hasRole(card, "draw") {
		score += 200
	}
	if hasRole(card, "ramp") {
		score += 180
	}
	if hasRole(card, "spot_removal") || card.ScoreFlags["counterspell"] {
		score += 200
		if card.ScoreFlags["instant"] {
			score += 30
		} else if card.ScoreFlags["sorcery"] {
			score += 15
		}
	}
	if hasRole(card, "wipe") {
		score += 145
	}
	if hasRole(card, "utility", "recursion", "protection", "finisher") {
		score += 160
	}
	if b.matchesPrimaryTheme(card) {
		score += 150
	}
	if card.CMC <= 4 {
		score += 80
	}
	if b.profile.PrimaryTheme != "" && !b.matchesPrimaryTheme(card) {
		score -= 40
	}
	score += b.commanderEngineFit(card) * 10
	score += b.edhrecFit(card) * 10
	if len(b.commander.ColorIdentity) == 0 && strings.Contains(strings.ToLower(card.OracleText), "your commander's color identity") {
		return 0
	}
	return score
}

func (b *builder) scoreLandCandidate(card CandidateCard, colorDemand map[string]int, requireFixing bool) int {
	score := 0
	if requireFixing && !hasRole(card, "land_fixing") {
		return 0
	}
	if hasRole(card, "land_fixing") {
		score += 260
	}
	if hasRole(card, "land_utility") {
		score += 80
	}
	if hasLandTag(card, "commander_fixing") {
		score += 180
	}
	if hasLandTag(card, "multi_fix") {
		score += 140
	}
	if hasLandTag(card, "tribal") && b.profile.Tribe != "" {
		score += 60
	}
	if hasLandTag(card, "tapped") {
		score -= 50
	}
	if isColorlessLand(card) && len(b.commander.ColorIdentity) > 0 {
		score -= 100
	}
	if !hasRole(card, "land_fixing") && len(b.commander.ColorIdentity) > 1 {
		score -= 120
	}

	coverage := 0
	if hasManaTag(card, "produces:any") && len(b.commander.ColorIdentity) > 0 {
		coverage = 5
	}
	for _, color := range b.commander.ColorIdentity {
		if hasManaTag(card, "produces:"+color) {
			coverage += 2 + colorDemand[color]
		}
	}
	score += coverage * 10
	score += b.edhrecFit(card) * 6
	return score
}

func (b *builder) themeFit(card CandidateCard, spec BucketSpec) int {
	score := 0
	for _, theme := range spec.Themes {
		if hasTheme(card, theme) {
			score += 4
		}
	}
	if b.profile.PrimaryTheme != "" && b.matchesPrimaryTheme(card) {
		score += 3
	}
	if b.profile.Tribe != "" && hasStrategyTag(card, "tribe:"+b.profile.Tribe) {
		score += 6
	}
	return score
}

func (b *builder) functionalFit(card CandidateCard, spec BucketSpec) int {
	for _, role := range spec.Roles {
		if hasRole(card, role) {
			return 10
		}
	}
	for _, tag := range spec.StrategyTags {
		if hasStrategyTag(card, tag) {
			return 10
		}
	}
	for _, flag := range spec.ScoreFlags {
		if card.ScoreFlags[flag] {
			return 10
		}
	}
	return 0
}

func (b *builder) curveFit(card CandidateCard, spec BucketSpec) int {
	if spec.MaxCMC <= 0 {
		return 6
	}
	switch {
	case card.CMC <= spec.MaxCMC:
		return 10
	case card.CMC <= spec.MaxCMC+1:
		return 6
	default:
		return 1
	}
}

func (b *builder) commanderEngineFit(card CandidateCard) int {
	score := 0
	switch b.profile.PrimaryTheme {
	case "Tokens":
		if hasRole(card, "token_maker", "token_payoff", "finisher") {
			score += 6
		}
	case "Aristocrats":
		if hasRole(card, "sac_outlet", "sac_payoff", "token_maker", "recursion") {
			score += 6
		}
	case "Reanimator":
		if hasRole(card, "graveyard_enabler", "graveyard_payoff", "recursion") {
			score += 6
		}
	case "Voltron":
		if hasRole(card, "voltron_piece", "protection") {
			score += 6
		}
	case "Spellslinger":
		if hasRole(card, "spellslinger_payoff") {
			score += 6
		}
		if card.ScoreFlags["instant"] || card.ScoreFlags["sorcery"] {
			score += 4
		}
	case "Tribal":
		if b.profile.Tribe != "" && hasStrategyTag(card, "tribe:"+b.profile.Tribe) {
			score += 6
		}
		if hasRole(card, "tribal_payoff") {
			score += 4
		}
	}

	for _, theme := range b.profile.Themes {
		if theme == b.profile.PrimaryTheme {
			continue
		}
		switch theme {
		case "Lifegain":
			if hasTheme(card, "Lifegain") || card.ScoreFlags["lifegain"] {
				score += 2
			}
		case "Graveyard":
			if hasRole(card, "graveyard_enabler", "graveyard_payoff", "recursion") {
				score += 2
			}
		}
	}
	if b.profile.Strategy == "Control" && (card.ScoreFlags["counterspell"] || hasRole(card, "draw", "spot_removal", "wipe")) {
		score += 2
	}
	if b.profile.Strategy == "Aggro" && card.ScoreFlags["creature"] && card.CMC <= 3 {
		score += 2
	}
	if score > 10 {
		return 10
	}
	return score
}

func (b *builder) edhrecFit(card CandidateCard) int {
	switch {
	case card.EDHRecRank > 0 && card.EDHRecRank <= 100:
		return 10
	case card.EDHRecRank <= 500 && card.EDHRecRank > 0:
		return 8
	case card.EDHRecRank <= 1000 && card.EDHRecRank > 0:
		return 6
	case card.EDHRecRank <= 3000 && card.EDHRecRank > 0:
		return 4
	case card.EDHRecRank > 0:
		return 2
	default:
		return 0
	}
}

func (b *builder) canUseCard(card CandidateCard) bool {
	if card.OracleID == "" || b.used[card.OracleID] != nil {
		return false
	}
	if !card.CommanderLegal {
		return false
	}
	if card.ScoreFlags["tutor"] && b.tutorCount >= 2 {
		return false
	}
	if card.ScoreFlags["fast_mana"] && b.fastManaCount >= 2 {
		return false
	}
	return true
}

func (b *builder) cardMatchesBucket(card CandidateCard, spec BucketSpec) bool {
	if spec.Name == "synergy" {
		return b.matchesThemeBucket(card, spec)
	}
	for _, role := range spec.Roles {
		if hasRole(card, role) {
			return true
		}
	}
	for _, flag := range spec.ScoreFlags {
		if card.ScoreFlags[flag] {
			return true
		}
	}
	return false
}

func (b *builder) matchesThemeBucket(card CandidateCard, spec BucketSpec) bool {
	for _, role := range spec.Roles {
		if hasRole(card, role) {
			if len(spec.Themes) == 0 {
				return true
			}
			for _, theme := range spec.Themes {
				if hasTheme(card, theme) || theme == b.profile.PrimaryTheme {
					return true
				}
			}
		}
	}
	for _, tag := range spec.StrategyTags {
		if hasStrategyTag(card, tag) {
			return true
		}
	}
	for _, flag := range spec.ScoreFlags {
		if card.ScoreFlags[flag] {
			return true
		}
	}
	return false
}

func (b *builder) matchesPrimaryTheme(card CandidateCard) bool {
	switch b.profile.PrimaryTheme {
	case "Tokens":
		return hasRole(card, "token_maker", "token_payoff", "finisher")
	case "Aristocrats":
		return hasRole(card, "sac_outlet", "sac_payoff", "token_maker", "recursion")
	case "Reanimator":
		return hasRole(card, "graveyard_enabler", "graveyard_payoff", "recursion")
	case "Voltron":
		return hasRole(card, "voltron_piece", "protection")
	case "Spellslinger":
		return hasRole(card, "spellslinger_payoff") || ((card.ScoreFlags["instant"] || card.ScoreFlags["sorcery"]) && card.CMC <= 3)
	case "Tribal":
		if b.profile.Tribe == "" {
			return hasRole(card, "tribal_payoff")
		}
		return hasRole(card, "tribal_payoff") || hasStrategyTag(card, "tribe:"+b.profile.Tribe)
	default:
		return false
	}
}

func (b *builder) colorDemand() map[string]int {
	demand := map[string]int{}
	for _, color := range b.commander.ColorIdentity {
		demand[color] = 1
	}
	for color, qty := range b.commander.ColorPips {
		demand[color] += qty * 4
	}
	for _, built := range b.used {
		if b.isLand(built.Card) {
			continue
		}
		weight := 1
		switch {
		case built.Card.CMC <= 2:
			weight = 3
		case built.Card.CMC <= 4:
			weight = 2
		}
		for color, qty := range built.Card.ColorPips {
			demand[color] += qty * weight * built.Qty
		}
	}
	if len(demand) == 0 {
		demand["C"] = 1
	}
	return demand
}

func (b *builder) buildStats() builderStats {
	stats := builderStats{
		SourceCounts: map[string]int{},
	}
	for _, built := range b.used {
		stats.TotalSlots += built.Qty
		if b.isLand(built.Card) {
			stats.LandCount += built.Qty
			if hasLandTag(built.Card, "tapped") {
				stats.TappedLands += built.Qty
			}
			if isColorlessLand(built.Card) {
				stats.ColorlessLands += built.Qty
			}
			if hasManaTag(built.Card, "produces:any") {
				for _, color := range b.commander.ColorIdentity {
					stats.SourceCounts[color] += built.Qty
				}
			}
			for _, color := range b.commander.ColorIdentity {
				if hasManaTag(built.Card, "produces:"+color) {
					stats.SourceCounts[color] += built.Qty
				}
			}
			continue
		}

		stats.NonLandSlots += built.Qty
		if hasRole(built.Card, "ramp") {
			stats.RampCount += built.Qty
		}
		if hasRole(built.Card, "draw") {
			stats.DrawCount += built.Qty
		}
		if hasRole(built.Card, "spot_removal") || built.Card.ScoreFlags["counterspell"] {
			stats.InteractionCount += built.Qty
		}
		if hasRole(built.Card, "wipe") {
			stats.WipeCount += built.Qty
		}
		if hasRole(built.Card, "protection") {
			stats.ProtectionCount += built.Qty
		}
		if built.Card.CMC <= 3 {
			stats.CheapPlays += built.Qty
		}
		if b.matchesPrimaryTheme(built.Card) {
			stats.ThemeHits += built.Qty
		}
	}
	return stats
}

func (b *builder) addCard(card CandidateCard, bucket string, qty int) {
	if qty <= 0 {
		return
	}
	existing := b.used[card.OracleID]
	if existing == nil {
		existing = &BuiltCard{Card: card}
		b.used[card.OracleID] = existing
	}
	existing.Qty += qty
	if existing.Bucket == "" {
		existing.Bucket = bucket
	}
	b.bucketCounts[bucket] += qty
	if card.ScoreFlags["tutor"] {
		b.tutorCount += qty
	}
	if card.ScoreFlags["fast_mana"] {
		b.fastManaCount += qty
	}
}

func (b *builder) removeCard(card CandidateCard, qty int) {
	if qty <= 0 {
		return
	}
	existing := b.used[card.OracleID]
	if existing == nil {
		return
	}
	existing.Qty -= qty
	if existing.Qty <= 0 {
		delete(b.used, card.OracleID)
	} else {
		b.used[card.OracleID] = existing
	}
	if existing.Bucket != "" {
		b.bucketCounts[existing.Bucket] -= qty
		if b.bucketCounts[existing.Bucket] < 0 {
			b.bucketCounts[existing.Bucket] = 0
		}
	}
	if card.ScoreFlags["tutor"] {
		b.tutorCount -= qty
		if b.tutorCount < 0 {
			b.tutorCount = 0
		}
	}
	if card.ScoreFlags["fast_mana"] {
		b.fastManaCount -= qty
		if b.fastManaCount < 0 {
			b.fastManaCount = 0
		}
	}
}

func (b *builder) sortedBuiltCards() []BuiltCard {
	out := make([]BuiltCard, 0, len(b.used))
	for _, built := range b.used {
		out = append(out, *built)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bucket == out[j].Bucket {
			return out[i].Card.Name < out[j].Card.Name
		}
		return out[i].Bucket < out[j].Bucket
	})
	return out
}

func (b *builder) pickScoredCandidate(scored []scoredCandidate, topK int) (CandidateCard, bool) {
	if len(scored) == 0 {
		return CandidateCard{}, false
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].card.Name < scored[j].card.Name
		}
		return scored[i].score > scored[j].score
	})
	if topK <= 0 || topK > len(scored) {
		topK = len(scored)
	}
	if topK == 1 {
		return scored[0].card, true
	}
	ceiling := scored[topK-1].score
	weights := make([]int, topK)
	totalWeight := 0
	for i := 0; i < topK; i++ {
		weight := scored[i].score - ceiling + 1
		if weight < 1 {
			weight = 1
		}
		if i == 0 {
			weight += 2
		}
		weights[i] = weight
		totalWeight += weight
	}
	if totalWeight <= 0 {
		return scored[0].card, true
	}
	pick := b.rng.Intn(totalWeight)
	for i, weight := range weights {
		if pick < weight {
			return scored[i].card, true
		}
		pick -= weight
	}
	return scored[0].card, true
}

func (b *builder) totalSlots() int {
	total := 0
	for _, built := range b.used {
		total += built.Qty
	}
	return total
}

func (b *builder) nonLandSlots() int {
	total := 0
	for _, built := range b.used {
		if b.isLand(built.Card) {
			continue
		}
		total += built.Qty
	}
	return total
}

func (b *builder) countNonBasicLands() int {
	total := 0
	for _, built := range b.used {
		if !b.isLand(built.Card) || hasRole(built.Card, "land_basic") {
			continue
		}
		total += built.Qty
	}
	return total
}

func (b *builder) countFixingLands() int {
	total := 0
	for _, built := range b.used {
		if !b.isLand(built.Card) || !hasRole(built.Card, "land_fixing") {
			continue
		}
		total += built.Qty
	}
	return total
}

func (b *builder) countTappedLands() int {
	total := 0
	for _, built := range b.used {
		if !b.isLand(built.Card) || !hasLandTag(built.Card, "tapped") {
			continue
		}
		total += built.Qty
	}
	return total
}

func (b *builder) countColorlessLands() int {
	total := 0
	for _, built := range b.used {
		if !b.isLand(built.Card) || !isColorlessLand(built.Card) {
			continue
		}
		total += built.Qty
	}
	return total
}

func (b *builder) isLand(card CandidateCard) bool {
	return strings.Contains(strings.ToLower(card.TypeLine), "land")
}

func (b *builder) findNamedCard(name string) (CandidateCard, bool) {
	for _, card := range b.pool {
		if strings.EqualFold(strings.TrimSpace(card.Name), strings.TrimSpace(name)) {
			return card, true
		}
	}
	return CandidateCard{}, false
}

func (b *builder) addBasicLands(slots int) int {
	if slots <= 0 {
		return 0
	}
	colorDemand := b.colorDemand()
	toAdd := distributeBasics(b.commander.ColorIdentity, colorDemand, slots)
	added := 0
	for basicName, qty := range toAdd {
		if qty <= 0 {
			continue
		}
		card, ok := b.findNamedCard(basicName)
		if !ok {
			continue
		}
		b.addCard(card, "lands", qty)
		added += qty
	}
	return added
}

func (b *builder) landRampSpellCount() int {
	total := 0
	for _, built := range b.used {
		if b.isLand(built.Card) {
			continue
		}
		if built.Card.ScoreFlags["land_ramp"] {
			total += built.Qty
		}
	}
	return total
}

func (b *builder) desiredThemeHits() int {
	if b.profile.PrimaryTheme == "" {
		return 0
	}
	switch b.profile.PrimaryTheme {
	case "Voltron", "Spellslinger":
		return maxInt(8, b.profile.SynergyCount/2)
	default:
		return maxInt(10, b.profile.SynergyCount/2)
	}
}

func (b *builder) hardRampFloor() int {
	return 8
}

func (b *builder) hardDrawFloor() int {
	return 8
}

func (b *builder) hardInteractionFloor() int {
	switch b.profile.Strategy {
	case "Control":
		return 10
	case "Aggro":
		return 8
	default:
		return 9
	}
}

func (b *builder) hardWipeFloor() int {
	return 1
}

func (b *builder) cheapPlayFloor() int {
	switch b.profile.PrimaryTheme {
	case "Voltron", "Spellslinger":
		return 12
	default:
		return 10
	}
}

func (b *builder) landMixSummary() map[string]int {
	mix := map[string]int{
		"basic":     0,
		"fixing":    0,
		"utility":   0,
		"colorless": 0,
		"tapped":    0,
	}
	for _, built := range b.used {
		if !b.isLand(built.Card) {
			continue
		}
		if hasRole(built.Card, "land_basic") {
			mix["basic"] += built.Qty
			continue
		}
		if hasRole(built.Card, "land_fixing") {
			mix["fixing"] += built.Qty
		} else {
			mix["utility"] += built.Qty
		}
		if isColorlessLand(built.Card) {
			mix["colorless"] += built.Qty
		}
		if hasLandTag(built.Card, "tapped") {
			mix["tapped"] += built.Qty
		}
	}
	return mix
}

func (b *builder) addRepairAction(note string) {
	note = strings.TrimSpace(note)
	if note == "" || hasString(b.repairActions, note) {
		return
	}
	b.repairActions = append(b.repairActions, note)
}

func (b *builder) addFallbackNote(note string) {
	note = strings.TrimSpace(note)
	if note == "" || hasString(b.fallbackNotes, note) {
		return
	}
	b.fallbackNotes = append(b.fallbackNotes, note)
}

func (b *builder) specLabel(spec BucketSpec) string {
	if len(spec.Roles) > 0 {
		return strings.Join(spec.Roles, "/")
	}
	if len(spec.ScoreFlags) > 0 {
		return strings.Join(spec.ScoreFlags, "/")
	}
	if len(spec.StrategyTags) > 0 {
		return strings.Join(spec.StrategyTags, "/")
	}
	return spec.Name
}

func desiredNonbasicLandCount(colorCount int) int {
	switch colorCount {
	case 0:
		return 10
	case 1:
		return 4
	case 2:
		return 7
	case 3:
		return 9
	case 4:
		return 11
	default:
		return 13
	}
}

func desiredFixingLandCount(colorCount int) int {
	switch colorCount {
	case 0:
		return 0
	case 1:
		return 1
	case 2:
		return 6
	case 3:
		return 8
	case 4:
		return 10
	default:
		return 12
	}
}

func minimumBasicLandCount(colorCount, landRampCount int) int {
	if landRampCount > 0 {
		switch colorCount {
		case 0:
			return 10
		case 1:
			return 10
		case 2:
			return 9
		case 3:
			return 8
		case 4:
			return 7
		default:
			return 6
		}
	}
	switch colorCount {
	case 0:
		return 8
	case 1:
		return 8
	case 2:
		return 7
	case 3:
		return 6
	case 4:
		return 5
	default:
		return 4
	}
}

func tappedLandCap(colorCount int) int {
	switch colorCount {
	case 0:
		return 2
	case 1:
		return 3
	case 2:
		return 5
	case 3:
		return 6
	case 4:
		return 7
	default:
		return 8
	}
}

func desiredColorlessLandCap(colorCount int) int {
	switch colorCount {
	case 0:
		return 37
	case 1, 2:
		return 1
	default:
		return 0
	}
}

func distributeBasics(colors []string, demand map[string]int, slots int) map[string]int {
	out := map[string]int{}
	if slots <= 0 {
		return out
	}
	if len(colors) == 0 {
		out["Wastes"] = slots
		return out
	}

	remaining := slots
	if slots >= len(colors) {
		for _, color := range colors {
			out[basicLandName(color)] = 1
			remaining--
		}
	}

	type share struct {
		Name      string
		Weight    int
		Allocated int
	}
	shares := make([]share, 0, len(colors))
	for _, color := range colors {
		shares = append(shares, share{Name: basicLandName(color), Weight: maxInt(demand[color], 1)})
	}

	for remaining > 0 {
		bestIdx := 0
		bestRatio := -1.0
		for i, item := range shares {
			ratio := float64(item.Weight) / float64(item.Allocated+1)
			if ratio > bestRatio {
				bestRatio = ratio
				bestIdx = i
			}
		}
		shares[bestIdx].Allocated++
		out[shares[bestIdx].Name]++
		remaining--
	}
	return out
}

func preferredBasicLandName(colors []string, demand, sources map[string]int) string {
	bestColor := ""
	bestGap := -1
	bestDemand := -1
	for _, color := range colors {
		gap := demand[color] - sources[color]
		if gap > bestGap || (gap == bestGap && demand[color] > bestDemand) {
			bestColor = color
			bestGap = gap
			bestDemand = demand[color]
		}
	}
	if bestColor == "" {
		return basicLandName("")
	}
	return basicLandName(bestColor)
}

func basicLandName(color string) string {
	switch color {
	case "W":
		return "Plains"
	case "U":
		return "Island"
	case "B":
		return "Swamp"
	case "R":
		return "Mountain"
	case "G":
		return "Forest"
	default:
		return "Wastes"
	}
}

func isColorlessLand(card CandidateCard) bool {
	if hasLandTag(card, "colorless") {
		return true
	}
	for _, tag := range card.ManaTags {
		if strings.HasPrefix(tag, "produces:") && tag != "produces:any" && tag != "produces:C" {
			return false
		}
	}
	return hasManaTag(card, "produces:C")
}

func hasRole(card CandidateCard, roles ...string) bool {
	for _, role := range roles {
		if hasString(card.Roles, role) {
			return true
		}
	}
	return false
}

func hasTheme(card CandidateCard, themes ...string) bool {
	for _, theme := range themes {
		if hasString(card.Themes, theme) {
			return true
		}
	}
	return false
}

func hasStrategyTag(card CandidateCard, tags ...string) bool {
	for _, tag := range tags {
		if hasString(card.StrategyTags, tag) {
			return true
		}
	}
	return false
}

func hasLandTag(card CandidateCard, tags ...string) bool {
	for _, tag := range tags {
		if hasString(card.LandTags, tag) {
			return true
		}
	}
	return false
}

func hasManaTag(card CandidateCard, tags ...string) bool {
	for _, tag := range tags {
		if hasString(card.ManaTags, tag) {
			return true
		}
	}
	return false
}

func filterNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
