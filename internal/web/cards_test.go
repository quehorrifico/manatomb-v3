package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
)

func TestCardDetailPath(t *testing.T) {
	t.Parallel()

	got := cardDetailPath("123e4567-e89b-12d3-a456-426614174000")
	want := "/cards/view/123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("cardDetailPath() = %q, want %q", got, want)
	}
}

func TestSelectCardDetailPrintingUsesNewestRealPrintingUnlessExplicit(t *testing.T) {
	t.Parallel()

	staleDefault := cards.Card{ID: "stale-default", Name: "Test Card"}
	newest := cards.Card{ID: "newest-print", Name: "Test Card"}
	older := cards.Card{ID: "older-print", Name: "Test Card"}
	printings := []cards.Card{newest, older}

	if got := selectCardDetailPrinting(staleDefault, printings, ""); got.ID != newest.ID {
		t.Fatalf("normal card detail selected %q, want newest printing %q", got.ID, newest.ID)
	}
	if got := selectCardDetailPrinting(older, printings, older.ID); got.ID != older.ID {
		t.Fatalf("explicit card detail selected %q, want requested printing %q", got.ID, older.ID)
	}
}

func TestCardPrintingDetailPath(t *testing.T) {
	t.Parallel()

	got := cardPrintingDetailPath("123e4567-e89b-12d3-a456-426614174000", "223e4567-e89b-12d3-a456-426614174000")
	want := "/cards/view/123e4567-e89b-12d3-a456-426614174000?printing=223e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("cardPrintingDetailPath() = %q, want %q", got, want)
	}
}

func TestBuildCardShareMetaKeepsExplicitPrintingCanonicalURL(t *testing.T) {
	t.Parallel()

	oracleID := "123e4567-e89b-12d3-a456-426614174000"
	printingID := "223e4567-e89b-12d3-a456-426614174000"
	req := httptest.NewRequest(http.MethodGet, "https://manatomb.example/cards/view/"+oracleID+"?printing="+printingID, nil)
	meta := buildCardShareMeta("https://manatomb.example", req, cards.Card{
		ID:       printingID,
		OracleID: oracleID,
		Name:     "Exact Printing",
	})
	if !strings.Contains(meta.CanonicalURL, "?printing="+printingID) {
		t.Fatalf("explicit printing canonical URL = %q, want printing ID", meta.CanonicalURL)
	}

	canonicalReq := httptest.NewRequest(http.MethodGet, "https://manatomb.example/cards/view/"+oracleID, nil)
	canonicalMeta := buildCardShareMeta("https://manatomb.example", canonicalReq, cards.Card{
		ID:       printingID,
		OracleID: oracleID,
		Name:     "Latest Printing",
	})
	if strings.Contains(canonicalMeta.CanonicalURL, "printing=") {
		t.Fatalf("latest card canonical URL unexpectedly pins a printing: %q", canonicalMeta.CanonicalURL)
	}
}

func TestBuildCardShareMetaUsesTrustedOriginAndArtCrop(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://attacker.example/cards/view/oracle", nil)
	req.Host = "attacker.example"
	meta := buildCardShareMeta("https://manatomb.app", req, cards.Card{
		OracleID:   "oracle",
		Name:       "Lightning Bolt",
		TypeLine:   "Instant",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
		ImageURI:   "https://images.example/card.jpg",
		ArtCropURI: "https://images.example/art.jpg",
	})

	if meta.CanonicalURL != "https://manatomb.app/cards/view/oracle" {
		t.Fatalf("canonical URL = %q, want trusted public origin", meta.CanonicalURL)
	}
	if meta.ImageURL != "https://images.example/art.jpg" || meta.ImageAlt != "Lightning Bolt artwork" {
		t.Fatalf("card share image metadata = %#v", meta)
	}
	if meta.Type != "article" {
		t.Fatalf("card share type = %q, want article", meta.Type)
	}
}

func TestKnownGameAndAwardPrintingsKeepExactDetailPaths(t *testing.T) {
	t.Parallel()

	oracleID := "123e4567-e89b-12d3-a456-426614174000"
	printingID := "223e4567-e89b-12d3-a456-426614174000"
	want := cardPrintingDetailPath(oracleID, printingID)
	card := cards.Card{OracleID: oracleID, Name: "Exact Game Card"}

	guessPage := buildGuessCardPageData(guessCardGame{
		Status:           "won",
		TargetScryfallID: printingID,
	}, card)
	if guessPage.TargetDetailPath != want {
		t.Fatalf("guess-card detail path = %q, want %q", guessPage.TargetDetailPath, want)
	}

	spellifyPage := buildSpellifyPageData(spellifyGame{
		Status:           "won",
		TargetScryfallID: printingID,
	}, card)
	if spellifyPage.TargetDetailPath != want {
		t.Fatalf("Tombscript detail path = %q, want %q", spellifyPage.TargetDetailPath, want)
	}

	guessWins := buildProfileGuessWinViews([]guessCardWin{{
		OracleID:   oracleID,
		ScryfallID: printingID,
		CardName:   "Exact Game Card",
	}})
	if len(guessWins) != 1 || guessWins[0].DetailPath != want {
		t.Fatalf("guess win detail path = %#v, want %q", guessWins, want)
	}

	spellifyAwards := buildProfileSpellifyAwardViews([]spellifyAward{{
		OracleID:   oracleID,
		ScryfallID: printingID,
		CardName:   "Exact Game Card",
	}})
	if len(spellifyAwards) != 1 || spellifyAwards[0].DetailPath != want {
		t.Fatalf("Tombscript award detail path = %#v, want %q", spellifyAwards, want)
	}
}

func TestSingleCardResultPath(t *testing.T) {
	t.Parallel()

	got := singleCardResultPath([]cards.Card{{
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Mystic Card",
	}})
	want := "/cards/view/123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("singleCardResultPath() = %q, want %q", got, want)
	}
}

func TestSingleCardResultPathMultipleResults(t *testing.T) {
	t.Parallel()

	got := singleCardResultPath([]cards.Card{
		{OracleID: "123e4567-e89b-12d3-a456-426614174000"},
		{OracleID: "223e4567-e89b-12d3-a456-426614174000"},
	})
	if got != "" {
		t.Fatalf("singleCardResultPath() = %q, want empty string", got)
	}
}

func TestCardSearchRedirectPathUsesStandardExactMatchesOnly(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Mystic Card",
	}
	want := cardDetailPath(card.OracleID)

	if got := cardSearchRedirectPath(cardResultsModeStandard, true, false, []cards.Card{card}); got != want {
		t.Fatalf("standard exact redirect = %q, want %q", got, want)
	}
	if got := cardSearchRedirectPath(cardResultsModeStandard, false, false, []cards.Card{card}); got != "" {
		t.Fatalf("standard fuzzy singleton redirect = %q, want empty", got)
	}
	if got := cardSearchRedirectPath(cardResultsModeAdvanced, true, false, []cards.Card{card}); got != "" {
		t.Fatalf("advanced exact singleton redirect = %q, want empty", got)
	}
	if got := cardSearchRedirectPath(cardResultsModeStandard, true, false, []cards.Card{
		card,
		{OracleID: "223e4567-e89b-12d3-a456-426614174000", Name: "Another Card"},
	}); got != "" {
		t.Fatalf("standard multi-result redirect = %q, want empty", got)
	}
	card.ID = "323e4567-e89b-12d3-a456-426614174000"
	wantPrinting := cardPrintingDetailPath(card.OracleID, card.ID)
	if got := cardSearchRedirectPath(cardResultsModeStandard, true, true, []cards.Card{card}); got != wantPrinting {
		t.Fatalf("standard exact matching-print redirect = %q, want %q", got, wantPrinting)
	}
}

func TestEnsureCardDetailPrintingReplacesLoadedVersion(t *testing.T) {
	t.Parallel()

	printingID := "223e4567-e89b-12d3-a456-426614174000"
	got := ensureCardDetailPrinting([]cards.Card{
		{ID: printingID, SetName: "Stale payload"},
		{ID: "323e4567-e89b-12d3-a456-426614174000", SetName: "Another set"},
	}, cards.Card{ID: printingID, SetName: "Exact payload"})

	if len(got) != 2 {
		t.Fatalf("ensureCardDetailPrinting() length = %d, want 2", len(got))
	}
	if got[0].SetName != "Exact payload" {
		t.Fatalf("ensureCardDetailPrinting() did not replace the selected printing: %#v", got[0])
	}
}

func TestEnsureCardDetailPrintingPrependsVersionOutsideGalleryLimit(t *testing.T) {
	t.Parallel()

	selected := cards.Card{
		ID:      "423e4567-e89b-12d3-a456-426614174000",
		SetName: "Very Old Set",
	}
	got := ensureCardDetailPrinting([]cards.Card{{
		ID:      "523e4567-e89b-12d3-a456-426614174000",
		SetName: "Loaded Set",
	}}, selected)

	if len(got) != 2 {
		t.Fatalf("ensureCardDetailPrinting() length = %d, want 2", len(got))
	}
	if got[0].ID != selected.ID {
		t.Fatalf("ensureCardDetailPrinting() first ID = %q, want %q", got[0].ID, selected.ID)
	}
}

func TestParseCardOracleIDFromPath(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest("GET", "/cards/view/123e4567-e89b-12d3-a456-426614174000", nil)
	got := parseCardOracleIDFromPath(r)
	want := "123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("parseCardOracleIDFromPath() = %q, want %q", got, want)
	}
}

func TestFormatCardColorNamesColorless(t *testing.T) {
	t.Parallel()

	if got := formatCardColorNames(nil); got != "Colorless" {
		t.Fatalf("formatCardColorNames(nil) = %q, want %q", got, "Colorless")
	}
}

func TestBuildCardDetailPageDataUsesFallbackPrinting(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Mystic Card",
		TypeLine:   "Instant",
		OracleText: "Draw a card.",
		CMC:        1,
	}

	got := buildCardDetailPageData(card, nil)
	if got.Card.Name != "Mystic Card" {
		t.Fatalf("buildCardDetailPageData().Card.Name = %q, want %q", got.Card.Name, "Mystic Card")
	}
	if len(got.Printings) != 1 {
		t.Fatalf("buildCardDetailPageData() printings = %d, want 1", len(got.Printings))
	}
	if got.Printings[0].SetName != "Unknown set" {
		t.Fatalf("buildCardDetailPageData() fallback set = %q, want %q", got.Printings[0].SetName, "Unknown set")
	}
}

func TestBuildCardDetailPageDataBuildsRichPrintingPayload(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Sample Card",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}.",
		FlavorText: "A quiet hum filled the vault.",
		CMC:        2,
		Faces: []cards.CardFace{{
			Name:       "Front Face",
			ManaCost:   "{2}",
			TypeLine:   "Artifact",
			OracleText: "{T}: Add {C}.",
			FlavorText: "Its song never ends.",
			ImageURI:   "https://example.com/front.jpg",
			Artist:     "Face Artist",
		}},
	}

	printings := []cards.Card{{
		Name:            "Sample Card",
		ManaCost:        "{2}",
		TypeLine:        "Artifact",
		OracleText:      "{T}: Add {C}.",
		FlavorText:      "Printed in the first vault run.",
		SetName:         "Foundations",
		SetCode:         "fdn",
		CollectorNumber: "321",
		ReleasedAt:      "2026-01-15",
		PriceUSD:        "4.25",
		Artist:          "Print Artist",
		Lang:            "en",
	}}

	got := buildCardDetailPageData(card, printings)
	if len(got.Printings) != 1 {
		t.Fatalf("buildCardDetailPageData() printings = %d, want 1", len(got.Printings))
	}

	printing := got.Printings[0]
	if printing.PriceUSD != "$4.25" {
		t.Fatalf("buildCardDetailPageData().Printings[0].PriceUSD = %q, want %q", printing.PriceUSD, "$4.25")
	}
	if printing.PriceSort != 4.25 {
		t.Fatalf("buildCardDetailPageData().Printings[0].PriceSort = %v, want %v", printing.PriceSort, 4.25)
	}
	if printing.SetCode != "FDN" {
		t.Fatalf("buildCardDetailPageData().Printings[0].SetCode = %q, want %q", printing.SetCode, "FDN")
	}
	if len(printing.Faces) != 1 {
		t.Fatalf("buildCardDetailPageData().Printings[0].Faces = %d, want 1", len(printing.Faces))
	}
	if printing.Faces[0].Name != "Front Face" {
		t.Fatalf("buildCardDetailPageData().Printings[0].Faces[0].Name = %q, want %q", printing.Faces[0].Name, "Front Face")
	}
	if got.Card.FlavorText != "A quiet hum filled the vault." {
		t.Fatalf("buildCardDetailPageData().Card.FlavorText = %q, want %q", got.Card.FlavorText, "A quiet hum filled the vault.")
	}
	if printing.FlavorText != "Printed in the first vault run." {
		t.Fatalf("buildCardDetailPageData().Printings[0].FlavorText = %q, want %q", printing.FlavorText, "Printed in the first vault run.")
	}
	if printing.Faces[0].FlavorText != "Its song never ends." {
		t.Fatalf("buildCardDetailPageData().Printings[0].Faces[0].FlavorText = %q, want %q", printing.Faces[0].FlavorText, "Its song never ends.")
	}
}

func TestBuildCardDetailPageDataDoesNotBorrowFlavorFromAnotherPrinting(t *testing.T) {
	t.Parallel()

	card := cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Fallback Card",
		TypeLine:   "Creature",
		OracleText: "Flying",
		CMC:        3,
	}

	printings := []cards.Card{
		{
			Name:       "Fallback Card",
			TypeLine:   "Creature",
			OracleText: "Flying",
			SetName:    "No Flavor Set",
			SetCode:    "nfs",
			ReleasedAt: "2026-01-01",
		},
		{
			Name:       "Fallback Card",
			TypeLine:   "Creature",
			OracleText: "Flying",
			SetName:    "Face Flavor Set",
			SetCode:    "ffs",
			ReleasedAt: "2026-02-01",
			Faces: []cards.CardFace{{
				Name:       "Front Face",
				TypeLine:   "Creature",
				OracleText: "Flying",
				FlavorText: "The sky remembers its name.",
			}},
		},
	}

	got := buildCardDetailPageData(card, printings)
	if got.Card.FlavorText != "" {
		t.Fatalf("buildCardDetailPageData().Card.FlavorText = %q, want empty for the selected printing", got.Card.FlavorText)
	}
	if got.Printings[0].FlavorText != "" {
		t.Fatalf("buildCardDetailPageData().Printings[0].FlavorText = %q, want empty string", got.Printings[0].FlavorText)
	}
	if got.Printings[1].FlavorText != "The sky remembers its name." {
		t.Fatalf("buildCardDetailPageData().Printings[1].FlavorText = %q, want %q", got.Printings[1].FlavorText, "The sky remembers its name.")
	}
}

func TestCardShowTemplateDoesNotShipFlavorFetchScript(t *testing.T) {
	page := buildCardDetailPageData(cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "No Fetch Card",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}.",
		CMC:        2,
	}, []cards.Card{{
		OracleID:        "123e4567-e89b-12d3-a456-426614174000",
		Name:            "No Fetch Card",
		TypeLine:        "Artifact",
		OracleText:      "{T}: Add {C}.",
		SetName:         "Foundations",
		SetCode:         "FDN",
		CollectorNumber: "321",
		ReleasedAt:      "2026-01-15",
	}})

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "card_show", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	if strings.Contains(body, "https://api.scryfall.com/cards/") {
		t.Fatal("rendered card_show HTML still contains a direct browser fetch to the Scryfall card API")
	}
}

func TestFormatCardLegalitiesFiltersAndOrdersDisplayedFormats(t *testing.T) {
	t.Parallel()

	got := formatCardLegalities(map[string]string{
		"standard":         "not_legal",
		"alchemy":          "legal",
		"pioneer":          "restricted",
		"historic":         "banned",
		"modern":           "legal",
		"brawl":            "legal",
		"legacy":           "legal",
		"competitivebrawl": "banned",
		"vintage":          "restricted",
		"timeless":         "legal",
		"commander":        "restricted",
		"pauper":           "legal",
		"oathbreaker":      "not_legal",
		"penny":            "legal",
		"future":           "legal",
		"tlr":              "legal",
		"frontier":         "unexpected",
	})
	want := []cardFormatLegalityData{
		{Format: "Standard", Status: "not_legal", StatusLabel: "Not legal"},
		{Format: "Alchemy", Status: "legal", StatusLabel: "Legal"},
		{Format: "Pioneer", Status: "restricted", StatusLabel: "Restricted"},
		{Format: "Historic", Status: "banned", StatusLabel: "Banned"},
		{Format: "Modern", Status: "legal", StatusLabel: "Legal"},
		{Format: "Brawl", Status: "legal", StatusLabel: "Legal"},
		{Format: "Legacy", Status: "legal", StatusLabel: "Legal"},
		{Format: "Competitive Brawl", Status: "banned", StatusLabel: "Banned"},
		{Format: "Vintage", Status: "restricted", StatusLabel: "Restricted"},
		{Format: "Timeless", Status: "legal", StatusLabel: "Legal"},
		{Format: "Commander", Status: "restricted", StatusLabel: "Restricted"},
		{Format: "Pauper", Status: "legal", StatusLabel: "Legal"},
		{Format: "Oathbreaker", Status: "not_legal", StatusLabel: "Not legal"},
		{Format: "Penny", Status: "legal", StatusLabel: "Legal"},
	}
	if len(got) != len(want) {
		t.Fatalf("formatCardLegalities() returned %d rows, want %d: %#v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("formatCardLegalities()[%d] = %#v, want %#v", index, got[index], want[index])
		}
	}

	if empty := formatCardLegalities(nil); len(empty) != 0 {
		t.Fatalf("formatCardLegalities(nil) = %#v, want empty slice", empty)
	}
}

func TestBuildCardDetailPageDataTracksPowerToughnessAvailability(t *testing.T) {
	t.Parallel()

	withoutStats := buildCardDetailPageData(cards.Card{
		Name:     "Quiet Land",
		TypeLine: "Land",
	}, nil)
	if withoutStats.Card.HasPowerToughness {
		t.Fatal("statless card unexpectedly reports power/toughness")
	}
	if len(withoutStats.Printings) != 1 || withoutStats.Printings[0].HasPowerToughness {
		t.Fatalf("statless printing unexpectedly reports power/toughness: %#v", withoutStats.Printings)
	}

	withStats := buildCardDetailPageData(cards.Card{
		Name:      "Test Creature",
		TypeLine:  "Creature",
		Power:     "2",
		Toughness: "3",
		Faces: []cards.CardFace{{
			Name:      "Test Creature",
			Power:     "2",
			Toughness: "3",
		}, {
			Name: "Statless Back",
		}},
	}, nil)
	if !withStats.Card.HasPowerToughness || !withStats.Printings[0].HasPowerToughness {
		t.Fatalf("creature card did not report power/toughness: %#v", withStats)
	}
	if len(withStats.Card.Faces) != 2 || !withStats.Card.Faces[0].HasPowerToughness {
		t.Fatalf("creature face did not report power/toughness: %#v", withStats.Card.Faces)
	}
	if withStats.Card.Faces[1].HasPowerToughness {
		t.Fatalf("statless back face unexpectedly reports power/toughness: %#v", withStats.Card.Faces)
	}
}

func TestCardShowPowerToughnessRowIsConditionalAndUnlabeled(t *testing.T) {
	rowTag := func(body string) string {
		t.Helper()
		start := strings.Index(body, `<p data-card-power-toughness`)
		if start == -1 {
			t.Fatalf("card page is missing its power/toughness row: %s", body)
		}
		end := strings.Index(body[start:], `>`)
		if end == -1 {
			t.Fatalf("card page has an unterminated power/toughness row: %s", body[start:])
		}
		return body[start : start+end+1]
	}

	statlessBody := renderTemplate(t, "card_show", TemplateData{
		Data: buildCardDetailPageData(cards.Card{
			Name:     "Quiet Land",
			TypeLine: "Land",
		}, nil),
	})
	if tag := rowTag(statlessBody); !strings.Contains(tag, ` hidden`) {
		t.Fatalf("statless card power/toughness row is visible: %s", tag)
	}

	creatureBody := renderTemplate(t, "card_show", TemplateData{
		Data: buildCardDetailPageData(cards.Card{
			Name:      "Test Creature",
			TypeLine:  "Creature",
			Power:     "*",
			Toughness: "3",
		}, nil),
	})
	if tag := rowTag(creatureBody); strings.Contains(tag, ` hidden`) {
		t.Fatalf("creature power/toughness row is hidden: %s", tag)
	}
	if !strings.Contains(creatureBody, `data-card-power>*</span>/<span data-card-toughness>3</span>`) {
		t.Fatalf("creature page did not preserve non-numeric power/toughness values: %s", creatureBody)
	}
	if strings.Contains(creatureBody, "Power/Toughness") {
		t.Fatalf("creature page still labels the power/toughness row: %s", creatureBody)
	}
}

func TestCardShowTemplateUsesPrintingGrid(t *testing.T) {
	page := buildCardDetailPageData(cards.Card{
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Grid Card",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}.",
		CMC:        2,
		Legalities: map[string]string{
			"standard":  "not_legal",
			"commander": "legal",
		},
	}, []cards.Card{{
		ID:              "223e4567-e89b-12d3-a456-426614174000",
		OracleID:        "123e4567-e89b-12d3-a456-426614174000",
		Name:            "Grid Card",
		TypeLine:        "Artifact",
		OracleText:      "{T}: Add {C}.",
		SetName:         "Foundations",
		SetCode:         "FDN",
		CollectorNumber: "321",
		ReleasedAt:      "2026-01-15",
		ImageURI:        "https://example.test/card.jpg",
		Legalities: map[string]string{
			"standard":  "not_legal",
			"commander": "legal",
		},
	}})

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "card_show", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	for _, needle := range []string{
		`data-theme="tomb"`,
		`data-page="card-show"`,
		`href="/assets/card_show.css"`,
		`class="mt-card-hero"`,
		`class="mt-card-art-shell"`,
		`class="mt-card-info"`,
		`class="mt-card-primary-group"`,
		`class="mt-card-format-legalities"`,
		`data-card-format-legalities`,
		`Format legalities`,
		`data-legality-status="legal">Legal</span>`,
		`data-legality-status="not_legal">Not legal</span>`,
		`data-card-power-toughness class="mt-card-primary-line" hidden`,
		`class="sr-only">Price: </span><span data-card-price>`,
		`var heroPower = face`,
		`? cardStatValue(face.Power)`,
		`? cardStatValue(face.Toughness)`,
		`powerToughnessRow.hidden = !hasPowerToughness`,
		`class="mt-card-printing-group"`,
		`<summary>Technical details</summary>`,
		`data-printings-grid`,
		`Printings`,
		`class="mt-printings-sort"`,
		`class="mt-printings-sort__select"`,
		`className = "mt-printing-tile" + (isCurrent ? " is-active" : "")`,
		`link.setAttribute("aria-current", "true")`,
		`mt-printing-tile__current`,
		`baseCard.ScryfallID`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("rendered card_show HTML missing %q", needle)
		}
	}
	if strings.Contains(body, `window.scrollTo(`) {
		t.Fatal("selecting a printing should not force the page to scroll")
	}
	heroIndex := strings.Index(body, `class="mt-card-hero"`)
	printingsIndex := strings.Index(body, `class="mt-printings-section"`)
	if heroIndex == -1 || printingsIndex <= heroIndex {
		t.Fatal("other printings are not rendered below the card and detail hero")
	}
	cardInfoStart := strings.Index(body, `<article class="mt-card-info">`)
	if cardInfoStart == -1 {
		t.Fatal("could not find rendered card details")
	}
	cardInfoEnd := strings.Index(body[cardInfoStart:], `</article>`)
	if cardInfoEnd == -1 {
		t.Fatal("could not isolate rendered card details")
	}
	cardInfo := body[cardInfoStart : cardInfoStart+cardInfoEnd]
	orderedDetails := []string{
		`data-card-name`,
		`data-card-mana`,
		`data-card-type`,
		`data-card-oracle`,
		`data-card-flavor-group`,
		`data-card-power`,
		`data-card-toughness`,
		`data-card-price`,
		`data-card-artist`,
		`data-card-format-legalities`,
		`class="mt-card-printing-group"`,
		`data-card-set>`,
		`data-card-set-code`,
		`data-card-collector`,
		`data-card-rarity`,
	}
	previousIndex := -1
	for _, needle := range orderedDetails {
		nextIndex := strings.Index(cardInfo, needle)
		if nextIndex <= previousIndex {
			t.Fatalf("card detail %q is missing or out of order: %s", needle, cardInfo)
		}
		previousIndex = nextIndex
	}
	if got := strings.Count(cardInfo, `data-card-price`); got != 1 {
		t.Fatalf("card details contain %d price fields, want one in the primary group: %s", got, cardInfo)
	}
	for _, forbidden := range []string{
		`data-card-favorite-login-link`,
		`data-card-scryfall-link`,
		`data-printings-english-only`,
		`Select a printing to update the card above.`,
		`match the current filters`,
		`Price —`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("guest card page unexpectedly contains %q", forbidden)
		}
	}
	if strings.Contains(body, `data-card-printing-select`) {
		t.Fatal("rendered card_show HTML still contains old printing select")
	}
	if strings.Contains(body, `data-printings-list`) {
		t.Fatal("rendered card_show HTML still contains old printing list")
	}
	if strings.Contains(body, "Show Above") {
		t.Fatal("rendered card_show HTML still contains old printing row action")
	}
	if strings.Contains(body, "Multi-face cards can be previewed") {
		t.Fatal("rendered card_show HTML still contains old multi-face helper text")
	}
	if strings.Contains(body, `data-card-current-printing`) {
		t.Fatal("rendered card_show HTML still contains redundant current printing summary")
	}
	if strings.Contains(body, "Power/Toughness") {
		t.Fatal("rendered card_show HTML still labels the power/toughness row")
	}
	if strings.Contains(body, `item.index !== state.printingIndex`) {
		t.Fatal("rendered card_show HTML still filters the active printing out of the gallery")
	}
	if !strings.Contains(body, "Turn Over") {
		t.Fatal("rendered card_show HTML did not include single turn-over control logic")
	}
	if !strings.Contains(body, "Rotate") || !strings.Contains(body, "isSplitLayout") {
		t.Fatal("rendered card_show HTML did not include split-card rotate control logic")
	}
	if !strings.Contains(body, "formatPrintingFooter") {
		t.Fatal("rendered card_show HTML did not include printing footer logic")
	}
	if strings.Contains(body, `behavior: "smooth"`) {
		t.Fatal("rendered card_show HTML still animates scrolling between printings")
	}

	templateSource, err := os.ReadFile(filepath.Join("internal", "web", "templates", "card_show.html.tmpl"))
	if err != nil {
		t.Fatalf("read card_show template: %v", err)
	}
	scriptSource, err := os.ReadFile(filepath.Join("internal", "web", "templates", "card_show_script.html.tmpl"))
	if err != nil {
		t.Fatalf("read card_show script: %v", err)
	}
	for _, forbidden := range []string{"border-slate-", "bg-slate-", "text-slate-", "text-sky-", "outline-sky-"} {
		if strings.Contains(string(templateSource), forbidden) || strings.Contains(string(scriptSource), forbidden) {
			t.Fatalf("card_show still hardcodes legacy palette class %q", forbidden)
		}
	}
	for _, forbidden := range []string{
		`favoriteLoginLink`,
		`scryfallLink`,
		`printingsEnglishOnly`,
		`data-card-favorite-login-link`,
		`data-card-scryfall-link`,
		`data-printings-english-only`,
		`data-card-commander`,
		`mt-card-detail-columns`,
		`mt-printings-language`,
	} {
		if strings.Contains(string(templateSource), forbidden) || strings.Contains(string(scriptSource), forbidden) {
			t.Fatalf("card_show still contains removed UI hook %q", forbidden)
		}
	}

	cssSource, err := os.ReadFile(filepath.Join("internal", "web", "assets", "card_show.css"))
	if err != nil {
		t.Fatalf("read card_show stylesheet: %v", err)
	}
	for _, needle := range []string{
		`grid-template-columns: minmax(0, 20.5rem) minmax(0, 1fr);`,
		`border-top: 1px solid var(--mt-border-subtle);`,
		`color: var(--mt-text);`,
		`background: var(--mt-surface);`,
		`.mt-card-primary-line {`,
		`font-size: 1rem;`,
		`.mt-card-primary-group {`,
		`gap: 0.8rem;`,
		`.mt-card-primary-head h1 {`,
		`.mt-card-format-legalities__list {`,
		`grid-template-columns: repeat(2, minmax(0, 1fr));`,
		`gap: 0.12rem clamp(0.7rem, 2vw, 1.15rem);`,
		`.mt-card-format-legalities__list li {`,
		`font-size: 0.78rem;`,
		`.mt-card-format-legalities__status {`,
		`.mt-card-format-legalities__status[data-legality-status="not_legal"] {`,
		`color: var(--mt-negative);`,
		`.mt-card-printing-group {`,
		`border-top: 1px solid var(--mt-border-strong);`,
		`.mt-card-printing-facts > div + div {`,
		`.mt-printings-sort {`,
		`border: 1px solid var(--mt-border-control);`,
		`background: var(--mt-surface-strong);`,
		`.mt-printings-sort__select {`,
		`.mt-printing-tile.is-active .mt-printing-tile__media {`,
		`border-color: var(--mt-accent-strong);`,
		`@media (min-width: 960px)`,
		`grid-template-columns: repeat(3, minmax(0, 1fr));`,
		`@media (max-width: 699px)`,
		`.mt-card-primary-head {`,
		`flex-direction: column;`,
	} {
		if !strings.Contains(string(cssSource), needle) {
			t.Fatalf("card_show stylesheet missing %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "border-slate-", "bg-slate-", "text-sky-"} {
		if strings.Contains(string(cssSource), forbidden) {
			t.Fatalf("card_show stylesheet hardcodes palette value %q", forbidden)
		}
	}
	for _, forbidden := range []string{
		`gap: 0.38rem clamp(1rem, 3vw, 2rem);`,
		`grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));`,
		`.mt-card-primary-group > * + * {`,
	} {
		if strings.Contains(string(cssSource), forbidden) {
			t.Fatalf("card_show stylesheet still contains loose legality layout %q", forbidden)
		}
	}
	for _, forbidden := range []string{".mt-card-detail-columns", ".mt-card-stat-line", ".mt-printings-language"} {
		if strings.Contains(string(cssSource), forbidden) {
			t.Fatalf("card_show stylesheet still contains removed selector %q", forbidden)
		}
	}
}

func TestCardShowTemplateKeepsFavoriteActionSignedInOnly(t *testing.T) {
	page := buildCardDetailPageData(cards.Card{
		ID:         "223e4567-e89b-12d3-a456-426614174000",
		OracleID:   "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Loved Card",
		TypeLine:   "Artifact",
		OracleText: "{T}: Add {C}.",
	}, nil)
	body := renderTemplate(t, "card_show", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Collector"},
		Data:        page,
	})

	start := strings.Index(body, `<article class="mt-card-info">`)
	if start == -1 {
		t.Fatalf("could not find signed-in card details: %s", body)
	}
	end := strings.Index(body[start:], `</article>`)
	if end == -1 {
		t.Fatalf("could not isolate signed-in card details: %s", body)
	}
	cardInfo := body[start : start+end]
	for _, needle := range []string{
		`data-card-favorite-form`,
		`data-card-favorite-button`,
		`data-card-favorite-symbol`,
		`data-open-deck-picker`,
	} {
		if !strings.Contains(cardInfo, needle) {
			t.Fatalf("signed-in card actions missing %q: %s", needle, cardInfo)
		}
	}
	if strings.Contains(cardInfo, `data-card-add-login-link`) {
		t.Fatalf("signed-in card actions contain guest Add To Deck link: %s", cardInfo)
	}
}

func TestParseCardSearchRequestAdvancedFiltersMarkSearched(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"type_value": []string{"Instant"},
		"type_mode":  []string{"is"},
		"color":      []string{"U"},
		"set":        []string{"bro"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if !req.HasSearched {
		t.Fatalf("parseCardSearchRequest().HasSearched = false, want true")
	}
	if req.SetQuery != "bro" {
		t.Fatalf("parseCardSearchRequest().SetQuery = %q, want %q", req.SetQuery, "bro")
	}
	if len(req.TypeFilters) != 1 || req.TypeFilters[0].Value != "Instant" {
		t.Fatalf("parseCardSearchRequest().TypeFilters = %#v, want Instant filter", req.TypeFilters)
	}
}

func TestParseCardSearchRequestSupportsAdvancedOptions(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"q":               []string{"Atraxa"},
		"name_exact":      []string{"1"},
		"mana_cost":       []string{"{W}{W}"},
		"text":            []string{"draw"},
		"type_value":      []string{"Angel"},
		"type_mode":       []string{"is"},
		"type_partial":    []string{"1"},
		"stat":            []string{"power", "mana_value"},
		"stat_op":         []string{"gte", "lte"},
		"stat_value":      []string{"2", "5"},
		"rarity":          []string{"rare", "mythic", "rare", "invalid"},
		"price_op":        []string{"gte", "lte"},
		"price_value":     []string{"1.00", "9.99"},
		"commander_legal": []string{"1"},
		"include_tokens":  []string{"1"},
		"sort":            []string{"mana_value"},
		"sort_dir":        []string{"desc"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if !req.HasSearched {
		t.Fatalf("parseCardSearchRequest().HasSearched = false, want true")
	}
	if !req.NameExact {
		t.Fatalf("parseCardSearchRequest().NameExact = false, want true")
	}
	if req.ManaCostQuery != "{W}{W}" {
		t.Fatalf("parseCardSearchRequest().ManaCostQuery = %q, want %q", req.ManaCostQuery, "{W}{W}")
	}
	if len(req.TypeFilters) != 1 || req.TypeFilters[0].Value != "Angel" {
		t.Fatalf("parseCardSearchRequest().TypeFilters = %#v, want Angel filter", req.TypeFilters)
	}
	if !req.TypePartial {
		t.Fatalf("parseCardSearchRequest().TypePartial = false, want true")
	}
	if req.Stat != "power" {
		t.Fatalf("parseCardSearchRequest().Stat = %q, want %q", req.Stat, "power")
	}
	if req.StatOperator != "gte" {
		t.Fatalf("parseCardSearchRequest().StatOperator = %q, want %q", req.StatOperator, "gte")
	}
	if req.StatValue == nil || *req.StatValue != 2 {
		t.Fatalf("parseCardSearchRequest().StatValue = %#v, want 2", req.StatValue)
	}
	if len(req.StatFilters) != 2 {
		t.Fatalf("parseCardSearchRequest().StatFilters = %#v, want two filters", req.StatFilters)
	}
	if req.StatFilters[1].Stat != "mana_value" || req.StatFilters[1].Operator != "lte" ||
		req.StatFilters[1].Value == nil || *req.StatFilters[1].Value != 5 {
		t.Fatalf("parseCardSearchRequest().StatFilters[1] = %#v, want mana value <= 5", req.StatFilters[1])
	}
	if got := req.Rarities; len(got) != 2 || got[0] != "rare" || got[1] != "mythic" {
		t.Fatalf("parseCardSearchRequest().Rarities = %#v, want [rare mythic]", got)
	}
	if req.PriceOperator != "gte" {
		t.Fatalf("parseCardSearchRequest().PriceOperator = %q, want %q", req.PriceOperator, "gte")
	}
	if req.PriceValue == nil || *req.PriceValue != 1 {
		t.Fatalf("parseCardSearchRequest().PriceValue = %#v, want 1", req.PriceValue)
	}
	if len(req.PriceFilters) != 2 || req.PriceFilters[1].Operator != "lte" ||
		req.PriceFilters[1].Value == nil || *req.PriceFilters[1].Value != 9.99 {
		t.Fatalf("parseCardSearchRequest().PriceFilters = %#v, want price >= 1 and price <= 9.99", req.PriceFilters)
	}
	if !req.CommanderLegal {
		t.Fatalf("parseCardSearchRequest().CommanderLegal = false, want true")
	}
	if !req.IncludeTokens {
		t.Fatalf("parseCardSearchRequest().IncludeTokens = false, want true")
	}
	if req.Sort != "mana_value" {
		t.Fatalf("parseCardSearchRequest().Sort = %q, want %q", req.Sort, "mana_value")
	}
	if req.SortDirection != "desc" {
		t.Fatalf("parseCardSearchRequest().SortDirection = %q, want %q", req.SortDirection, "desc")
	}
	if !req.SortDirectionExplicit {
		t.Fatalf("parseCardSearchRequest().SortDirectionExplicit = false, want true")
	}
}

func TestCardSearchQueryValuesRoundTripMultipleStatsPricesAndRarities(t *testing.T) {
	t.Parallel()

	powerValue := 2
	manaValue := 5
	minPrice := 1.0
	maxPrice := 25.0
	req := cardSearchRequest{
		StatFilters: []cardSearchStatFilter{
			{Stat: "power", Operator: "gte", Value: &powerValue, ValueRaw: "2"},
			{Stat: "mana_value", Operator: "lte", Value: &manaValue, ValueRaw: "5"},
		},
		PriceFilters: []cardSearchPriceFilter{
			{Operator: "gte", Value: &minPrice, ValueRaw: "1.00"},
			{Operator: "lte", Value: &maxPrice, ValueRaw: "25.00"},
		},
		Rarities: []string{"rare", "mythic"},
		Sort:     "relevance",
	}

	values := cardSearchQueryValues(req)
	for key, want := range map[string][]string{
		"stat":        {"power", "mana_value"},
		"stat_op":     {"gte", "lte"},
		"stat_value":  {"2", "5"},
		"price_op":    {"gte", "lte"},
		"price_value": {"1.00", "25.00"},
		"rarity":      {"rare", "mythic"},
	} {
		got := values[key]
		if len(got) != len(want) {
			t.Fatalf("cardSearchQueryValues()[%q] = %#v, want %#v", key, got, want)
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("cardSearchQueryValues()[%q][%d] = %q, want %q", key, idx, got[idx], want[idx])
			}
		}
	}

	roundTripped, errMsg := parseCardSearchRequest(values)
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest(round trip) errMsg = %q, want empty", errMsg)
	}
	if len(roundTripped.StatFilters) != 2 || len(roundTripped.PriceFilters) != 2 || len(roundTripped.Rarities) != 2 {
		t.Fatalf("parseCardSearchRequest(round trip) = %#v, want two stats, prices, and rarities", roundTripped)
	}

	params := roundTripped.searchParams(120)
	if len(params.StatFilters) != 2 || params.StatFilters[1].Stat != "mana_value" || params.StatFilters[1].Value != 5 {
		t.Fatalf("cardSearchRequest.searchParams().StatFilters = %#v, want both stat filters", params.StatFilters)
	}
	if len(params.PriceFilters) != 2 || params.PriceFilters[0].Value != 1 || params.PriceFilters[1].Value != 25 {
		t.Fatalf("cardSearchRequest.searchParams().PriceFilters = %#v, want both price filters", params.PriceFilters)
	}
	if len(params.Rarities) != 2 || params.Rarities[0] != "rare" || params.Rarities[1] != "mythic" {
		t.Fatalf("cardSearchRequest.searchParams().Rarities = %#v, want [rare mythic]", params.Rarities)
	}
}

func TestCardResultsModePersistsThroughCanonicalSearchNavigation(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		NameQuery:   "Atraxa",
		Sort:        "relevance",
		HasSearched: true,
		ResultsMode: cardResultsModeAdvanced,
	}

	values := cardSearchQueryValues(req)
	if got := values.Get("search_mode"); got != string(cardResultsModeAdvanced) {
		t.Fatalf("cardSearchQueryValues().search_mode = %q, want %q", got, cardResultsModeAdvanced)
	}
	roundTripped, errMsg := parseCardSearchRequest(values)
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest(round trip) errMsg = %q, want empty", errMsg)
	}
	if roundTripped.ResultsMode != cardResultsModeAdvanced {
		t.Fatalf("parseCardSearchRequest(round trip).ResultsMode = %q, want %q", roundTripped.ResultsMode, cardResultsModeAdvanced)
	}

	for label, rawPath := range map[string]string{
		"results": cardSearchPath(req),
		"editor":  cardSearchEditPath(req),
	} {
		parsed, err := url.Parse(rawPath)
		if err != nil {
			t.Fatalf("url.Parse(%s path %q): %v", label, rawPath, err)
		}
		if got := parsed.Query().Get("search_mode"); got != string(cardResultsModeAdvanced) {
			t.Fatalf("%s path search_mode = %q, want %q", label, got, cardResultsModeAdvanced)
		}
	}

	fields := cardSearchQueryFields(req, "sort", "sort_dir")
	foundMode := false
	for _, field := range fields {
		if field.Name == "search_mode" && field.Value == string(cardResultsModeAdvanced) {
			foundMode = true
			break
		}
	}
	if !foundMode {
		t.Fatalf("cardSearchQueryFields() = %#v, want advanced results mode", fields)
	}

	chips := buildCardSearchFilterChips(req)
	if len(chips) == 0 {
		t.Fatal("buildCardSearchFilterChips() returned no name chip")
	}
	chipPath, err := url.Parse(chips[0].RemovePath)
	if err != nil {
		t.Fatalf("url.Parse(remove path %q): %v", chips[0].RemovePath, err)
	}
	if got := chipPath.Query().Get("search_mode"); got != string(cardResultsModeAdvanced) {
		t.Fatalf("filter removal search_mode = %q, want %q", got, cardResultsModeAdvanced)
	}

	standardValues := cardSearchQueryValues(cardSearchRequest{
		NameQuery:   "Atraxa",
		Sort:        "relevance",
		ResultsMode: cardResultsModeStandard,
	})
	if got := standardValues.Get("search_mode"); got != "" {
		t.Fatalf("standard search_mode = %q, want canonical omission", got)
	}
	if got := normalizeCardResultsMode("unexpected"); got != cardResultsModeStandard {
		t.Fatalf("normalizeCardResultsMode(unexpected) = %q, want %q", got, cardResultsModeStandard)
	}
}

func TestCardSearchPagePersistsForPagingAndResetsForSearchChanges(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		NameQuery:   "Angel",
		TypeFilters: []cardSearchTypeFilter{{Value: "Creature", Mode: "is"}},
		Sort:        "mana_value",
		ResultsMode: cardResultsModeAdvanced,
		Page:        3,
	}

	values := cardSearchQueryValues(req)
	if got := values.Get("page"); got != "3" {
		t.Fatalf("cardSearchQueryValues().page = %q, want 3", got)
	}
	roundTripped, errMsg := parseCardSearchRequest(values)
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest(round trip) errMsg = %q, want empty", errMsg)
	}
	if roundTripped.Page != 3 {
		t.Fatalf("parseCardSearchRequest(round trip).Page = %d, want 3", roundTripped.Page)
	}

	nextURL, err := url.Parse(cardSearchPagePath(req, 4))
	if err != nil {
		t.Fatalf("url.Parse(next page): %v", err)
	}
	nextQuery := nextURL.Query()
	if got := nextQuery.Get("page"); got != "4" {
		t.Fatalf("next page query = %q, want 4", got)
	}
	if got := nextQuery.Get("search_mode"); got != string(cardResultsModeAdvanced) {
		t.Fatalf("next page search_mode = %q, want %q", got, cardResultsModeAdvanced)
	}
	if got := nextQuery["type_value"]; len(got) != 1 || got[0] != "Creature" {
		t.Fatalf("next page type filters = %#v, want [Creature]", got)
	}

	firstPageURL, err := url.Parse(cardSearchPagePath(req, 1))
	if err != nil {
		t.Fatalf("url.Parse(first page): %v", err)
	}
	if got := firstPageURL.Query().Get("page"); got != "" {
		t.Fatalf("first page query = %q, want canonical omission", got)
	}

	sortFields := cardSearchQueryFields(req, "sort", "sort_dir", "page")
	for _, field := range sortFields {
		if field.Name == "page" {
			t.Fatalf("sort fields retained page: %#v", sortFields)
		}
	}

	chips := buildCardSearchFilterChips(req)
	if len(chips) == 0 {
		t.Fatal("buildCardSearchFilterChips() returned no removable filters")
	}
	for _, chip := range chips {
		removeURL, parseErr := url.Parse(chip.RemovePath)
		if parseErr != nil {
			t.Fatalf("url.Parse(remove path %q): %v", chip.RemovePath, parseErr)
		}
		if got := removeURL.Query().Get("page"); got != "" {
			t.Fatalf("filter removal %q retained page %q, want reset", chip.Label, got)
		}
	}

	for _, raw := range []string{"", "0", "-1", "invalid"} {
		parsed, parseErrMsg := parseCardSearchRequest(url.Values{"page": {raw}})
		if parseErrMsg != "" {
			t.Fatalf("parseCardSearchRequest(page=%q) errMsg = %q, want empty", raw, parseErrMsg)
		}
		if parsed.Page != 1 {
			t.Fatalf("parseCardSearchRequest(page=%q).Page = %d, want 1", raw, parsed.Page)
		}
	}
}

func TestParseCardSearchRequestPriceRowsValidateAndDefaultOperators(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"price_value": {"0", "9.99"},
		"price_op":    {"gte"},
		"price_min":   {"50"},
		"price_max":   {"100"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if len(req.PriceFilters) != 2 || req.PriceFilters[0].Value == nil || *req.PriceFilters[0].Value != 0 {
		t.Fatalf("parseCardSearchRequest().PriceFilters = %#v, want zero-valued first row", req.PriceFilters)
	}
	if req.PriceFilters[1].Operator != "eq" {
		t.Fatalf("parseCardSearchRequest().PriceFilters[1].Operator = %q, want eq", req.PriceFilters[1].Operator)
	}
	if req.PriceMinRaw != "" || req.PriceMaxRaw != "" {
		t.Fatalf("parseCardSearchRequest() retained ignored legacy price ranges: min=%q max=%q", req.PriceMinRaw, req.PriceMaxRaw)
	}

	for _, raw := range []string{"NaN", "Inf", "-Inf"} {
		invalid, invalidErr := parseCardSearchRequest(url.Values{"price_value": {raw}})
		if invalidErr == "" {
			t.Fatalf("parseCardSearchRequest(price_value=%q) errMsg is empty, want validation error", raw)
		}
		if len(invalid.PriceFilters) != 1 || invalid.PriceFilters[0].ValueRaw != raw {
			t.Fatalf("parseCardSearchRequest(price_value=%q) did not preserve invalid row: %#v", raw, invalid.PriceFilters)
		}
	}
}

func TestParseCardSearchRequestDetectsImpossibleRepeatedRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query url.Values
	}{
		{
			name: "stat bounds cross",
			query: url.Values{
				"stat":       {"power", "power"},
				"stat_op":    {"gte", "lte"},
				"stat_value": {"5", "3"},
			},
		},
		{
			name: "strict price bounds meet",
			query: url.Values{
				"price_op":    {"gt", "lte"},
				"price_value": {"10", "10"},
			},
		},
		{
			name: "only exact value excluded",
			query: url.Values{
				"price_op":    {"eq", "neq"},
				"price_value": {"10", "10"},
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, errMsg := parseCardSearchRequest(tc.query)
			if errMsg != "Impossible range." {
				t.Fatalf("parseCardSearchRequest() errMsg = %q, want Impossible range.", errMsg)
			}
		})
	}
}

func TestParseCardSearchRequestAllowsCompatibleAndSeparateRanges(t *testing.T) {
	t.Parallel()

	_, errMsg := parseCardSearchRequest(url.Values{
		"stat":        {"power", "power", "toughness", "toughness"},
		"stat_op":     {"gte", "lte", "gte", "lte"},
		"stat_value":  {"3", "5", "8", "10"},
		"price_op":    {"gte", "lte"},
		"price_value": {"10", "10"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
}

func TestParseCardSearchRequestSupportsMultipleTypeModes(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"type_value": []string{"Artifact", "Creature", "Artifact"},
		"type_mode":  []string{"is", "not", "not"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if len(req.TypeFilters) != 2 {
		t.Fatalf("parseCardSearchRequest().TypeFilters length = %d, want 2", len(req.TypeFilters))
	}
	if req.TypeFilters[0].Value != "Artifact" || req.TypeFilters[0].Mode != "not" {
		t.Fatalf("parseCardSearchRequest().TypeFilters[0] = %#v, want Artifact/not", req.TypeFilters[0])
	}
	if req.TypeFilters[1].Value != "Creature" || req.TypeFilters[1].Mode != "not" {
		t.Fatalf("parseCardSearchRequest().TypeFilters[1] = %#v, want Creature/not", req.TypeFilters[1])
	}
}

func TestParseCardSearchRequestInvalidSortDefaultsToRelevance(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"sort": []string{"mystery"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if req.Sort != "relevance" {
		t.Fatalf("parseCardSearchRequest().Sort = %q, want %q", req.Sort, "relevance")
	}
	if req.SortDirection != "asc" {
		t.Fatalf("parseCardSearchRequest().SortDirection = %q, want %q", req.SortDirection, "asc")
	}
	if req.SortDirectionExplicit {
		t.Fatalf("parseCardSearchRequest().SortDirectionExplicit = true, want false")
	}
}

func TestParseCardSearchRequestRelevanceQueryDefaultsDescendingDirection(t *testing.T) {
	t.Parallel()

	req, errMsg := parseCardSearchRequest(url.Values{
		"q":    []string{"Atraxa"},
		"sort": []string{"relevance"},
	})
	if errMsg != "" {
		t.Fatalf("parseCardSearchRequest() errMsg = %q, want empty", errMsg)
	}
	if req.SortDirection != "desc" {
		t.Fatalf("parseCardSearchRequest().SortDirection = %q, want %q", req.SortDirection, "desc")
	}
	if req.SortDirectionExplicit {
		t.Fatalf("parseCardSearchRequest().SortDirectionExplicit = true, want false")
	}
}

func TestCardSearchQueryValuesIncludeSort(t *testing.T) {
	t.Parallel()

	values := cardSearchQueryValues(cardSearchRequest{
		NameQuery:             "Atraxa",
		Sort:                  "mana_value",
		SortDirection:         "desc",
		SortDirectionExplicit: true,
	})
	if got := values.Get("q"); got != "Atraxa" {
		t.Fatalf("cardSearchQueryValues() q = %q, want %q", got, "Atraxa")
	}
	if got := values.Get("sort"); got != "mana_value" {
		t.Fatalf("cardSearchQueryValues() sort = %q, want %q", got, "mana_value")
	}
	if got := values.Get("sort_dir"); got != "desc" {
		t.Fatalf("cardSearchQueryValues() sort_dir = %q, want %q", got, "desc")
	}
}

func TestCardSearchQueryValuesOmitImplicitSortDirection(t *testing.T) {
	t.Parallel()

	values := cardSearchQueryValues(cardSearchRequest{
		NameQuery:     "Atraxa",
		Sort:          "relevance",
		SortDirection: "desc",
		HasSearched:   true,
	})
	if got := values.Get("sort"); got != "relevance" {
		t.Fatalf("cardSearchQueryValues() sort = %q, want %q", got, "relevance")
	}
	if got := values.Get("sort_dir"); got != "" {
		t.Fatalf("cardSearchQueryValues() sort_dir = %q, want empty", got)
	}
}

func TestBuildSearchResultsIncludesDeckbuilderMetadata(t *testing.T) {
	t.Parallel()

	results := buildSearchResults([]cards.Card{{
		OracleID:             "123e4567-e89b-12d3-a456-426614174000",
		Name:                 "Atraxa, Grand Unifier",
		ManaCost:             "{3}{G}{W}{U}{B}",
		TypeLine:             "Legendary Creature — Phyrexian Angel",
		OracleText:           "Flying, vigilance, deathtouch, lifelink",
		ImageURI:             "https://example.com/atraxa.jpg",
		ColorIdentity:        []string{"U", "W", "B", "G"},
		CMC:                  7,
		IsCommanderCandidate: true,
	}})

	if len(results) != 1 {
		t.Fatalf("buildSearchResults() len = %d, want 1", len(results))
	}

	got := results[0]
	if got.ColorIdentity != "White, Blue, Black, Green" {
		t.Fatalf("buildSearchResults().ColorIdentity = %q, want %q", got.ColorIdentity, "White, Blue, Black, Green")
	}
	if got.ManaValue != "7" {
		t.Fatalf("buildSearchResults().ManaValue = %q, want %q", got.ManaValue, "7")
	}
	if !got.IsCommanderCandidate {
		t.Fatalf("buildSearchResults().IsCommanderCandidate = false, want true")
	}
}

func TestBuildSearchResultsMarksSplitLayouts(t *testing.T) {
	t.Parallel()

	results := buildSearchResults([]cards.Card{{
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Greenhouse // Rickety Gazebo",
		ImageURI: "https://example.com/room.jpg",
		Layout:   "split",
		PriceUSD: "0.25",
		SetCode:  "dsk",
		Faces: []cards.CardFace{
			{Name: "Greenhouse"},
			{Name: "Rickety Gazebo"},
		},
	}})

	if len(results) != 1 {
		t.Fatalf("buildSearchResults() len = %d, want 1", len(results))
	}

	got := results[0]
	if !got.IsSplitLayout {
		t.Fatal("buildSearchResults().IsSplitLayout = false, want true")
	}
	if got.Layout != "split" {
		t.Fatalf("buildSearchResults().Layout = %q, want split", got.Layout)
	}
	if got.PriceUSD != "$0.25" {
		t.Fatalf("buildSearchResults().PriceUSD = %q, want $0.25", got.PriceUSD)
	}
	if got.SetCode != "DSK" {
		t.Fatalf("buildSearchResults().SetCode = %q, want DSK", got.SetCode)
	}
	if got.FacesJSON == "" {
		t.Fatal("buildSearchResults().FacesJSON is empty, want encoded split faces")
	}
}

func TestBuildCardSearchFilterChipsPreserveOtherFilters(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		NameQuery:     "Atraxa",
		TypeFilters:   []cardSearchTypeFilter{{Value: "Creature", Mode: "is"}},
		ColorParams:   []string{"W", "U"},
		ColorMode:     "exact",
		Stat:          "mana_value",
		StatOperator:  "gte",
		StatValueRaw:  "3",
		SetQuery:      "bro",
		CommanderOnly: true,
		HasSearched:   true,
	}

	chips := buildCardSearchFilterChips(req)
	chipMap := map[string]string{}
	for _, chip := range chips {
		chipMap[chip.Label] = chip.RemovePath
	}

	removeType, ok := chipMap["Type: Creature"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing type chip: %#v", chips)
	}
	if parsed, err := url.Parse(removeType); err != nil {
		t.Fatalf("url.Parse(%q) error = %v", removeType, err)
	} else if parsed.Path != "/cards" {
		t.Fatalf("type remove path = %q, want %q", parsed.Path, "/cards")
	}
	typeQuery := mustParseQuery(t, removeType)
	if got := typeQuery.Get("q"); got != "Atraxa" {
		t.Fatalf("type remove query q = %q, want %q", got, "Atraxa")
	}
	if got := typeQuery.Get("set"); got != "bro" {
		t.Fatalf("type remove query set = %q, want %q", got, "bro")
	}
	if got := typeQuery.Get("stat"); got != "mana_value" {
		t.Fatalf("type remove query stat = %q, want %q", got, "mana_value")
	}
	if got := typeQuery.Get("stat_op"); got != "gte" {
		t.Fatalf("type remove query stat_op = %q, want %q", got, "gte")
	}
	if got := typeQuery.Get("stat_value"); got != "3" {
		t.Fatalf("type remove query stat_value = %q, want %q", got, "3")
	}
	if got := typeQuery.Get("commander"); got != "1" {
		t.Fatalf("type remove query commander = %q, want %q", got, "1")
	}
	if got := typeQuery["type_value"]; len(got) != 0 {
		t.Fatalf("type remove query type_value = %#v, want empty", got)
	}

	removeWhite, ok := chipMap["Color: White"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing white chip: %#v", chips)
	}
	colorQuery := mustParseQuery(t, removeWhite)
	if got := colorQuery["color"]; len(got) != 1 || got[0] != "U" {
		t.Fatalf("white remove query color = %#v, want [U]", got)
	}
	if got := colorQuery.Get("color_mode"); got != "exact" {
		t.Fatalf("white remove query color_mode = %q, want %q", got, "exact")
	}
}

func TestBuildCardSearchFilterChipsRemoveOneStatPriceOrRarityKeepsSiblings(t *testing.T) {
	t.Parallel()

	powerValue := 2
	manaValue := 5
	minPrice := 1.0
	maxPrice := 25.0
	req := cardSearchRequest{
		StatFilters: []cardSearchStatFilter{
			{Stat: "power", Operator: "gte", Value: &powerValue, ValueRaw: "2"},
			{Stat: "mana_value", Operator: "lte", Value: &manaValue, ValueRaw: "5"},
		},
		PriceFilters: []cardSearchPriceFilter{
			{Operator: "gte", Value: &minPrice, ValueRaw: "1.00"},
			{Operator: "lte", Value: &maxPrice, ValueRaw: "25.00"},
		},
		Rarities:    []string{"rare", "mythic"},
		Rarity:      "rare",
		Sort:        "rarity",
		HasSearched: true,
	}

	chipMap := map[string]string{}
	for _, chip := range buildCardSearchFilterChips(req) {
		chipMap[chip.Label] = chip.RemovePath
	}

	powerQuery := mustParseQuery(t, chipMap["Power >= 2"])
	if got := powerQuery["stat"]; len(got) != 1 || got[0] != "mana_value" {
		t.Fatalf("power removal stat query = %#v, want [mana_value]", got)
	}
	if got := powerQuery["stat_value"]; len(got) != 1 || got[0] != "5" {
		t.Fatalf("power removal stat values = %#v, want [5]", got)
	}
	if got := powerQuery["rarity"]; len(got) != 2 || got[0] != "rare" || got[1] != "mythic" {
		t.Fatalf("power removal rarity query = %#v, want [rare mythic]", got)
	}
	if got := powerQuery["price_value"]; len(got) != 2 || got[0] != "1.00" || got[1] != "25.00" {
		t.Fatalf("power removal price values = %#v, want [1.00 25.00]", got)
	}

	minPriceQuery := mustParseQuery(t, chipMap["Price >= $1.00"])
	if got := minPriceQuery["price_value"]; len(got) != 1 || got[0] != "25.00" {
		t.Fatalf("minimum price removal values = %#v, want [25.00]", got)
	}
	if got := minPriceQuery["price_op"]; len(got) != 1 || got[0] != "lte" {
		t.Fatalf("minimum price removal operators = %#v, want [lte]", got)
	}
	if got := minPriceQuery["stat_value"]; len(got) != 2 || got[0] != "2" || got[1] != "5" {
		t.Fatalf("minimum price removal stat values = %#v, want [2 5]", got)
	}

	rareQuery := mustParseQuery(t, chipMap["Rarity: Rare"])
	if got := rareQuery["rarity"]; len(got) != 1 || got[0] != "mythic" {
		t.Fatalf("rare removal rarity query = %#v, want [mythic]", got)
	}
	if got := rareQuery["stat"]; len(got) != 2 || got[0] != "power" || got[1] != "mana_value" {
		t.Fatalf("rare removal stat query = %#v, want [power mana_value]", got)
	}
}

func TestBuildCardSearchFilterChipsSupportAdvancedFilterRemovals(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		NameQuery:             "Atraxa",
		NameExact:             true,
		ManaCostQuery:         "{W}{W}",
		TextQuery:             "draw",
		TypeFilters:           []cardSearchTypeFilter{{Value: "Angel", Mode: "is"}},
		TypePartial:           true,
		Stat:                  "power",
		StatOperator:          "gte",
		StatValueRaw:          "2",
		PriceOperator:         "lte",
		PriceValueRaw:         "1.25",
		CommanderLegal:        true,
		IncludeTokens:         true,
		Sort:                  "rarity",
		SortDirection:         "desc",
		SortDirectionExplicit: true,
		HasSearched:           true,
	}

	chips := buildCardSearchFilterChips(req)
	chipMap := map[string]string{}
	for _, chip := range chips {
		chipMap[chip.Label] = chip.RemovePath
	}

	exactPath, ok := chipMap["Exact name match"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing exact-name chip: %#v", chips)
	}
	exactQuery := mustParseQuery(t, exactPath)
	if got := exactQuery.Get("name_exact"); got != "" {
		t.Fatalf("exact remove query name_exact = %q, want empty", got)
	}
	if got := exactQuery.Get("q"); got != "Atraxa" {
		t.Fatalf("exact remove query q = %q, want %q", got, "Atraxa")
	}
	if got := exactQuery.Get("sort"); got != "rarity" {
		t.Fatalf("exact remove query sort = %q, want %q", got, "rarity")
	}
	if got := exactQuery.Get("sort_dir"); got != "desc" {
		t.Fatalf("exact remove query sort_dir = %q, want %q", got, "desc")
	}

	manaCostPath, ok := chipMap["Mana Cost: {W}{W}"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing mana-cost chip: %#v", chips)
	}
	manaCostQuery := mustParseQuery(t, manaCostPath)
	if got := manaCostQuery.Get("mana_cost"); got != "" {
		t.Fatalf("mana-cost remove query mana_cost = %q, want empty", got)
	}
	if got := manaCostQuery.Get("commander_legal"); got != "1" {
		t.Fatalf("mana-cost remove query commander_legal = %q, want %q", got, "1")
	}

	typePartialPath, ok := chipMap["Allow partial type matches"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing type-partial chip: %#v", chips)
	}
	typePartialQuery := mustParseQuery(t, typePartialPath)
	if got := typePartialQuery.Get("type_partial"); got != "" {
		t.Fatalf("type-partial remove query type_partial = %q, want empty", got)
	}
	if got := typePartialQuery.Get("type_value"); got != "Angel" {
		t.Fatalf("type-partial remove query type_value = %q, want %q", got, "Angel")
	}

	pricePath, ok := chipMap["Price <= $1.25"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing price chip: %#v", chips)
	}
	priceQuery := mustParseQuery(t, pricePath)
	if got := priceQuery.Get("price_value"); got != "" {
		t.Fatalf("price remove query price_value = %q, want empty", got)
	}
	if got := priceQuery.Get("price_op"); got != "" {
		t.Fatalf("price remove query price_op = %q, want empty", got)
	}
	if got := priceQuery.Get("stat_value"); got != "2" {
		t.Fatalf("price remove query stat_value = %q, want %q", got, "2")
	}
	if got := priceQuery.Get("sort"); got != "rarity" {
		t.Fatalf("price remove query sort = %q, want %q", got, "rarity")
	}
	if got := priceQuery.Get("sort_dir"); got != "desc" {
		t.Fatalf("price remove query sort_dir = %q, want %q", got, "desc")
	}

	commanderLegalPath, ok := chipMap["Commander legal"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing commander-legal chip: %#v", chips)
	}
	commanderLegalQuery := mustParseQuery(t, commanderLegalPath)
	if got := commanderLegalQuery.Get("commander_legal"); got != "" {
		t.Fatalf("commander-legal remove query commander_legal = %q, want empty", got)
	}
	if got := commanderLegalQuery.Get("include_tokens"); got != "1" {
		t.Fatalf("commander-legal remove query include_tokens = %q, want %q", got, "1")
	}
	if got := commanderLegalQuery.Get("stat"); got != "power" {
		t.Fatalf("commander-legal remove query stat = %q, want %q", got, "power")
	}
	if got := commanderLegalQuery.Get("stat_op"); got != "gte" {
		t.Fatalf("commander-legal remove query stat_op = %q, want %q", got, "gte")
	}
	if got := commanderLegalQuery.Get("stat_value"); got != "2" {
		t.Fatalf("commander-legal remove query stat_value = %q, want %q", got, "2")
	}
	if got := commanderLegalQuery.Get("price_op"); got != "lte" {
		t.Fatalf("commander-legal remove query price_op = %q, want %q", got, "lte")
	}
	if got := commanderLegalQuery.Get("price_value"); got != "1.25" {
		t.Fatalf("commander-legal remove query price_value = %q, want %q", got, "1.25")
	}
	if got := commanderLegalQuery.Get("sort"); got != "rarity" {
		t.Fatalf("commander-legal remove query sort = %q, want %q", got, "rarity")
	}
	if got := commanderLegalQuery.Get("sort_dir"); got != "desc" {
		t.Fatalf("commander-legal remove query sort_dir = %q, want %q", got, "desc")
	}
}

func TestBuildCardSearchFilterChipsRemoveOneTypeKeepsOtherTypeModes(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		TypeFilters: []cardSearchTypeFilter{
			{Value: "Artifact", Mode: "is"},
			{Value: "Creature", Mode: "not"},
		},
		TypePartial: true,
		HasSearched: true,
	}

	chips := buildCardSearchFilterChips(req)
	chipMap := map[string]string{}
	for _, chip := range chips {
		chipMap[chip.Label] = chip.RemovePath
	}

	removeArtifact, ok := chipMap["Type: Artifact"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing artifact chip: %#v", chips)
	}

	artifactQuery := mustParseQuery(t, removeArtifact)
	if got := artifactQuery["type_value"]; len(got) != 1 || got[0] != "Creature" {
		t.Fatalf("artifact remove query type_value = %#v, want [Creature]", got)
	}
	if got := artifactQuery["type_mode"]; len(got) != 1 || got[0] != "not" {
		t.Fatalf("artifact remove query type_mode = %#v, want [not]", got)
	}
	if got := artifactQuery.Get("type_partial"); got != "1" {
		t.Fatalf("artifact remove query type_partial = %q, want %q", got, "1")
	}

	removeCreature, ok := chipMap["Type: NOT Creature"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing creature chip: %#v", chips)
	}

	creatureQuery := mustParseQuery(t, removeCreature)
	if got := creatureQuery["type_value"]; len(got) != 1 || got[0] != "Artifact" {
		t.Fatalf("creature remove query type_value = %#v, want [Artifact]", got)
	}
	if got := creatureQuery["type_mode"]; len(got) != 1 || got[0] != "is" {
		t.Fatalf("creature remove query type_mode = %#v, want [is]", got)
	}
	if got := creatureQuery.Get("type_partial"); got != "1" {
		t.Fatalf("creature remove query type_partial = %q, want %q", got, "1")
	}
}

func TestBuildCardSearchFilterChipsDropsColorModeWhenRemovingLastColor(t *testing.T) {
	t.Parallel()

	req := cardSearchRequest{
		ColorParams: []string{"C"},
		ColorMode:   "at_most",
		HasSearched: true,
	}

	chips := buildCardSearchFilterChips(req)
	chipMap := map[string]string{}
	for _, chip := range chips {
		chipMap[chip.Label] = chip.RemovePath
	}

	removeColor, ok := chipMap["Color: Colorless"]
	if !ok {
		t.Fatalf("buildCardSearchFilterChips() missing colorless chip: %#v", chips)
	}
	query := mustParseQuery(t, removeColor)
	if got := query["color"]; len(got) != 0 {
		t.Fatalf("last color remove query color = %#v, want empty", got)
	}
	if got := query.Get("color_mode"); got != "" {
		t.Fatalf("last color remove query color_mode = %q, want empty", got)
	}
}

func TestCardSearchEditPathUsesSearchEditorRoute(t *testing.T) {
	t.Parallel()

	got := cardSearchEditPath(cardSearchRequest{
		NameQuery:             "Atraxa",
		NameExact:             true,
		TextQuery:             "draw",
		TextMode:              "not",
		Sort:                  "oldest_printing",
		SortDirection:         "desc",
		SortDirectionExplicit: true,
		HasSearched:           true,
	})

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", got, err)
	}
	if parsed.Path != "/cards/search" {
		t.Fatalf("cardSearchEditPath() path = %q, want %q", parsed.Path, "/cards/search")
	}

	query := parsed.Query()
	if got := query.Get("edit"); got != "1" {
		t.Fatalf("cardSearchEditPath() edit = %q, want %q", got, "1")
	}
	if got := query.Get("q"); got != "Atraxa" {
		t.Fatalf("cardSearchEditPath() q = %q, want %q", got, "Atraxa")
	}
	if got := query.Get("name_exact"); got != "1" {
		t.Fatalf("cardSearchEditPath() name_exact = %q, want %q", got, "1")
	}
	if got := query.Get("text_mode"); got != "not" {
		t.Fatalf("cardSearchEditPath() text_mode = %q, want %q", got, "not")
	}
	if got := query.Get("sort"); got != "oldest_printing" {
		t.Fatalf("cardSearchEditPath() sort = %q, want %q", got, "oldest_printing")
	}
	if got := query.Get("sort_dir"); got != "desc" {
		t.Fatalf("cardSearchEditPath() sort_dir = %q, want %q", got, "desc")
	}
}

func TestHandleCardSearchRedirectsToResultsWithoutFiltersWhenViewRequested(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodGet, "/cards/search?view=1", nil)
	rr := httptest.NewRecorder()

	app.HandleCardSearch(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("HandleCardSearch() status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	parsed, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("url.Parse(Location) error = %v", err)
	}
	if parsed.Path != "/cards" {
		t.Fatalf("HandleCardSearch() redirect path = %q, want %q", parsed.Path, "/cards")
	}
	if got := parsed.Query().Get("sort"); got != "relevance" {
		t.Fatalf("HandleCardSearch() redirect sort = %q, want %q", got, "relevance")
	}
	if got := parsed.Query().Get("sort_dir"); got != "" {
		t.Fatalf("HandleCardSearch() redirect sort_dir = %q, want empty", got)
	}
	if got := parsed.Query().Get("search_mode"); got != string(cardResultsModeAdvanced) {
		t.Fatalf("HandleCardSearch() redirect search_mode = %q, want %q", got, cardResultsModeAdvanced)
	}
}

func TestHandleCardSearchRedirectsPreserveSelectedSort(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodGet, "/cards/search?view=1&q=Atraxa&sort=mana_value&sort_dir=desc", nil)
	rr := httptest.NewRecorder()

	app.HandleCardSearch(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("HandleCardSearch() status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	parsed, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("url.Parse(Location) error = %v", err)
	}
	if parsed.Path != "/cards" {
		t.Fatalf("HandleCardSearch() redirect path = %q, want %q", parsed.Path, "/cards")
	}
	if got := parsed.Query().Get("q"); got != "Atraxa" {
		t.Fatalf("HandleCardSearch() redirect q = %q, want %q", got, "Atraxa")
	}
	if got := parsed.Query().Get("sort"); got != "mana_value" {
		t.Fatalf("HandleCardSearch() redirect sort = %q, want %q", got, "mana_value")
	}
	if got := parsed.Query().Get("sort_dir"); got != "desc" {
		t.Fatalf("HandleCardSearch() redirect sort_dir = %q, want %q", got, "desc")
	}
}

func TestHandleCardSearchRedirectsRelevanceQueryWithoutImplicitDirection(t *testing.T) {
	t.Parallel()

	app := &App{}
	req := httptest.NewRequest(http.MethodGet, "/cards/search?view=1&q=Atraxa&sort=relevance", nil)
	rr := httptest.NewRecorder()

	app.HandleCardSearch(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("HandleCardSearch() status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	parsed, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("url.Parse(Location) error = %v", err)
	}
	if got := parsed.Query().Get("sort_dir"); got != "" {
		t.Fatalf("HandleCardSearch() redirect sort_dir = %q, want empty", got)
	}
}

func TestCardsListTemplateShowsSelectedSort(t *testing.T) {
	page := cardListPageData{
		Results: []searchResult{{
			OracleID:   "123e4567-e89b-12d3-a456-426614174000",
			DetailPath: cardDetailPath("123e4567-e89b-12d3-a456-426614174000"),
			Name:       "Atraxa, Grand Unifier",
		}},
		HasSearched:            true,
		CurrentPath:            "/cards?q=Atraxa&sort=mana_value&sort_dir=desc",
		ClearPath:              "/cards/search",
		EditFiltersPath:        "/cards/search?edit=1&q=Atraxa&sort=mana_value&sort_dir=desc",
		SortOptions:            advancedCardSortOptions,
		DirectionOptions:       advancedCardSortDirectionOptions,
		SelectedSort:           "mana_value",
		SelectedSortDirection:  "desc",
		CurrentSortLabel:       "Mana Value",
		CurrentDirectionLabel:  "Descending",
		SortFields:             []cardSearchQueryField{{Name: "q", Value: "Atraxa"}},
		ShowOldestPrintingNote: false,
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "cards_list", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	if !strings.Contains(body, `<option value="mana_value" selected>Mana Value</option>`) {
		t.Fatalf("cards_list template did not render the selected sort option: %s", body)
	}
	if !strings.Contains(body, `<option value="desc" selected>Descending</option>`) {
		t.Fatalf("cards_list template did not render the selected direction option: %s", body)
	}
	if !strings.Contains(body, `name="q" value="Atraxa"`) {
		t.Fatalf("cards_list template did not preserve hidden query fields: %s", body)
	}
	if !strings.Contains(body, `onchange="this.form.requestSubmit()"`) {
		t.Fatalf("cards_list template did not render auto-submit sort controls: %s", body)
	}
	if !strings.Contains(body, `<noscript>`) {
		t.Fatalf("cards_list template did not render a noscript fallback: %s", body)
	}
}

func TestCardsListTemplateUsesDirectCardArtResults(t *testing.T) {
	page := cardListPageData{
		Results: []searchResult{
			{
				OracleID:   "123e4567-e89b-12d3-a456-426614174000",
				DetailPath: cardDetailPath("123e4567-e89b-12d3-a456-426614174000"),
				Name:       "Delver of Secrets",
				ImageURI:   "https://example.com/delver.jpg",
				SetName:    "Innistrad",
				SetCode:    "ISD",
				PriceUSD:   "$0.50",
				FacesJSON:  `[{"name":"Delver of Secrets","image_uri":"https://example.com/delver.jpg"},{"name":"Insectile Aberration","image_uri":"https://example.com/insect.jpg"}]`,
			},
			{
				OracleID:   "223e4567-e89b-12d3-a456-426614174000",
				DetailPath: cardDetailPath("223e4567-e89b-12d3-a456-426614174000"),
				Name:       "Lightning Bolt",
				ImageURI:   "https://example.com/bolt.jpg",
				SetName:    "Magic 2010",
				SetCode:    "M10",
				PriceUSD:   "$1.25",
			},
			{
				OracleID:      "323e4567-e89b-12d3-a456-426614174000",
				DetailPath:    cardDetailPath("323e4567-e89b-12d3-a456-426614174000"),
				Name:          "Greenhouse // Rickety Gazebo",
				ImageURI:      "https://example.com/greenhouse.jpg",
				SetName:       "Duskmourn: House of Horror",
				SetCode:       "DSK",
				PriceUSD:      "$0.25",
				Layout:        "split",
				IsSplitLayout: true,
				FacesJSON:     `[{"name":"Greenhouse"},{"name":"Rickety Gazebo"}]`,
			},
		},
		HasSearched:           true,
		CurrentPath:           "/cards?q=delver",
		ClearPath:             "/cards/search",
		EditFiltersPath:       "/cards/search?edit=1&q=delver",
		SortOptions:           advancedCardSortOptions,
		DirectionOptions:      advancedCardSortDirectionOptions,
		SelectedSort:          "relevance",
		CurrentSortLabel:      "Relevance",
		CurrentDirectionLabel: "Descending",
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "cards_list", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	if strings.Contains(body, `id="card-detail-modal"`) {
		t.Fatalf("cards_list template rendered the quick-view modal: %s", body)
	}
	if strings.Contains(body, `data-card-detail`) {
		t.Fatalf("cards_list template rendered quick-view hooks: %s", body)
	}
	if strings.Contains(body, `quick view`) {
		t.Fatalf("cards_list template still references quick view: %s", body)
	}
	if !strings.Contains(body, `href="/cards/view/123e4567-e89b-12d3-a456-426614174000"`) {
		t.Fatalf("cards_list template did not link result cards to detail pages: %s", body)
	}
	if !strings.Contains(body, `data-card-result-image`) {
		t.Fatalf("cards_list template did not render card art images: %s", body)
	}
	if !strings.Contains(body, `Innistrad (ISD)`) || !strings.Contains(body, `$0.50`) {
		t.Fatalf("cards_list template did not render the set and price footer: %s", body)
	}
	if got := strings.Count(body, `data-card-result-control="turn-over"`); got != 1 {
		t.Fatalf("cards_list template rendered %d turn-over controls, want 1: %s", got, body)
	}
	if got := strings.Count(body, `data-card-result-control="rotate"`); got != 1 {
		t.Fatalf("cards_list template rendered %d rotate controls, want 1: %s", got, body)
	}
	if !strings.Contains(body, `Rotate`) || !strings.Contains(body, `Turn Over`) {
		t.Fatalf("cards_list template did not render both split and MDFC controls: %s", body)
	}
}

func TestCardsSearchTemplateOmitsImplicitSortDirectionHiddenField(t *testing.T) {
	page := cardSearchPageData{
		Sort:                  "relevance",
		SortDirection:         "desc",
		SortDirectionExplicit: false,
		CurrentSortLabel:      "Relevance",
		CurrentDirectionLabel: "Descending",
		ShowCurrentSort:       true,
		SearchActionPath:      "/cards/search",
		ClearPath:             "/cards/search",
		ColorsSelected: map[string]bool{
			"W": false,
			"U": false,
			"B": false,
			"R": false,
			"G": false,
			"C": false,
		},
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "cards_search", TemplateData{
		Data: page,
	})

	body := rec.Body.String()
	if !strings.Contains(body, `name="sort" value="relevance"`) {
		t.Fatalf("cards_search template did not preserve hidden sort field: %s", body)
	}
	searchFormMarker := `<form method="GET" action="/cards/search" class="mt-advanced-search-form" data-card-search-form>`
	searchFormStart := strings.Index(body, searchFormMarker)
	if searchFormStart < 0 {
		t.Fatalf("cards_search template did not render the advanced search form: %s", body)
	}
	searchFormBody := body[searchFormStart:]
	if nextForm := strings.Index(searchFormBody[len(searchFormMarker):], "</form>"); nextForm >= 0 {
		searchFormBody = searchFormBody[:len(searchFormMarker)+nextForm]
	}
	if strings.Contains(searchFormBody, `name="sort_dir"`) {
		t.Fatalf("cards_search template unexpectedly rendered hidden sort_dir field in the advanced search form: %s", searchFormBody)
	}
}

func mustParseQuery(t *testing.T, rawPath string) url.Values {
	t.Helper()

	parsed, err := url.Parse(rawPath)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", rawPath, err)
	}
	return parsed.Query()
}
