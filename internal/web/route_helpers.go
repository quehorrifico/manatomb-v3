package web

import (
	"net/url"
	"strings"
)

type deckWorkbenchOptions struct {
	Format        string
	CommanderName string
	Sandbox       bool
	SaveWorkbench bool
	Reset         bool
}

func normalizeLocalReturnPath(raw, fallback string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return fallback
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return fallback
	}
	return path
}

func canonicalizeLocalReturnPath(raw, fallback string) string {
	path := normalizeLocalReturnPath(raw, fallback)
	parsed, err := url.Parse(path)
	if err != nil {
		return fallback
	}
	if parsed.Scheme != "" || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return fallback
	}

	switch parsed.Path {
	case "/decks/edit":
		parsed.Path = "/decks/settings"
	case "/decks/new/commander":
		parsed.Path = "/decks/new/commander/"
	case "/decks/workbench", "/decks/guest":
		parsed.Path = "/decks/new/workbench"
	case "/decks/sandbox":
		parsed.Path = "/decks/new/sandbox"
	case "/decks/playtest/workbench", "/decks/playtest/guest":
		parsed.Path = "/decks/new/playtest"
	case "/decks/new/leader", "/decks/select-leader", "/decks/create-from-leader":
		parsed.Path = "/decks/new/commander/"
	case "/decks/leader":
		parsed.Path = "/decks/commander"
	case "/cards/search/autocomplete", "/cards/search/deck":
		parsed.Path = "/cards/autocomplete"
	}

	query := parsed.Query()
	if parsed.Path == "/decks/new" && strings.EqualFold(strings.TrimSpace(query.Get("mode")), "commander") {
		query.Del("mode")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func mergeLocalReturnPath(raw, fallback string, updates map[string]string) string {
	path := canonicalizeLocalReturnPath(raw, fallback)
	parsed, err := url.Parse(path)
	if err != nil {
		return fallback
	}
	if parsed.Scheme != "" || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return fallback
	}

	query := parsed.Query()
	for key, value := range updates {
		value = strings.TrimSpace(value)
		if value == "" {
			query.Del(key)
			continue
		}
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func isDeckSettingsPath(path string) bool {
	return strings.HasPrefix(path, "/decks/settings") || strings.HasPrefix(path, "/decks/edit")
}

func commanderSearchReturnLabel(path string) string {
	path = canonicalizeLocalReturnPath(path, "/decks/new")
	switch {
	case isDeckSettingsPath(path):
		return "Back to Deck Settings"
	case strings.HasPrefix(path, "/decks/"):
		return "Back to Deck"
	default:
		return "Back to Builder"
	}
}

func deckWorkbenchPath(opts deckWorkbenchOptions) string {
	path := "/decks/new/workbench"
	values := url.Values{}

	mode := ""
	if opts.Sandbox {
		mode = "sandbox"
		values.Set("sandbox", "1")
	}

	format := defaultDeckFormat(opts.Format, opts.CommanderName, mode)
	if format != "" {
		values.Set("format", format)
	}

	if commanderName := strings.TrimSpace(opts.CommanderName); commanderName != "" {
		values.Set("commander_name", commanderName)
	}
	if opts.SaveWorkbench {
		values.Set("save_guest", "1")
	}
	if opts.Reset {
		values.Set("reset", "1")
	}

	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}
