package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedLayoutOwnsTheOnlyMainLandmark(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("templates", "*.html.tmpl"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	for _, path := range paths {
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		openingCount := strings.Count(string(source), "<main")
		closingCount := strings.Count(string(source), "</main>")
		switch filepath.Base(path) {
		case "layout_header.html.tmpl":
			if openingCount != 1 || closingCount != 0 {
				t.Fatalf("shared header main tags = (%d open, %d close), want (1, 0)", openingCount, closingCount)
			}
		case "layout_footer.html.tmpl":
			if openingCount != 0 || closingCount != 1 {
				t.Fatalf("shared footer main tags = (%d open, %d close), want (0, 1)", openingCount, closingCount)
			}
		default:
			if openingCount != 0 || closingCount != 0 {
				t.Fatalf("%s declares nested main tags = (%d open, %d close)", path, openingCount, closingCount)
			}
		}
	}
}

func TestSharedLayoutProvidesSkipNavigationAndBrowserIdentity(t *testing.T) {
	body := renderTemplate(t, "login", TemplateData{})
	for _, needle := range []string{
		`<meta name="theme-color" content="#0b1512">`,
		`getPropertyValue('--mt-palette-bg')`,
		`<link rel="icon" href="/assets/manatomb-square-logo.svg" type="image/svg+xml">`,
		`<a class="mt-skip-link" href="#main-content">Skip to content</a>`,
		`<main id="main-content" class="flex-1" tabindex="-1">`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("shared layout missing %q", needle)
		}
	}
}
