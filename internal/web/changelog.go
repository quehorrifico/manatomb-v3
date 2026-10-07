package web

import "net/http"

// currentSiteVersion is the single source of truth for the version shown in
// the footer and on the public changelog. Add future releases to
// changelogReleases newest-first, then update this value to match the first
// release.
const currentSiteVersion = "2.0"

type changelogRelease struct {
	Version   string
	Title     string
	DateISO   string
	DateLabel string
	Summary   string
}

type changelogPageData struct {
	CurrentVersion string
	Releases       []changelogRelease
}

var changelogReleases = []changelogRelease{
	{
		Version:   "2.0",
		Title:     "Commander playtesting against Forge",
		DateISO:   "2026-09-22",
		DateLabel: "September 22, 2026",
		Summary:   "Play one human-versus-CPU Commander game using custom decks or five inspectable opponents, with Forge handling rules and CPU decisions. Start from saved decks or builder drafts, reconnect to a live session, and return to your deck. CPU play requires the separately configured engine service and deliberate enablement; the initial site-wide capacity is one game. Commander-name matching and saved-note recovery are also improved.",
	},
	{
		Version:   "1.0",
		Title:     "The first complete ManaTomb release",
		DateISO:   "2026-08-25",
		DateLabel: "August 25, 2026",
		Summary:   "ManaTomb 1.0 brings card discovery, deck building and sharing, player profiles, and all three games into one cohesive, responsive experience.",
	},
}

func currentChangelog() changelogPageData {
	releases := append([]changelogRelease(nil), changelogReleases...)
	return changelogPageData{
		CurrentVersion: currentSiteVersion,
		Releases:       releases,
	}
}

func (a *App) HandleChangelog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.Renderer.Render(w, "changelog", TemplateData{
		CurrentUser: CurrentUser(r),
		Flash:       readFlash(w, r),
		Data:        currentChangelog(),
	})
}
