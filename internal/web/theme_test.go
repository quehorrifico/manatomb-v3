package web

import (
	"strings"
	"testing"

	"manatomb/app/internal/account"
)

func TestParseSiteThemeAllowlist(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want SiteTheme
		ok   bool
	}{
		{name: "classic", raw: "classic", want: siteThemeClassic, ok: true},
		{name: "tomb", raw: "tomb", want: siteThemeTomb, ok: true},
		{name: "moonlit", raw: "moonlit", want: siteThemeMoonlit, ok: true},
		{name: "verdigris", raw: "verdigris", want: siteThemeVerdigris, ok: true},
		{name: "cinder", raw: "cinder", want: siteThemeCinder, ok: true},
		{name: "ash", raw: "ash", want: siteThemeAsh, ok: true},
		{name: "trimmed and normalized", raw: " TOMB ", want: siteThemeTomb, ok: true},
		{name: "blank", raw: "", ok: false},
		{name: "unknown", raw: "custom-css", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSiteTheme(tt.raw)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("parseSiteTheme(%q) = (%q, %t), want (%q, %t)", tt.raw, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSelectableSiteThemeCatalogIsUniqueAndParseable(t *testing.T) {
	seen := make(map[SiteTheme]bool, len(availableSiteThemes))
	for _, option := range selectableSiteThemes() {
		if option.Value == "" || option.Label == "" || option.Description == "" {
			t.Fatalf("theme catalog contains an incomplete option: %#v", option)
		}
		if seen[option.Value] {
			t.Fatalf("theme catalog contains duplicate value %q", option.Value)
		}
		seen[option.Value] = true
		if got, ok := parseSiteTheme(string(option.Value)); !ok || got != option.Value {
			t.Fatalf("catalog theme %q is not accepted by parser: got (%q, %t)", option.Value, got, ok)
		}
	}
}

func TestResolvedSiteThemePrecedenceAndFallbacks(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		data         TemplateData
		want         SiteTheme
	}{
		{
			name:         "explicit theme wins",
			templateName: "home",
			data: TemplateData{
				Theme:       siteThemeClassic,
				CurrentUser: &account.User{SiteTheme: string(siteThemeTomb)},
			},
			want: siteThemeClassic,
		},
		{
			name:         "signed in preference",
			templateName: "settings",
			data:         TemplateData{CurrentUser: &account.User{SiteTheme: string(siteThemeMoonlit)}},
			want:         siteThemeMoonlit,
		},
		{
			name:         "signed in explicit classic preference is preserved",
			templateName: "settings",
			data:         TemplateData{CurrentUser: &account.User{SiteTheme: string(siteThemeClassic)}},
			want:         siteThemeClassic,
		},
		{
			name:         "invalid signed in preference",
			templateName: "settings",
			data:         TemplateData{CurrentUser: &account.User{SiteTheme: "unknown"}},
			want:         siteThemeTomb,
		},
		{
			name:         "guest home default",
			templateName: "home",
			want:         siteThemeTomb,
		},
		{
			name:         "refreshed guest card page default",
			templateName: "card_show",
			want:         siteThemeTomb,
		},
		{
			name:         "guest advanced card search default",
			templateName: "cards_search",
			want:         siteThemeTomb,
		},
		{
			name:         "guest public decks default",
			templateName: "decks_public",
			want:         siteThemeTomb,
		},
		{
			name:         "guest public deck detail default",
			templateName: "decks_public_show",
			want:         siteThemeTomb,
		},
		{
			name:         "guest deck builder default",
			templateName: "decks_new",
			want:         siteThemeTomb,
		},
		{
			name:         "guest commander picker default",
			templateName: "decks_new_commander",
			want:         siteThemeTomb,
		},
		{
			name:         "guest deck editor default",
			templateName: "deck_show",
			want:         siteThemeTomb,
		},
		{
			name:         "guest sign in default",
			templateName: "login",
			want:         siteThemeTomb,
		},
		{
			name:         "guest sign up default",
			templateName: "signup",
			want:         siteThemeTomb,
		},
		{
			name:         "guest forgot password default",
			templateName: "forgot_password",
			want:         siteThemeTomb,
		},
		{
			name:         "guest reset password default",
			templateName: "reset_password",
			want:         siteThemeTomb,
		},
		{
			name:         "guest guess the card default",
			templateName: "guess_card",
			want:         siteThemeTomb,
		},
		{
			name:         "guest Pack Crack default",
			templateName: "pack_opening",
			want:         siteThemeTomb,
		},
		{
			name:         "all other guest pages use global default",
			templateName: "privacy",
			want:         siteThemeTomb,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvedSiteTheme(tt.templateName, tt.data); got != tt.want {
				t.Fatalf("resolvedSiteTheme(%q) = %q, want %q", tt.templateName, got, tt.want)
			}
		})
	}
}

func TestSignedInThemePreferenceReachesEveryRenderedPage(t *testing.T) {
	for _, option := range selectableSiteThemes() {
		body := renderTemplate(t, "privacy", TemplateData{
			CurrentUser: &account.User{
				ID:        42,
				SiteTheme: string(option.Value),
			},
		})

		htmlTag := renderedOpeningTag(t, body, "html")
		if !strings.Contains(htmlTag, `data-theme="`+string(option.Value)+`"`) {
			t.Fatalf("signed-in preference %q did not reach rendered page: %s", option.Value, htmlTag)
		}
	}
}

func TestEverySelectableThemeDefinesCentralizedPaletteAndPreview(t *testing.T) {
	css := readThemeCSS(t)
	selectors := map[SiteTheme]string{
		siteThemeClassic:   ":root,\nhtml[data-theme=\"classic\"],\n[data-theme-preview=\"classic\"]",
		siteThemeTomb:      "html[data-theme=\"tomb\"],\n[data-theme-preview=\"tomb\"]",
		siteThemeMoonlit:   "html[data-theme=\"moonlit\"],\n[data-theme-preview=\"moonlit\"]",
		siteThemeVerdigris: "html[data-theme=\"verdigris\"],\n[data-theme-preview=\"verdigris\"]",
		siteThemeCinder:    "html[data-theme=\"cinder\"],\n[data-theme-preview=\"cinder\"]",
		siteThemeAsh:       "html[data-theme=\"ash\"],\n[data-theme-preview=\"ash\"]",
	}
	primitives := []string{
		"--mt-palette-bg:",
		"--mt-palette-surface:",
		"--mt-palette-raised:",
		"--mt-palette-border-subtle:",
		"--mt-palette-border:",
		"--mt-palette-text:",
		"--mt-palette-text-soft:",
		"--mt-palette-muted:",
		"--mt-palette-disabled:",
		"--mt-palette-accent:",
		"--mt-palette-accent-strong:",
		"--mt-palette-accent-text:",
		"--mt-palette-positive:",
		"--mt-palette-negative:",
		"--mt-palette-on-accent:",
		"--mt-palette-shadow:",
	}

	for _, option := range selectableSiteThemes() {
		selector, ok := selectors[option.Value]
		if !ok {
			t.Fatalf("theme %q has no palette selector", option.Value)
		}
		rule := renderedCSSRule(t, css, selector)
		for _, primitive := range primitives {
			if !strings.Contains(rule, primitive) {
				t.Fatalf("theme %q palette missing %q: %s", option.Value, primitive, rule)
			}
		}
	}
}

func TestSettingsOffersPersistedThemeChoices(t *testing.T) {
	body := renderTemplate(t, "settings", TemplateData{
		CurrentUser: &account.User{
			ID:          42,
			DisplayName: "Theme Tester",
			Email:       "theme@example.com",
			SiteTheme:   string(siteThemeTomb),
		},
		Data: settingsFormData{
			DisplayName: "Theme Tester",
			Email:       "theme@example.com",
			SiteTheme:   siteThemeTomb,
			Themes:      selectableSiteThemes(),
		},
	})

	for _, needle := range []string{
		`name="action" value="update_theme"`,
		`data-theme-settings-form`,
		`name="site_theme" value="classic"`,
		`name="site_theme" value="tomb" checked`,
		`data-theme-preview="moonlit"`,
		`data-theme-preview="verdigris"`,
		`data-theme-preview="cinder"`,
		`data-theme-preview="ash"`,
		`data-theme-settings-status`,
		`form.requestSubmit()`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("settings theme selector missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, `>Save Appearance</button>`) {
		t.Fatalf("settings theme selector still requires an appearance save button: %s", body)
	}
}
