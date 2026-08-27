# ManaTomb releases

The public changelog lives at `/changelog`. Its release data and the current
site version live together in `internal/web/changelog.go` so the footer and
page cannot drift apart.

## Version scheme

- Feature releases increment the minor number: `1.1`, `1.2`, `1.3`.
- Smaller follow-up releases add a patch number: `1.2.1`, `1.2.2`, `1.2.3`.
- Major product milestones increment the first number when the scope warrants
  it.

## Publishing an entry

1. Add a `changelogRelease` to the beginning of `changelogReleases`.
2. Keep the entry concise and group user-facing changes by area.
3. Set `currentSiteVersion` to the new entry's exact version.
4. Add or update the changelog tests when the public structure changes.

Do not create a second version constant elsewhere in the app. The shared
footer reads `currentSiteVersion` through the renderer's `siteVersion`
template helper.
