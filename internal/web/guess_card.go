package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"manatomb/app/internal/cards"
)

const guessCardAwardGuessLimit = 10

type guessQuestion struct {
	ID   string
	Text string
}

type guessAnswerView struct {
	Question string
	Answer   string
	Yes      bool
}

type guessClueView struct {
	Label     string
	Value     string
	ValueHTML template.HTML
}

type guessCardPageData struct {
	GameID             int64
	Status             string
	Completed          bool
	Won                bool
	QuestionCount      int
	GuessCount         int
	IsDaily            bool
	HasAccount         bool
	AwardGuessLimit    int
	AwardGuessesLeft   int
	AwardStatus        string
	GameModeLabel      string
	History            []guessAnswerView
	Clues              []guessClueView
	AvailableQuestions []guessQuestion
	CanAsk             bool
	CanGuess           bool
	TargetName         string
	TargetImageURI     string
	TargetDetailPath   string
}

var guessCardQuestions = []guessQuestion{
	{ID: "color_w", Text: "Is it white?"},
	{ID: "color_u", Text: "Is it blue?"},
	{ID: "color_b", Text: "Is it black?"},
	{ID: "color_r", Text: "Is it red?"},
	{ID: "color_g", Text: "Is it green?"},
	{ID: "colorless", Text: "Is it colorless?"},
	{ID: "monocolored", Text: "Is it monocolored?"},
	{ID: "multicolor", Text: "Is it multicolored?"},
	{ID: "permanent", Text: "Is it a permanent?"},
	{ID: "nonpermanent", Text: "Is it a nonpermanent?"},
	{ID: "creature", Text: "Is it a creature?"},
	{ID: "instant", Text: "Is it an instant?"},
	{ID: "sorcery", Text: "Is it a sorcery?"},
	{ID: "artifact", Text: "Is it an artifact?"},
	{ID: "enchantment", Text: "Is it an enchantment?"},
	{ID: "planeswalker", Text: "Is it a planeswalker?"},
	{ID: "land", Text: "Is it a land?"},
	{ID: "legendary", Text: "Is it legendary?"},
	{ID: "mv_le_2", Text: "Is its mana value 2 or less?"},
	{ID: "mv_le_3", Text: "Is its mana value 3 or less?"},
	{ID: "mv_ge_5", Text: "Is its mana value 5 or greater?"},
	{ID: "commander_legal", Text: "Is it legal in Commander?"},
	{ID: "draws_cards", Text: "Does it mention drawing cards?"},
	{ID: "makes_tokens", Text: "Does it make tokens?"},
	{ID: "destroys", Text: "Does it destroy something?"},
	{ID: "searches_library", Text: "Does it search a library?"},
	{ID: "graveyard", Text: "Does it mention graveyards?"},
	{ID: "flying", Text: "Does it have or mention flying?"},
	{ID: "generates_mana", Text: "Does it generate mana?"},
	{ID: "deals_damage", Text: "Does it deal damage?"},
	{ID: "protects", Text: "Is it used to protect something?"},
}

func questionByID(questionID string) *guessQuestion {
	questionID = strings.TrimSpace(questionID)
	for i := range guessCardQuestions {
		if guessCardQuestions[i].ID == questionID {
			return &guessCardQuestions[i]
		}
	}
	return nil
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

	card, err := cards.GetCardByOracleID(r.Context(), a.DB, game.TargetOracleID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	a.Renderer.Render(w, "guess_card", TemplateData{
		CurrentUser: user,
		Data:        buildGuessCardPageData(*game, *card),
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
		if err := abandonActiveGuessCardGames(r.Context(), a.DB, player); err != nil {
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
		if err := abandonActiveGuessCardGames(r.Context(), a.DB, player); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		if _, err := createReplayGuessCardGame(r.Context(), a.DB, player); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		setFlash(w, "Practice game started. Only the daily card is eligible for a profile card award.")
		http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
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

func (a *App) handleGuessCardQuestionPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadActiveGuessCardGame(r.Context(), a.DB, player)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			setFlash(w, "Start a new game first.")
			http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
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
	if err := addGuessCardQuestion(r.Context(), a.DB, game.ID, player, questionID); err != nil {
		if errors.Is(err, errGuessCardQuestionUnavailable) {
			setFlash(w, "That question has already been asked.")
			http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
			return
		}
		setFlash(w, "Try rephrasing your question.")
	}
	http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
}

func (a *App) handleGuessCardFinalGuessPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadActiveGuessCardGame(r.Context(), a.DB, player)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			setFlash(w, "Start a new game first.")
			http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	target, err := cards.GetCardByOracleID(r.Context(), a.DB, game.TargetOracleID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	guess := strings.TrimSpace(r.Form.Get("guess"))
	if guess == "" {
		setFlash(w, "Enter a card name before making your final guess.")
		http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
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

	won := len(matches) > 0 && strings.EqualFold(strings.TrimSpace(matches[0].OracleID), strings.TrimSpace(target.OracleID))
	if err := incrementGuessCardGuessCount(r.Context(), a.DB, game.ID, player); err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	guessNumber := game.GuessCount + 1
	awardEligible := guessCardGameAwardEligible(*game)

	if won && awardEligible && guessNumber < guessCardAwardGuessLimit {
		if err := completeGuessCardGameWithAward(r.Context(), a.DB, *game, *target, guessNumber); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		setFlash(w, "Correct. You kept the card.")
	} else if won {
		if err := completeGuessCardGame(r.Context(), a.DB, game.ID, player, true); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		if player.IsGuest() {
			setFlash(w, "Correct. Sign in before playing tomorrow's daily card to earn profile awards.")
		} else if awardEligible {
			setFlash(w, "Correct. Cards are awarded for daily solves in fewer than 10 guesses.")
		} else if game.IsDaily {
			setFlash(w, "Correct. That daily card has expired; only today's daily card awards a profile card.")
		} else {
			setFlash(w, "Correct. Practice games do not award profile cards.")
		}
	} else {
		setFlash(w, "Not quite. Ask another question for a new clue.")
		http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/games/guess-card?game_id=%d", game.ID), http.StatusSeeOther)
}

func (a *App) handleGuessCardGiveUpPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadActiveGuessCardGame(r.Context(), a.DB, player)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			setFlash(w, "Start a new game first.")
			http.Redirect(w, r, "/games/guess-card", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	target, err := cards.GetCardByOracleID(r.Context(), a.DB, game.TargetOracleID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	if err := completeGuessCardGame(r.Context(), a.DB, game.ID, player, false); err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	setFlash(w, "The answer was "+target.Name+".")
	http.Redirect(w, r, fmt.Sprintf("/games/guess-card?game_id=%d", game.ID), http.StatusSeeOther)
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
		"land",
		"legendary",
		"mv_le_2",
		"mv_le_3",
		"mv_ge_5",
		"commander_legal",
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
	history := make([]guessAnswerView, 0, len(game.AskedQuestions))
	for _, questionID := range game.AskedQuestions {
		question := questionByID(questionID)
		if question == nil {
			continue
		}
		answer := guessQuestionAnswer(card, question.ID)
		label := "No"
		if answer {
			label = "Yes"
		}
		history = append(history, guessAnswerView{
			Question: question.Text,
			Answer:   label,
			Yes:      answer,
		})
	}

	completed := game.Status != "active"
	awardLeft := guessCardAwardGuessLimit - 1 - game.GuessCount
	if awardLeft < 0 {
		awardLeft = 0
	}
	awardStatus := "Eligible"
	if game.GuestID != "" {
		awardStatus = "Sign in to earn"
	} else if !game.IsDaily {
		awardStatus = "Practice"
	} else if completed && game.Status == "won" && game.GuessCount < guessCardAwardGuessLimit {
		awardStatus = "Earned"
	} else if game.GuessCount >= guessCardAwardGuessLimit-1 {
		awardStatus = "Practice"
	}
	gameMode := "Daily Game"
	if !game.IsDaily {
		gameMode = "Practice Game"
	}

	clues := buildGuessCardClues(game, card)

	return guessCardPageData{
		GameID:             game.ID,
		Status:             guessGameStatusLabel(game.Status),
		Completed:          completed,
		Won:                game.Status == "won",
		QuestionCount:      game.QuestionCount,
		GuessCount:         game.GuessCount,
		IsDaily:            game.IsDaily,
		HasAccount:         game.GuestID == "",
		AwardGuessLimit:    guessCardAwardGuessLimit,
		AwardGuessesLeft:   awardLeft,
		AwardStatus:        awardStatus,
		GameModeLabel:      gameMode,
		History:            history,
		Clues:              clues,
		AvailableQuestions: availableGuessCardQuestions(game.AskedQuestions, clues),
		CanAsk:             !completed,
		CanGuess:           !completed,
		TargetName:         card.Name,
		TargetImageURI:     card.ImageURI,
		TargetDetailPath:   cardDetailPath(card.OracleID),
	}
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
				"legendary",
			)
		case "rules text":
			add(
				"draws_cards",
				"makes_tokens",
				"destroys",
				"searches_library",
				"graveyard",
				"flying",
				"generates_mana",
				"deals_damage",
				"protects",
			)
		case "cast cost", "card cost":
			add("mv_le_2", "mv_le_3", "mv_ge_5")
		case "mana value":
			add("mv_le_2", "mv_le_3", "mv_ge_5")
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
	pool := guessCardCluePool(card)
	if len(pool) == 0 || len(game.AskedQuestions) == 0 {
		return nil
	}

	clueCount := len(game.AskedQuestions)
	if clueCount > len(pool) {
		clueCount = len(pool)
	}
	used := map[int]bool{}
	out := make([]guessClueView, 0, clueCount)
	for i := 0; i < clueCount; i++ {
		index := guessCardClueIndex(game.ID, i, len(pool), used)
		used[index] = true
		out = append(out, pool[index])
	}
	return out
}

func guessCardClueIndex(gameID int64, clueIndex int, poolSize int, used map[int]bool) int {
	if poolSize <= 0 {
		return 0
	}
	start := int((gameID*31 + int64(clueIndex*17+7)) % int64(poolSize))
	if start < 0 {
		start = -start
	}
	for offset := 0; offset < poolSize; offset++ {
		index := (start + offset) % poolSize
		if !used[index] {
			return index
		}
	}
	return start
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
	case "creature", "instant", "sorcery", "artifact", "enchantment", "planeswalker", "land":
		return strings.Contains(guessTypeLine(card), questionID)
	case "legendary":
		return strings.Contains(guessTypeLine(card), "legendary")
	case "mv_le_2":
		return card.CMC <= 2
	case "mv_le_3":
		return card.CMC <= 3
	case "mv_ge_5":
		return card.CMC >= 5
	case "commander_legal":
		return card.CommanderLegal
	case "draws_cards":
		return strings.Contains(guessOracleText(card), "draw")
	case "makes_tokens":
		return strings.Contains(guessOracleText(card), "token")
	case "destroys":
		return strings.Contains(guessOracleText(card), "destroy")
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
