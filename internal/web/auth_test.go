package web

import "testing"

func TestAuthNextPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "blank falls back", raw: "", want: "/decks"},
		{name: "safe local path", raw: "/decks/public", want: "/decks/public"},
		{name: "trimmed local path", raw: " /settings ", want: "/settings"},
		{name: "reject scheme relative", raw: "//evil.example", want: "/decks"},
		{name: "reject absolute url", raw: "https://evil.example", want: "/decks"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := authNextPath(tc.raw); got != tc.want {
				t.Fatalf("authNextPath(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
