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
	"1 - Thematic",
	"2 - Core",
	"3 - Upgraded",
	"4 - Optimized",
	"5 - cEDH",
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

func powerBracketFilterLabels(raw string) []string {
	switch NormalizePowerBracket(raw) {
	case "":
		return nil
	case "1 - Thematic":
		return []string{"1 - Thematic", "1", "Bracket 1", "Power 1", "Thematic", "Casual"}
	case "2 - Core":
		return []string{"2 - Core", "2", "Bracket 2", "Power 2", "Core", "Focused"}
	case "3 - Upgraded":
		return []string{"3 - Upgraded", "3", "Bracket 3", "Power 3", "Upgraded"}
	case "4 - Optimized":
		return []string{"4 - Optimized", "4", "Bracket 4", "Power 4", "Optimized", "Competitive", "High Power", "High-Power", "HighPower"}
	case "5 - cEDH":
		return []string{"5 - cEDH", "5", "Bracket 5", "Power 5", "cEDH", "CEDH", "cedh"}
	default:
		return []string{NormalizePowerBracket(raw)}
	}
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
	case "1", "bracket 1", "power 1", "casual", "thematic", "1 - thematic":
		return "1 - Thematic"
	case "2", "bracket 2", "power 2", "core", "focused", "2 - core":
		return "2 - Core"
	case "3", "bracket 3", "power 3", "upgraded", "3 - upgraded":
		return "3 - Upgraded"
	case "4", "bracket 4", "power 4", "optimized", "high power", "high-power", "highpower", "competitive", "4 - optimized":
		return "4 - Optimized"
	case "5", "bracket 5", "power 5", "cedh", "5 - cedh":
		return "5 - cEDH"
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
