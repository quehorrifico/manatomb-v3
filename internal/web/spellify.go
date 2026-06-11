package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"manatomb/app/internal/cards"
)

const spellifyMaxGuesses = 13
const spellifyAwardGuessLimit = 7

type spellifyPageData struct {
	GameID            int64
	Status            string
	Completed         bool
	Won               bool
	GuessCount        int
	RemainingGuesses  int
	IsDaily           bool
	HasAccount        bool
	AwardGuessLimit   int
	AwardStatus       string
	GameModeLabel     string
	CanRevealChar     bool
	CanGuess          bool
	MaskedName        string
	MaskedRulesText   string
	MaskedFlavorText  string
	TargetName        string
	TargetImageURI    string
	TargetDetailPath  string
	TargetDescription string
}

type spellifyJSONResponse struct {
	OK      bool
	Message string
	Data    spellifyPageData
}

func (a *App) HandleSpellifyShow(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	player := a.gamePlayer(w, r)

	game, err := a.spellifyGameForShow(r, player)
	if err != nil {
		if errors.Is(err, cards.ErrCardNotFound) {
			setFlash(w, "No cards are ready for Tombscript yet.")
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

	a.Renderer.Render(w, "spellify", TemplateData{
		CurrentUser: user,
		Data:        buildSpellifyPageData(*game, *card),
		Flash:       readFlash(w, r),
	})
}

func (a *App) spellifyGameForShow(r *http.Request, player gamePlayer) (*spellifyGame, error) {
	rawGameID := strings.TrimSpace(r.URL.Query().Get("game_id"))
	if rawGameID == "" {
		return activeOrNewSpellifyGame(r.Context(), a.DB, player)
	}
	gameID, err := strconv.ParseInt(rawGameID, 10, 64)
	if err != nil || gameID <= 0 {
		return activeOrNewSpellifyGame(r.Context(), a.DB, player)
	}
	game, err := loadSpellifyGameByID(r.Context(), a.DB, player, gameID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return activeOrNewSpellifyGame(r.Context(), a.DB, player)
		}
		return nil, err
	}
	if spellifyActiveGameExpired(*game) {
		if err := abandonActiveSpellifyGames(r.Context(), a.DB, player); err != nil {
			return nil, err
		}
		return activeOrNewSpellifyGame(r.Context(), a.DB, player)
	}
	return game, nil
}

func (a *App) HandleSpellifyPost(w http.ResponseWriter, r *http.Request) {
	player := a.gamePlayer(w, r)
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		setFlash(w, "Invalid Tombscript request.")
		http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
		return
	}

	switch strings.ToLower(strings.TrimSpace(r.Form.Get("action"))) {
	case "new":
		a.handleSpellifyNewPost(w, r, player)
	case "char":
		a.handleSpellifyCharPost(w, r, player)
	case "guess":
		a.handleSpellifyFinalGuessPost(w, r, player)
	case "give_up":
		a.handleSpellifyGiveUpPost(w, r, player)
	default:
		setFlash(w, "Choose a Tombscript action.")
		http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
	}
}

func (a *App) handleSpellifyNewPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	if err := abandonActiveSpellifyGames(r.Context(), a.DB, player); err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	if _, err := createReplaySpellifyGame(r.Context(), a.DB, player); err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	setFlash(w, "Practice Tombscript loaded. Only the daily card is eligible for awards.")
	http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
}

func (a *App) handleSpellifyCharPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	wantsJSON := spellifyWantsJSON(r)
	game, err := loadActiveSpellifyGame(r.Context(), a.DB, player)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if wantsJSON {
				writeSpellifyJSONError(w, http.StatusNotFound, "Start a new Tombscript game first.")
				return
			}
			setFlash(w, "Start a new Tombscript game first.")
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	guessChar := spellifyNormalizeGuessChar(r.Form.Get("char"))
	if guessChar == "" {
		if wantsJSON {
			a.writeSpellifyStateJSON(w, r, *game, false, "Enter one letter, number, or mana symbol.", http.StatusBadRequest)
			return
		}
		setFlash(w, "Enter one letter, number, or mana symbol.")
		http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
		return
	}
	if err := addSpellifyGuessChar(r.Context(), a.DB, game.ID, player, guessChar); err != nil {
		if errors.Is(err, errSpellifyGuessUnavailable) {
			message := "That character has already been guessed."
			if game.GuessCount >= spellifyMaxGuesses {
				message = "No reveal guesses left. Submit a card name or give up."
			}
			if wantsJSON {
				a.writeSpellifyStateJSON(w, r, *game, false, message, http.StatusOK)
				return
			}
			setFlash(w, message)
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}
	if wantsJSON {
		updated, err := loadActiveSpellifyGame(r.Context(), a.DB, player)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		a.writeSpellifyStateJSON(w, r, *updated, true, strings.ToUpper(guessChar)+" revealed.", http.StatusOK)
		return
	}
	http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
}

func (a *App) handleSpellifyFinalGuessPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadActiveSpellifyGame(r.Context(), a.DB, player)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			setFlash(w, "Start a new Tombscript game first.")
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
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
		setFlash(w, "Enter a card name before submitting.")
		http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
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

	won := strings.EqualFold(guess, target.Name)
	if len(matches) > 0 && strings.EqualFold(strings.TrimSpace(matches[0].OracleID), strings.TrimSpace(target.OracleID)) {
		won = true
	}
	if !won {
		setFlash(w, "Not quite. Reveal another character or try a different card.")
		http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
		return
	}

	if spellifyGameAwardEligible(*game) && game.GuessCount < spellifyAwardGuessLimit {
		if err := completeSpellifyGameWithAward(r.Context(), a.DB, *game, *target); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		setFlash(w, "Correct. You won the Tombscript card.")
	} else {
		if err := completeSpellifyGame(r.Context(), a.DB, game.ID, player, true); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		if player.IsGuest() {
			setFlash(w, "Correct. Sign in before playing tomorrow's daily Tombscript to earn profile awards.")
		} else if game.IsDaily {
			setFlash(w, "Correct. Daily Tombscript cards are awarded in fewer than 7 reveal guesses.")
		} else {
			setFlash(w, "Correct. Practice Tombscript games do not award cards.")
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/games/spellify?game_id=%d", game.ID), http.StatusSeeOther)
}

func (a *App) handleSpellifyGiveUpPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadActiveSpellifyGame(r.Context(), a.DB, player)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			setFlash(w, "Start a new Tombscript game first.")
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
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
	if err := completeSpellifyGame(r.Context(), a.DB, game.ID, player, false); err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	setFlash(w, "The Tombscript answer was "+target.Name+".")
	http.Redirect(w, r, fmt.Sprintf("/games/spellify?game_id=%d", game.ID), http.StatusSeeOther)
}

func spellifyWantsJSON(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json") ||
		strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Requested-With")), "fetch")
}

func writeSpellifyJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(spellifyJSONResponse{
		OK:      false,
		Message: message,
	})
}

func (a *App) writeSpellifyStateJSON(w http.ResponseWriter, r *http.Request, game spellifyGame, ok bool, message string, status int) {
	card, err := cards.GetCardByOracleID(r.Context(), a.DB, game.TargetOracleID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(spellifyJSONResponse{
		OK:      ok,
		Message: message,
		Data:    buildSpellifyPageData(game, *card),
	})
}

func buildSpellifyPageData(game spellifyGame, card cards.Card) spellifyPageData {
	completed := game.Status != "" && game.Status != "active"
	remaining := spellifyMaxGuesses - game.GuessCount
	if remaining < 0 {
		remaining = 0
	}

	rulesText := spellifyTargetRulesText(card)
	if strings.TrimSpace(rulesText) == "" {
		rulesText = "No rules text."
	}
	flavorText := spellifyTargetFlavorText(card)
	if strings.TrimSpace(flavorText) == "" {
		flavorText = "No flavor text."
	}

	maskedName := spellifyMaskText(card.Name, game.GuessedChars)
	maskedRules := spellifyMaskText(rulesText, game.GuessedChars)
	maskedFlavor := spellifyMaskText(flavorText, game.GuessedChars)
	if completed {
		maskedName = card.Name
		maskedRules = rulesText
		maskedFlavor = flavorText
	}
	awardStatus := "Eligible"
	if game.GuestID != "" {
		awardStatus = "Sign in to earn"
	} else if !game.IsDaily {
		awardStatus = "Practice"
	} else if completed && game.Status == "won" && game.GuessCount < spellifyAwardGuessLimit {
		awardStatus = "Earned"
	} else if game.GuessCount >= spellifyAwardGuessLimit {
		awardStatus = "Practice"
	}
	gameMode := "Daily Tombscript"
	if !game.IsDaily {
		gameMode = "Practice Tombscript"
	}

	return spellifyPageData{
		GameID:            game.ID,
		Status:            guessGameStatusLabel(game.Status),
		Completed:         completed,
		Won:               game.Status == "won",
		GuessCount:        game.GuessCount,
		RemainingGuesses:  remaining,
		IsDaily:           game.IsDaily,
		HasAccount:        game.GuestID == "",
		AwardGuessLimit:   spellifyAwardGuessLimit,
		AwardStatus:       awardStatus,
		GameModeLabel:     gameMode,
		CanRevealChar:     !completed && remaining > 0,
		CanGuess:          !completed,
		MaskedName:        maskedName,
		MaskedRulesText:   maskedRules,
		MaskedFlavorText:  maskedFlavor,
		TargetName:        card.Name,
		TargetImageURI:    card.ImageURI,
		TargetDetailPath:  cardDetailPath(card.OracleID),
		TargetDescription: strings.TrimSpace(card.TypeLine),
	}
}

func spellifyActiveGameExpired(game spellifyGame) bool {
	return game.Status == "active" &&
		strings.TrimSpace(game.DailyKey) != "" &&
		strings.TrimSpace(game.DailyKey) != guessCardDailyKey(time.Now().UTC())
}

func spellifyGameAwardEligible(game spellifyGame) bool {
	return game.GuestID == "" &&
		game.IsDaily &&
		strings.TrimSpace(game.DailyKey) != "" &&
		strings.TrimSpace(game.DailyKey) == guessCardDailyKey(time.Now().UTC())
}

func spellifyNormalizeGuessChar(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "{")
	value = strings.TrimSuffix(value, "}")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	r, _ := utf8.DecodeRuneInString(value)
	if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
		return ""
	}
	return strings.ToLower(string(r))
}

func spellifyMaskText(value string, guessedChars []string) string {
	guessed := spellifyGuessedSet(guessedChars)
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			key := strings.ToLower(string(r))
			if guessed[key] {
				b.WriteRune(r)
			} else {
				b.WriteByte('_')
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func spellifyTargetRulesText(card cards.Card) string {
	return guessCardCombinedFaceValue(card.OracleText, card.Faces, func(face cards.CardFace) string {
		return face.OracleText
	})
}

func spellifyTargetFlavorText(card cards.Card) string {
	return guessCardCombinedFaceValue(card.FlavorText, card.Faces, func(face cards.CardFace) string {
		return face.FlavorText
	})
}

func spellifyGuessedSet(guessedChars []string) map[string]bool {
	out := map[string]bool{}
	for _, guessChar := range guessedChars {
		guessChar = spellifyNormalizeGuessChar(guessChar)
		if guessChar != "" {
			out[guessChar] = true
		}
	}
	return out
}
