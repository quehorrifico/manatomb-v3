package web

import (
	"os"
	"strings"
	"testing"
)

func cssRuleSource(t *testing.T, source, selector string) string {
	t.Helper()
	start := strings.Index(source, selector)
	if start < 0 {
		t.Fatalf("shared layout is missing CSS rule %q", selector)
	}
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("shared layout CSS rule %q has no opening brace", selector)
	}
	end := strings.Index(source[start+open:], "}")
	if end < 0 {
		t.Fatalf("shared layout CSS rule %q has no closing brace", selector)
	}
	return source[start : start+open+end+1]
}

func TestSignedInSharedWidgetsUseThemeRoles(t *testing.T) {
	sourceBytes, err := os.ReadFile("templates/layout_header.html.tmpl")
	if err != nil {
		t.Fatalf("read shared layout: %v", err)
	}
	source := string(sourceBytes)

	selectors := []string{
		".mt-heart-button {",
		".mt-heart-button:hover {",
		".mt-heart-button.is-loved {",
		".mt-art-tile__frame {",
		".mt-art-tile__fallback {",
		".mt-art-tile__badge {",
		".mt-art-tile__menu .mt-action-menu > summary,",
		"\n    .mt-art-tile__label {",
	}
	for _, selector := range selectors {
		rule := cssRuleSource(t, source, selector)
		if !strings.Contains(rule, "var(--mt-") {
			t.Fatalf("shared signed-in widget rule %q bypasses theme roles: %s", selector, rule)
		}
	}
}
