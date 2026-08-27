package web

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strings"
)

var sitemapPublicPaths = []string{
	"/",
	"/cards/search",
	"/decks/public",
	"/games/guess-card",
	"/games/spellify",
	"/games/pack-opening",
	"/changelog",
	"/privacy",
	"/terms",
}

var robotsDisallowedPaths = []string{
	"/settings",
	"/login",
	"/signup",
	"/logout",
	"/forgot-password",
	"/reset-password",
	"/profile/",
	"/decks/new",
	"/decks/import",
	"/decks/settings",
	"/decks/edit",
	"/decks/delete",
	"/decks/analytics",
	"/decks/quick-build",
	"/decks/commander",
	"/decks/playtest",
	"/cards/autocomplete",
	"/cards/resolve",
	"/cards/versions",
	"/cards/favorites",
	"/healthz",
}

func allowSEORead(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func (a *App) HandleRobotsTXT(w http.ResponseWriter, r *http.Request) {
	if !allowSEORead(w, r) {
		return
	}

	var body strings.Builder
	body.WriteString("User-agent: *\nAllow: /\n")
	for _, path := range robotsDisallowedPaths {
		body.WriteString("Disallow: ")
		body.WriteString(path)
		body.WriteByte('\n')
	}
	if sitemapURL := absoluteSiteURL(a.PublicBaseURL, "/sitemap.xml"); sitemapURL != "" {
		body.WriteString("Sitemap: ")
		body.WriteString(sitemapURL)
		body.WriteByte('\n')
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(body.String()))
}

func (a *App) HandleSitemapXML(w http.ResponseWriter, r *http.Request) {
	if !allowSEORead(w, r) {
		return
	}

	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	body.WriteString("\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, path := range sitemapPublicPaths {
		location := absoluteSiteURL(a.PublicBaseURL, path)
		if location == "" {
			continue
		}
		body.WriteString("  <url><loc>")
		_ = xml.EscapeText(&body, []byte(location))
		body.WriteString("</loc></url>\n")
	}
	body.WriteString("</urlset>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(body.Bytes())
}
