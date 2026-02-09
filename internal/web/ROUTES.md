# Web Route and Feature Index

This file is a quick map of HTTP surface area in `internal/web` so new modules
can be added without hunting through handler files.

## Route Groups

### Home and auth
- `/` -> `HandleHome`
- `/signup` (GET/POST) -> `HandleSignupShow` / `HandleSignupPost`
- `/login` (GET/POST) -> `HandleLoginShow` / `HandleLoginPost`
- `/logout` -> `HandleLogout`
- File: `internal/web/auth.go`

### Settings
- `/settings` (GET/POST) -> `HandleSettingsShow` / `HandleSettingsPost`
- File: `internal/web/settings.go`

### Decks
- `/decks` -> list
- `/decks/new` (GET/POST) -> deck-builder entry and commander save flow
- `/decks/{id}` -> deck detail and add/remove cards
- `/decks/edit` (GET/POST) -> deck metadata editing
- `/decks/delete` -> delete deck
- `/decks/commander` -> update commander from cards in deck
- `/decks/import-text` -> import from pasted text
- `/decks/import-draft` -> guest draft save-to-account endpoint
- `/decks/analytics` -> deck analytics JSON (saved deck or guest draft payload)
- `/decks/create-from-commander` -> commander search handoff
- `/decks/guest` -> guest deck builder
- `/decks/sandbox` -> sandbox WIP page
- `/decks/playtest/{id}` -> playtest deck
- Files:
  - `internal/web/decks_helpers.go`
  - `internal/web/decks_create_list.go`
  - `internal/web/decks_show_edit.go`
  - `internal/web/decks_import.go`
  - `internal/web/decks_playtest.go`

### Cards and commanders
- `/cards/search` -> card search
- `/cards/add-to-deck` -> add card to saved deck
- `/commanders/search` -> commander search
- File: `internal/web/cards.go`

### Rules
- `/rules` -> rulings home
- File: `internal/web/rules.go`

### Public decks
- `/decks/public` -> public decks landing
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
