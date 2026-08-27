package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestChangelogVersionDataStaysConsistent(t *testing.T) {
	page := currentChangelog()
	if len(page.Releases) == 0 {
		t.Fatal("changelog has no releases")
	}
	if page.CurrentVersion != page.Releases[0].Version {
		t.Fatalf("current version %q does not match newest release %q", page.CurrentVersion, page.Releases[0].Version)
	}

	versionPattern := regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)
	seen := make(map[string]bool, len(page.Releases))
	for _, release := range page.Releases {
		if !versionPattern.MatchString(release.Version) {
			t.Fatalf("release version %q does not follow major.minor[.patch]", release.Version)
		}
		if seen[release.Version] {
			t.Fatalf("duplicate changelog version %q", release.Version)
		}
		seen[release.Version] = true
		if release.Title == "" || release.DateISO == "" || release.DateLabel == "" || release.Summary == "" {
			t.Fatalf("release %q is incomplete: %#v", release.Version, release)
		}
	}
}

func TestChangelogRendersFlatPublicReleaseHistory(t *testing.T) {
	body := renderTemplate(t, "changelog", TemplateData{Data: currentChangelog()})
	main := staticPageMain(t, body)

	for _, needle := range []string{
		`data-page="changelog"`,
		`<title>Changelog | ManaTomb</title>`,
		`href="/assets/static_pages.css"`,
		`class="mt-static-title">Changelog</h1>`,
		`class="mt-changelog-current">v1.0<span class="sr-only">, current version</span>`,
		`<article class="mt-changelog-release"`,
		`<time datetime="2026-08-25">August 25, 2026</time>`,
		`ManaTomb 1.0 brings card discovery, deck building and sharing`,
		`href="https://github.com/quehorrifico/manatomb-v3"`,
		`More details on GitHub`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("changelog missing %q: %s", needle, body)
		}
	}
	for _, removed := range []string{`mt-changelog-groups`, `Cards and discovery`, `The foundation`} {
		if strings.Contains(body, removed) {
			t.Fatalf("changelog retained expanded release copy %q: %s", removed, body)
		}
	}
	for _, forbidden := range []string{"mt-panel", "mt-kicker", "bg-slate-", "border-slate-"} {
		if strings.Contains(main, forbidden) {
			t.Fatalf("changelog uses nested or legacy presentation %q: %s", forbidden, main)
		}
	}
}

func TestHandleChangelogSupportsReadsAndRejectsWrites(t *testing.T) {
	app := &App{Renderer: NewRenderer("https://manatomb.app")}

	getRequest := httptest.NewRequest(http.MethodGet, "/changelog", nil)
	getResponse := httptest.NewRecorder()
	app.HandleChangelog(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("GET changelog status = %d: %s", getResponse.Code, getResponse.Body.String())
	}
	for _, needle := range []string{
		`<link rel="canonical" href="https://manatomb.app/changelog">`,
		`<meta name="robots" content="index,follow">`,
	} {
		if !strings.Contains(getResponse.Body.String(), needle) {
			t.Fatalf("GET changelog metadata missing %q", needle)
		}
	}

	postRequest := httptest.NewRequest(http.MethodPost, "/changelog", nil)
	postResponse := httptest.NewRecorder()
	app.HandleChangelog(postResponse, postRequest)
	if postResponse.Code != http.StatusMethodNotAllowed || postResponse.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST changelog = %d Allow=%q", postResponse.Code, postResponse.Header().Get("Allow"))
	}
}
