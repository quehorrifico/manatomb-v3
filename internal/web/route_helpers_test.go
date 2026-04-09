package web

import "testing"

func TestDeckWorkbenchPath(t *testing.T) {
	t.Parallel()

	path := deckWorkbenchPath(deckWorkbenchOptions{
		Format:        "modern",
		CommanderName: "Atraxa, Grand Unifier",
		SaveWorkbench: true,
	})

	want := "/decks/new/workbench?commander_name=Atraxa%2C+Grand+Unifier&format=Commander&save_guest=1"
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
