package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedLayoutUsesManaTombBrand(t *testing.T) {
	body := renderTemplate(t, "login", TemplateData{})
	for _, want := range []string{
		`<title>Sign In | ManaTomb</title>`,
		`<meta property="og:site_name" content="ManaTomb">`,
		`<span class="mt-site-brand__name">ManaTomb</span>`,
		`<a href="/" class="mt-footer-brand text-sm">ManaTomb</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("shared layout missing ManaTomb branding %q: %s", want, body)
		}
	}
}

func TestUserFacingSourcesDoNotUseSpacedManaTombBrand(t *testing.T) {
	withRendererRoot(t)

	forbidden := "Mana" + " Tomb"
	paths := []string{"README.md", "docs", "internal", "cmd"}
	extensions := map[string]bool{
		".css":  true,
		".go":   true,
		".html": true,
		".js":   true,
		".md":   true,
		".tmpl": true,
		".txt":  true,
	}

	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !extensions[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(body), forbidden) {
				t.Errorf("%s still contains the spaced ManaTomb brand", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s for branding: %v", root, err)
		}
	}
}
