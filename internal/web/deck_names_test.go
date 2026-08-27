package web

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateDeckNameStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values []int
		want   string
	}{
		{
			name:   "classic camel case",
			values: []int{0, 0, 0, 4},
			want:   "ShadowWeaver04",
		},
		{
			name:   "framed gamer tag",
			values: []int{1, 1, 1, 1337},
			want:   "xX_NeonPhantom1337_Xx",
		},
		{
			name:   "underscore separated",
			values: []int{2, 2, 2, 7},
			want:   "Void_Walker_07",
		},
		{
			name:   "noun first",
			values: []int{3, 3, 3, 42},
			want:   "TigerFurious42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			call := 0
			got := generateDeckName(func(n int) int {
				if call >= len(tt.values) {
					t.Fatalf("Intn called more than %d times", len(tt.values))
				}
				value := tt.values[call]
				call++
				if value < 0 || value >= n {
					t.Fatalf("scripted Intn value %d is outside [0, %d)", value, n)
				}
				return value
			})

			if got != tt.want {
				t.Fatalf("generateDeckName() = %q, want %q", got, tt.want)
			}
			if call != len(tt.values) {
				t.Fatalf("Intn called %d times, want %d", call, len(tt.values))
			}
		})
	}
}

func TestGenerateDeckNameMaximumLengthCombination(t *testing.T) {
	t.Parallel()

	values := []int{
		1,
		len(deckNameAdjectives) - 1,
		len(deckNameNouns) - 1,
		deckNameNumberRange - 1,
	}
	call := 0
	got := generateDeckName(func(n int) int {
		value := values[call]
		call++
		if value < 0 || value >= n {
			t.Fatalf("scripted Intn value %d is outside [0, %d)", value, n)
		}
		return value
	})

	const want = "xX_UnstoppableCouchPotato9999_Xx"
	if got != want {
		t.Fatalf("generateDeckName() = %q, want %q", got, want)
	}
	if len(got) != generatedDeckNameMaxLength {
		t.Fatalf("maximum generated name length = %d, want %d", len(got), generatedDeckNameMaxLength)
	}
}

func TestGenerateDeckNameInvariantsAndVariety(t *testing.T) {
	t.Parallel()

	validName := regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	hasNumericSuffix := regexp.MustCompile(`[0-9]{2,4}(?:_Xx)?$`)
	sentinels := map[string]bool{
		"Imported Deck":  true,
		"New Deck":       true,
		"New Guest Deck": true,
		"Untitled Deck":  true,
	}

	rng := rand.New(rand.NewSource(20260728))
	const sampleSize = 10_000
	seen := make(map[string]struct{}, sampleSize)
	for i := 0; i < sampleSize; i++ {
		name := generateDeckName(rng.Intn)
		if name == "" {
			t.Fatalf("sample %d generated an empty name", i)
		}
		if len(name) > generatedDeckNameMaxLength {
			t.Fatalf("sample %d generated %q with length %d, max is %d", i, name, len(name), generatedDeckNameMaxLength)
		}
		if !validName.MatchString(name) {
			t.Fatalf("sample %d generated non-ASCII gamertag %q", i, name)
		}
		if !hasNumericSuffix.MatchString(name) {
			t.Fatalf("sample %d generated name without numeric suffix: %q", i, name)
		}
		if sentinels[name] {
			t.Fatalf("sample %d generated reserved placeholder %q", i, name)
		}
		seen[name] = struct{}{}
	}

	if len(seen) < 9_900 {
		t.Fatalf("generated only %d distinct names from %d deterministic samples", len(seen), sampleSize)
	}
}

func TestGenerateDeckNameWordListsStaySafeAndBounded(t *testing.T) {
	t.Parallel()

	validWord := regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	for listName, words := range map[string][]string{
		"adjectives": deckNameAdjectives[:],
		"nouns":      deckNameNouns[:],
	} {
		for _, word := range words {
			if !validWord.MatchString(word) {
				t.Errorf("%s contains unsafe word %q", listName, word)
			}
			if len(word) > 11 {
				t.Errorf("%s word %q has length %d, max is 11", listName, word, len(word))
			}
		}
	}

	for style := 0; style < deckNameStyleCount; style++ {
		for adjectiveIndex := range deckNameAdjectives {
			for nounIndex := range deckNameNouns {
				values := []int{style, adjectiveIndex, nounIndex, deckNameNumberRange - 1}
				call := 0
				name := generateDeckName(func(int) int {
					value := values[call]
					call++
					return value
				})
				if len(name) > generatedDeckNameMaxLength {
					t.Fatalf("generated %q with length %d, max is %d", name, len(name), generatedDeckNameMaxLength)
				}
				if strings.ContainsAny(name, " \t\r\n") {
					t.Fatalf("generated name contains whitespace: %q", name)
				}
			}
		}
	}
}
