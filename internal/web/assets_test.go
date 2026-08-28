package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetHandlerServesPlaytestScript(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/playtest.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/javascript") {
		t.Fatalf("expected JavaScript content type, got %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=3600, stale-while-revalidate=86400" {
		t.Fatalf("expected reusable asset cache policy, got %q", got)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "window.ManatombPlaytestConfig") {
		t.Fatal("expected playtest script to read the playtest config")
	}
	if strings.Contains(body, "{{") {
		t.Fatal("playtest script still contains template delimiters")
	}
}

func TestAssetHandlerServesSharedDeckTileStyles(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/deck_tile.css", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/css") {
		t.Fatalf("expected CSS content type, got %q", got)
	}
	if body := rr.Body.String(); !strings.Contains(body, ".mt-deck-tile") {
		t.Fatal("shared deck tile stylesheet is missing its component rule")
	}
}

func TestAssetHandlerServesFavicon(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/favicon.svg", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("favicon status = %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "image/svg+xml") {
		t.Fatalf("favicon content type = %q", got)
	}
	if !strings.Contains(rr.Body.String(), `<svg`) {
		t.Fatal("favicon response is not SVG")
	}
}

func TestAssetHandlerServesRoundedManaTombTabLogo(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/manatomb-square-logo.svg", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("tab logo status = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `rx="267"`) || strings.Contains(body, `<rect width="1254" height="1254" fill="#000"/>`) {
		t.Fatalf("tab logo should preserve transparent rounded corners: %s", body)
	}
}

func TestPlaytestAssetPreservesWorkbenchFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/playtest.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `var format = String(baseDraft.format || "").trim() || "Sandbox";`) {
		t.Fatal("playtest workbench return should preserve supported non-Commander formats")
	}
	if strings.Contains(body, `trim() === "Commander" ? "Commander" : "Sandbox"`) {
		t.Fatal("playtest workbench return still collapses non-Commander formats to Sandbox")
	}
}

func TestPlaytestAssetPreservesWorkbenchDeckName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/playtest.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, needle := range []string{
		`var deckName = String(baseDraft.name || "").trim() || "Untitled Deck";`,
		`name: deckName,`,
		`name: String(payload.name || "").trim() || "Untitled Deck",`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("playtest workbench return is missing deck-name preservation %q", needle)
		}
	}
	if strings.Contains(body, `name: "New Guest Deck"`) {
		t.Fatal("playtest workbench return still replaces the deck name")
	}
}

func TestPlaytestAssetPreservesWorkbenchPrintingMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/playtest.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, needle := range []string{
		`var commanderPrintID = String(baseDraft.commanderPrintID || "").trim();`,
		`commander_print_id: commanderPrintID,`,
		`board_card_meta: (baseDraft && baseDraft.boardCardMeta && typeof baseDraft.boardCardMeta === "object") ? baseDraft.boardCardMeta : {},`,
		`commanderPrintID: String(payload.commander_print_id || "").trim(),`,
		`boardCardMeta: (payload.board_card_meta && typeof payload.board_card_meta === "object") ? payload.board_card_meta : {},`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("playtest workbench return is missing printing-metadata preservation %q", needle)
		}
	}
}

func TestDeckEditorAssetProvidesKeyboardNavigation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/deck_show.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, needle := range []string{
		`function setupDeckBoardKeyboard()`,
		`event.key === "ArrowRight" || event.key === "ArrowDown"`,
		`tabs[nextIndex].click();`,
		`function setupAddCardShortcut()`,
		`if (event.key !== "/"`,
		`window.mtDeckOpenWorkspaceView("decklist")`,
		`function setupDeckFocusMode()`,
		`mt-deck-editor--focus-mode`,
		`mt-deck-focus-mode`,
		`enabled ? "Exit full screen" : "Full screen"`,
		`if (event.key !== "Escape"`,
		`nestedEscapeTargetIsOpen(event)`,
		`document.querySelector('details[data-card-action-menu][open]')`,
		`addInput.getAttribute("aria-expanded") === "true"`,
		`window.dispatchEvent(new Event("resize"))`,
		`case "details":`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck editor keyboard support missing %q", needle)
		}
	}
}

func TestDeckEditorAssetRelayoutsCardsWhenWorkspaceBecomesVisible(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/deck_show.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	toggleIndex := strings.Index(body, `panel.hidden = !selected;`)
	decklistIndex := strings.Index(body, `if (next === "decklist") {`)
	if toggleIndex == -1 || decklistIndex == -1 {
		t.Fatal("Cards workspace reveal is missing its explicit grouped-card relayout branch")
	}
	frameIndex := strings.Index(body[decklistIndex:], `window.requestAnimationFrame(function () {`)
	refreshIndex := strings.Index(body[decklistIndex:], `window.mtDeckRefreshCardLayout();`)
	if frameIndex == -1 || refreshIndex == -1 {
		t.Fatal("Cards workspace reveal is missing its explicit grouped-card relayout hook")
	}
	if decklistIndex < toggleIndex || refreshIndex < frameIndex {
		t.Fatal("Cards workspace must relayout after the hidden panels become visible")
	}
}

func TestDeckEditorAssetOpensPartialImportsOnAnalysisWithOneShotWarning(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/deck_show.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, needle := range []string{
		`return new URLSearchParams(window.location.search).get("view");`,
		`selectView(requestedView());`,
		`function setupDeckImportWarningToast()`,
		`var warningKey = "manatomb.deckImportWarning";`,
		`query.get("import_warning") === "1"`,
		`sessionStorage.removeItem(warningKey);`,
		`window.mtDeckOpenWorkspaceView("analysis");`,
		`setupDeckImportWarningToast();`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("partial-import workspace feedback missing %q", needle)
		}
	}
}

func TestAssetHandlerServesDeckBrowserAssets(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/assets/deck_browser.js", contentType: "text/javascript", contains: "global.ManatombDeckBrowser"},
		{path: "/assets/public_deck.js", contentType: "text/javascript", contains: "ManatombPublicDeckConfig"},
		{path: "/assets/public_deck.css", contentType: "text/css", contains: ".mt-public-single"},
		{path: "/assets/profile.js", contentType: "text/javascript", contains: "profile-deck-filter"},
		{path: "/assets/profile.css", contentType: "text/css", contains: ".mt-profile-hero"},
		{path: "/assets/theme.css", contentType: "text/css", contains: `html[data-theme="tomb"]`},
		{path: "/assets/og.png", contentType: "image/png"},
		{path: "/assets/tailwind.css", contentType: "text/css", contains: ".flex"},
		{path: "/assets/card_show.css", contentType: "text/css", contains: ".mt-card-hero"},
		{path: "/assets/cards_search.css", contentType: "text/css", contains: ".mt-advanced-search-page"},
		{path: "/assets/cards_search.js", contentType: "text/javascript", contains: "setupCardSearchForm"},
		{path: "/assets/cards_list.css", contentType: "text/css", contains: ".mt-card-results-page"},
		{path: "/assets/cards_list.js", contentType: "text/javascript", contains: "data-card-result-tile"},
		{path: "/assets/auth.css", contentType: "text/css", contains: ".mt-auth-page"},
		{path: "/assets/account_pages.css", contentType: "text/css", contains: ".mt-deck-library"},
		{path: "/assets/settings.css", contentType: "text/css", contains: ".mt-settings-page"},
		{path: "/assets/decks_public.css", contentType: "text/css", contains: ".mt-public-decks-page"},
		{path: "/assets/decks_public.js", contentType: "text/javascript", contains: "data-public-decks-color-options"},
		{path: "/assets/deck_show.css", contentType: "text/css", contains: ".mt-deck-editor"},
		{path: "/assets/deck_show.js", contentType: "text/javascript", contains: "data-deck-workspace-tab"},
		{path: "/assets/guess_card.css", contentType: "text/css", contains: ".mt-guess-game"},
		{path: "/assets/guess_card.js", contentType: "text/javascript", contains: "data-guess-card-reveal-form"},
		{path: "/assets/spellify.css", contentType: "text/css", contains: ".mt-tombscript"},
		{path: "/assets/spellify.js", contentType: "text/javascript", contains: "data-tombscript-character-form"},
		{path: "/assets/pack_opening.css", contentType: "text/css", contains: ".mt-pack-wrapper"},
		{path: "/assets/pack_opening.js", contentType: "text/javascript", contains: "data-pack-open-slider"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			AssetHandler().ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rr.Code)
			}
			if got := rr.Header().Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Fatalf("expected %s content type, got %q", tt.contentType, got)
			}
			if !strings.Contains(rr.Body.String(), tt.contains) {
				t.Fatalf("expected asset to contain %q", tt.contains)
			}
		})
	}
}

func TestDeckBrowserSortDirectionPreservesSentinelOrdering(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/deck_browser.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, needle := range []string{
		`var TYPE_SORT_ORDER = TYPE_ORDER.slice();`,
		`function normalizeSortDirection(value)`,
		`direction = normalizeSortDirection(direction);`,
		`var directionMultiplier = direction === "desc" ? -1 : 1;`,
		`var leftLand = primaryType(left.typeLine) === "Lands";`,
		`if (leftLand !== rightLand) return leftLand ? 1 : -1;`,
		`if (leftPrice === null && rightPrice !== null) return 1;`,
		`compareCards(a, b, sort, direction)`,
		`normalizeSortDirection: normalizeSortDirection,`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck browser land ordering is missing %q", needle)
		}
	}
}

func TestDeckBrowserExportsRoundTripDeckAndPrintingMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/deck_browser.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, needle := range []string{
		`lines.push("Name: " + trim(metadata.name))`,
		`lines.push("Format: " + trim(metadata.format))`,
		`identity += " (" + card.setCode + ") " + card.collectorNumber`,
		`identity += " {scryfall:" + printID + "}"`,
		`function exportBoardCards(boards, board, metadata)`,
		`var commander = normalizeCard(metadata.commander || { name: metadata.commanderName }, -1);`,
		`rows.push([`,
		`"Commander",`,
		`["Board", "Quantity", "Name", "Set", "Collector Number", "Print ID", "Price USD", "Deck Name", "Format"]`,
		`card.printID || card.preferredPrintID`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck browser export is missing %q", needle)
		}
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/assets/public_deck.js", nil)
	publicRR := httptest.NewRecorder()
	AssetHandler().ServeHTTP(publicRR, publicReq)
	if publicRR.Code != http.StatusOK {
		t.Fatalf("expected public deck asset status 200, got %d", publicRR.Code)
	}
	if !strings.Contains(publicRR.Body.String(), `core.csvExport(boards, config)`) {
		t.Fatal("public deck CSV export does not pass deck metadata")
	}
}

func TestAssetHandlerRejectsUnsupportedMethods(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/assets/playtest.js", nil)
	rr := httptest.NewRecorder()

	AssetHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rr.Code)
	}
}
