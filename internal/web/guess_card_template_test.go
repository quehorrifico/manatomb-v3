package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func guessCardTemplateFixture() guessCardPageData {
	return guessCardPageData{
		GameID:             91,
		Status:             "In Progress",
		QuestionCount:      2,
		GuessCount:         2,
		PossibleCardsLeft:  37,
		PossibleCardsTotal: 250,
		NextGuessNumber:    3,
		IsDaily:            true,
		HasAccount:         true,
		AwardStatus:        "Eligible",
		GameModeLabel:      "First game today",
		Clues: []guessClueView{
			{Label: "Rarity", Value: "Rare"},
			{Label: "Mana value", Value: "3"},
		},
		QuestionGroups: groupGuessCardQuestions(guessCardQuestions),
		History: []guessAnswerView{
			{Number: 1, Kind: "question", Question: "Is it blue?", Answer: "No"},
			{Number: 2, Kind: "question", Question: "Is it a creature?", Answer: "Yes", Yes: true},
			{Number: 3, Kind: "guess", Question: "Storm Crow?", Answer: "No"},
			{Number: 4, Kind: "guess", Question: "Polluted Delta?", Answer: "No"},
		},
		HasLatestReveal: true,
		LatestReveal: guessRevealView{
			Number:   2,
			Label:    "Creature",
			Question: "Is it a creature?",
			Answer:   "Yes",
			Yes:      true,
			HasClue:  true,
			Clue: guessClueView{
				Label: "Mana value",
				Value: "3",
			},
		},
		PreviousReveals: []guessRevealView{{
			Number:   1,
			Label:    "Blue",
			Question: "Is it blue?",
			Answer:   "No",
			HasClue:  true,
			Clue: guessClueView{
				Label: "Rarity",
				Value: "Rare",
			},
		}},
		QuestionsLeft: 6,
		MaxQuestions:  8,
		CanAsk:        true,
		CanGuess:      true,
	}
}

func TestGuessCardTemplateUsesTwoRowOneTapGameBoard(t *testing.T) {
	page := guessCardTemplateFixture()
	page.TargetName = "Secret Test Card"
	page.TargetImageURI = "https://example.test/secret-card.jpg"
	body := renderTemplate(t, "guess_card", TemplateData{
		Data:  page,
		Flash: "One game notice.",
	})

	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("guest guess-card game is missing the Tomb palette: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="guess-card"`) {
		t.Fatalf("guess-card body is missing its page scope: %s", bodyTag)
	}
	if strings.Count(body, "<main") != 1 {
		t.Fatalf("guess-card page rendered %d main landmarks, want the shared one only", strings.Count(body, "<main"))
	}
	if strings.Count(body, "One game notice.") != 1 {
		t.Fatalf("guess-card flash rendered %d times, want one shared message", strings.Count(body, "One game notice."))
	}

	for _, needle := range []string{
		`href="/assets/guess_card.css"`,
		`class="mt-guess-game"`,
		`data-guess-card-help-open`,
		`Possible Cards Left 37`,
		`id="play-area"`,
		`class="mt-guess-game__stage"`,
		`class="mt-guess-game__stage-tools"`,
		`class="mt-guess-game__lower"`,
		`id="guess-card-clues-title">Revealed clues</h2>`,
		`<dt>Mana value</dt>`,
		`id="guess-card-history-title">Asked so far</h2>`,
		`<span class="mt-guess-game__turn">Q1</span>`,
		`<p>Is it blue?</p>`,
		`mt-guess-game__answer--no">No</span>`,
		`data-guess-latest-reveal`,
		`id="latest-reveal"`,
		`<span class="mt-guess-game__turn">Q2</span>`,
		`<p>Is it a creature?</p>`,
		`mt-guess-game__answer--yes">Yes</span>`,
		`<span class="mt-guess-game__turn">Q4</span>`,
		`<p>Polluted Delta?</p>`,
		`data-guess-question-board`,
		`id="question-board"`,
		`class="mt-guess-game__question-scroll"`,
		`data-guess-question-carousel`,
		`data-guess-question-previous`,
		`aria-label="Previous question categories"`,
		`data-guess-question-next`,
		`aria-label="Next question categories"`,
		`data-guess-question-viewport`,
		`data-guess-question-page`,
		`data-page-label="Color &amp; Rules Text"`,
		`data-page-label="Card Type &amp; Mana Value"`,
		`<fieldset class="mt-guess-game__question-group mt-guess-game__question-group--color">`,
		`<legend>Color</legend>`,
		`<legend>Card type</legend>`,
		`<legend>Mana value</legend>`,
		`<legend>Rules text</legend>`,
		`name="question_id"`,
		`value="color_w"`,
		`aria-label="Does its color identity include white?"`,
		`card-symbols/W.svg`,
		`class="mt-guess-game__question-symbol"`,
		`value="monocolored"`,
		`>Monocolored</span>`,
		`value="battle"`,
		`>Battle</span>`,
		`value="mv_ge_5"`,
		`value="mv_le_5"`,
		`value="mv_eq_0"`,
		`value="mv_eq_5"`,
		`value="exiles"`,
		`>Exile</span>`,
		`6 left`,
		`name="game_id" value="91"`,
		`for="guess-card-final" class="sr-only">Exact card name</label>`,
		`role="combobox"`,
		`data-card-autocomplete-submit="false"`,
		`name="guess_number" value="3"`,
		`data-guess-card-guess-form`,
		`data-guess-card-reveal-form`,
		`src="/assets/guess_card.js"`,
		`id="guess-card-final-results"`,
		`id="exact-guess"`,
		`Reveal card`,
		`id="guess-card-help-modal"`,
		`role="dialog"`,
		`aria-modal="true"`,
		`aria-hidden="true"`,
		`aria-labelledby="guess-card-help-title"`,
		`aria-describedby="guess-card-help-description"`,
		`data-auto-open="true"`,
		`data-guess-card-help-modal
    hidden>`,
		`aria-label="Close How to Play"`,
		`id="guess-card-help-title">How to Play</h2>`,
		`<h3>Card pool</h3>`,
		`Nonland mystery cards come from Scryfall’s EDHREC ranks 1–250 and have a supported paper printing, card art, and legality in at least one tracked format. Daily uses one shared card; practice chooses randomly from the same pool.`,
		`This is your first game today and the only prize-eligible Guess the Card round. Solve it to win the card; every later round today is just for fun.`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("guess-card page missing %q", needle)
		}
	}
	if got := strings.Count(body, `name="game_id" value="91"`); got != 3 {
		t.Fatalf("active game rendered %d scoped game IDs, want question, guess, and reveal forms", got)
	}
	if strings.Contains(body, `card-symbols/P.svg`) || strings.Contains(body, `card-symbols/2+.svg`) {
		t.Fatal("non-mana question markers must not be requested as Scryfall mana assets")
	}
	if strings.Contains(body, "Secret Test Card") || strings.Contains(body, "secret-card.jpg") {
		t.Fatal("active game HTML leaked the hidden card identity")
	}
	firstQuestionPage := strings.Index(body, `aria-label="Color and Rules Text questions"`)
	secondQuestionPage := strings.Index(body, `aria-label="Card Type and Mana Value questions"`)
	colorGroup := strings.Index(body, `<legend>Color</legend>`)
	rulesGroup := strings.Index(body, `<legend>Rules text</legend>`)
	typeGroup := strings.Index(body, `<legend>Card type</legend>`)
	manaGroup := strings.Index(body, `<legend>Mana value</legend>`)
	if firstQuestionPage < 0 || colorGroup < firstQuestionPage || rulesGroup < colorGroup || secondQuestionPage < rulesGroup {
		t.Fatal("first question page must contain Color followed by Rules Text")
	}
	if typeGroup < secondQuestionPage || manaGroup < typeGroup {
		t.Fatal("second question page must contain Card Type followed by Mana Value")
	}
	if strings.Index(body, `id="question-board"`) < strings.Index(body, `id="exact-guess"`) {
		t.Fatal("question board must remain in the lower row after the stage guess controls")
	}
	stageIndex := strings.Index(body, `class="mt-guess-game__stage"`)
	lowerIndex := strings.Index(body, `class="mt-guess-game__lower"`)
	historyIndex := strings.Index(body, `class="mt-guess-game__history"`)
	questionsIndex := strings.Index(body, `id="question-board"`)
	if stageIndex < 0 || lowerIndex < 0 || stageIndex > lowerIndex {
		t.Fatal("active game must render the mystery-card stage as the first board row and the history/questions split as the second")
	}
	if historyIndex < lowerIndex || questionsIndex < historyIndex {
		t.Fatal("the lower row must render concise history beside the always-visible question board")
	}
	for _, verbose := range []string{
		`aria-label="Game status"`,
		`<dt>Questions</dt>`,
		`<dt>Card guesses</dt>`,
		`<dt>Award</dt>`,
		`class="mt-guess-game__feedback"`,
		"Guess the Card, Win the Card",
		"Ask any yes-or-no question",
		"Your investigation",
		"Evidence trail",
		`<legend>Format</legend>`,
		`value="commander_legal"`,
		`value="legendary"`,
		`data-guess-question-menu`,
	} {
		if strings.Contains(body, verbose) {
			t.Fatalf("active guess-card page retained verbose copy %q", verbose)
		}
	}
}

func TestGuessCardCompletedTemplateShowsOneClearResultFlow(t *testing.T) {
	page := guessCardTemplateFixture()
	page.Completed = true
	page.Won = true
	page.Status = "Won"
	page.CanAsk = false
	page.CanGuess = false
	page.TargetName = "Sample Card"
	page.TargetImageURI = "https://example.test/sample-card.jpg"
	page.TargetDetailPath = "/cards/view/sample-card"

	body := renderTemplate(t, "guess_card", TemplateData{Data: page})
	for _, needle := range []string{
		`class="mt-guess-game__result mt-guess-game__result--won"`,
		`id="guess-card-result-title">Sample Card</h2>`,
		`aria-label="View Sample Card card details"`,
		`name="action" value="new"`,
		`Play Again`,
		`>Sign In</a>`,
		`data-guess-card-help-open`,
		`data-auto-open="true"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("completed guess-card page missing %q", needle)
		}
	}
	for _, forbidden := range []string{
		`data-guess-question-board`,
		`id="guess-card-final"`,
		`name="action" value="give_up"`,
		`<dt>Possible cards</dt>`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("completed guess-card page retained active control %q", forbidden)
		}
	}
}

func TestGuessCardHelpModalPersistsDismissalAndManagesFocus(t *testing.T) {
	jsBody, err := os.ReadFile(filepath.Join("assets", "guess_card.js"))
	if err != nil {
		t.Fatalf("read guess-card script: %v", err)
	}
	js := string(jsBody)
	for _, needle := range []string{
		`var helpStorageKey = "manatomb.guessCard.howToPlaySeen.v1";`,
		`window.localStorage.getItem(helpStorageKey) !== "1"`,
		`window.localStorage.setItem(helpStorageKey, "1")`,
		`helpModal.getAttribute("data-auto-open") === "true"`,
		`openHelpModal(null)`,
		`helpModal.setAttribute("aria-hidden", "false")`,
		`helpModal.setAttribute("aria-hidden", "true")`,
		`document.body.style.overflow = "hidden"`,
		`element.setAttribute("inert", "")`,
		`element.setAttribute("aria-hidden", "true")`,
		`state.element.removeAttribute("inert")`,
		`helpClose.focus({ preventScroll: true })`,
		`event.key === "Escape"`,
		`event.key !== "Tab"`,
		`document.activeElement === first`,
		`document.activeElement === last`,
		`focusTarget.focus({ preventScroll: true })`,
	} {
		if !strings.Contains(js, needle) {
			t.Fatalf("guess-card How to Play behavior missing %q", needle)
		}
	}
	if strings.Contains(js, "sessionStorage") {
		t.Fatal("How to Play dismissal must persist across sessions, not reset with session storage")
	}
}

func TestGuessCardQuestionCarouselSwitchesWithoutNavigation(t *testing.T) {
	jsBody, err := os.ReadFile(filepath.Join("assets", "guess_card.js"))
	if err != nil {
		t.Fatalf("read guess-card script: %v", err)
	}
	js := string(jsBody)
	for _, needle := range []string{
		`document.querySelector("[data-guess-question-carousel]")`,
		`questionCarousel.querySelector("[data-guess-question-previous]")`,
		`questionCarousel.querySelector("[data-guess-question-next]")`,
		`questionViewport.addEventListener("scroll"`,
		`previousQuestions.addEventListener("click"`,
		`nextQuestions.addEventListener("click"`,
		`questionViewport.scrollLeft = questionViewport.clientWidth * questionPage`,
		`questionPages[pageIndex].setAttribute("aria-hidden", active ? "false" : "true")`,
		`questionPages[pageIndex].setAttribute("inert", "")`,
		`updateQuestionPage(storedQuestionPage(), true)`,
	} {
		if !strings.Contains(js, needle) {
			t.Fatalf("guess-card question carousel behavior missing %q", needle)
		}
	}
	if strings.Contains(js, "window.location.reload") || strings.Contains(js, "window.location.href") {
		t.Fatal("question category controls must switch locally without navigating or reloading")
	}

	cssBody, err := os.ReadFile(filepath.Join("assets", "guess_card.css"))
	if err != nil {
		t.Fatalf("read guess-card stylesheet: %v", err)
	}
	css := string(cssBody)
	for _, needle := range []string{
		`.mt-guess-game__question-carousel-controls`,
		`grid-template-columns: 2.75rem minmax(0, 1fr) 2.75rem;`,
		`width: 2.75rem;`,
		`height: 2.75rem;`,
		`.mt-guess-game__question-carousel-viewport`,
		`overflow-x: auto;`,
		`scroll-snap-type: x mandatory;`,
		`touch-action: pan-x pan-y;`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("guess-card question carousel styling missing %q", needle)
		}
	}
}

func TestGuessCardActionsPreserveViewportWithoutHashNavigation(t *testing.T) {
	jsBody, err := os.ReadFile(filepath.Join("assets", "guess_card.js"))
	if err != nil {
		t.Fatalf("read guess-card script: %v", err)
	}
	js := string(jsBody)
	for _, needle := range []string{
		`var scrollStorageKey = "manatomb.guessCard.scroll.v1";`,
		`window.history.scrollRestoration = "manual";`,
		`window.localStorage.setItem(scrollStorageKey`,
		`if (!event.defaultPrevented) rememberGameScroll();`,
		`window.scrollTo(0, Math.min(Math.max(0, Number(storedScroll.top)), maximum));`,
	} {
		if !strings.Contains(js, needle) {
			t.Fatalf("guess-card viewport preservation missing %q", needle)
		}
	}

	goBody, err := os.ReadFile("guess_card.go")
	if err != nil {
		t.Fatalf("read guess-card handler: %v", err)
	}
	if strings.Contains(string(goBody), `/games/guess-card#`) {
		t.Fatal("guess-card handler still redirects to a viewport-jumping hash anchor")
	}
}

func TestGuessCardPresentationUsesSharedTokensAndNoMotion(t *testing.T) {
	templateBody, err := os.ReadFile(filepath.Join("templates", "guess_card.html.tmpl"))
	if err != nil {
		t.Fatalf("read guess-card template: %v", err)
	}
	templateSource := string(templateBody)
	for _, forbidden := range []string{
		`<main`,
		`data-guess-question-input`,
		`data-guess-question-menu`,
		`data-guess-card-back`,
		`<details`,
		`<script>`,
		`bg-slate-`,
		`text-slate-`,
		`amber-`,
		`emerald-`,
		`rose-`,
		`transition`,
		`{{ if .Flash }}`,
	} {
		if strings.Contains(templateSource, forbidden) {
			t.Fatalf("guess-card template retained obsolete presentation %q", forbidden)
		}
	}

	cssBody, err := os.ReadFile(filepath.Join("assets", "guess_card.css"))
	if err != nil {
		t.Fatalf("read guess-card stylesheet: %v", err)
	}
	css := string(cssBody)
	for _, needle := range []string{
		`body[data-page="guess-card"] *`,
		`animation: none !important;`,
		`transition: none !important;`,
		`.mt-guess-game__board`,
		`.mt-guess-game__stage`,
		`.mt-guess-game__lower`,
		`.mt-guess-game__history-list > li`,
		`.mt-guess-game__question-scroll`,
		`.mt-guess-game__question-carousel-controls`,
		`.mt-guess-game__question-carousel-viewport`,
		`.mt-guess-game__question-groups`,
		`.mt-guess-game__answer--yes`,
		`var(--mt-positive)`,
		`var(--mt-negative)`,
		`var(--mt-accent-text)`,
		`@media (max-width: 43rem)`,
		`grid-template-columns: 1fr;`,
		`position: absolute;`,
		`overflow-y: auto;`,
		`.mt-guess-help`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("guess-card stylesheet missing %q", needle)
		}
	}
	if regexp.MustCompile(`(?i)(#[0-9a-f]{3,8}\b|rgba?\()`).MatchString(css) {
		t.Fatal("guess-card stylesheet contains a hard-coded color instead of a shared theme token")
	}
	if strings.Contains(css, "@keyframes") {
		t.Fatal("guess-card stylesheet must not define motion")
	}
}
