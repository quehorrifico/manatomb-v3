package decks

import "testing"

func TestParseCommanderDecklistText_CommanderStyleInput(t *testing.T) {
	t.Parallel()

	commander, cards, err := ParseCommanderDecklistText(`
Commander: Atraxa, Grand Unifier
1 Sol Ring
1 Swords to Plowshares
`)
	if err != nil {
		t.Fatalf("ParseCommanderDecklistText returned error: %v", err)
	}
	if commander != "Atraxa, Grand Unifier" {
		t.Fatalf("commander = %q, want %q", commander, "Atraxa, Grand Unifier")
	}
	if len(cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(cards))
	}
}

func TestParseCommanderDecklistText_GenericDecklistAllowsNoCommander(t *testing.T) {
	t.Parallel()

	commander, cards, err := ParseCommanderDecklistText(`
4 Lightning Bolt
4 Counterspell
20 Island
`)
	if err != nil {
		t.Fatalf("ParseCommanderDecklistText returned error: %v", err)
	}
	if commander != "" {
		t.Fatalf("commander = %q, want empty", commander)
	}
	if len(cards) != 3 {
		t.Fatalf("len(cards) = %d, want 3", len(cards))
	}
}

func TestParseDecklistTextPreservesBoardsAndPrintingIdentity(t *testing.T) {
	t.Parallel()

	parsed, err := ParseDecklistText(`
Commander
1 Atraxa, Grand Unifier (ONE) 196

Deck
1x Sol Ring [CMM] 396
Swords to Plowshares x2

Sideboard:
SB: 1 Swan Song {scryfall:123e4567-e89b-12d3-a456-426614174000}

Maybeboard
1 Fierce Guardianship (C20)
`)
	if err != nil {
		t.Fatalf("ParseDecklistText returned error: %v", err)
	}
	if parsed.Format != "Commander" {
		t.Fatalf("Format = %q, want Commander", parsed.Format)
	}
	if len(parsed.Items) != 5 {
		t.Fatalf("len(Items) = %d, want 5: %#v", len(parsed.Items), parsed.Items)
	}

	commander := parsed.Items[0]
	if commander.Board != ImportBoardCommander || commander.SetCode != "ONE" || commander.CollectorNumber != "196" {
		t.Fatalf("commander metadata = %#v", commander)
	}
	main := parsed.Items[1]
	if main.Board != ImportBoardMain || main.SetCode != "CMM" || main.CollectorNumber != "396" {
		t.Fatalf("main metadata = %#v", main)
	}
	if parsed.Items[2].Qty != 2 || parsed.Items[2].Name != "Swords to Plowshares" {
		t.Fatalf("suffix quantity row = %#v", parsed.Items[2])
	}
	side := parsed.Items[3]
	if side.Board != ImportBoardSide || side.PrintID != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("sideboard metadata = %#v", side)
	}
	maybe := parsed.Items[4]
	if maybe.Board != ImportBoardMaybe || maybe.SetCode != "C20" || maybe.CollectorNumber != "" {
		t.Fatalf("maybeboard metadata = %#v", maybe)
	}
}

func TestParseDecklistTextDetectsSandboxAndSupportsFormatHint(t *testing.T) {
	t.Parallel()

	sandbox, err := ParseDecklistText("4 Lightning Bolt\n20 Mountain")
	if err != nil {
		t.Fatalf("ParseDecklistText sandbox: %v", err)
	}
	if sandbox.Format != "Sandbox" {
		t.Fatalf("sandbox Format = %q", sandbox.Format)
	}

	commander, err := ParseDecklistText("Format: EDH\n1 Sol Ring")
	if err != nil {
		t.Fatalf("ParseDecklistText format hint: %v", err)
	}
	if commander.Format != "Commander" {
		t.Fatalf("hinted Format = %q, want Commander", commander.Format)
	}

	standard, err := ParseDecklistText("Format: Standard\n4 Lightning Bolt")
	if err != nil {
		t.Fatalf("ParseDecklistText Standard hint: %v", err)
	}
	if standard.Format != "Standard" {
		t.Fatalf("standard Format = %q, want Standard", standard.Format)
	}
}

func TestParseDecklistTextRejectsEmptyCardInput(t *testing.T) {
	t.Parallel()

	if _, err := ParseDecklistText("# notes only\n// more notes"); err == nil {
		t.Fatal("ParseDecklistText returned nil error for comment-only input")
	}
}

func TestParseDecklistTextTreatsCardsAfterInlineCommanderAsMainboard(t *testing.T) {
	t.Parallel()

	parsed, err := ParseDecklistText("Commander: Atraxa, Grand Unifier\n1 Sol Ring")
	if err != nil {
		t.Fatalf("ParseDecklistText: %v", err)
	}
	if len(parsed.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(parsed.Items))
	}
	if parsed.Items[0].Board != ImportBoardCommander || parsed.Items[1].Board != ImportBoardMain {
		t.Fatalf("boards = %q, %q", parsed.Items[0].Board, parsed.Items[1].Board)
	}
}

func TestParseDecklistTextKeepsInvalidQuantityForInlineReview(t *testing.T) {
	t.Parallel()

	parsed, err := ParseDecklistText("0 Sol Ring")
	if err != nil {
		t.Fatalf("ParseDecklistText: %v", err)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].Qty != 0 {
		t.Fatalf("Items = %#v", parsed.Items)
	}
}

func TestParseDecklistRecognizesManaTombTextExportEnvelope(t *testing.T) {
	t.Parallel()

	parsed, err := ParseDecklist(`Neon Phantom's Side Quest
Format: Standard

Mainboard (8)
4 Lightning Bolt
4 Atraxa, Grand Unifier

Sideboard (2 cards):
2 Negate

Maybeboard (1)
1 Opt
`)
	if err != nil {
		t.Fatalf("ParseDecklist: %v", err)
	}
	if parsed.Name != "Neon Phantom's Side Quest" {
		t.Fatalf("Name = %q", parsed.Name)
	}
	if parsed.Format != "Standard" {
		t.Fatalf("Format = %q, want Standard", parsed.Format)
	}
	if len(parsed.Items) != 4 {
		t.Fatalf("len(Items) = %d, want 4: %#v", len(parsed.Items), parsed.Items)
	}
	if parsed.Items[0].Name != "Lightning Bolt" || parsed.Items[0].Qty != 4 || parsed.Items[0].Board != ImportBoardMain {
		t.Fatalf("first mainboard item = %#v", parsed.Items[0])
	}
	if parsed.Items[2].Name != "Negate" || parsed.Items[2].Qty != 2 || parsed.Items[2].Board != ImportBoardSide {
		t.Fatalf("sideboard item = %#v", parsed.Items[2])
	}
	if parsed.Items[3].Name != "Opt" || parsed.Items[3].Board != ImportBoardMaybe {
		t.Fatalf("maybeboard item = %#v", parsed.Items[3])
	}
	for _, item := range parsed.Items {
		if item.Name == parsed.Name {
			t.Fatalf("exported title was parsed as a card: %#v", item)
		}
	}
}

func TestParseDecklistRecognizesExplicitNameMetadata(t *testing.T) {
	t.Parallel()

	parsed, err := ParseDecklist(`Name: Couch Potato Control
Format: Sandbox
Mainboard (1)
1 Sol Ring
`)
	if err != nil {
		t.Fatalf("ParseDecklist: %v", err)
	}
	if parsed.Name != "Couch Potato Control" {
		t.Fatalf("Name = %q", parsed.Name)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].Name != "Sol Ring" {
		t.Fatalf("Items = %#v", parsed.Items)
	}
}

func TestParseDecklistTextPreservesCombinedPrintingMetadata(t *testing.T) {
	t.Parallel()

	const printID = "123e4567-e89b-12d3-a456-426614174000"
	parsed, err := ParseDecklist(`Name: Printing Round Trip
Format: Standard
Mainboard (1)
1 Sol Ring (CMM) 396 {scryfall:` + printID + `}
`)
	if err != nil {
		t.Fatalf("ParseDecklist: %v", err)
	}
	if len(parsed.Items) != 1 {
		t.Fatalf("Items = %#v", parsed.Items)
	}
	item := parsed.Items[0]
	if item.Name != "Sol Ring" || item.SetCode != "CMM" || item.CollectorNumber != "396" || item.PrintID != printID {
		t.Fatalf("printing metadata = %#v", item)
	}
}

func TestParseDecklistDoesNotGuessOrdinaryFirstCardIsTitle(t *testing.T) {
	t.Parallel()

	parsed, err := ParseDecklist("Atraxa, Grand Unifier\n1 Sol Ring")
	if err != nil {
		t.Fatalf("ParseDecklist: %v", err)
	}
	if parsed.Name != "" {
		t.Fatalf("Name = %q, want empty", parsed.Name)
	}
	if len(parsed.Items) != 2 || parsed.Items[0].Name != "Atraxa, Grand Unifier" {
		t.Fatalf("Items = %#v", parsed.Items)
	}
}

func TestParseDecklistAutoDetectsPublicDeckCSV(t *testing.T) {
	t.Parallel()

	parsed, err := ParseDecklist("\ufeffBoard,Quantity,Name,Set,Collector Number,Price USD\r\n" +
		`Mainboard,4,"Atraxa, Grand Unifier",one,196,0.25` + "\r\n" +
		`Sideboard,2,Negate,m20,69,0.10` + "\r\n" +
		`Maybeboard,1,Opt,dom,60,0.05`)
	if err != nil {
		t.Fatalf("ParseDecklist: %v", err)
	}
	if parsed.Format != "Sandbox" {
		t.Fatalf("Format = %q, want Sandbox", parsed.Format)
	}
	if len(parsed.Items) != 3 {
		t.Fatalf("len(Items) = %d, want 3: %#v", len(parsed.Items), parsed.Items)
	}
	first := parsed.Items[0]
	if first.Name != "Atraxa, Grand Unifier" || first.Qty != 4 || first.Board != ImportBoardMain || first.SetCode != "ONE" || first.CollectorNumber != "196" {
		t.Fatalf("mainboard CSV item = %#v", first)
	}
	if parsed.Items[1].Board != ImportBoardSide || parsed.Items[2].Board != ImportBoardMaybe {
		t.Fatalf("CSV boards = %#v", parsed.Items)
	}
}

func TestParseDecklistAutoDetectsEditorCSVAndPreservesMetadata(t *testing.T) {
	t.Parallel()

	const printID = "123e4567-e89b-12d3-a456-426614174000"
	parsed, err := ParseDecklist(`Format,Deck Name,Name,Collector Number,Board,Print ID,Quantity,Set
Standard,"Ninja, Panda",Lightning Bolt,150,Mainboard,` + printID + `,4,2XM
Standard,"Ninja, Panda",Negate,69,Sideboard,,2,M20
`)
	if err != nil {
		t.Fatalf("ParseDecklist: %v", err)
	}
	if parsed.Name != "Ninja, Panda" || parsed.Format != "Standard" {
		t.Fatalf("metadata = Name %q, Format %q", parsed.Name, parsed.Format)
	}
	if len(parsed.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(parsed.Items))
	}
	main := parsed.Items[0]
	if main.Name != "Lightning Bolt" || main.Qty != 4 || main.Board != ImportBoardMain || main.SetCode != "2XM" || main.CollectorNumber != "150" || main.PrintID != printID {
		t.Fatalf("editor CSV mainboard item = %#v", main)
	}
	if parsed.Items[1].Board != ImportBoardSide {
		t.Fatalf("editor CSV sideboard item = %#v", parsed.Items[1])
	}
}

func TestParseDecklistAcceptsLegacyEditorCSVAndInfersCommander(t *testing.T) {
	t.Parallel()

	parsed, err := ParseDecklist(`Board,Quantity,Name,Set,Collector Number,Print ID
Commander,1,"Atraxa, Grand Unifier",ONE,196,
Mainboard,1,Sol Ring,CMM,396,
`)
	if err != nil {
		t.Fatalf("ParseDecklist: %v", err)
	}
	if parsed.Name != "" || parsed.Format != "Commander" {
		t.Fatalf("legacy metadata = Name %q, Format %q", parsed.Name, parsed.Format)
	}
	if len(parsed.Items) != 2 || parsed.Items[0].Board != ImportBoardCommander || parsed.Items[1].Board != ImportBoardMain {
		t.Fatalf("Items = %#v", parsed.Items)
	}
}

func TestParseDecklistCSVReportsInvalidQuantity(t *testing.T) {
	t.Parallel()

	_, err := ParseDecklist("Board,Quantity,Name,Set,Collector Number,Price USD\nMainboard,many,Sol Ring,CMM,396,1.00")
	if err == nil || err.Error() != "decklist CSV row 2 has an invalid quantity" {
		t.Fatalf("error = %v", err)
	}
}

func TestParseDecklistCSVReportsInvalidPrintID(t *testing.T) {
	t.Parallel()

	_, err := ParseDecklist("Board,Quantity,Name,Print ID\nMainboard,1,Sol Ring,not-a-uuid")
	if err == nil || err.Error() != "decklist CSV row 2 has an invalid Print ID" {
		t.Fatalf("error = %v", err)
	}
}
