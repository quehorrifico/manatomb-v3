package web

import (
	"context"
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
const spellifyMaxCardGuesses = 3

var errSpellifyRoundStale = errors.New("Tombscript round is no longer active")

type spellifyPageData struct {
	GameID               int64
	Status               string
	Completed            bool
	Won                  bool
	GuessCount           int
	MaxGuesses           int
	RemainingGuesses     int
	CardGuessCount       int
	MaxCardGuesses       int
	RemainingCardGuesses int
	PreviousWrongGuesses []string
	GuessedChars         []string
	SymbolKeys           []spellifySymbolKey
	IsDaily              bool
	HasAccount           bool
	AwardGuessLimit      int
	AwardGuessesLeft     int
	AwardStatus          string
	GameModeLabel        string
	CanRevealChar        bool
	CanGuess             bool
	HasManaCost          bool
	HasRulesText         bool
	HasFlavorText        bool
	LastGuessedChar      string
	LastRevealCount      int
	MaskedName           string
	MaskedManaCost       string
	MaskedRulesText      string
	MaskedFlavorText     string
	TargetName           string
	TargetImageURI       string
	TargetDetailPath     string
	TargetDescription    string
}

// spellifySymbolKey is a fixed keyboard option, not a target-derived list.
// Hit remains false until a player guesses the symbol so serialized page state
// cannot reveal which symbols occur on the mystery card.
type spellifySymbolKey struct {
	Value     string
	Label     string
	AssetName string
	Guessed   bool
	Hit       bool
}

var spellifySymbolKeyDefinitions = []spellifySymbolKey{
	{Value: "{W}", Label: "White mana", AssetName: "W"},
	{Value: "{U}", Label: "Blue mana", AssetName: "U"},
	{Value: "{B}", Label: "Black mana", AssetName: "B"},
	{Value: "{R}", Label: "Red mana", AssetName: "R"},
	{Value: "{G}", Label: "Green mana", AssetName: "G"},
	{Value: "{C}", Label: "Colorless mana", AssetName: "C"},
	{Value: "{0}", Label: "Zero mana", AssetName: "0"},
	{Value: "{1}", Label: "One generic mana", AssetName: "1"},
	{Value: "{2}", Label: "Two generic mana", AssetName: "2"},
	{Value: "{3}", Label: "Three generic mana", AssetName: "3"},
	{Value: "{4}", Label: "Four generic mana", AssetName: "4"},
	{Value: "{5}", Label: "Five generic mana", AssetName: "5"},
	{Value: "{6}", Label: "Six generic mana", AssetName: "6"},
	{Value: "{7}", Label: "Seven generic mana", AssetName: "7"},
	{Value: "{8}", Label: "Eight generic mana", AssetName: "8"},
	{Value: "{9}", Label: "Nine generic mana", AssetName: "9"},
	{Value: "{X}", Label: "X generic mana", AssetName: "X"},
	{Value: "{T}", Label: "Tap symbol", AssetName: "T"},
	{Value: "{Q}", Label: "Untap symbol", AssetName: "Q"},
	{Value: "{S}", Label: "Snow mana", AssetName: "S"},
	{Value: "{E}", Label: "Energy symbol", AssetName: "E"},
	{Value: "{W/P}", Label: "White Phyrexian mana", AssetName: "WP"},
	{Value: "{U/P}", Label: "Blue Phyrexian mana", AssetName: "UP"},
	{Value: "{B/P}", Label: "Black Phyrexian mana", AssetName: "BP"},
	{Value: "{R/P}", Label: "Red Phyrexian mana", AssetName: "RP"},
	{Value: "{G/P}", Label: "Green Phyrexian mana", AssetName: "GP"},
}

type spellifyJSONResponse struct {
	OK       bool
	Message  string
	Data     spellifyPageData
	Redirect string
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

	card, err := a.loadSpellifyTarget(r.Context(), *game)
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
	previousGameID, ok := parseSpellifyGameID(r.Form.Get("game_id"))
	if !ok {
		setFlash(w, "That Tombscript round could not be replayed.")
		http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
		return
	}
	previous, err := loadSpellifyGameByID(r.Context(), a.DB, player, previousGameID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			setFlash(w, "That Tombscript round could not be replayed.")
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}
	if previous.Status == "active" {
		setFlash(w, "Finish or reveal the current Tombscript card before starting a practice round.")
		http.Redirect(w, r, spellifyRoundPath(previous.ID), http.StatusSeeOther)
		return
	}
	if active, err := loadActiveSpellifyGame(r.Context(), a.DB, player); err == nil {
		setFlash(w, "A Tombscript round is already in progress.")
		http.Redirect(w, r, spellifyRoundPath(active.ID), http.StatusSeeOther)
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		a.RenderServerError(w, r, err)
		return
	}
	game, err := createReplaySpellifyGame(r.Context(), a.DB, player)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	http.Redirect(w, r, spellifyRoundPath(game.ID), http.StatusSeeOther)
}

func (a *App) handleSpellifyCharPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	wantsJSON := spellifyWantsJSON(r)
	game, err := loadSpellifyGameForMutation(r.Context(), a.DB, player, r.Form.Get("game_id"))
	if err != nil {
		if errors.Is(err, errSpellifyRoundStale) {
			message := "That Tombscript round is no longer active. No reveal guess was used."
			if wantsJSON {
				writeSpellifyJSONError(w, http.StatusConflict, message)
				return
			}
			setFlash(w, message)
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
			return
		}
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
			a.writeSpellifyStateJSON(w, r, *game, false, "Choose a letter, number, or symbol.", http.StatusBadRequest)
			return
		}
		setFlash(w, "Choose a letter, number, or symbol.")
		http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
		return
	}
	if err := addSpellifyGuessChar(r.Context(), a.DB, game.ID, player, guessChar); err != nil {
		if errors.Is(err, errSpellifyGuessUnavailable) {
			message := "That character has already been guessed."
			if game.GuessCount >= spellifyMaxGuesses {
				message = "No character reveals remain. Make a guess or reveal the card."
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
		a.writeSpellifyStateJSON(w, r, *updated, true, "", http.StatusOK)
		return
	}
	http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
}

func (a *App) handleSpellifyFinalGuessPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	wantsJSON := spellifyWantsJSON(r)
	game, err := loadSpellifyGameForMutation(r.Context(), a.DB, player, r.Form.Get("game_id"))
	if err != nil {
		if errors.Is(err, errSpellifyRoundStale) {
			message := "That Tombscript round is no longer active. No card guess was submitted."
			if wantsJSON {
				writeSpellifyJSONError(w, http.StatusConflict, message)
				return
			}
			setFlash(w, message)
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			message := "Start a new Tombscript game first."
			if wantsJSON {
				writeSpellifyJSONError(w, http.StatusNotFound, message)
				return
			}
			setFlash(w, message)
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	target, err := a.loadSpellifyTarget(r.Context(), *game)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	guess := strings.TrimSpace(r.Form.Get("guess"))
	if guess == "" {
		message := "Enter a card name before submitting."
		if wantsJSON {
			a.writeSpellifyStateJSON(w, r, *game, false, message, http.StatusBadRequest)
			return
		}
		setFlash(w, message)
		http.Redirect(w, r, spellifyRoundPath(game.ID), http.StatusSeeOther)
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
		attempt, err := recordSpellifyWrongCardGuess(
			r.Context(),
			a.DB,
			game.ID,
			player,
			guess,
			spellifyMaxCardGuesses,
		)
		if err != nil {
			if errors.Is(err, errSpellifyCardGuessUnavailable) {
				message := "No card guesses remain for this Tombscript round."
				if wantsJSON {
					a.writeSpellifyStateJSON(w, r, *game, false, message, http.StatusConflict, spellifyRoundPath(game.ID))
					return
				}
				setFlash(w, message)
				http.Redirect(w, r, spellifyRoundPath(game.ID), http.StatusSeeOther)
				return
			}
			a.RenderServerError(w, r, err)
			return
		}

		updated, err := loadSpellifyGameByID(r.Context(), a.DB, player, game.ID)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		remaining := spellifyMaxCardGuesses - attempt.CardGuessCount
		message := fmt.Sprintf("Not quite. %d card guesses remain.", remaining)
		if remaining == 1 {
			message = "Not quite. 1 card guess remains."
		}
		redirect := ""
		if attempt.Exhausted {
			message = "No card guesses remain. The answer was " + target.Name + "."
			redirect = spellifyRoundPath(game.ID)
		}
		if wantsJSON {
			a.writeSpellifyStateJSON(w, r, *updated, false, message, http.StatusOK, redirect)
			return
		}
		setFlash(w, message)
		http.Redirect(w, r, spellifyRoundPath(game.ID), http.StatusSeeOther)
		return
	}

	message := ""
	if spellifyGameAwardEligible(*game) && game.GuessCount < spellifyAwardGuessLimit {
		if err := completeSpellifyGameWithAward(r.Context(), a.DB, *game, *target); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		message = "Correct. You won the Tombscript card."
	} else {
		if err := completeSpellifyGame(r.Context(), a.DB, game.ID, player, true); err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		if player.IsGuest() {
			message = "Correct. Sign in before playing tomorrow's daily Tombscript to earn profile awards."
		} else if game.IsDaily {
			message = "Correct. Daily Tombscript cards are awarded in fewer than 7 reveal guesses."
		} else {
			message = "Correct. Practice Tombscript games do not award cards."
		}
	}
	redirect := spellifyRoundPath(game.ID)
	if wantsJSON {
		updated, err := loadSpellifyGameByID(r.Context(), a.DB, player, game.ID)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}
		a.writeSpellifyStateJSON(w, r, *updated, true, message, http.StatusOK, redirect)
		return
	}
	setFlash(w, message)
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (a *App) handleSpellifyGiveUpPost(w http.ResponseWriter, r *http.Request, player gamePlayer) {
	game, err := loadSpellifyGameForMutation(r.Context(), a.DB, player, r.Form.Get("game_id"))
	if err != nil {
		if errors.Is(err, errSpellifyRoundStale) {
			setFlash(w, "That Tombscript round is no longer active.")
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			setFlash(w, "Start a new Tombscript game first.")
			http.Redirect(w, r, "/games/spellify", http.StatusSeeOther)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}
	target, err := a.loadSpellifyTarget(r.Context(), *game)
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

func parseSpellifyGameID(rawGameID string) (int64, bool) {
	gameID, err := strconv.ParseInt(strings.TrimSpace(rawGameID), 10, 64)
	if err != nil || gameID <= 0 {
		return 0, false
	}
	return gameID, true
}

func spellifyRoundPath(gameID int64) string {
	return fmt.Sprintf("/games/spellify?game_id=%d", gameID)
}

// loadSpellifyGameForMutation binds an action to the round rendered in the
// submitting tab so an old or duplicated tab cannot mutate a newer round.
func loadSpellifyGameForMutation(
	ctx context.Context,
	db *sql.DB,
	player gamePlayer,
	rawGameID string,
) (*spellifyGame, error) {
	gameID, ok := parseSpellifyGameID(rawGameID)
	if !ok {
		return nil, errSpellifyRoundStale
	}
	game, err := loadSpellifyGameByID(ctx, db, player, gameID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errSpellifyRoundStale
	}
	if err != nil {
		return nil, err
	}
	if game.Status != "active" || spellifyActiveGameExpired(*game) {
		return nil, errSpellifyRoundStale
	}
	return game, nil
}

func writeSpellifyJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(spellifyJSONResponse{
		OK:      false,
		Message: message,
	})
}

func (a *App) writeSpellifyStateJSON(w http.ResponseWriter, r *http.Request, game spellifyGame, ok bool, message string, status int, redirects ...string) {
	card, err := a.loadSpellifyTarget(r.Context(), game)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	data := buildSpellifyPageData(game, *card)
	if ok && strings.TrimSpace(message) == "" {
		message = spellifyRevealMessage(data.LastGuessedChar, data.LastRevealCount)
	}
	redirect := ""
	if len(redirects) > 0 {
		redirect = strings.TrimSpace(redirects[0])
	}
	_ = json.NewEncoder(w).Encode(spellifyJSONResponse{
		OK:       ok,
		Message:  message,
		Data:     data,
		Redirect: redirect,
	})
}

func buildSpellifyPageData(game spellifyGame, card cards.Card) spellifyPageData {
	completed := game.Status != "" && game.Status != "active"
	remaining := spellifyMaxGuesses - game.GuessCount
	if remaining < 0 {
		remaining = 0
	}
	remainingCardGuesses := spellifyMaxCardGuesses - game.CardGuessCount
	if remainingCardGuesses < 0 {
		remainingCardGuesses = 0
	}

	manaCost := spellifyTargetManaCost(card)
	hasManaCost := strings.TrimSpace(manaCost) != ""
	rulesText := spellifyTargetRulesText(card)
	hasRulesText := strings.TrimSpace(rulesText) != ""
	if !hasRulesText {
		rulesText = "No rules text."
	}
	flavorText := spellifyTargetFlavorText(card)
	hasFlavorText := strings.TrimSpace(flavorText) != ""
	if !hasFlavorText {
		flavorText = "No flavor text."
	}

	maskedName := spellifyMaskText(card.Name, game.GuessedChars)
	maskedManaCost := spellifyMaskText(manaCost, game.GuessedChars)
	maskedRules := spellifyMaskText(rulesText, game.GuessedChars)
	maskedFlavor := spellifyMaskText(flavorText, game.GuessedChars)
	if completed {
		maskedName = card.Name
		maskedManaCost = manaCost
		maskedRules = rulesText
		maskedFlavor = flavorText
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
		case game.Status == "won" && game.GuessCount < spellifyAwardGuessLimit:
			awardStatus = "Earned"
		case game.Status == "won":
			awardStatus = "Solved"
		default:
			awardStatus = "Not earned"
		}
	} else if game.GuessCount >= spellifyAwardGuessLimit {
		awardStatus = "Award closed"
	} else if game.GuessCount == spellifyAwardGuessLimit-1 {
		awardStatus = "Solve now"
	}
	gameMode := "Daily Tombscript"
	if !game.IsDaily {
		gameMode = "Practice Tombscript"
	}
	awardGuessesLeft := spellifyAwardGuessLimit - 1 - game.GuessCount
	if awardGuessesLeft < 0 {
		awardGuessesLeft = 0
	}

	// Target identity is result-only data. Active page data is serialized for
	// live character reveals, so populating these fields before completion
	// would expose the answer in the HTML/JSON even when the card is hidden.
	var targetName, targetImageURI, targetDetailPath, targetDescription string
	if completed {
		targetName = card.Name
		targetImageURI = card.ImageURI
		targetDetailPath = cardPrintingDetailPath(card.OracleID, guessCardDetailPrintingID(card, game.TargetScryfallID))
		targetDescription = strings.TrimSpace(card.TypeLine)
	}
	guessedChars := spellifyNormalizedGuessedChars(game.GuessedChars)
	lastGuessedChar := ""
	lastRevealCount := 0
	if len(guessedChars) > 0 {
		lastGuessedChar = guessedChars[len(guessedChars)-1]
		lastRevealCount = spellifyCharacterRevealCount(card, lastGuessedChar)
	}

	return spellifyPageData{
		GameID:               game.ID,
		Status:               guessGameStatusLabel(game.Status),
		Completed:            completed,
		Won:                  game.Status == "won",
		GuessCount:           game.GuessCount,
		MaxGuesses:           spellifyMaxGuesses,
		RemainingGuesses:     remaining,
		CardGuessCount:       game.CardGuessCount,
		MaxCardGuesses:       spellifyMaxCardGuesses,
		RemainingCardGuesses: remainingCardGuesses,
		PreviousWrongGuesses: append([]string(nil), game.PreviousWrongGuesses...),
		GuessedChars:         guessedChars,
		SymbolKeys:           spellifySymbolKeys(card, guessedChars),
		IsDaily:              game.IsDaily,
		HasAccount:           game.GuestID == "",
		AwardGuessLimit:      spellifyAwardGuessLimit,
		AwardGuessesLeft:     awardGuessesLeft,
		AwardStatus:          awardStatus,
		GameModeLabel:        gameMode,
		CanRevealChar:        !completed && remaining > 0,
		CanGuess:             !completed && remainingCardGuesses > 0,
		HasManaCost:          hasManaCost,
		HasRulesText:         hasRulesText,
		HasFlavorText:        hasFlavorText,
		LastGuessedChar:      lastGuessedChar,
		LastRevealCount:      lastRevealCount,
		MaskedName:           maskedName,
		MaskedManaCost:       maskedManaCost,
		MaskedRulesText:      maskedRules,
		MaskedFlavorText:     maskedFlavor,
		TargetName:           targetName,
		TargetImageURI:       targetImageURI,
		TargetDetailPath:     targetDetailPath,
		TargetDescription:    targetDescription,
	}
}

// loadSpellifyTarget keeps a round pinned to the printing selected when it
// was created. Rows from before printing persistence have an empty printing
// ID and continue to use the oracle card's current default printing.
func (a *App) loadSpellifyTarget(ctx context.Context, game spellifyGame) (*cards.Card, error) {
	if printingID := strings.TrimSpace(game.TargetScryfallID); printingID != "" {
		card, err := cards.GetCardPrintingByID(ctx, a.DB, game.TargetOracleID, printingID)
		if err == nil {
			return card, nil
		}
		if !errors.Is(err, cards.ErrCardNotFound) {
			return nil, err
		}
	}
	return cards.GetCardByOracleID(ctx, a.DB, game.TargetOracleID)
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
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "{") || strings.HasSuffix(value, "}") {
		return spellifyCanonicalSymbol(value)
	}
	r, size := utf8.DecodeRuneInString(value)
	if size != len(value) {
		return ""
	}
	if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
		return ""
	}
	return strings.ToLower(string(r))
}

func spellifyCanonicalSymbol(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 3 || value[0] != '{' || value[len(value)-1] != '}' {
		return ""
	}
	inner := strings.ToUpper(strings.TrimSpace(value[1 : len(value)-1]))
	if inner == "" || strings.ContainsAny(inner, "{}") {
		return ""
	}
	canonical := "{" + inner + "}"
	for _, definition := range spellifySymbolKeyDefinitions {
		if canonical == definition.Value {
			return canonical
		}
	}
	return ""
}

func spellifyMaskText(value string, guessedChars []string) string {
	guessed := spellifyGuessedSet(guessedChars)
	var b strings.Builder
	for offset := 0; offset < len(value); {
		if value[offset] == '{' {
			if end := strings.IndexByte(value[offset+1:], '}'); end >= 0 {
				end += offset + 1
				symbol := spellifyCanonicalSymbol(value[offset : end+1])
				if symbol != "" && guessed[symbol] {
					b.WriteString(symbol)
				} else {
					// Every unrevealed brace token uses the same placeholder so a
					// hybrid or Phyrexian symbol cannot leak its value or length.
					b.WriteString("{_}")
				}
				offset = end + 1
				continue
			}
		}

		r, size := utf8.DecodeRuneInString(value[offset:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			key := strings.ToLower(string(r))
			if guessed[key] {
				b.WriteRune(r)
			} else {
				b.WriteByte('_')
			}
		} else {
			b.WriteRune(r)
		}
		offset += size
	}
	return b.String()
}

func spellifyTargetManaCost(card cards.Card) string {
	return guessCardCombinedFaceValue(card.ManaCost, card.Faces, func(face cards.CardFace) string {
		return face.ManaCost
	})
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

func spellifyNormalizedGuessedChars(guessedChars []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(guessedChars))
	for _, guessedChar := range guessedChars {
		guessedChar = spellifyNormalizeGuessChar(guessedChar)
		if guessedChar == "" || seen[guessedChar] {
			continue
		}
		seen[guessedChar] = true
		out = append(out, guessedChar)
	}
	return out
}

func spellifySymbolKeys(card cards.Card, guessedChars []string) []spellifySymbolKey {
	guessed := spellifyGuessedSet(guessedChars)
	keys := make([]spellifySymbolKey, 0, len(spellifySymbolKeyDefinitions))
	for _, definition := range spellifySymbolKeyDefinitions {
		key := definition
		key.Guessed = guessed[key.Value]
		// Never expose symbol presence before it has been selected.
		key.Hit = key.Guessed && spellifyCharacterRevealCount(card, key.Value) > 0
		keys = append(keys, key)
	}
	return keys
}

func spellifyCharacterRevealCount(card cards.Card, guessedChar string) int {
	guessedChar = spellifyNormalizeGuessChar(guessedChar)
	if guessedChar == "" {
		return 0
	}
	isSymbol := strings.HasPrefix(guessedChar, "{")
	target, _ := utf8.DecodeRuneInString(guessedChar)
	count := 0
	for _, value := range []string{
		card.Name,
		spellifyTargetManaCost(card),
		spellifyTargetRulesText(card),
		spellifyTargetFlavorText(card),
	} {
		for offset := 0; offset < len(value); {
			if value[offset] == '{' {
				if end := strings.IndexByte(value[offset+1:], '}'); end >= 0 {
					end += offset + 1
					if isSymbol && spellifyCanonicalSymbol(value[offset:end+1]) == guessedChar {
						count++
					}
					offset = end + 1
					continue
				}
			}

			character, size := utf8.DecodeRuneInString(value[offset:])
			if !isSymbol && unicode.ToLower(character) == unicode.ToLower(target) {
				count++
			}
			offset += size
		}
	}
	return count
}

func spellifyRevealMessage(guessedChar string, revealCount int) string {
	guessedChar = strings.ToUpper(spellifyNormalizeGuessChar(guessedChar))
	if guessedChar == "" {
		return "Guess recorded."
	}
	if revealCount == 0 {
		return guessedChar + " is not in the card."
	}
	if revealCount == 1 {
		return guessedChar + " revealed 1 character."
	}
	return fmt.Sprintf("%s revealed %d characters.", guessedChar, revealCount)
}
