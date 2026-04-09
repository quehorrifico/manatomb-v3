package decks

import "testing"

func TestNormalizeFormat(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"commander":         "Commander",
		"sandbox":           "Sandbox",
		"casual":            "Sandbox",
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

func TestFormatRequiresCommander(t *testing.T) {
	t.Parallel()

	if !FormatRequiresCommander("Commander") {
		t.Fatal("Commander should require a commander")
	}
	if FormatRequiresCommander("Modern") {
		t.Fatal("Modern should not require a commander")
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
