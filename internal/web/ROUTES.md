# Web Route and Feature Index

This file is a quick map of HTTP surface area in `internal/web` so new modules
can be added without hunting through handler files.

## Route Groups

### Home and auth
- `/` -> `HandleHome`
- `/healthz` -> `HandleHealthz`
- `/privacy` -> `HandlePrivacy`
- `/terms` -> `HandleTerms`
- `/profile/avatar` (POST) -> choose profile commander-art avatar
- `/signup` (GET/POST) -> `HandleSignupShow` / `HandleSignupPost`
- `/login` (GET/POST) -> `HandleLoginShow` / `HandleLoginPost`
- `/logout` -> `HandleLogout`
- Files: `internal/web/auth.go`, `internal/web/legal.go`

### Settings
- `/settings` (GET/POST) -> `HandleSettingsShow` / `HandleSettingsPost`
- File: `internal/web/settings.go`

### Decks
- `/decks` -> list
- `/decks/new` (GET/POST) -> deck-builder entry and saved deck creation
- `/decks/new/commander/` -> commander deck launcher
- `/decks/new/commander` (POST) -> select a commander and continue to the deck editor
- `/decks/new/workbench` -> local unsaved deck workbench
- `/decks/new/sandbox` -> sandbox shortcut to an empty local workbench
- `/decks/new/playtest` -> playtest the local workbench deck
- `/decks/import` (GET/POST) -> import page and pasted deck text submit
- `/decks/import/save` -> local workbench save-to-account endpoint
- `/decks/{id}` -> deck detail and add/remove cards
- `/decks/settings` (GET/POST) -> deck metadata editing
- `/decks/delete` -> delete deck
- `/decks/commander` -> update a saved deck's commander from cards already in the deck
- `/decks/analytics` -> deck analytics JSON (saved deck or local workbench payload)
- `/decks/quick-build` -> build a commander starter shell for an empty saved or guest deck
- `/decks/playtest/{id}` -> playtest deck
- Compatibility aliases kept for older links:
  `/decks/new/commander/create`, `/decks/new/leader`, `/decks/leader`, `/decks/select-leader`,
  `/decks/create-from-leader`, `/decks/workbench`, `/decks/guest`,
  `/decks/sandbox`, `/decks/import-text`, `/decks/import-draft`,
  `/decks/edit`, `/decks/playtest/workbench`,
  `/decks/playtest/guest`
- Files:
  - `internal/web/decks_helpers.go`
  - `internal/web/route_helpers.go`
  - `internal/web/decks_create_list.go`
  - `internal/web/decks_show_edit.go`
  - `internal/web/decks_quickbuild.go`
  - `internal/web/decks_import.go`
  - `internal/web/decks_playtest.go`

### Cards and commanders
- `/cards` -> quick card results from the home search box
- `/cards/search` -> advanced card search
- `/cards/view/{oracle_id}` -> dedicated card detail page
- `/cards/autocomplete` -> deck-builder autocomplete results
- `/commanders/search` -> commander search
- Compatibility aliases kept for older links:
  `/cards/search/autocomplete`, `/cards/search/deck`
- File: `internal/web/cards.go`

### Rules
- `/rules` -> rulings home
- File: `internal/web/rules.go`

### Public decks
- `/decks/public` -> public deck browse page
- `/decks/public/{slug}` -> public deck detail page
- `/decks/public/fork` -> copy a public deck into the current account
- File: `internal/web/decks_public.go`

## Supporting Features

- Rendering: `internal/web/renderer.go`
- Flash messages: `internal/web/flash.go`
- Middleware and recovery/not-found wrappers: `internal/web/auth.go`
- Templates: `internal/web/templates/*.html.tmpl`

## Expansion Targets

These planned features should get dedicated route groups and handler files:

- Solo and AI playtesting
- Public deck forums and comments
- News and announcements
- Advanced deck statistics and analysis

Suggested convention:
- Keep one route group per file pair: `*_routes` in `cmd/server/main.go` and
  one or more focused handler files in `internal/web`.
- Keep handlers thin; move parsing/domain logic into `internal/<domain>` when
  possible.
