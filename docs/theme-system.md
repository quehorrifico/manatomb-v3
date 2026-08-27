# ManaTomb theme system

ManaTomb keeps its site-wide color primitives in
`internal/web/assets/theme.css`. Components should use the derived semantic
roles such as `--mt-bg`, `--mt-surface`, `--mt-text`, `--mt-border`, and
`--mt-accent`; they should not copy a palette's literal colors.

## Adjust an existing theme

Edit only that theme's short `--mt-palette-*` block in `theme.css`. The shared
semantic roles below the palette blocks propagate those values throughout the
site.

Fixed semantic colors are appropriate for content whose color conveys meaning,
including mana colors, legality/error states, deck archetype tags, and the
Magic card back. Ordinary page chrome should always use a `--mt-*` role.

## Add a theme

1. Add its value, label, and description to the catalog in
   `internal/web/theme.go`.
2. Add a paired selector to `theme.css`:

   ```css
   html[data-theme="example"],
   [data-theme-preview="example"] {
     /* Define every --mt-palette-* primitive used by the other themes. */
   }
   ```

3. Run `npm run build:css` and `go test ./...`.

The settings page reads the catalog automatically, so a catalog entry and its
palette block are enough to expose a new persisted choice to signed-in users.
Unknown or missing stored values safely fall back to Tomb Brass.
