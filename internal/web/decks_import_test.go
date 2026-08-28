package web

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

func TestDeckImportDoesNotQueueGenericSuccessFlash(t *testing.T) {
	withRendererRoot(t)
	path := filepath.Join("internal", "web", "decks_import.go")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read deck import handler: %v", err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
	if err != nil {
		t.Fatalf("parse deck import handler: %v", err)
	}
	foundHandler := false
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != "HandleDeckImportText" {
			continue
		}
		foundHandler = true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if ok && identifier.Name == "setFlash" {
				t.Error("clean deck imports must open the populated editor without a generic flash toast")
			}
			return true
		})
	}
	if !foundHandler {
		t.Fatal("HandleDeckImportText not found")
	}
}

func TestWorkbenchImportPayloadGeneratesMissingNameAndPreservesExplicitName(t *testing.T) {
	page := deckImportPageData{Format: "Sandbox"}

	generated := workbenchPayloadFromImportReview(page, "", "", false)
	if !regexp.MustCompile(`^[A-Za-z0-9_]{6,32}$`).MatchString(generated.Name) {
		t.Fatalf("expected imported draft to receive a generated gamertag, got %q", generated.Name)
	}

	const explicitName = "Keep This Name"
	named := workbenchPayloadFromImportReview(page, explicitName, "", false)
	if named.Name != explicitName {
		t.Fatalf("explicit imported deck name = %q, want %q", named.Name, explicitName)
	}
}

func TestImportDraftJSONAcceptsRichClientCardMetadata(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/decks/import-draft", strings.NewReader(`{
  "name":"Not Yet 100 Cards",
  "format":"Commander",
  "cards":[{"name":"Plains","qty":7}],
  "card_meta":{"Plains":{"cardID":"oracle-id","name":"Plains","typeLine":"Basic Land — Plains","imageURI":"https://example.test/plains.jpg","unrelated_client_field":true}}
}`))
	var payload importDraftRequest
	if err := parseImportDraftJSONBody(req, &payload); err != nil {
		t.Fatalf("parseImportDraftJSONBody() rejected rich card metadata: %v", err)
	}
	if payload.Name != "Not Yet 100 Cards" || len(payload.Cards) != 1 || payload.CardMeta["Plains"].resolvedOracleID() != "oracle-id" {
		t.Fatalf("decoded import payload = %#v", payload)
	}
}

func TestDeckImportTemplateSubmitsDirectlyToTheEditor(t *testing.T) {
	body := renderTemplate(t, "decks_import", TemplateData{
		Data: deckImportPageData{
			Step: "paste",
		},
	})

	for _, needle := range []string{
		`<h1>Import a Decklist</h1>`,
		`method="POST" action="/decks/import"`,
		`data-import-form`,
		`name="intent" value="import"`,
		`name="source" value="text"`,
		`data-import-dropzone`,
		`accept=".txt,.csv,text/plain,text/csv"`,
		`data-import-file`,
		`id="deck-import-file-status"`,
		`role="status"`,
		`aria-live="polite"`,
		`data-import-submit`,
		`aria-disabled="false">Import</button>`,
		`data-existing-draft-warning`,
		`data-storage-warning`,
		`name="replace_existing_draft"`,
		`function loadDecklistFile(file)`,
		`reader.readAsText(file);`,
		`dropzone.addEventListener('drop'`,
		`function draftHasCards(raw)`,
		`hasExistingDraft = draftHasCards(localStorage.getItem('manatomb.draftDeck'));`,
		`storageUnavailable = true;`,
		`replacementConfirmation.required = true;`,
		`submit.textContent = 'Importing…';`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("direct import template missing %q: %s", needle, body)
		}
	}
	for _, forbidden := range []string{
		`class="mt-deck-import-eyebrow"`,
		`Quantities, Commander, Mainboard, Sideboard, Maybeboard`,
		`Review Decklist`,
		`Review cards`,
		`Open in Builder`,
		`data-import-row`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("direct import template retained obsolete review UI %q", forbidden)
		}
	}
}

func TestDeckImportTemplateRendersFieldErrorsBesideTheTextarea(t *testing.T) {
	body := renderTemplate(t, "decks_import", TemplateData{
		Data: deckImportPageData{
			Step:       "paste",
			Decklist:   "Definitely Not A Card",
			FieldError: "No matching cards were found.",
		},
	})

	for _, needle := range []string{
		`aria-invalid="true"`,
		`aria-describedby="deck-import-field-error deck-import-file-status"`,
		`id="deck-import-field-error"`,
		`role="alert">No matching cards were found.</p>`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("accessible deck-import field error missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, `id="site-error"`) {
		t.Fatalf("deck-import field error leaked into the global error banner: %s", body)
	}
}

func TestDirectImportPayloadSkipsProblemRowsAndAddsNotes(t *testing.T) {
	page := deckImportPageData{
		Format: "Commander",
		Items: []deckImportReviewItem{
			{
				Qty:        1,
				Name:       "Atraxa, Grand Unifier",
				Board:      "commander",
				BoardLabel: "Commander",
				resolvedCard: cards.DBCard{
					OracleID: "11111111-1111-1111-1111-111111111111",
					Name:     "Atraxa, Grand Unifier",
				},
			},
			{
				Qty:        1,
				Name:       "Sol Ring",
				Board:      "main",
				BoardLabel: "Mainboard",
				resolvedCard: cards.DBCard{
					OracleID: "22222222-2222-2222-2222-222222222222",
					Name:     "Sol Ring",
					TypeLine: "Artifact",
					CMC:      1,
					ImageURI: "https://img.example/default-sol-ring.jpg",
				},
				PreferredPrintID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				resolvedPrinting: &cards.Card{
					ID:              "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
					OracleID:        "22222222-2222-2222-2222-222222222222",
					Name:            "Sol Ring",
					ImageURI:        "https://img.example/chosen-sol-ring.jpg",
					PriceUSD:        "2.50",
					SetCode:         "CMM",
					SetName:         "Commander Masters",
					CollectorNumber: "396",
					Rarity:          "uncommon",
				},
			},
			{
				Line:             7,
				Qty:              2,
				Name:             "Sol Rin",
				Board:            "main",
				BoardLabel:       "Mainboard",
				StatusDetail:     "No exact card-name match was found.",
				NeedsAttention:   true,
				SuggestedName:    "Sol Ring",
				resolvedCard:     cards.DBCard{},
				resolvedPrinting: nil,
			},
			{
				Line:           8,
				Qty:            1,
				Name:           "Swan Song",
				Board:          "side",
				BoardLabel:     "Sideboard",
				StatusDetail:   "The requested printing could not be matched.",
				NeedsAttention: true,
				resolvedCard: cards.DBCard{
					OracleID: "33333333-3333-3333-3333-333333333333",
					Name:     "Swan Song",
				},
			},
		},
	}

	payload := directImportPayloadFromReview(page, "Imported Deck", "Existing notes", true)
	if payload.CommanderName != "Atraxa, Grand Unifier" {
		t.Fatalf("commander = %q", payload.CommanderName)
	}
	if len(payload.Cards) != 1 || payload.Cards[0].Name != "Sol Ring" {
		t.Fatalf("mainboard payload = %#v", payload.Cards)
	}
	solRingMeta := payload.CardMeta["Sol Ring"]
	if solRingMeta.TypeLine != "Artifact" || solRingMeta.CMC != 1 {
		t.Fatalf("full Sol Ring metadata was lost: %#v", solRingMeta)
	}
	if solRingMeta.ImageURI != "https://img.example/chosen-sol-ring.jpg" {
		t.Fatalf("Sol Ring image = %q, want selected printing art", solRingMeta.ImageURI)
	}
	if payload.Cards[0].Meta.TypeLine != "Artifact" ||
		payload.Cards[0].Meta.CMC != 1 ||
		payload.Cards[0].Meta.ImageURI != "https://img.example/chosen-sol-ring.jpg" {
		t.Fatalf("per-board Sol Ring metadata was lost: %#v", payload.Cards[0].Meta)
	}
	if len(payload.SideboardCards) != 0 || len(payload.MaybeCards) != 0 {
		t.Fatalf("problem rows reached a board: side=%#v maybe=%#v", payload.SideboardCards, payload.MaybeCards)
	}
	if _, exists := payload.CardMeta["Swan Song"]; exists {
		t.Fatal("printing-mismatch row reached card metadata")
	}
	if len(payload.ImportWarnings) != 2 {
		t.Fatalf("warnings = %#v, want two omitted rows", payload.ImportWarnings)
	}
	for _, want := range []string{
		"Existing notes",
		"Import notes",
		"2x Sol Rin (Mainboard, line 7)",
		"Swan Song (Sideboard, line 8)",
	} {
		if !strings.Contains(payload.Description, want) {
			t.Fatalf("description %q missing %q", payload.Description, want)
		}
	}
}

func TestDirectImportPayloadAllowsUnmatchedCommanderToReachEmptyCommanderEditor(t *testing.T) {
	page := deckImportPageData{
		Format: "Commander",
		Issues: []string{"Commander decks need exactly one commander row."},
		Items: []deckImportReviewItem{
			{
				Line:           2,
				Qty:            1,
				Name:           "Not A Real Commander",
				Board:          "commander",
				BoardLabel:     "Commander",
				StatusDetail:   "No matching card was found.",
				NeedsAttention: true,
			},
			{
				Qty:        1,
				Name:       "Sol Ring",
				Board:      "main",
				BoardLabel: "Mainboard",
				resolvedCard: cards.DBCard{
					OracleID: "22222222-2222-2222-2222-222222222222",
					Name:     "Sol Ring",
				},
			},
		},
	}

	payload := directImportPayloadFromReview(page, "Imported Deck", "", false)
	if payload.CommanderName != "" {
		t.Fatalf("unmatched commander was selected: %q", payload.CommanderName)
	}
	if len(payload.Cards) != 1 || payload.Cards[0].Name != "Sol Ring" {
		t.Fatalf("resolved mainboard card was lost: %#v", payload.Cards)
	}
	for _, want := range []string{"Not A Real Commander", "Deck setup — Commander decks need exactly one commander row."} {
		if !strings.Contains(payload.Description, want) {
			t.Fatalf("description %q missing %q", payload.Description, want)
		}
	}
}

func TestWorkbenchImportSeedRestoresPriorDraftOnStorageFailure(t *testing.T) {
	body := renderTemplate(t, "decks_workbench_import_seed", TemplateData{
		Data: workbenchImportSeedData{
			PayloadJSON: `{"format":"Sandbox","cards":[],"import_warnings":["Missing Card"]}`,
		},
	})

	for _, needle := range []string{
		`previousRaw = localStorage.getItem(key)`,
		`hadPreviousDraft = draftHasCards(previousRaw)`,
		`if (localStorage.getItem(key) !== raw)`,
		`localStorage.setItem(key, previousRaw)`,
		`localStorage.removeItem(key)`,
		`payload.replace_existing_draft`,
		`replacement confirmation required`,
		`boardCardMeta`,
		`payload.card_meta`,
		`var meta = item.meta && typeof item.meta === 'object'`,
		`printingMeta[item.name] = meta;`,
		`manatomb.deckImportWarning`,
		`sessionStorage.setItem(importWarningKey, JSON.stringify(importWarnings))`,
		`nextParams.set('view', 'analysis')`,
		`nextParams.set('import_warning', '1')`,
		`sessionStorage.removeItem(importWarningKey)`,
		`Back to import`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("workbench import seed missing %q: %s", needle, body)
		}
	}
}

func TestParsedDecklistFromReviewFormSkipsRemovedRowsAndPreservesBoards(t *testing.T) {
	form := url.Values{
		"row_count":       {"3"},
		"format":          {"Commander"},
		"row_name_0":      {"Atraxa, Grand Unifier"},
		"row_qty_0":       {"1"},
		"row_board_0":     {"commander"},
		"row_name_1":      {"Sol Ring"},
		"row_qty_1":       {"1"},
		"row_board_1":     {"main"},
		"row_set_1":       {"CMM"},
		"row_collector_1": {"396"},
		"row_name_2":      {"Typo"},
		"row_qty_2":       {"1"},
		"row_board_2":     {"side"},
		"row_remove_2":    {"1"},
	}
	req := httptest.NewRequest("POST", "/decks/import", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}

	parsed, err := parsedDecklistFromReviewForm(req)
	if err != nil {
		t.Fatalf("parsedDecklistFromReviewForm: %v", err)
	}
	if parsed.Format != "Commander" || len(parsed.Items) != 2 {
		t.Fatalf("parsed = %#v", parsed)
	}
	if parsed.Items[0].Board != decks.ImportBoardCommander {
		t.Fatalf("commander board = %q", parsed.Items[0].Board)
	}
	if parsed.Items[1].Board != decks.ImportBoardMain ||
		parsed.Items[1].SetCode != "CMM" ||
		parsed.Items[1].CollectorNumber != "396" {
		t.Fatalf("main row = %#v", parsed.Items[1])
	}
}

func TestDraftRequestMetadataAcceptsPerBoardPrintingAndCardMetaFallback(t *testing.T) {
	const (
		directPrint = "123e4567-e89b-12d3-a456-426614174000"
		metaPrint   = "223e4567-e89b-12d3-a456-426614174000"
	)
	req := importDraftRequest{
		Format: "Sandbox",
		Cards: []importDraftCardRequest{
			{Name: "Sol Ring", Qty: 1, PreferredPrintID: directPrint},
		},
		SideboardCards: []importDraftCardRequest{
			{Name: "Swan Song", Qty: 1},
		},
		CardMeta: map[string]importDraftCardMeta{
			"Swan Song": {PreferredPrintID: metaPrint},
		},
	}

	parsed := parsedDecklistFromDraftRequest(req)
	if len(parsed.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(parsed.Items))
	}
	if parsed.Items[0].Board != decks.ImportBoardMain || parsed.Items[0].PrintID != directPrint {
		t.Fatalf("main metadata = %#v", parsed.Items[0])
	}
	if parsed.Items[1].Board != decks.ImportBoardSide || parsed.Items[1].PrintID != metaPrint {
		t.Fatalf("side metadata = %#v", parsed.Items[1])
	}
}
