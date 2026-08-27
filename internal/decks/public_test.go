package decks

import (
	"database/sql/driver"
	"strings"
	"testing"
)

func TestNormalizePublicDeckColors(t *testing.T) {
	tests := []struct {
		name string
		raw  []string
		want []string
	}{
		{
			name: "canonical order and deduplication",
			raw:  []string{"g", "W", "u", "W", "unknown"},
			want: []string{"W", "U", "G"},
		},
		{
			name: "colorless alone",
			raw:  []string{"c", "C"},
			want: []string{"C"},
		},
		{
			name: "colored identity wins over colorless",
			raw:  []string{"C", "B", "R"},
			want: []string{"B", "R"},
		},
		{
			name: "invalid values are discarded",
			raw:  []string{"", "X"},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePublicDeckColors(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("NormalizePublicDeckColors(%v) = %v, want %v", tt.raw, got, tt.want)
			}
			for idx := range tt.want {
				if got[idx] != tt.want[idx] {
					t.Fatalf("NormalizePublicDeckColors(%v)[%d] = %q, want %q", tt.raw, idx, got[idx], tt.want[idx])
				}
			}
		})
	}
}

func TestNormalizePublicDeckColorMode(t *testing.T) {
	tests := map[string]string{
		"":          "includes",
		"includes":  "includes",
		" INCLUDE ": "includes",
		"exact":     "exact",
		"AT_MOST":   "at_most",
		"invalid":   "includes",
	}
	for raw, want := range tests {
		if got := NormalizePublicDeckColorMode(raw); got != want {
			t.Fatalf("NormalizePublicDeckColorMode(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizePublicDeckArchetypes(t *testing.T) {
	got := NormalizePublicDeckArchetypes([]string{
		" voltron ",
		"COMBO",
		"not-supported",
		"Combo",
		"aggro",
	})
	want := []string{"Aggro", "Combo", "Voltron"}
	if len(got) != len(want) {
		t.Fatalf("NormalizePublicDeckArchetypes() = %v, want %v", got, want)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("NormalizePublicDeckArchetypes()[%d] = %q, want %q", idx, got[idx], want[idx])
		}
	}
}

func TestNormalizePublicDeckSort(t *testing.T) {
	tests := map[string]string{
		"":              "recent",
		"recent":        "recent",
		" UPDATED ":     "updated",
		"name":          "name",
		"commander":     "commander",
		"oldest":        "oldest",
		"published_at;": "recent",
	}
	for raw, want := range tests {
		if got := NormalizePublicDeckSort(raw); got != want {
			t.Fatalf("NormalizePublicDeckSort(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestBuildPublicDeckListQueryDefaultsToAllFormatsNewestFirst(t *testing.T) {
	query, args := buildPublicDeckListQuery(PublicDeckFilters{})

	if strings.Contains(query, "AND d.format =") {
		t.Fatalf("blank public deck filters unexpectedly restricted format: %s", query)
	}
	if !strings.Contains(query, "ORDER BY d.published_at DESC NULLS LAST, d.updated_at DESC, d.id DESC") {
		t.Fatalf("default public deck query is not newest-first with a stable tie-breaker: %s", query)
	}
	if !strings.Contains(query, "LIMIT $1") || !strings.Contains(query, "OFFSET $2") {
		t.Fatalf("default public deck query is missing pagination placeholders: %s", query)
	}
	if len(args) != 2 || args[0] != 60 || args[1] != 0 {
		t.Fatalf("default public deck query args = %#v, want []any{60, 0}", args)
	}
}

func TestBuildPublicDeckListQueryFiltersByDeckName(t *testing.T) {
	query, args := buildPublicDeckListQuery(PublicDeckFilters{
		DeckName: " artifacts ",
	})

	if !strings.Contains(query, "d.name ILIKE '%' || $1 || '%'") {
		t.Fatalf("public deck name search is missing its parameterized condition: %s", query)
	}
	if !strings.Contains(query, "LIMIT $2") || !strings.Contains(query, "OFFSET $3") {
		t.Fatalf("public deck name search has incorrect pagination placeholders: %s", query)
	}
	if len(args) != 3 || args[0] != "artifacts" || args[1] != 60 || args[2] != 0 {
		t.Fatalf("public deck name search args = %#v, want []any{\"artifacts\", 60, 0}", args)
	}
}

func TestBuildPublicDeckListQueryFiltersByAnyExactArchetypeTag(t *testing.T) {
	query, args := buildPublicDeckListQuery(PublicDeckFilters{
		Archetypes: []string{" voltron ", "COMBO", "combo"},
	})

	wantClause := "EXISTS (SELECT 1 FROM unnest(string_to_array(COALESCE(d.tags, ''), ',')) AS stored_tag(tag), unnest($1::text[]) AS selected_tag(tag) WHERE lower(btrim(stored_tag.tag)) = lower(selected_tag.tag))"
	if !strings.Contains(query, wantClause) {
		t.Fatalf("public deck archetype filter is missing its exact match-any tag condition: %s", query)
	}
	if !strings.Contains(query, "LIMIT $2") || !strings.Contains(query, "OFFSET $3") {
		t.Fatalf("public deck archetype filter has incorrect pagination placeholders: %s", query)
	}
	if len(args) != 3 || args[1] != 60 || args[2] != 0 {
		t.Fatalf("public deck archetype args = %#v, want archetype array, limit, and offset", args)
	}
	if got := publicDeckArrayArg(t, args[0]); got != `{"Combo","Voltron"}` {
		t.Fatalf("public deck archetype array = %q, want %q", got, `{"Combo","Voltron"}`)
	}
}

func TestBuildPublicDeckListQueryIgnoresUnsupportedArchetype(t *testing.T) {
	query, args := buildPublicDeckListQuery(PublicDeckFilters{
		Archetypes: []string{"not-a-real-archetype"},
	})

	if strings.Contains(query, "unnest(string_to_array") {
		t.Fatalf("unsupported archetype unexpectedly reached public deck SQL: %s", query)
	}
	if len(args) != 2 || args[0] != 60 || args[1] != 0 {
		t.Fatalf("unsupported archetype query args = %#v, want []any{60, 0}", args)
	}
}

func TestBuildPublicDeckListQueryKeepsArchetypeAndColorFacetsIndependent(t *testing.T) {
	query, args := buildPublicDeckListQuery(PublicDeckFilters{
		Archetypes:    []string{"Combo", "Voltron"},
		ColorIdentity: []string{"U", "R"},
		ColorMode:     "exact",
	})

	if !strings.Contains(query, "unnest($1::text[]) AS selected_tag(tag)") {
		t.Fatalf("combined public deck query is missing archetype array placeholder: %s", query)
	}
	if !strings.Contains(query, "COALESCE(oc.color_identity, ARRAY[]::text[]) @> $2::text[]") ||
		!strings.Contains(query, "COALESCE(oc.color_identity, ARRAY[]::text[]) <@ $2::text[]") {
		t.Fatalf("combined public deck query is missing independent color conditions: %s", query)
	}
	if !strings.Contains(query, "LIMIT $3") || !strings.Contains(query, "OFFSET $4") {
		t.Fatalf("combined public deck query has incorrect pagination placeholders: %s", query)
	}
	if len(args) != 4 {
		t.Fatalf("combined public deck query args = %#v, want archetypes, colors, limit, and offset", args)
	}
	if got := publicDeckArrayArg(t, args[0]); got != `{"Combo","Voltron"}` {
		t.Fatalf("combined public deck archetype array = %q", got)
	}
	if got := publicDeckArrayArg(t, args[1]); got != `{"U","R"}` {
		t.Fatalf("combined public deck color array = %q", got)
	}
}

func TestBuildPublicDeckListQueryColorModes(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		want       []string
		notWant    []string
		wantColors string
	}{
		{
			name:       "includes",
			mode:       "includes",
			want:       []string{"COALESCE(oc.color_identity, ARRAY[]::text[]) @> $1::text[]"},
			notWant:    []string{"COALESCE(oc.color_identity, ARRAY[]::text[]) <@ $1::text[]"},
			wantColors: `{"W","U"}`,
		},
		{
			name: "exact",
			mode: "exact",
			want: []string{
				"COALESCE(oc.color_identity, ARRAY[]::text[]) @> $1::text[]",
				"COALESCE(oc.color_identity, ARRAY[]::text[]) <@ $1::text[]",
			},
			wantColors: `{"W","U"}`,
		},
		{
			name:       "at most",
			mode:       "at_most",
			want:       []string{"COALESCE(oc.color_identity, ARRAY[]::text[]) <@ $1::text[]"},
			notWant:    []string{"COALESCE(oc.color_identity, ARRAY[]::text[]) @> $1::text[]"},
			wantColors: `{"W","U"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := buildPublicDeckListQuery(PublicDeckFilters{
				ColorIdentity: []string{"U", "W"},
				ColorMode:     tt.mode,
			})
			for _, snippet := range tt.want {
				if !strings.Contains(query, snippet) {
					t.Fatalf("public deck query missing %q: %s", snippet, query)
				}
			}
			for _, snippet := range tt.notWant {
				if strings.Contains(query, snippet) {
					t.Fatalf("public deck query unexpectedly contains %q: %s", snippet, query)
				}
			}
			if len(args) != 3 {
				t.Fatalf("public deck color query args = %#v, want color array, limit, and offset", args)
			}
			if got := publicDeckArrayArg(t, args[0]); got != tt.wantColors {
				t.Fatalf("public deck color array = %q, want %q", got, tt.wantColors)
			}
		})
	}
}

func TestBuildPublicDeckListQueryColorless(t *testing.T) {
	query, args := buildPublicDeckListQuery(PublicDeckFilters{
		ColorIdentity: []string{"C"},
		ColorMode:     "exact",
	})

	if !strings.Contains(query, "COALESCE(array_length(oc.color_identity, 1), 0) = 0") {
		t.Fatalf("colorless public deck query is missing empty-identity condition: %s", query)
	}
	if strings.Contains(query, "::text[]") {
		t.Fatalf("colorless public deck query unexpectedly used an array comparison: %s", query)
	}
	if len(args) != 2 || args[0] != 60 || args[1] != 0 {
		t.Fatalf("colorless public deck query args = %#v, want limit and offset", args)
	}
}

func TestBuildPublicDeckListQueryNormalizesPagination(t *testing.T) {
	query, args := buildPublicDeckListQuery(PublicDeckFilters{
		Limit:  500,
		Offset: -10,
	})
	if !strings.Contains(query, "LIMIT $1") || !strings.Contains(query, "OFFSET $2") {
		t.Fatalf("public deck query is missing pagination placeholders: %s", query)
	}
	if len(args) != 2 || args[0] != 200 || args[1] != 0 {
		t.Fatalf("normalized public deck pagination args = %#v, want []any{200, 0}", args)
	}
}

func TestBuildPublicDeckListQueryUsesAllowlistedSort(t *testing.T) {
	query, _ := buildPublicDeckListQuery(PublicDeckFilters{Sort: "updated; DROP TABLE decks"})
	if !strings.Contains(query, "ORDER BY d.published_at DESC NULLS LAST, d.updated_at DESC, d.id DESC") {
		t.Fatalf("invalid sort did not fall back to recent: %s", query)
	}
	if strings.Contains(query, "DROP TABLE") {
		t.Fatalf("raw sort reached public deck SQL: %s", query)
	}

	query, _ = buildPublicDeckListQuery(PublicDeckFilters{Sort: "commander"})
	if !strings.Contains(query, "ORDER BY lower(COALESCE(d.commander_name, '')) ASC, lower(d.name) ASC, d.id ASC") {
		t.Fatalf("commander sort missing expected order: %s", query)
	}
}

func publicDeckArrayArg(t *testing.T, arg any) string {
	t.Helper()
	valuer, ok := arg.(driver.Valuer)
	if !ok {
		t.Fatalf("array argument %T does not implement driver.Valuer", arg)
	}
	value, err := valuer.Value()
	if err != nil {
		t.Fatalf("array argument Value(): %v", err)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("array argument value = %#v, want string", value)
	}
	return text
}
