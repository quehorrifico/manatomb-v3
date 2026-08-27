package decks

import "testing"

func TestNormalizeFormat(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"commander":         "Commander",
		"sandbox":           "Sandbox",
		"casual":            "Casual",
		"historic brawl":    "Historic Brawl",
		"modern":            "Modern",
		"\"Commander\"":     "Commander",
		"\\\"Commander\\\"": "Commander",
		"":                  "Commander",
	}

	for input, want := range cases {
		if got := NormalizeFormat(input); got != want {
			t.Fatalf("NormalizeFormat(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSupportedFormatsShareOneDefinitionRegistry(t *testing.T) {
	t.Parallel()

	for _, format := range SupportedFormats() {
		definition := FormatDefinitionFor(format)
		if definition.Name != format {
			t.Fatalf("FormatDefinitionFor(%q).Name = %q", format, definition.Name)
		}
		if got := FormatRequiresCommander(format); got != definition.RequiresCommander {
			t.Fatalf("FormatRequiresCommander(%q) = %t, want %t", format, got, definition.RequiresCommander)
		}
		if got := FormatTargetMainboardSize(format); got != definition.TargetMainboardSize {
			t.Fatalf("FormatTargetMainboardSize(%q) = %d, want %d", format, got, definition.TargetMainboardSize)
		}
		if got := FormatCopyLimit(format); got != definition.CopyLimit {
			t.Fatalf("FormatCopyLimit(%q) = %d, want %d", format, got, definition.CopyLimit)
		}
	}
}

func TestBuildableFormatsMatchCurrentBuilderEntryPoints(t *testing.T) {
	t.Parallel()

	want := []string{"Commander", "Standard", "Sandbox"}
	got := BuildableFormats()
	if len(got) != len(want) {
		t.Fatalf("BuildableFormats() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("BuildableFormats()[%d] = %q, want %q", i, got[i], want[i])
		}
		if !FormatIsBuildable(got[i]) {
			t.Fatalf("FormatIsBuildable(%q) = false", got[i])
		}
	}
	if FormatIsBuildable("Modern") {
		t.Fatal("Modern should remain known for existing data, not selectable in the current builder")
	}
}

func TestFormatRequiresCommander(t *testing.T) {
	t.Parallel()

	if !FormatRequiresCommander("Commander") {
		t.Fatal("Commander should require a commander")
	}
	if FormatRequiresCommander("Modern") {
		t.Fatal("Modern should not require a commander")
	}
}

func TestNormalizePowerBracket(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"1":             "1 - Thematic",
		"Thematic":      "1 - Thematic",
		"Casual":        "1 - Thematic",
		"2":             "2 - Core",
		"Focused":       "2 - Core",
		"3 - Upgraded":  "3 - Upgraded",
		"Competitive":   "4 - Optimized",
		"High Power":    "4 - Optimized",
		"5":             "5 - cEDH",
		"cedh":          "5 - cEDH",
		"Custom Rating": "Custom Rating",
	}

	for input, want := range cases {
		if got := NormalizePowerBracket(input); got != want {
			t.Fatalf("NormalizePowerBracket(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizePublicSlug(t *testing.T) {
	t.Parallel()

	got := NormalizePublicSlug(" Atraxa, Grand Unifier !! ")
	want := "atraxa-grand-unifier"
	if got != want {
		t.Fatalf("NormalizePublicSlug() = %q, want %q", got, want)
	}
}

func TestNormalizeTags(t *testing.T) {
	t.Parallel()

	got := NormalizeTags(" Ramp, Control,  ramp \nCombo, Unknown, tribal ")
	want := "Ramp, Control, Combo, Tribal"
	if got != want {
		t.Fatalf("NormalizeTags() = %q, want %q", got, want)
	}
}

func TestSupportedDeckTags(t *testing.T) {
	t.Parallel()

	got := SupportedDeckTags()
	want := []string{
		"Aggro",
		"Midrange",
		"Control",
		"Combo",
		"Ramp",
		"Stax",
		"Aristocrats",
		"Spellslinger",
		"Tokens",
		"Reanimator",
		"Voltron",
		"Tribal",
	}

	if len(got) != len(want) {
		t.Fatalf("SupportedDeckTags() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SupportedDeckTags()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
