package web

import "testing"

func TestNormalizeLocalReturnPathRejectsBackslashes(t *testing.T) {
	t.Parallel()

	if got := normalizeLocalReturnPath(`/\evil.example`, "/safe"); got != "/safe" {
		t.Fatalf("normalizeLocalReturnPath() = %q, want fallback", got)
	}
}

func TestDeckWorkbenchPath(t *testing.T) {
	t.Parallel()

	path := deckWorkbenchPath(deckWorkbenchOptions{
		Format:        "modern",
		CommanderName: "Atraxa, Grand Unifier",
		SaveWorkbench: true,
	})

	want := "/decks/new/workbench?format=Modern&save_guest=1"
	if path != want {
		t.Fatalf("deckWorkbenchPath() = %q, want %q", path, want)
	}
}

func TestDeckWorkbenchPathStartsFreshStandardDraft(t *testing.T) {
	t.Parallel()

	path := deckWorkbenchPath(deckWorkbenchOptions{
		Format: "Standard",
		Reset:  true,
	})

	want := "/decks/new/workbench?format=Standard&reset=1"
	if path != want {
		t.Fatalf("deckWorkbenchPath() = %q, want %q", path, want)
	}
}

func TestDeckWorkbenchPathPreservesCommanderPrinting(t *testing.T) {
	t.Parallel()

	path := deckWorkbenchPath(deckWorkbenchOptions{
		Format:           "Commander",
		CommanderName:    "Atraxa, Grand Unifier",
		CommanderPrintID: "223e4567-e89b-12d3-a456-426614174000",
		SaveWorkbench:    true,
	})

	want := "/decks/new/workbench?commander_name=Atraxa%2C+Grand+Unifier&commander_print_id=223e4567-e89b-12d3-a456-426614174000&format=Commander&save_guest=1"
	if path != want {
		t.Fatalf("deckWorkbenchPath() = %q, want %q", path, want)
	}
}

func TestDeckWorkbenchPathSandboxDefaultsToSandbox(t *testing.T) {
	t.Parallel()

	path := deckWorkbenchPath(deckWorkbenchOptions{
		Sandbox: true,
		Reset:   true,
	})

	want := "/decks/new/workbench?format=Sandbox&reset=1&sandbox=1"
	if path != want {
		t.Fatalf("deckWorkbenchPath() = %q, want %q", path, want)
	}
}

func TestDeckWorkbenchPathSandboxDropsCommanderPrinting(t *testing.T) {
	t.Parallel()

	path := deckWorkbenchPath(deckWorkbenchOptions{
		Format:           "Commander",
		CommanderName:    "Atraxa, Grand Unifier",
		CommanderPrintID: "223e4567-e89b-12d3-a456-426614174000",
		Sandbox:          true,
	})

	want := "/decks/new/workbench?format=Sandbox&sandbox=1"
	if path != want {
		t.Fatalf("deckWorkbenchPath() = %q, want %q", path, want)
	}
}

func TestCommanderDeckBuilderPath(t *testing.T) {
	t.Parallel()

	path := commanderDeckBuilderPath(commanderDeckBuilderState{
		Query: "nadu",
	})

	want := "/decks/new/commander/?q=nadu"
	if path != want {
		t.Fatalf("commanderDeckBuilderPath() = %q, want %q", path, want)
	}
}

func TestMergeLocalReturnPath(t *testing.T) {
	t.Parallel()

	got := mergeLocalReturnPath("/decks/import", "/decks/new", map[string]string{
		"format":         "Commander",
		"commander_name": "The Ur-Dragon",
	})

	want := "/decks/import?commander_name=The+Ur-Dragon&format=Commander"
	if got != want {
		t.Fatalf("mergeLocalReturnPath() = %q, want %q", got, want)
	}
}

func TestAbsoluteSiteURLUsesOnlyConfiguredOrigin(t *testing.T) {
	t.Parallel()

	if got := absoluteSiteURL("https://manatomb.app/", "/cards/view/example"); got != "https://manatomb.app/cards/view/example" {
		t.Fatalf("absoluteSiteURL() = %q", got)
	}
	if got := absoluteSiteURL("javascript:alert(1)", "/cards/view/example"); got != "" {
		t.Fatalf("absoluteSiteURL() accepted invalid base: %q", got)
	}
	for _, invalid := range []string{
		"https://manatomb.app/subpath",
		"https://manatomb.app?preview=1",
		"https://user:secret@manatomb.app",
	} {
		if got := absoluteSiteURL(invalid, "/cards/view/example"); got != "" {
			t.Fatalf("absoluteSiteURL() accepted non-origin base %q: %q", invalid, got)
		}
	}
}

func TestMergeLocalReturnPathPreservesLegacyImportQuery(t *testing.T) {
	t.Parallel()

	got := mergeLocalReturnPath("/decks/new?mode=import", "/decks/new", map[string]string{
		"format":         "Commander",
		"commander_name": "The Ur-Dragon",
	})

	want := "/decks/new?commander_name=The+Ur-Dragon&format=Commander&mode=import"
	if got != want {
		t.Fatalf("mergeLocalReturnPath() = %q, want %q", got, want)
	}
}

func TestCanonicalizeLocalReturnPathUsesCommanderRoutes(t *testing.T) {
	t.Parallel()

	got := canonicalizeLocalReturnPath("/decks/select-leader?return_to=%2Fdecks%2Fedit%3Fid%3D7", "/decks/new")
	want := "/decks/new/commander/?return_to=%2Fdecks%2Fedit%3Fid%3D7"
	if got != want {
		t.Fatalf("canonicalizeLocalReturnPath() = %q, want %q", got, want)
	}
}

func TestCanonicalizeLocalReturnPathAddsCommanderTrailingSlash(t *testing.T) {
	t.Parallel()

	got := canonicalizeLocalReturnPath("/decks/new/commander", "/decks/new")
	want := "/decks/new/commander/"
	if got != want {
		t.Fatalf("canonicalizeLocalReturnPath() = %q, want %q", got, want)
	}
}

func TestCanonicalizeLocalReturnPathUsesCanonicalSettingsPath(t *testing.T) {
	t.Parallel()

	got := canonicalizeLocalReturnPath("/decks/edit?id=7", "/decks/new")
	want := "/decks/settings?id=7"
	if got != want {
		t.Fatalf("canonicalizeLocalReturnPath() = %q, want %q", got, want)
	}
}
