package web

import (
	"strings"
	"testing"

	"manatomb/app/internal/account"
)

func TestSettingsTemplateUsesFlatThemeAwareAccountLayout(t *testing.T) {
	user := &account.User{
		ID:          42,
		DisplayName: "Theme Tester",
		Email:       "theme@example.com",
		SiteTheme:   string(siteThemeTomb),
	}
	body := renderTemplate(t, "settings", TemplateData{
		CurrentUser: user,
		Data: settingsFormData{
			DisplayName: user.DisplayName,
			Email:       user.Email,
			SiteTheme:   siteThemeTomb,
			Themes:      selectableSiteThemes(),
		},
	})

	for _, needle := range []string{
		`href="/assets/settings.css"`,
		`class="mt-settings-page"`,
		`name="action" value="update_theme"`,
		`name="action" value="update_profile"`,
		`name="action" value="change_password"`,
		`name="action" value="delete_account"`,
		`name="site_theme"`,
		`data-theme-preview=`,
		`data-theme-settings-status`,
		`form.requestSubmit()`,
		`aria-label="Back to profile"`,
		`> Back to Profile`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("settings template missing %q", needle)
		}
	}

	mainStart := strings.Index(body, `<div class="mt-settings-page">`)
	if mainStart < 0 {
		t.Fatalf("settings main region not found")
	}
	mainEnd := strings.Index(body[mainStart:], `<footer id="site-footer"`)
	if mainEnd < 0 {
		t.Fatalf("settings main region not found")
	}
	mainHTML := body[mainStart : mainStart+mainEnd]
	for _, forbidden := range []string{"mt-panel", "mt-stat-card", "text-slate-", "bg-slate-", ">Account</p>", ">View Profile</a>", ">Save Appearance</button>"} {
		if strings.Contains(mainHTML, forbidden) {
			t.Fatalf("settings template retained legacy/nested presentation %q", forbidden)
		}
	}
}

func TestSettingsTemplateMarksOnlyPersistedThemeChecked(t *testing.T) {
	user := &account.User{ID: 7, DisplayName: "Player", Email: "p@example.com", SiteTheme: string(siteThemeTomb)}
	body := renderTemplate(t, "settings", TemplateData{
		CurrentUser: user,
		Data: settingsFormData{
			DisplayName: user.DisplayName,
			Email:       user.Email,
			SiteTheme:   siteThemeTomb,
			Themes:      selectableSiteThemes(),
		},
	})

	if got := strings.Count(body, " checked"); got != 1 {
		t.Fatalf("checked theme count = %d, want 1", got)
	}
	if !strings.Contains(body, `value="tomb" checked`) {
		t.Fatalf("persisted theme was not checked")
	}
}
