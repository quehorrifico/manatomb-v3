package web

import (
	"testing"
	"time"

	"manatomb/app/internal/account"
	"manatomb/app/internal/decks"
)

func TestColorCombinationName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"W":         "Mono-white",
		"B,G":       "Golgari",
		"W,U,B":     "Esper",
		"W,U,B,R,G": "Five-color",
		"":          "Colorless",
	}

	for input, want := range cases {
		if got := colorCombinationName(input); got != want {
			t.Fatalf("colorCombinationName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildProfileStatsUsesPublicDecks(t *testing.T) {
	t.Parallel()

	joined := time.Date(2025, time.March, 14, 0, 0, 0, 0, time.UTC)
	stats := buildProfileStats(
		account.PublicProfile{
			DisplayName:            "Deck Brewer",
			ProfileAvatarCommander: "Meren",
			CreatedAt:              joined,
		},
		[]decks.Deck{
			{Format: "Commander", PowerBracket: "2 - Core", Tags: "Ramp, Control"},
			{Format: "Commander", PowerBracket: "4 - Optimized", Tags: "Ramp, Combo"},
			{Format: "Modern", PowerBracket: "", Tags: "Ramp"},
		},
		[]deckListItem{
			{
				CommanderName:       "Raffine",
				CommanderImageURI:   "https://example.test/raffine.jpg",
				CommanderArtCropURI: "https://example.test/raffine-art.jpg",
				ColorIdentityName:   "Esper",
				ColorPips:           manaPipsForColorIdentity("W,U,B"),
			},
			{
				CommanderName:       "Meren",
				CommanderImageURI:   "https://example.test/meren.jpg",
				CommanderArtCropURI: "https://example.test/meren-art.jpg",
				ColorIdentityName:   "Golgari",
				ColorPips:           manaPipsForColorIdentity("B,G"),
			},
			{
				CommanderName:     "Zur",
				CommanderImageURI: "https://example.test/zur.jpg",
				ColorIdentityName: "Esper",
				ColorPips:         manaPipsForColorIdentity("W,U,B"),
			},
		},
		nil,
	)

	if stats.JoinedLabel != "Mar 2025" {
		t.Fatalf("JoinedLabel = %q, want Mar 2025", stats.JoinedLabel)
	}
	if stats.AveragePowerBracket != "3 / 5" {
		t.Fatalf("AveragePowerBracket = %q, want 3 / 5", stats.AveragePowerBracket)
	}
	if stats.UsuallyPlays != "Commander" {
		t.Fatalf("UsuallyPlays = %q, want Commander", stats.UsuallyPlays)
	}
	if stats.FavoriteColorCombination != "Esper" {
		t.Fatalf("FavoriteColorCombination = %q, want Esper", stats.FavoriteColorCombination)
	}
	if len(stats.Likes) == 0 || stats.Likes[0] != "Ramp" {
		t.Fatalf("Likes = %#v, want Ramp first", stats.Likes)
	}
	if stats.AvatarImageURI != "https://example.test/meren-art.jpg" {
		t.Fatalf("AvatarImageURI = %q", stats.AvatarImageURI)
	}
	if len(stats.AvatarChoices) != 3 {
		t.Fatalf("AvatarChoices length = %d, want 3", len(stats.AvatarChoices))
	}
	if !stats.AvatarChoices[1].IsSelected {
		t.Fatalf("expected Meren avatar choice to be selected, got %#v", stats.AvatarChoices)
	}
}

func TestBuildProfileStatsUsesCustomAvatarChoice(t *testing.T) {
	t.Parallel()

	stats := buildProfileStats(
		account.PublicProfile{
			DisplayName:            "Deck Brewer",
			ProfileAvatarCommander: "Serra Angel",
			CreatedAt:              time.Date(2025, time.March, 14, 0, 0, 0, 0, time.UTC),
		},
		nil,
		[]deckListItem{
			{
				CommanderName:       "Raffine",
				CommanderArtCropURI: "https://example.test/raffine-art.jpg",
				ColorIdentityName:   "Esper",
				ColorPips:           manaPipsForColorIdentity("W,U,B"),
			},
		},
		&profileAvatarChoice{
			Name:     "Serra Angel",
			ImageURI: "https://example.test/serra-art.jpg",
		},
	)

	if stats.AvatarImageURI != "https://example.test/serra-art.jpg" {
		t.Fatalf("AvatarImageURI = %q", stats.AvatarImageURI)
	}
	if len(stats.AvatarChoices) == 0 || stats.AvatarChoices[0].Name != "Serra Angel" {
		t.Fatalf("AvatarChoices = %#v, want custom choice first", stats.AvatarChoices)
	}
	if !stats.AvatarChoices[0].IsSelected {
		t.Fatalf("custom avatar should be selected: %#v", stats.AvatarChoices)
	}
}
