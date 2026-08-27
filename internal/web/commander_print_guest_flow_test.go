package web

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"manatomb/app/internal/account"
	"manatomb/app/internal/decks"
)

func TestCommanderSelectionTemplatesSubmitExactPrinting(t *testing.T) {
	const printID = "223e4567-e89b-12d3-a456-426614174000"
	result := searchResult{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		ScryfallID: printID,
		Name:       "Atraxa, Grand Unifier",
	}

	builder := renderTemplate(t, "decks_new_commander", TemplateData{
		Data: commanderDeckBuilderPageData{
			Query:   "Atraxa",
			Results: []searchResult{result},
		},
	})
	for _, needle := range []string{
		`name="commander_name" value="Atraxa, Grand Unifier"`,
		`name="commander_print_id" value="` + printID + `"`,
	} {
		if !strings.Contains(builder, needle) {
			t.Fatalf("new commander template missing %q: %s", needle, builder)
		}
	}

	search := renderTemplate(t, "commanders_search", TemplateData{
		Data: struct {
			Query       string
			Results     []searchResult
			Recommended []searchResult
			ReturnTo    string
			ReturnLabel string
		}{
			Query:       "Atraxa",
			Results:     []searchResult{result},
			ReturnTo:    "/decks/new",
			ReturnLabel: "Back to Builder",
		},
	})
	for _, needle := range []string{
		`data-scryfall-id="` + printID + `"`,
		`name="commander_print_id" value="" data-field="sticky-print-input"`,
		`var stickyPrintInputEl = modal.querySelector('[data-field="sticky-print-input"]')`,
		`stickyPrintInputEl.value = trimValue(version.scryfall_id)`,
	} {
		if !strings.Contains(search, needle) {
			t.Fatalf("commander search template missing %q: %s", needle, search)
		}
	}
}

func TestGuestDeckTemplateCarriesCommanderPrintingIntoSavedImport(t *testing.T) {
	const printID = "223e4567-e89b-12d3-a456-426614174000"
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:             "Guest Commander",
				Format:           "Commander",
				CommanderName:    "Atraxa, Grand Unifier",
				CommanderPrintID: printID,
			},
			WorkbenchMode: true,
		},
	})

	for _, needle := range []string{
		`var commanderPrintID = "` + printID + `"`,
		`commanderPrintID: normalizeName(savedDeckSeed.commanderPrintID || commanderPrintID || '')`,
		`function currentCommanderPrintForDraft(d)`,
		`dd.commanderPrintID = commanderPrintID`,
		`commander_print_id: currentCommanderPrintForDraft(dd)`,
		`var previousCommanderMeta = normalizeDraftCardMeta(`,
		`var previousMeta = previousCommanderMeta || buildDraftCardMeta(previousResolved)`,
		`nextURL.searchParams.set('commander_print_id', nextCommanderPrintID)`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("guest deck template missing commander-print flow %q: %s", needle, body)
		}
	}
}

func TestGuestDeckTemplateCarriesPerBoardPrintingsIntoSavedImport(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		CurrentUser: &account.User{ID: 7},
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:   "Printing Draft",
				Format: "Sandbox",
			},
			WorkbenchMode: true,
		},
	})

	for _, needle := range []string{
		`function draftRowsForBoard(draft, board, quantities)`,
		`var meta = draftMetaForBoardCard(draft, board, name) || {};`,
		`preferred_print_id: normalizeName(meta.preferredPrintID || meta.printID || '')`,
		`var cards = draftRowsForBoard(dd, 'main', dd.cards);`,
		`var sideboardCards = draftRowsForBoard(dd, 'side', dd.sideboardCards);`,
		`var maybeCards = draftRowsForBoard(dd, 'maybe', dd.maybeCards);`,
		`card_meta: (dd.cardMeta && typeof dd.cardMeta === 'object') ? dd.cardMeta : {},`,
		`board_card_meta: (dd.boardCardMeta && typeof dd.boardCardMeta === 'object') ? dd.boardCardMeta : {},`,
		`dd.boardCardMeta[boardKey][name] = nextMeta;`,
		`applyGuestCardVersion(cardName, meta, version, board);`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("guest deck template missing per-board printing flow %q", needle)
		}
	}
}

func TestGuestDraftReportsLocalPersistenceFailure(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck:          &decks.Deck{Name: "Local Draft", Format: "Sandbox"},
			WorkbenchMode: true,
		},
	})

	for _, needle := range []string{
		`local draft verification failed`,
		`Could not save this draft on this device. Try again before leaving.`,
		`guestOverviewStatus.setAttribute('data-save-state', 'error')`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("guest deck template missing persistence feedback %q", needle)
		}
	}
}

func TestSavedCommanderOverviewDoesNotOverwriteExactPrinting(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		CurrentUser: &account.User{ID: 7},
		Data: deckPageData{
			Deck: &decks.Deck{
				ID:               42,
				Name:             "Saved Commander",
				Format:           "Commander",
				CommanderName:    "Atraxa, Grand Unifier",
				CommanderPrintID: "223e4567-e89b-12d3-a456-426614174000",
			},
		},
	})

	if !strings.Contains(body, `if (normalizeName(d && d.commanderPrintID ? d.commanderPrintID : ''))`) {
		t.Fatal("saved commander overview can still overwrite exact printing metadata")
	}
}

func TestImportDraftRequestDecodesCommanderPrinting(t *testing.T) {
	const printID = "223e4567-e89b-12d3-a456-426614174000"
	req := httptest.NewRequest("POST", "/decks/import/save", strings.NewReader(
		`{"commander_name":"Atraxa, Grand Unifier","commander_print_id":"`+printID+`","format":"Commander","cards":[]}`,
	))

	var payload importDraftRequest
	if err := parseJSONBody(req, &payload); err != nil {
		t.Fatalf("parseJSONBody: %v", err)
	}
	if payload.CommanderPrintID != printID {
		t.Fatalf("CommanderPrintID = %q, want %q", payload.CommanderPrintID, printID)
	}

	encoded, err := json.Marshal(workbenchImportPayload{
		CommanderName:    "Atraxa, Grand Unifier",
		CommanderPrintID: printID,
	})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"commander_print_id":"`+printID+`"`) {
		t.Fatalf("workbench import payload omitted commander printing: %s", encoded)
	}
}
