package web

import "strings"

type SiteTheme string

const (
	siteThemeClassic   SiteTheme = "classic"
	siteThemeTomb      SiteTheme = "tomb"
	siteThemeMoonlit   SiteTheme = "moonlit"
	siteThemeVerdigris SiteTheme = "verdigris"
	siteThemeCinder    SiteTheme = "cinder"
	siteThemeAsh       SiteTheme = "ash"
)

type siteThemeOption struct {
	Value       SiteTheme
	Label       string
	Description string
}

var availableSiteThemes = []siteThemeOption{
	{
		Value:       siteThemeClassic,
		Label:       "Classic Blue",
		Description: "The original ManaTomb blue and slate palette.",
	},
	{
		Value:       siteThemeTomb,
		Label:       "Tomb Brass",
		Description: "Buried amethyst surfaces with antique brass accents.",
	},
	{
		Value:       siteThemeMoonlit,
		Label:       "Moonlit Plum",
		Description: "Muted plum surfaces with soft moonlit lavender accents.",
	},
	{
		Value:       siteThemeVerdigris,
		Label:       "Verdigris",
		Description: "Deep mineral green with weathered teal accents.",
	},
	{
		Value:       siteThemeCinder,
		Label:       "Cinder",
		Description: "Charred brown surfaces with warm ember accents.",
	},
	{
		Value:       siteThemeAsh,
		Label:       "Ash",
		Description: "A restrained charcoal and silver palette.",
	},
}

func parseSiteTheme(raw string) (SiteTheme, bool) {
	theme := SiteTheme(strings.ToLower(strings.TrimSpace(raw)))
	for _, option := range availableSiteThemes {
		if option.Value == theme {
			return theme, true
		}
	}
	return "", false
}

func normalizedSiteTheme(raw string) SiteTheme {
	if theme, ok := parseSiteTheme(raw); ok {
		return theme
	}
	return siteThemeTomb
}

func selectableSiteThemes() []siteThemeOption {
	return append([]siteThemeOption(nil), availableSiteThemes...)
}

func resolvedSiteTheme(_ string, data TemplateData) SiteTheme {
	if theme, ok := parseSiteTheme(string(data.Theme)); ok {
		return theme
	}
	if data.CurrentUser != nil {
		return normalizedSiteTheme(data.CurrentUser.SiteTheme)
	}
	return siteThemeTomb
}

func resolvedPageID(templateName string, data TemplateData) string {
	if pageID := strings.TrimSpace(data.PageID); pageID != "" {
		return pageID
	}
	if templateName == "home" {
		return "guest-home"
	}
	return strings.ReplaceAll(templateName, "_", "-")
}
