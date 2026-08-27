package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"manatomb/app/internal/cards"
)

const (
	guessCardAwardGuessLimit     = 10
	guessCardDefaultMaxQuestions = 8
)

type guessQuestion struct {
	ID     string
	Label  string
	Text   string
	Symbol string
}

type guessAnswerView struct {
	Number   int
	Kind     string
	Question string
	Answer   string
	Yes      bool
}

type guessClueView struct {
	Label     string
	Value     string
	ValueHTML template.HTML
}

type guessQuestionGroupView struct {
	Name      string
	Questions []guessQuestion
}

type guessRevealView struct {
	Number   int
	Label    string
	Question string
	Answer   string
	Yes      bool
	HasClue  bool
	Clue     guessClueView
}

type guessCardPageData struct {
	GameID             int64
	Status             string
	Completed          bool
	Won                bool
	QuestionCount      int
	GuessCount         int
	PossibleCardsLeft  int
	PossibleCardsTotal int
	NextGuessNumber    int
	IsDaily            bool
	HasAccount         bool
	AwardGuessLimit    int
	AwardGuessesLeft   int
	AwardStatus        string
	AwardEarned        bool
	GameModeLabel      string
	History            []guessAnswerView
	Clues              []guessClueView
	AvailableQuestions []guessQuestion
	QuestionGroups     []guessQuestionGroupView
	HasLatestReveal    bool
	LatestReveal       guessRevealView
	PreviousReveals    []guessRevealView
	QuestionsLeft      int
	MaxQuestions       int
	CanAsk             bool
	CanGuess           bool
	TargetName         string
	TargetImageURI     string
	TargetDetailPath   string
}

var guessCardQuestions = []guessQuestion{
	{ID: "color_w", Label: "White", Text: "Does its color identity include white?", Symbol: "W"},
	{ID: "color_u", Label: "Blue", Text: "Does its color identity include blue?", Symbol: "U"},
	{ID: "color_b", Label: "Black", Text: "Does its color identity include black?", Symbol: "B"},
	{ID: "color_r", Label: "Red", Text: "Does its color identity include red?", Symbol: "R"},
	{ID: "color_g", Label: "Green", Text: "Does its color identity include green?", Symbol: "G"},
	{ID: "colorless", Label: "Colorless", Text: "Is its color identity colorless?", Symbol: "C"},
	{ID: "monocolored", Label: "Monocolored", Text: "Does its color identity contain exactly one color?", Symbol: "1"},
	{ID: "multicolor", Label: "Multicolor", Text: "Does its color identity contain multiple colors?", Symbol: "2+"},
	{ID: "permanent", Label: "Permanent", Text: "Is it a permanent?", Symbol: "P"},
	{ID: "nonpermanent", Label: "Nonpermanent", Text: "Is it a nonpermanent?", Symbol: "N"},
	{ID: "creature", Label: "Creature", Text: "Is it a creature?", Symbol: "C"},
	{ID: "instant", Label: "Instant", Text: "Is it an instant?", Symbol: "I"},
	{ID: "sorcery", Label: "Sorcery", Text: "Is it a sorcery?", Symbol: "S"},
	{ID: "artifact", Label: "Artifact", Text: "Is it an artifact?", Symbol: "A"},
	{ID: "enchantment", Label: "Enchantment", Text: "Is it an enchantment?", Symbol: "E"},
	{ID: "planeswalker", Label: "Planeswalker", Text: "Is it a planeswalker?", Symbol: "PW"},
	{ID: "battle", Label: "Battle", Text: "Is it a battle?", Symbol: "BT"},
	{ID: "mv_ge_5", Label: "Greater than or equal to 5", Text: "Is its mana value greater than or equal to 5?", Symbol: "≥5"},
	{ID: "mv_le_5", Label: "Less than or equal to 5", Text: "Is its mana value less than or equal to 5?", Symbol: "≤5"},
	{ID: "mv_eq_0", Label: "0", Text: "Is its mana value exactly 0?", Symbol: "0"},
	{ID: "mv_eq_1", Label: "1", Text: "Is its mana value exactly 1?", Symbol: "1"},
	{ID: "mv_eq_2", Label: "2", Text: "Is its mana value exactly 2?", Symbol: "2"},
	{ID: "mv_eq_3", Label: "3", Text: "Is its mana value exactly 3?", Symbol: "3"},
	{ID: "mv_eq_4", Label: "4", Text: "Is its mana value exactly 4?", Symbol: "4"},
	{ID: "mv_eq_5", Label: "5", Text: "Is its mana value exactly 5?", Symbol: "5"},
	{ID: "draws_cards", Label: "Card draw", Text: "Does its rules text mention drawing cards?", Symbol: "+"},
	{ID: "makes_tokens", Label: "Tokens", Text: "Does its rules text mention tokens?", Symbol: "○"},
	{ID: "destroys", Label: "Destroy", Text: "Does its rules text mention destroying something?", Symbol: "×"},
	{ID: "exiles", Label: "Exile", Text: "Does its rules text mention exile?", Symbol: "↗"},
	{ID: "searches_library", Label: "Search", Text: "Does its rules text search a library?", Symbol: "?"},
	{ID: "graveyard", Label: "Graveyard", Text: "Does its rules text mention a graveyard?", Symbol: "↶"},
	{ID: "flying", Label: "Flying", Text: "Does its rules text mention flying?", Symbol: "↑"},
	{ID: "generates_mana", Label: "Mana", Text: "Does it produce mana or Treasures?", Symbol: "M"},
	{ID: "deals_damage", Label: "Damage", Text: "Does its rules text deal damage?", Symbol: "✶"},
	{ID: "protects", Label: "Protection", Text: "Does it grant or use a protective effect?", Symbol: "◆"},
}

// Legacy questions remain resolvable so rounds created before a catalog
// change can still render their original evidence. Keeping them outside the
// current catalog prevents them from becoming new one-tap choices.
var guessCardLegacyQuestions = []guessQuestion{
	{ID: "mv_le_2", Label: "≤ 2", Text: "Is its mana value 2 or less?", Symbol: "≤2"},
	{ID: "mv_le_3", Label: "≤ 3", Text: "Is its mana value 3 or less?", Symbol: "≤3"},
	{ID: "commander_legal", Label: "Commander legal", Text: "Is it legal in Commander?", Symbol: "C"},
	{ID: "legendary", Label: "Legendary", Text: "Is it legendary?", Symbol: "★"},
	{ID: "land", Label: "Land", Text: "Is it a land?", Symbol: "L"},
}

var guessCardQuestionGroupDefinitions = []struct {
	Name string
	IDs  []string
}{
	{Name: "Color", IDs: []string{"color_w", "color_u", "color_b", "color_r", "color_g", "colorless", "monocolored", "multicolor"}},
	{Name: "Card type", IDs: []string{"permanent", "nonpermanent", "creature", "instant", "sorcery", "artifact", "enchantment", "planeswalker", "battle"}},
	{Name: "Mana value", IDs: []string{"mv_ge_5", "mv_le_5", "mv_eq_0", "mv_eq_1", "mv_eq_2", "mv_eq_3", "mv_eq_4", "mv_eq_5"}},
	{Name: "Rules text", IDs: []string{"draws_cards", "makes_tokens", "destroys", "exiles", "searches_library", "graveyard", "flying", "generates_mana", "deals_damage", "protects"}},
}

func questionByID(questionID string) *guessQuestion {
	questionID = strings.TrimSpace(questionID)
	for i := range guessCardQuestions {
		if guessCardQuestions[i].ID == questionID {
			return &guessCardQuestions[i]
		}
	}
	for i := range guessCardLegacyQuestions {
		if guessCardLegacyQuestions[i].ID == questionID {
			return &guessCardLegacyQuestions[i]
		}
	}
	return nil
}

func groupGuessCardQuestions(questions []guessQuestion) []guessQuestionGroupView {
	available := make(map[string]guessQuestion, len(questions))
	for _, question := range questions {
		available[question.ID] = question
	}

	groups := make([]guessQuestionGroupView, 0, len(guessCardQuestionGroupDefinitions))
	for _, definition := range guessCardQuestionGroupDefinitions {
		group := guessQuestionGroupView{Name: definition.Name}
		for _, questionID := range definition.IDs {
			question, ok := available[questionID]
			if !ok {
				continue
			}
			group.Questions = append(group.Questions, question)
		}
		if len(group.Questions) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

func guessCardMaxQuestions(game guessCardGame) int {
	if game.MaxQuestions > 0 {
		return game.MaxQuestions
	}
	return guessCardDefaultMaxQuestions
}

func guessCardQuestionsUsed(game guessCardGame) int {
	used := game.QuestionCount
	if len(game.AskedQuestions) > used {
		used = len(game.AskedQuestions)
	}
	if used < 0 {
		return 0
	}
	return used
}

func guessCardQuestionsLeft(game guessCardGame) int {
	left := guessCardMaxQuestions(game) - guessCardQuestionsUsed(game)
	if left < 0 {
		return 0
	}
	return left
}

func matchGuessQuestion(raw string) *guessQuestion {
	query := normalizeGuessQuestionText(raw)
	if query == "" {
		return nil
	}

	type candidate struct {
		index int
		score int
	}
	candidates := make([]candidate, 0, len(guessCardQuestions))
	for i, question := range guessCardQuestions {
		text := normalizeGuessQuestionText(question.Text)
		switch {
		case text == query:
			candidates = append(candidates, candidate{index: i, score: 100})
		case strings.Contains(text, query):
			candidates = append(candidates, candidate{index: i, score: 80})
		case guessQuestionContainsTerms(text, query):
			candidates = append(candidates, candidate{index: i, score: 60})
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return &guessCardQuestions[candidates[0].index]
}

func normalizeGuessQuestionText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastSpace := true
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	normalized := strings.TrimSpace(b.String())
	normalized = strings.ReplaceAll(normalized, "non permanent", "nonpermanent")
	normalized = strings.ReplaceAll(normalized, "mono colored", "monocolored")
	return normalized
}

func guessQuestionContainsTerms(text, query string) bool {
	terms := guessQuestionSignificantTerms(query)
	if len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

func guessQuestionSignificantTerms(value string) []string {
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "card": true, "does": true,
		"do": true, "have": true, "has": true, "is": true, "it": true,
		"its": true, "the": true, "this": true, "with": true,
	}
	terms := strings.Fields(value)
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		if stopWords[term] {
			continue
		}
		out = append(out, term)
	}
	return out
}

func (a *App) HandleGuessCardShow(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	player := a.gamePlayer(w, r)

	game, err := a.guessCardGameForShow(r, player)
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			setFlash(w, "No cards are ready for the game yet.")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	card, err := a.loadGuessCardTarget(r.Context(), *game)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	page := buildGuessCardPageData(*game, *card)
	if !page.Completed {
		counts, err := loadGuessCardPossibilityCounts(r.Context(), a.DB, *game, *card)
		if err != nil {
			log.Printf("guess card possibility count unavailable for game %d: %v", game.ID, err)
		} else {
			page.PossibleCardsLeft = counts.Possible
			page.PossibleCardsTotal = counts.Total
		}
	}

	a.Renderer.Render(w, "guess_card", TemplateData{
		CurrentUser: user,
		Data:        page,
		Flash:       readFlash(w, r),
	})
}

func (a *App) guessCardGameForShow(r *http.Request, player gamePlayer) (*guessCardGame, error) {
	rawGameID := strings.TrimSpace(r.URL.Query().Get("game_id"))
	if rawGameID == "" {
		return activeOrNewGuessCardGame(r.Context(), a.DB, player)
	}
	gameID, err := strconv.ParseInt(rawGameID, 10, 64)
	if err != nil || gameID <= 0 {
		return activeOrNewGuessCardGame(r.Context(), a.DB, player)
	}
	game, err := loadGuessCardGameByID(r.Context(), a.DB, player, gameID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return activeOrNewGuessCardGame(r.Context(), a.DB, player)
		}
		return nil, err
	}
	if guessCardActiveGameExpired(*game) {
		if err := abandonGuessCardGame(r.Context(), a.DB, player, game.ID); err != nil {
			return nil, err
		}
		return activeOrNewGuessCardGame(r.Context(), a.DB, player)
	}
	return game, nil
}

func (a *App) HandleGuessCardPost(w http.ResponseWriter, r *http.Request) {
	player := a.gamePlayer(w, r)
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		setFlash(w, "Invalid game request.")
		http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
		return
	}

	switch strings.ToLower(strings.TrimSpace(r.Form.Get("action"))) {
	case "new":
		a.handleGuessCardNewPost(w, r, player)
	case "question":
		a.handleGuessCardQuestionPost(w, r, player)
	case "guess":
		a.handleGuessCardFinalGuessPost(w, r, player)
	case "give_up":
		a.handleGuessCardGiveUpPost(w, r, player)
	default:
		setFlash(w, "Choose a game action.")
		http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
	}
}

func (a *App) handleGuessCardNewPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	previousGameID, ok := parseGuessCardGameID(r.Form.Get("game_id"))
	if !ok {
		setFlash(w, "That round could not be replayed. The current game has been refreshed.")
		http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
		return
	}
	previous, err := loadGuessCardGameByID(r.Context(), a.DB, player, previousGameID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			setFlash(w, "That round could not be replayed. The current game has been refreshed.")
			http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}
	if previous.Status == "active" {
		setFlash(w, "Finish or reveal the current card before starting a practice round.")
		http.Redirect(w, r, guessCardRoundPath(previous.ID), http.StatusSeeOther)
		return
	}

	if active, err := loadActiveGuessCardGame(r.Context(), a.DB, player); err == nil {
		setFlash(w, "A round is already in progress.")
		http.Redirect(w, r, guessCardRoundPath(active.ID), http.StatusSeeOther)
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		a.RenderServerError(w, r, err)
		return
	}

	game, err := createReplayGuessCardGame(r.Context(), a.DB, player)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
}

func (a *App) handleGuessCardQuestionPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadGuessCardGameForMutation(r.Context(), a.DB, player, r.Form.Get("game_id"))
	if err != nil {
		if errors.Is(err, errGuessCardRoundStale) {
			setFlash(w, "That round is no longer active. The current game has been refreshed.")
			http.Redirect(w, r, guessCardRoundRefreshPath(r.Form.Get("game_id")), http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	questionID := strings.TrimSpace(r.Form.Get("question_id"))
	if questionID == "" {
		if question := matchGuessQuestion(r.Form.Get("question")); question != nil {
			questionID = question.ID
		}
	}
	if guessCardQuestionsLeft(*game) == 0 {
		setFlash(w, "No questions remain. Make a guess or give up.")
		http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
		return
	}
	target, err := a.loadGuessCardTarget(r.Context(), *game)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	if !guessCardQuestionAvailable(*game, *target, questionID) {
		setFlash(w, "That question is no longer available.")
		http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
		return
	}
	if err := addGuessCardQuestion(r.Context(), a.DB, game.ID, player, questionID); err != nil {
		if errors.Is(err, errGuessCardQuestionUnavailable) {
			setFlash(w, "That question is no longer available.")
			http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}
	http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
}

func (a *App) handleGuessCardFinalGuessPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadGuessCardGameForMutation(r.Context(), a.DB, player, r.Form.Get("game_id"))
	if err != nil {
		if errors.Is(err, errGuessCardRoundStale) {
			setFlash(w, "That round is no longer active. The current game has been refreshed.")
			http.Redirect(w, r, guessCardRoundRefreshPath(r.Form.Get("game_id")), http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	guess := strings.TrimSpace(r.Form.Get("guess"))
	if guess == "" {
		setFlash(w, "Enter a card name before making your final guess.")
		http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
		return
	}
	expectedGuessNumber, err := strconv.Atoi(strings.TrimSpace(r.Form.Get("guess_number")))
	if err != nil || expectedGuessNumber <= 0 {
		setFlash(w, "That guess could not be verified. No attempt was used; please try again.")
		http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
		return
	}
	target, err := a.loadGuessCardTarget(r.Context(), *game)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	matches, err := cards.SearchCards(r.Context(), a.DB, cards.CardSearchParams{
		Query:     guess,
		NameExact: true,
		Limit:     1,
	})
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	guessedOracleID := ""
	if len(matches) > 0 {
		guessedOracleID = matches[0].OracleID
	}
	attempt, err := recordGuessCardFinalGuess(r.Context(), a.DB, game.ID, player, *target, guessedOracleID, guess, expectedGuessNumber)
	if err != nil {
		if errors.Is(err, errGuessCardAttemptDuplicate) {
			setFlash(w, "That guess was already submitted. No extra attempt was used.")
			http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
			return
		}
		if errors.Is(err, errGuessCardGameUnavailable) {
			setFlash(w, "That round is no longer active. The current game has been refreshed.")
			http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}
	if !attempt.Won {
		http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
}

func (a *App) handleGuessCardGiveUpPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadGuessCardGameForMutation(r.Context(), a.DB, player, r.Form.Get("game_id"))
	if err != nil {
		if errors.Is(err, errGuessCardRoundStale) {
			setFlash(w, "That round is no longer active. The current game has been refreshed.")
			http.Redirect(w, r, guessCardRoundRefreshPath(r.Form.Get("game_id")), http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	if err := completeGuessCardGame(r.Context(), a.DB, game.ID, player, false); err != nil {
		if errors.Is(err, errGuessCardGameUnavailable) {
			setFlash(w, "That round was already completed. The result has been refreshed.")
			http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}
	http.Redirect(w, r, guessCardRoundPath(game.ID), http.StatusSeeOther)
}

func addAutomaticGuessClue(ctx context.Context, db *sql.DB, game guessCardGame, card cards.Card) (bool, error) {
	question := automaticGuessQuestion(card, game.AskedQuestions)
	if question == nil {
		return false, nil
	}
	if err := addGuessCardQuestion(ctx, db, game.ID, gamePlayerFromGame(game.UserID, game.GuestID), question.ID); err != nil {
		return false, err
	}
	return true, nil
}

func automaticGuessQuestion(card cards.Card, askedQuestions []string) *guessQuestion {
	asked := map[string]bool{}
	for _, questionID := range askedQuestions {
		asked[questionID] = true
	}

	for _, questionID := range automaticGuessQuestionPriority() {
		question := questionByID(questionID)
		if question == nil || asked[question.ID] {
			continue
		}
		if guessQuestionAnswer(card, question.ID) {
			return question
		}
	}

	var fallback *guessQuestion
	for i := range guessCardQuestions {
		question := &guessCardQuestions[i]
		if asked[question.ID] {
			continue
		}
		if guessQuestionAnswer(card, question.ID) {
			return question
		}
		if fallback == nil {
			fallback = question
		}
	}
	return fallback
}

func automaticGuessQuestionPriority() []string {
	return []string{
		"permanent",
		"nonpermanent",
		"monocolored",
		"multicolor",
		"colorless",
		"color_w",
		"color_u",
		"color_b",
		"color_r",
		"color_g",
		"creature",
		"instant",
		"sorcery",
		"artifact",
		"enchantment",
		"planeswalker",
		"battle",
		"mv_ge_5",
		"mv_le_5",
		"mv_eq_0",
		"mv_eq_1",
		"mv_eq_2",
		"mv_eq_3",
		"mv_eq_4",
		"mv_eq_5",
	}
}

func guessCardActiveGameExpired(game guessCardGame) bool {
	return game.Status == "active" &&
		strings.TrimSpace(game.DailyKey) != "" &&
		strings.TrimSpace(game.DailyKey) != guessCardDailyKey(time.Now().UTC())
}

func guessCardGameAwardEligible(game guessCardGame) bool {
	return game.GuestID == "" &&
		game.IsDaily &&
		strings.TrimSpace(game.DailyKey) != "" &&
		strings.TrimSpace(game.DailyKey) == guessCardDailyKey(time.Now().UTC())
}

func buildGuessCardPageData(game guessCardGame, card cards.Card) guessCardPageData {
	completed := game.Status != "active"
	awardLeft := guessCardAwardGuessLimit - 1 - game.GuessCount
	if awardLeft < 0 {
		awardLeft = 0
	}
	awardStatus := "Eligible"
	if !game.IsDaily {
		awardStatus = "Practice"
	} else if game.GuestID != "" {
		if completed {
			awardStatus = "Guest result"
		} else {
			awardStatus = "Sign in to earn"
		}
	} else if completed {
		switch {
		case game.Status == "won" && game.AwardEarned:
			awardStatus = "Earned"
		case game.Status == "won":
			awardStatus = "Solved"
		default:
			awardStatus = "Not earned"
		}
	} else if game.GuessCount >= guessCardAwardGuessLimit-1 {
		awardStatus = "Award closed"
	}
	gameMode := "Daily Game"
	if !game.IsDaily {
		gameMode = "Practice Game"
	}

	clues := buildGuessCardClues(game, card)
	reveals := buildGuessCardReveals(game, card, clues)
	history := buildGuessCardHistory(game, card)

	availableQuestions := availableGuessCardQuestionsForCard(game.AskedQuestions, clues, card)
	maxQuestions := guessCardMaxQuestions(game)
	questionsLeft := guessCardQuestionsLeft(game)
	canAsk := !completed && questionsLeft > 0 && len(availableQuestions) > 0

	var latestReveal guessRevealView
	var previousReveals []guessRevealView
	if len(reveals) > 0 {
		latestReveal = reveals[len(reveals)-1]
		if completed {
			previousReveals = append(previousReveals, reveals...)
		} else {
			previousReveals = append(previousReveals, reveals[:len(reveals)-1]...)
		}
	}

	return guessCardPageData{
		GameID:             game.ID,
		Status:             guessGameStatusLabel(game.Status),
		Completed:          completed,
		Won:                game.Status == "won",
		QuestionCount:      guessCardQuestionsUsed(game),
		GuessCount:         game.GuessCount,
		NextGuessNumber:    game.GuessCount + 1,
		IsDaily:            game.IsDaily,
		HasAccount:         game.GuestID == "",
		AwardGuessLimit:    guessCardAwardGuessLimit,
		AwardGuessesLeft:   awardLeft,
		AwardStatus:        awardStatus,
		AwardEarned:        game.AwardEarned,
		GameModeLabel:      gameMode,
		History:            history,
		Clues:              clues,
		AvailableQuestions: availableQuestions,
		QuestionGroups:     groupGuessCardQuestions(availableQuestions),
		HasLatestReveal:    len(reveals) > 0,
		LatestReveal:       latestReveal,
		PreviousReveals:    previousReveals,
		QuestionsLeft:      questionsLeft,
		MaxQuestions:       maxQuestions,
		CanAsk:             canAsk,
		CanGuess:           !completed,
		TargetName:         card.Name,
		TargetImageURI:     card.ImageURI,
		TargetDetailPath:   cardPrintingDetailPath(card.OracleID, guessCardDetailPrintingID(card, game.TargetScryfallID)),
	}
}

func buildGuessCardHistory(game guessCardGame, card cards.Card) []guessAnswerView {
	events := game.HistoryEvents
	if len(events) == 0 {
		events = make([]string, 0, len(game.AskedQuestions))
		for _, questionID := range game.AskedQuestions {
			questionID = strings.TrimSpace(questionID)
			if questionID != "" {
				events = append(events, guessCardHistoryQuestionPrefix+questionID)
			}
		}
	}

	history := make([]guessAnswerView, 0, len(events))
	for _, event := range events {
		event = strings.TrimSpace(event)
		switch {
		case strings.HasPrefix(event, guessCardHistoryQuestionPrefix):
			question := questionByID(strings.TrimPrefix(event, guessCardHistoryQuestionPrefix))
			if question == nil {
				continue
			}
			yes := guessQuestionAnswer(card, question.ID)
			answer := "No"
			if yes {
				answer = "Yes"
			}
			history = append(history, guessAnswerView{
				Number:   len(history) + 1,
				Kind:     "question",
				Question: question.Text,
				Answer:   answer,
				Yes:      yes,
			})
		case strings.HasPrefix(event, guessCardHistoryGuessPrefix):
			name := strings.TrimSpace(strings.TrimPrefix(event, guessCardHistoryGuessPrefix))
			if name == "" {
				continue
			}
			if !strings.HasSuffix(name, "?") {
				name += "?"
			}
			history = append(history, guessAnswerView{
				Number:   len(history) + 1,
				Kind:     "guess",
				Question: name,
				Answer:   "No",
			})
		}
	}
	return history
}

func guessCardDetailPrintingID(card cards.Card, fallback string) string {
	if printingID := strings.TrimSpace(card.ID); printingID != "" {
		return printingID
	}
	return strings.TrimSpace(fallback)
}

func buildGuessCardReveals(game guessCardGame, card cards.Card, clues []guessClueView) []guessRevealView {
	reveals := make([]guessRevealView, 0, len(game.AskedQuestions))
	for index, questionID := range game.AskedQuestions {
		question := questionByID(questionID)
		if question == nil {
			continue
		}
		yes := guessQuestionAnswer(card, question.ID)
		answer := "No"
		if yes {
			answer = "Yes"
		}
		reveal := guessRevealView{
			Number:   index + 1,
			Label:    question.Label,
			Question: question.Text,
			Answer:   answer,
			Yes:      yes,
		}
		if index < len(clues) {
			reveal.HasClue = true
			reveal.Clue = clues[index]
		}
		reveals = append(reveals, reveal)
	}
	return reveals
}

func availableGuessCardQuestions(askedQuestions []string, clues []guessClueView) []guessQuestion {
	asked := map[string]bool{}
	for _, questionID := range askedQuestions {
		asked[strings.TrimSpace(questionID)] = true
	}
	eliminated := guessCardQuestionEliminations(clues)

	out := make([]guessQuestion, 0, len(guessCardQuestions))
	for _, question := range guessCardQuestions {
		if asked[question.ID] || eliminated[question.ID] {
			continue
		}
		out = append(out, question)
	}
	return out
}

func availableGuessCardQuestionsForCard(askedQuestions []string, clues []guessClueView, card cards.Card) []guessQuestion {
	answerEliminations := guessCardAnswerEliminations(guessCardAnswerFacts(askedQuestions, card))
	available := availableGuessCardQuestions(askedQuestions, clues)
	out := make([]guessQuestion, 0, len(available))
	for _, question := range available {
		if answerEliminations[question.ID] {
			continue
		}
		out = append(out, question)
	}
	return out
}

func guessCardAnswerFacts(askedQuestions []string, card cards.Card) map[string]bool {
	facts := make(map[string]bool, len(askedQuestions))
	for _, questionID := range askedQuestions {
		questionID = strings.TrimSpace(questionID)
		if questionByID(questionID) == nil {
			continue
		}
		facts[questionID] = guessQuestionAnswer(card, questionID)
	}
	return facts
}

func guessCardAnswerEliminations(facts map[string]bool) map[string]bool {
	out := map[string]bool{}
	merge := func(eliminations map[string]bool) {
		for questionID := range eliminations {
			out[questionID] = true
		}
	}

	merge(guessCardModelEliminations(facts, guessCardColorQuestionIDs(), guessCardColorModels()))
	merge(guessCardModelEliminations(facts, guessCardManaValueQuestionIDs(), guessCardManaValueModels()))
	merge(guessCardModelEliminations(facts, guessCardTypeQuestionIDs(), guessCardTypeModels()))

	// Once a player has chosen either broad colored-card question, the sibling
	// option is no longer useful. Colorless and the five individual colors stay
	// available when they can still narrow the identity further.
	if _, asked := facts["monocolored"]; asked {
		out["multicolor"] = true
	}
	if _, asked := facts["multicolor"]; asked {
		out["monocolored"] = true
	}

	return out
}

func guessCardModelEliminations(facts map[string]bool, questionIDs []string, models []map[string]bool) map[string]bool {
	relevantFacts := map[string]bool{}
	for _, questionID := range questionIDs {
		if answer, ok := facts[questionID]; ok {
			relevantFacts[questionID] = answer
		}
	}
	if len(relevantFacts) == 0 {
		return nil
	}

	consistent := make([]map[string]bool, 0, len(models))
	for _, model := range models {
		matches := true
		for questionID, answer := range relevantFacts {
			if model[questionID] != answer {
				matches = false
				break
			}
		}
		if matches {
			consistent = append(consistent, model)
		}
	}
	if len(consistent) == 0 {
		return nil
	}

	out := map[string]bool{}
	for _, questionID := range questionIDs {
		if _, asked := relevantFacts[questionID]; asked {
			continue
		}
		answer := consistent[0][questionID]
		settled := true
		for _, model := range consistent[1:] {
			if model[questionID] != answer {
				settled = false
				break
			}
		}
		if settled {
			out[questionID] = true
		}
	}
	return out
}

func guessCardColorQuestionIDs() []string {
	return []string{"color_w", "color_u", "color_b", "color_r", "color_g", "colorless", "monocolored", "multicolor"}
}

func guessCardColorModels() []map[string]bool {
	models := make([]map[string]bool, 0, 32)
	colorIDs := []string{"color_w", "color_u", "color_b", "color_r", "color_g"}
	for mask := 0; mask < 32; mask++ {
		model := map[string]bool{}
		colorCount := 0
		for index, questionID := range colorIDs {
			included := mask&(1<<index) != 0
			model[questionID] = included
			if included {
				colorCount++
			}
		}
		model["colorless"] = colorCount == 0
		model["monocolored"] = colorCount == 1
		model["multicolor"] = colorCount > 1
		models = append(models, model)
	}
	return models
}

func guessCardManaValueQuestionIDs() []string {
	return []string{
		"mv_ge_5",
		"mv_le_5",
		"mv_eq_0",
		"mv_eq_1",
		"mv_eq_2",
		"mv_eq_3",
		"mv_eq_4",
		"mv_eq_5",
		"mv_le_2",
		"mv_le_3",
	}
}

func guessCardManaValueModels() []map[string]bool {
	// Integer and half-step representatives cover every distinct combination of
	// the current exact/range questions and the two legacy range questions.
	values := []float64{0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5, 5.5}
	models := make([]map[string]bool, 0, len(values))
	for _, value := range values {
		model := map[string]bool{
			"mv_ge_5": value >= 5,
			"mv_le_5": value <= 5,
			"mv_le_2": value <= 2,
			"mv_le_3": value <= 3,
		}
		for exact := 0; exact <= 5; exact++ {
			model[fmt.Sprintf("mv_eq_%d", exact)] = value == float64(exact)
		}
		models = append(models, model)
	}
	return models
}

func guessCardTypeQuestionIDs() []string {
	return []string{"permanent", "nonpermanent", "creature", "instant", "sorcery", "artifact", "enchantment", "planeswalker", "land", "battle"}
}

func guessCardTypeModels() []map[string]bool {
	models := make([]map[string]bool, 0, 256)
	baseIDs := []string{"creature", "artifact", "enchantment", "planeswalker", "land", "battle", "instant", "sorcery"}
	for mask := 0; mask < 1<<len(baseIDs); mask++ {
		model := map[string]bool{}
		for index, questionID := range baseIDs {
			model[questionID] = mask&(1<<index) != 0
		}
		model["permanent"] = model["creature"] || model["artifact"] || model["enchantment"] || model["planeswalker"] || model["land"] || model["battle"]
		model["nonpermanent"] = model["instant"] || model["sorcery"]
		models = append(models, model)
	}
	return models
}

func guessCardQuestionAvailable(game guessCardGame, card cards.Card, questionID string) bool {
	questionID = strings.TrimSpace(questionID)
	if questionID == "" || guessCardQuestionsLeft(game) == 0 {
		return false
	}
	clues := buildGuessCardClues(game, card)
	for _, question := range availableGuessCardQuestionsForCard(game.AskedQuestions, clues, card) {
		if question.ID == questionID {
			return true
		}
	}
	return false
}

func guessCardQuestionEliminations(clues []guessClueView) map[string]bool {
	out := map[string]bool{}
	add := func(questionIDs ...string) {
		for _, questionID := range questionIDs {
			out[questionID] = true
		}
	}

	for _, clue := range clues {
		switch strings.ToLower(strings.TrimSpace(clue.Label)) {
		case "card type":
			add(
				"permanent",
				"nonpermanent",
				"creature",
				"instant",
				"sorcery",
				"artifact",
				"enchantment",
				"planeswalker",
				"land",
				"battle",
				"legendary",
			)
		case "rules text":
			add(
				"draws_cards",
				"makes_tokens",
				"destroys",
				"exiles",
				"searches_library",
				"graveyard",
				"flying",
				"generates_mana",
				"deals_damage",
				"protects",
			)
		case "cast cost", "card cost", "mana value":
			add(
				"mv_ge_5",
				"mv_le_5",
				"mv_eq_0",
				"mv_eq_1",
				"mv_eq_2",
				"mv_eq_3",
				"mv_eq_4",
				"mv_eq_5",
				"mv_le_2",
				"mv_le_3",
			)
		case "color identity":
			add(
				"color_w",
				"color_u",
				"color_b",
				"color_r",
				"color_g",
				"colorless",
				"monocolored",
				"multicolor",
			)
		case "commander":
			add("commander_legal")
		}
	}
	return out
}

func buildGuessCardClues(game guessCardGame, card cards.Card) []guessClueView {
	pool := stagedGuessCardCluePool(guessCardCluePool(card), guessCardMaxQuestions(game))
	if len(pool) == 0 || len(game.AskedQuestions) == 0 {
		return nil
	}

	clueCount := len(game.AskedQuestions)
	if clueCount > len(pool) {
		clueCount = len(pool)
	}
	out := make([]guessClueView, 0, clueCount)
	for i := 0; i < clueCount; i++ {
		clue := pool[i]
		if game.Status == "active" {
			clue = maskGuessCardClueNames(clue, card)
		}
		out = append(out, clue)
	}
	return out
}

func stagedGuessCardCluePool(pool []guessClueView, limit int) []guessClueView {
	if limit <= 0 || len(pool) == 0 {
		return nil
	}
	if limit > len(pool) {
		limit = len(pool)
	}

	stageLabels := [][]string{
		{"rarity"},
		{"mana value"},
		{"color identity"},
		{"card type"},
		{"power/toughness", "loyalty", "commander"},
		{"cast cost", "card cost"},
		{"default set", "release date", "artist"},
		{"rules text", "flavor text"},
	}
	buckets := make([][]guessClueView, len(stageLabels))
	clueByLabel := make(map[string]guessClueView, len(pool))
	for _, clue := range pool {
		clueByLabel[strings.ToLower(strings.TrimSpace(clue.Label))] = clue
	}
	usedLabels := make(map[string]bool, len(pool))
	for stage, labels := range stageLabels {
		for _, label := range labels {
			clue, ok := clueByLabel[label]
			if !ok {
				continue
			}
			buckets[stage] = append(buckets[stage], clue)
			usedLabels[label] = true
		}
	}
	for _, clue := range pool {
		label := strings.ToLower(strings.TrimSpace(clue.Label))
		if !usedLabels[label] {
			buckets[len(buckets)-2] = append(buckets[len(buckets)-2], clue)
		}
	}

	selected := make([]int, len(buckets))
	selectedCount := 0
	for stage := range buckets {
		if len(buckets[stage]) == 0 || selectedCount >= limit {
			continue
		}
		selected[stage] = 1
		selectedCount++
	}
	for stage := range buckets {
		for selectedCount < limit && selected[stage] < len(buckets[stage]) {
			selected[stage]++
			selectedCount++
		}
	}

	out := make([]guessClueView, 0, selectedCount)
	for stage := range buckets {
		out = append(out, buckets[stage][:selected[stage]]...)
	}
	return out
}

func maskGuessCardClueNames(clue guessClueView, card cards.Card) guessClueView {
	masked := maskGuessCardNames(clue.Value, card)
	if masked == clue.Value {
		return clue
	}
	clue.Value = masked
	if clue.ValueHTML != "" {
		clue.ValueHTML = renderGuessCardRulesTextHTML(masked)
	}
	return clue
}

func maskGuessCardNames(value string, card cards.Card) string {
	names := make([]string, 0, len(card.Faces)+3)
	addName := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	addName(card.Name)
	for _, name := range strings.Split(card.Name, "//") {
		addName(name)
	}
	for _, face := range card.Faces {
		addName(face.Name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})

	seen := map[string]bool{}
	masked := value
	for _, name := range names {
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		pattern, err := regexp.Compile(`(?i)(^|[^\p{L}\p{N}])(` + regexp.QuoteMeta(name) + `)([^\p{L}\p{N}]|$)`)
		if err != nil {
			continue
		}
		masked = pattern.ReplaceAllString(masked, `${1}[card name]${3}`)
	}
	return masked
}

func guessCardCluePool(card cards.Card) []guessClueView {
	clues := make([]guessClueView, 0, 12)
	add := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		clues = append(clues, guessClueView{Label: label, Value: value})
	}

	manaCost := guessCardCombinedFaceValue(card.ManaCost, card.Faces, func(face cards.CardFace) string {
		return face.ManaCost
	})
	if strings.TrimSpace(manaCost) == "" {
		manaCost = "No mana cost"
	}
	clues = append(clues, guessClueView{
		Label:     "Cast Cost",
		Value:     manaCost,
		ValueHTML: renderGuessCardRulesTextHTML(manaCost),
	})
	add("Card type", guessCardCombinedFaceValue(card.TypeLine, card.Faces, func(face cards.CardFace) string {
		return face.TypeLine
	}))
	rulesText := guessCardCombinedFaceValue(card.OracleText, card.Faces, func(face cards.CardFace) string {
		return face.OracleText
	})
	if strings.TrimSpace(rulesText) != "" {
		clues = append(clues, guessClueView{
			Label:     "Rules text",
			Value:     rulesText,
			ValueHTML: renderGuessCardRulesTextHTML(rulesText),
		})
	}
	add("Flavor text", guessCardCombinedFaceValue(card.FlavorText, card.Faces, func(face cards.CardFace) string {
		return face.FlavorText
	}))
	add("Mana value", formatCardManaValue(card.CMC))
	add("Color identity", formatCardColorNames(card.ColorIdentity))
	if card.CommanderLegal {
		add("Commander", "Legal in Commander")
	} else {
		add("Commander", "Not legal in Commander")
	}
	if strings.TrimSpace(card.Power) != "" || strings.TrimSpace(card.Toughness) != "" {
		add("Power/Toughness", formatCardStatText(card.Power)+"/"+formatCardStatText(card.Toughness))
	}
	add("Loyalty", strings.TrimSpace(card.Loyalty))
	add("Rarity", guessCardReadableValue(card.Rarity))
	add("Default set", card.SetName)
	add("Release date", card.ReleasedAt)
	add("Artist", card.Artist)
	return clues
}

func renderGuessCardRulesTextHTML(value string) template.HTML {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var b strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '{' {
			next := strings.IndexByte(value[i:], '{')
			if next < 0 {
				next = len(value) - i
			}
			b.WriteString(strings.ReplaceAll(template.HTMLEscapeString(value[i:i+next]), "\n", "<br>"))
			i += next
			continue
		}

		end := strings.IndexByte(value[i:], '}')
		if end <= 0 {
			b.WriteString(template.HTMLEscapeString(value[i:]))
			break
		}
		rawSymbol := strings.TrimSpace(value[i+1 : i+end])
		if rawSymbol == "" {
			b.WriteString(template.HTMLEscapeString(value[i : i+end+1]))
			i += end + 1
			continue
		}
		assetName := guessCardManaSymbolAssetName(rawSymbol)
		if assetName == "" {
			b.WriteString(template.HTMLEscapeString(value[i : i+end+1]))
			i += end + 1
			continue
		}
		escapedSymbol := template.HTMLEscapeString(rawSymbol)
		b.WriteString(`<img src="https://svgs.scryfall.io/card-symbols/`)
		b.WriteString(url.PathEscape(assetName))
		b.WriteString(`.svg" alt="{`)
		b.WriteString(escapedSymbol)
		b.WriteString(`}" title="{`)
		b.WriteString(escapedSymbol)
		b.WriteString(`}" class="inline-block h-4 w-4 mx-[1px] align-text-bottom">`)
		i += end + 1
	}
	return template.HTML(b.String())
}

func guessCardManaSymbolAssetName(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return ""
	}
	return strings.ReplaceAll(symbol, "/", "")
}

func guessCardCombinedFaceValue(cardValue string, faces []cards.CardFace, faceValue func(cards.CardFace) string) string {
	cardValue = strings.TrimSpace(cardValue)
	if cardValue != "" {
		return cardValue
	}

	values := make([]string, 0, len(faces))
	for _, face := range faces {
		value := strings.TrimSpace(faceValue(face))
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return strings.Join(values, " // ")
}

func guessCardReadableValue(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "_", " "))
	if value == "" {
		return ""
	}
	parts := strings.Fields(value)
	for i, part := range parts {
		if len(part) <= 1 {
			parts[i] = strings.ToUpper(part)
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func guessGameStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "won":
		return "Won"
	case "lost":
		return "Lost"
	case "abandoned":
		return "Abandoned"
	default:
		return "In Progress"
	}
}

func guessQuestionAnswer(card cards.Card, questionID string) bool {
	switch questionID {
	case "color_w":
		return guessHasColor(card, "W")
	case "color_u":
		return guessHasColor(card, "U")
	case "color_b":
		return guessHasColor(card, "B")
	case "color_r":
		return guessHasColor(card, "R")
	case "color_g":
		return guessHasColor(card, "G")
	case "colorless":
		return len(card.ColorIdentity) == 0
	case "monocolored":
		return len(card.ColorIdentity) == 1
	case "multicolor":
		return len(card.ColorIdentity) > 1
	case "permanent":
		return guessIsPermanent(card)
	case "nonpermanent":
		return guessIsNonpermanent(card)
	case "creature", "instant", "sorcery", "artifact", "enchantment", "planeswalker", "land", "battle":
		return strings.Contains(guessTypeLine(card), questionID)
	case "legendary":
		return strings.Contains(guessTypeLine(card), "legendary")
	case "mv_le_2":
		return card.CMC <= 2
	case "mv_le_3":
		return card.CMC <= 3
	case "mv_ge_5":
		return card.CMC >= 5
	case "mv_le_5":
		return card.CMC <= 5
	case "mv_eq_0":
		return card.CMC == 0
	case "mv_eq_1":
		return card.CMC == 1
	case "mv_eq_2":
		return card.CMC == 2
	case "mv_eq_3":
		return card.CMC == 3
	case "mv_eq_4":
		return card.CMC == 4
	case "mv_eq_5":
		return card.CMC == 5
	case "commander_legal":
		return card.CommanderLegal
	case "draws_cards":
		return strings.Contains(guessOracleText(card), "draw")
	case "makes_tokens":
		return strings.Contains(guessOracleText(card), "token")
	case "destroys":
		return strings.Contains(guessOracleText(card), "destroy")
	case "exiles":
		return strings.Contains(guessOracleText(card), "exile")
	case "searches_library":
		text := guessOracleText(card)
		return strings.Contains(text, "search") && strings.Contains(text, "library")
	case "graveyard":
		return strings.Contains(guessOracleText(card), "graveyard")
	case "flying":
		return strings.Contains(guessOracleText(card), "flying")
	case "generates_mana":
		return guessGeneratesMana(card)
	case "deals_damage":
		return guessDealsDamage(card)
	case "protects":
		return guessProtectsSomething(card)
	default:
		return false
	}
}

func guessHasColor(card cards.Card, color string) bool {
	for _, item := range card.ColorIdentity {
		if strings.EqualFold(strings.TrimSpace(item), color) {
			return true
		}
	}
	return false
}

func guessIsPermanent(card cards.Card) bool {
	typeLine := guessTypeLine(card)
	permanentTypes := []string{"artifact", "battle", "creature", "enchantment", "land", "planeswalker"}
	for _, cardType := range permanentTypes {
		if strings.Contains(typeLine, cardType) {
			return true
		}
	}
	return false
}

func guessIsNonpermanent(card cards.Card) bool {
	typeLine := guessTypeLine(card)
	return strings.Contains(typeLine, "instant") || strings.Contains(typeLine, "sorcery")
}

func guessTypeLine(card cards.Card) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(card.TypeLine))
	for _, face := range card.Faces {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(face.TypeLine))
	}
	return b.String()
}

func guessOracleText(card cards.Card) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(card.OracleText))
	for _, face := range card.Faces {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(face.OracleText))
	}
	return b.String()
}

func guessGeneratesMana(card cards.Card) bool {
	text := guessOracleText(card)
	return strings.Contains(text, "add ") ||
		strings.Contains(text, "add{") ||
		strings.Contains(text, "treasure token") ||
		strings.Contains(text, "treasure tokens")
}

func guessDealsDamage(card cards.Card) bool {
	text := guessOracleText(card)
	return strings.Contains(text, "damage") &&
		(strings.Contains(text, "deal ") || strings.Contains(text, "deals "))
}

func guessProtectsSomething(card cards.Card) bool {
	text := guessOracleText(card)
	return strings.Contains(text, "hexproof") ||
		strings.Contains(text, "shroud") ||
		strings.Contains(text, "phase out") ||
		strings.Contains(text, "phases out") ||
		strings.Contains(text, "protection from") ||
		strings.Contains(text, "indestructible") ||
		strings.Contains(text, "prevent") ||
		strings.Contains(text, "ward") ||
		strings.Contains(text, "can't be the target") ||
		strings.Contains(text, "cannot be the target")
}
