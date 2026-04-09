package decks

import (
	"strconv"
	"strings"
)

var supportedFormats = []string{
	"Commander",
	"Sandbox",
	"Duel Commander",
	"Standard",
	"Pioneer",
	"Modern",
	"Legacy",
	"Vintage",
	"Pauper",
	"Brawl",
	"Historic Brawl",
	"Historic",
	"Explorer",
	"Timeless",
	"Alchemy",
	"Oathbreaker",
	"Premodern",
	"Draft",
	"Sealed",
	"Cube",
	"Casual",
}

var supportedPowerBrackets = []string{
	"",
	"Casual",
	"Upgraded",
	"Focused",
	"High Power",
	"Competitive",
	"cEDH",
}

func SupportedFormats() []string {
	out := make([]string, len(supportedFormats))
	copy(out, supportedFormats)
	return out
}

func SupportedPowerBrackets() []string {
	out := make([]string, len(supportedPowerBrackets))
	copy(out, supportedPowerBrackets)
	return out
}

func NormalizeFormat(raw string) string {
	trimmed := normalizeLooseLabel(raw)
	if trimmed == "" {
		return "Commander"
	}

	switch strings.ToLower(trimmed) {
	case "commander", "edh":
		return "Commander"
	case "sandbox", "casual":
		return "Sandbox"
	case "duel commander", "duel":
		return "Duel Commander"
	case "standard":
		return "Standard"
	case "pioneer":
		return "Pioneer"
	case "modern":
		return "Modern"
	case "legacy":
		return "Legacy"
	case "vintage":
		return "Vintage"
	case "pauper":
		return "Pauper"
	case "brawl":
		return "Brawl"
	case "historic brawl":
		return "Historic Brawl"
	case "historic":
		return "Historic"
	case "explorer":
		return "Explorer"
	case "timeless":
		return "Timeless"
	case "alchemy":
		return "Alchemy"
	case "oathbreaker":
		return "Oathbreaker"
	case "premodern":
		return "Premodern"
	case "draft":
		return "Draft"
	case "sealed":
		return "Sealed"
	case "cube":
		return "Cube"
	default:
		return trimmed
	}
}

func NormalizePowerBracket(raw string) string {
	switch strings.ToLower(normalizeLooseLabel(raw)) {
	case "":
		return ""
	case "casual":
		return "Casual"
	case "upgraded":
		return "Upgraded"
	case "focused":
		return "Focused"
	case "high power", "high-power", "highpower":
		return "High Power"
	case "competitive":
		return "Competitive"
	case "cedh":
		return "cEDH"
	default:
		return normalizeLooseLabel(raw)
	}
}

func normalizeLooseLabel(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	for {
		changed := false

		if unquoted, err := strconv.Unquote(trimmed); err == nil {
			unquoted = strings.TrimSpace(unquoted)
			if unquoted != trimmed {
				trimmed = unquoted
				changed = true
			}
		}

		if len(trimmed) >= 2 {
			if (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') || (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') {
				trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
				changed = true
			} else if strings.HasPrefix(trimmed, `\"`) && strings.HasSuffix(trimmed, `\"`) && len(trimmed) >= 4 {
				trimmed = strings.TrimSpace(trimmed[2 : len(trimmed)-2])
				changed = true
			}
		}

		if !changed {
			return trimmed
		}
	}
}

func FormatRequiresCommander(format string) bool {
	switch NormalizeFormat(format) {
	case "Commander", "Duel Commander", "Brawl", "Historic Brawl", "Oathbreaker":
		return true
	default:
		return false
	}
}

func FormatTargetMainboardSize(format string) int {
	switch NormalizeFormat(format) {
	case "Commander", "Duel Commander":
		return 100
	case "Brawl", "Historic Brawl", "Oathbreaker":
		return 60
	case "Draft", "Sealed":
		return 40
	case "Cube", "Sandbox":
		return 0
	default:
		return 60
	}
}

func FormatEnforcesSingleton(format string) bool {
	switch NormalizeFormat(format) {
	case "Commander", "Duel Commander", "Brawl", "Historic Brawl", "Oathbreaker":
		return true
	default:
		return false
	}
}

func FormatCopyLimit(format string) int {
	switch NormalizeFormat(format) {
	case "Commander", "Duel Commander", "Brawl", "Historic Brawl", "Oathbreaker":
		return 1
	case "Draft", "Sealed", "Cube", "Sandbox":
		return 0
	default:
		return 4
	}
}
