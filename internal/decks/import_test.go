package decks

import "testing"

func TestNormalizeImportedDeckCardsCombinesRowsByBoard(t *testing.T) {
	t.Parallel()

	const oracleID = "123e4567-e89b-12d3-a456-426614174000"
	const printID = "223e4567-e89b-12d3-a456-426614174000"
	got, err := normalizeImportedDeckCards([]ImportedDeckCardInput{
		{OracleID: oracleID, Qty: 1, Board: "main"},
		{OracleID: oracleID, Qty: 2, Board: "mainboard", PreferredPrintID: printID},
		{OracleID: oracleID, Qty: 1, Board: "side", PreferredPrintID: printID},
	})
	if err != nil {
		t.Fatalf("normalizeImportedDeckCards: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2: %#v", len(got), got)
	}
	if got[0].Board != "main" || got[0].Qty != 3 || got[0].PreferredPrintID != printID {
		t.Fatalf("main row = %#v", got[0])
	}
	if got[1].Board != "side" || got[1].Qty != 1 {
		t.Fatalf("side row = %#v", got[1])
	}
}

func TestNormalizeImportedDeckCardsRejectsConflictingPrints(t *testing.T) {
	t.Parallel()

	const oracleID = "123e4567-e89b-12d3-a456-426614174000"
	_, err := normalizeImportedDeckCards([]ImportedDeckCardInput{
		{
			OracleID:         oracleID,
			Qty:              1,
			Board:            "main",
			PreferredPrintID: "223e4567-e89b-12d3-a456-426614174000",
		},
		{
			OracleID:         oracleID,
			Qty:              1,
			Board:            "main",
			PreferredPrintID: "323e4567-e89b-12d3-a456-426614174000",
		},
	})
	if err == nil {
		t.Fatal("normalizeImportedDeckCards returned nil error for conflicting printings")
	}
}
