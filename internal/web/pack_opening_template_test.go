package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackOpeningTemplateUsesSetRailBoosterButtonsAndInteractiveStage(t *testing.T) {
	body := renderTemplate(t, "pack_opening", TemplateData{
		Data: packOpeningPageData{
			Sets: []packOpeningSetOption{
				{Code: "old", Name: "Older Set", ReleasedAt: "2025-01-01", Label: "Older Set (2025)"},
				{Code: "new", Name: "Newest Set", ReleasedAt: "2026-08-01", Label: "Newest Set (2026)"},
			},
			PackTypeOptions: map[string][]packOpeningPackType{
				"new": {{ID: "play", Name: "Play Booster", CardCount: 14, Accuracy: packOpeningAccuracySourced, AccuracyLabel: packOpeningAccuracySourcedLabel}},
			},
		},
	})

	htmlTag := renderedOpeningTag(t, body, "html")
	bodyTag := renderedOpeningTag(t, body, "body")
	if !strings.Contains(htmlTag, `data-theme="tomb"`) {
		t.Fatalf("pack opening page is missing the Tomb palette: %s", htmlTag)
	}
	if !strings.Contains(bodyTag, `data-page="pack-opening"`) {
		t.Fatalf("pack opening page is missing its page scope: %s", bodyTag)
	}
	if strings.Count(body, "<main") != 1 {
		t.Fatalf("pack opening rendered %d main landmarks, want the shared one only", strings.Count(body, "<main"))
	}
	if got := strings.Count(body, `href="/assets/pack_opening.css"`); got != 1 {
		t.Fatalf("pack opening rendered its stylesheet %d times, want once", got)
	}
	if got := strings.Count(body, `src="/assets/pack_opening.js"`); got != 1 {
		t.Fatalf("pack opening rendered its script %d times, want once", got)
	}

	for _, needle := range []string{
		`href="/assets/pack_opening.css"`,
		`src="/assets/pack_opening.js"`,
		`<h1>Pack Crack</h1>`,
		`data-pack-set-rail`,
		`aria-label="Pack-ready Magic sets, oldest to newest"`,
		`data-pack-type-section`,
		`data-pack-type-list`,
		`data-pack-simulation-open`,
		`aria-haspopup="dialog"`,
		`data-pack-simulation-dialog`,
		`aria-labelledby="pack-simulation-title"`,
		`data-pack-simulation-close`,
		`data-pack-simulation-recipe`,
		`data-pack-simulation-limitations`,
		`data-pack-simulation-source`,
		`data-pack-opening`,
		`data-pack-wrapper`,
		`data-pack-wrapper-symbol-image`,
		`data-pack-wrapper-symbol-image src="/assets/favicon.svg"`,
		`data-pack-wrapper-symbol-fallback`,
		`decoding="async"`,
		`data-pack-open-slider`,
		`data-pack-open-slider-handle`,
		`aria-label="Slide right to open this booster pack"`,
		`role="slider"`,
		`aria-valuemin="0"`,
		`aria-valuemax="100"`,
		`Slide to open`,
		`data-pack-reveal-all hidden>Reveal all</button>`,
		`data-pack-stack`,
		`data-pack-stack role="group" aria-label="Cards in this pack"`,
		`data-pack-current-price`,
		`data-pack-current-pulls`,
		`data-pack-current-pulls tabindex="-1" hidden`,
		`data-pack-current-pulls-grid`,
		`id="pack-pulls-title">Your Pulls</h2>`,
		`data-pack-pulls-grid`,
		`data-pack-reset hidden>Crack another one?</button>`,
		`class="mt-pack-wrapper__brand" aria-hidden="true">ManaTomb</div>`,
		`role="status" aria-live="polite"`,
		`"code":"old"`,
		`"code":"new"`,
		`"id":"play"`,
		`"accuracy":"sourced-recipe"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("pack opening page missing %q: %s", needle, body)
		}
	}
	openingIndex := strings.Index(body, `data-pack-opening`)
	openingCloseOffset := strings.Index(body[openingIndex:], `</section>`)
	pullsIndex := strings.Index(body, `data-pack-pulls`)
	if openingIndex < 0 || openingCloseOffset < 0 || pullsIndex < openingIndex+openingCloseOffset {
		t.Fatalf("Your Pulls must remain a sibling after the active pack-opening section: %s", body)
	}
	symbolIndex := strings.Index(body, `data-pack-wrapper-symbol-image`)
	setNameIndex := strings.Index(body, `data-pack-wrapper-set`)
	packTypeIndex := strings.Index(body, `data-pack-wrapper-type`)
	if symbolIndex < 0 || setNameIndex <= symbolIndex || packTypeIndex <= setNameIndex {
		t.Fatalf("sealed wrapper identity must render symbol, set name, then booster type: %s", body)
	}
	sceneIndex := strings.Index(body, `data-pack-scene`)
	stackIndex := strings.Index(body, `data-pack-stack`)
	currentPriceIndex := strings.Index(body, `data-pack-current-price`)
	currentPullsIndex := strings.Index(body, `data-pack-current-pulls`)
	currentPullsGridIndex := strings.Index(body, `data-pack-current-pulls-grid`)
	replayIndex := strings.Index(body, `data-pack-reset`)
	liveIndex := strings.Index(body, `data-pack-live`)
	if sceneIndex < 0 || stackIndex <= sceneIndex || currentPriceIndex <= stackIndex || currentPullsIndex <= currentPriceIndex || currentPullsGridIndex < currentPullsIndex || replayIndex <= currentPullsGridIndex || liveIndex <= replayIndex {
		t.Fatalf("current-pack pulls must follow the focused card, with replay below the staged grid: %s", body)
	}
	headingIndex := strings.Index(body, `mt-pack-opening__heading`)
	headingCloseOffset := strings.Index(body[headingIndex:], `</header>`)
	if headingIndex < 0 || headingCloseOffset < 0 || strings.Contains(body[headingIndex:headingIndex+headingCloseOffset], `data-pack-reset`) {
		t.Fatalf("replay control must not remain in the opening header: %s", body)
	}

	for _, forbidden := range []string{
		`<select`,
		`<option`,
		`data-pack-generate`,
		`data-pack-rail-previous`,
		`data-pack-rail-next`,
		`data-pack-current-hint`,
		`Tear here`,
		`data-pack-tear`,
		`mt-pack-tear-strip`,
		`mt-pack-wrapper__depth`,
		`data-pack-wrapper-art`,
		`mt-pack-wrapper__art`,
		`top-right corner`,
		`Type a Set`,
		`Or Choose a Set`,
		`Pack Simulator`,
		`Open another`,
		`MT &middot; ManaTomb`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("pack opening page still contains obsolete UI %q", forbidden)
		}
	}
}

func TestPackOpeningAssetsKeepStateMachineAndThemeTokens(t *testing.T) {
	withRendererRoot(t)
	cssBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "pack_opening.css"))
	if err != nil {
		t.Fatal(err)
	}
	jsBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "pack_opening.js"))
	if err != nil {
		t.Fatal(err)
	}
	css := string(cssBytes)
	js := string(jsBytes)

	for _, token := range []string{
		"var(--mt-text)",
		"var(--mt-surface)",
		"var(--mt-border-control)",
		"var(--mt-accent)",
		"var(--mt-focus)",
		"prefers-reduced-motion: reduce",
		"min-height: 2.5rem",
		`url("https://cards.scryfall.io/back.png")`,
		".mt-pack-set__symbol-image",
		"filter: invert(1)",
		".mt-pack-wrapper.is-unsealed",
		".mt-pack-wrapper__symbol-image",
		".mt-pack-wrapper__symbol-fallback",
		".mt-pack-open-slider",
		".mt-pack-reveal-all",
		".mt-pack-current-pulls",
		".mt-pack-current-pulls__actions",
		".mt-pack-replay",
		".mt-pack-replay[hidden]",
		`.mt-pack-pull[data-finish="foil"]`,
		".mt-pack-simulation::backdrop",
		".mt-pack-simulation__section",
	} {
		if !strings.Contains(css, token) {
			t.Fatalf("pack opening CSS missing %q", token)
		}
	}
	for _, forbidden := range []string{"#fff", "#000", "rgb(", "rgba("} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("pack opening CSS hardcodes palette color %q", forbidden)
		}
	}
	for _, forbidden := range []string{
		`.mt-pack-pull--rare .mt-pack-pull__image`,
		`.mt-pack-pull--mythic .mt-pack-pull__image`,
		`.mt-pack-pull--mythic .mt-pack-pull__price`,
	} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("pack opening CSS still applies rarity-specific pull styling with %q", forbidden)
		}
	}
	if !strings.Contains(css, `.mt-pack .mt-pack-replay:hover`) || !strings.Contains(css, `transform: none;`) {
		t.Fatal("replay hover must preserve its resting position")
	}

	for _, phase := range []string{`setPhase("idle")`, `setPhase("loading")`, `setPhase("sealed")`, `setPhase("opening")`, `setPhase("revealing")`, `setPhase("fast-forwarding")`, `setPhase("complete")`} {
		if !strings.Contains(js, phase) {
			t.Fatalf("pack opening state machine missing %q", phase)
		}
	}
	for _, feature := range []string{
		`rail.scrollLeft = rail.scrollWidth`,
		`value[snake]`,
		`fetch("/games/pack-opening"`,
		`function selectPackType`,
		`function updateSimulationDetails`,
		`function attachSimulationDetails`,
		`field(packType, "AccuracySummary")`,
		`textList(packType, "SlotRecipe")`,
		`textList(packType, "Limitations")`,
		`simulationDialog.showModal()`,
		`event.target === simulationDialog`,
		`simulationDialog.addEventListener("close"`,
		`function setIconURL`,
		`field(item, "IconSVGURI")`,
		`icon.addEventListener("error"`,
		`mt-pack-set__symbol-image`,
		`function cancelChoreography`,
		`function preloadCardImages`,
		`function preloadImages`,
		`function applyWrapperIdentity`,
		`setIconURL(state.set)`,
		`wrapperSymbolFallback.textContent = code || "MTG"`,
		`wrapperSymbolImage.src = iconURL`,
		`function openPack`,
		`function autoOpenPack`,
		`function flipStackFaceUp`,
		`function attachOpenSlider`,
		`function updateOpenSlider`,
		`function sliderTravel`,
		`aria-valuetext`,
		`function attachCardSwipe`,
		`function addPull`,
		`function revealAllCards`,
		`revealAllButton.addEventListener("click", revealAllCards)`,
		`function commitCurrentPack`,
		`while (currentPullsGrid.firstChild)`,
		`pullsGrid.appendChild(currentPullsGrid.firstChild)`,
		`function settleOpenedPack`,
		`state.sequenceToken`,
		`state.suppressClickNode`,
		`}, 450);`,
		`var dragTransform = node.style.transform`,
		`translate(-50%, -58%)`,
		`scaleX(.035)`,
		`card.finish`,
		`var currentPackType = state.packType`,
		`loadPack(typeButton, { autoOpen: true, scroll: true })`,
		`prepareSealedPack(Boolean(options.autoOpen))`,
		`var allCardsDismissed = state.cards.length > 0 && state.currentIndex >= state.cards.length`,
		`resetButton.hidden = !allCardsDismissed`,
		`currentPulls.focus({ preventScroll: true })`,
	} {
		if !strings.Contains(js, feature) {
			t.Fatalf("pack opening script missing %q", feature)
		}
	}
	for _, forbidden := range []string{
		`function selectionLocked`,
		`function flipCard`,
		`function attachTearGesture`,
		`tearDash`,
		`tearStrip`,
		`railPrevious`,
		`railNext`,
		`currentHint`,
		`PackArtURI`,
		`packArtURL`,
		`applyPackArt`,
		`wrapperArt`,
		`resetButton.hidden = false`,
	} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("pack opening script still contains obsolete interaction %q", forbidden)
		}
	}

	assets := strings.ToLower(css + "\n" + js)
	for _, forbidden := range []string{
		`perspective`,
		`transform-style`,
		`preserve-3d`,
		`translate3d`,
		`translatez`,
		`rotatex`,
		`rotatey`,
		`rotate3d`,
		`matrix3d`,
		`backface-visibility`,
		`--mt-pack-tilt`,
		`mt-pack-wrapper__depth`,
		`attachwrapperinteraction`,
		`resetwrappertilt`,
	} {
		if strings.Contains(assets, forbidden) {
			t.Fatalf("pack opening assets reintroduced 3D presentation %q", forbidden)
		}
	}
}

func TestPackOpeningAssetsAccumulatePullsForPageSession(t *testing.T) {
	withRendererRoot(t)
	jsBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "pack_opening.js"))
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)

	for _, feature := range []string{
		`packPullCount: 0`,
		`packPullValueCents: 0`,
		`packCommitted: false`,
		`sessionPullCount: 0`,
		`sessionPullValueCents: 0`,
		`function priceCents`,
		`state.packPullCount += 1`,
		`state.packPullValueCents += cents`,
		`addPull(card, currentPullsGrid)`,
		`state.sessionPullValueCents += state.packPullValueCents`,
		`state.sessionPullCount += state.packPullCount`,
		`pullCount.textContent = String(state.sessionPullCount)`,
		`totalValue.textContent = dollarsFromCents(state.sessionPullValueCents)`,
		`pulls.hidden = state.sessionPullCount === 0`,
		`dollarsFromCents(state.packPullValueCents)`,
	} {
		if !strings.Contains(js, feature) {
			t.Fatalf("pack opening session pull history is missing %q", feature)
		}
	}

	for _, forbidden := range []string{
		`pullsGrid.innerHTML = ""`,
		`state.sessionPullCount = 0`,
		`state.sessionPullValueCents = 0`,
		`state.sessionPullValueCents += cents`,
	} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("pack opening resets page-session pull history with %q", forbidden)
		}
	}

	recordStart := strings.Index(js, `function recordPull`)
	recordEnd := strings.Index(js, `function addPull`)
	if recordStart < 0 || recordEnd <= recordStart {
		t.Fatal("could not isolate recordPull")
	}
	recordPull := js[recordStart:recordEnd]
	for _, forbidden := range []string{`sessionPull`, `pullsGrid.appendChild`, `pulls.hidden = false`} {
		if strings.Contains(recordPull, forbidden) {
			t.Fatalf("recordPull committed staged cards too early with %q", forbidden)
		}
	}

	commitStart := strings.Index(js, `function commitCurrentPack`)
	commitEnd := strings.Index(js, `function settleOpenedPack`)
	if commitStart < 0 || commitEnd <= commitStart {
		t.Fatal("could not isolate commitCurrentPack")
	}
	commit := js[commitStart:commitEnd]
	for _, required := range []string{`state.packCommitted`, `state.sessionPullCount += state.packPullCount`, `state.sessionPullValueCents += state.packPullValueCents`, `pullsGrid.appendChild(currentPullsGrid.firstChild)`, `updatePullSummary()`} {
		if !strings.Contains(commit, required) {
			t.Fatalf("commitCurrentPack is missing %q", required)
		}
	}

	replayStart := strings.Index(js, `function resetForAnother`)
	replayEnd := strings.Index(js, `function sliderTravel`)
	if replayStart < 0 || replayEnd <= replayStart {
		t.Fatal("could not isolate resetForAnother")
	}
	replay := js[replayStart:replayEnd]
	commitIndex := strings.Index(replay, `commitCurrentPack()`)
	resetIndex := strings.Index(replay, `resetOpening(false)`)
	loadIndex := strings.Index(replay, `loadPack(typeButton, { autoOpen: true, scroll: true })`)
	if !strings.Contains(replay, `state.phase !== "complete"`) || commitIndex < 0 || resetIndex <= commitIndex || loadIndex <= resetIndex {
		t.Fatalf("replay must gate, commit, reset, and auto-open in order: %s", replay)
	}
}

func TestPackOpeningAssetsRevealAllUsesTheNormalCompletionPath(t *testing.T) {
	withRendererRoot(t)
	jsBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "pack_opening.js"))
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	start := strings.Index(js, `async function revealAllCards`)
	end := strings.Index(js, `function recordPull`)
	if start < 0 || end <= start {
		t.Fatal("could not isolate revealAllCards")
	}
	revealAll := js[start:end]
	for _, required := range []string{`state.phase !== "revealing"`, `setPhase("fast-forwarding")`, `revealAllButton.disabled = true`, `await Promise.all`, `recordPull(item.index)`, `state.currentIndex = state.cards.length`, `completePack()`} {
		if !strings.Contains(revealAll, required) {
			t.Fatalf("revealAllCards is missing %q", required)
		}
	}
}

func TestPackOpeningAssetsStageOnlyDismissedCards(t *testing.T) {
	withRendererRoot(t)
	jsBytes, err := os.ReadFile(filepath.Join("internal", "web", "assets", "pack_opening.js"))
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	openStart := strings.Index(js, `async function openPack`)
	openEnd := strings.Index(js, `async function flipStackFaceUp`)
	if openStart < 0 || openEnd <= openStart {
		t.Fatal("could not isolate openPack")
	}
	if strings.Contains(js[openStart:openEnd], `recordPull(`) {
		t.Fatal("opening the pack must not stage the still-visible first card")
	}

	revealStart := strings.Index(js, `async function revealCurrentCard`)
	revealEnd := strings.Index(js, `async function revealAllCards`)
	if revealStart < 0 || revealEnd <= revealStart {
		t.Fatal("could not isolate revealCurrentCard")
	}
	reveal := js[revealStart:revealEnd]
	animationIndex := strings.Index(reveal, `await play(node`)
	recordIndex := strings.Index(reveal, `recordPull(index)`)
	hideIndex := strings.Index(reveal, `node.hidden = true`)
	advanceIndex := strings.Index(reveal, `state.currentIndex += 1`)
	if animationIndex < 0 || recordIndex <= animationIndex || hideIndex <= recordIndex || advanceIndex <= hideIndex {
		t.Fatalf("dismissal must finish before the current card is staged and advanced: %s", reveal)
	}
	if strings.Contains(reveal, `recordPull(state.currentIndex)`) {
		t.Fatal("the next visible card must not be staged before it is dismissed")
	}
}
