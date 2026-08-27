package decks

import (
	"strconv"
	"strings"
)

type FormatDefinition struct {
	Name                string
	RequiresCommander   bool
	TargetMainboardSize int
	CopyLimit           int
}

var supportedFormatDefinitions = []FormatDefinition{
	{Name: "Commander", RequiresCommander: true, TargetMainboardSize: 100, CopyLimit: 1},
	{Name: "Sandbox", TargetMainboardSize: 0, CopyLimit: 0},
	{Name: "Duel Commander", RequiresCommander: true, TargetMainboardSize: 100, CopyLimit: 1},
	{Name: "Standard", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Pioneer", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Modern", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Legacy", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Vintage", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Pauper", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Brawl", RequiresCommander: true, TargetMainboardSize: 60, CopyLimit: 1},
	{Name: "Historic Brawl", RequiresCommander: true, TargetMainboardSize: 60, CopyLimit: 1},
	{Name: "Historic", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Explorer", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Timeless", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Alchemy", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Oathbreaker", RequiresCommander: true, TargetMainboardSize: 60, CopyLimit: 1},
	{Name: "Premodern", TargetMainboardSize: 60, CopyLimit: 4},
	{Name: "Draft", TargetMainboardSize: 40, CopyLimit: 0},
	{Name: "Sealed", TargetMainboardSize: 40, CopyLimit: 0},
	{Name: "Cube", TargetMainboardSize: 0, CopyLimit: 0},
	{Name: "Casual", TargetMainboardSize: 60, CopyLimit: 4},
}

var buildableFormats = []string{
	"Commander",
	"Standard",
	"Sandbox",
}

var formatAliases = map[string]string{
	"commander":      "Commander",
	"edh":            "Commander",
	"sandbox":        "Sandbox",
	"duel commander": "Duel Commander",
	"duel":           "Duel Commander",
	"standard":       "Standard",
	"pioneer":        "Pioneer",
	"modern":         "Modern",
	"legacy":         "Legacy",
	"vintage":        "Vintage",
	"pauper":         "Pauper",
	"brawl":          "Brawl",
	"historic brawl": "Historic Brawl",
	"historic":       "Historic",
	"explorer":       "Explorer",
	"timeless":       "Timeless",
	"alchemy":        "Alchemy",
	"oathbreaker":    "Oathbreaker",
	"premodern":      "Premodern",
	"draft":          "Draft",
	"sealed":         "Sealed",
	"cube":           "Cube",
	"casual":         "Casual",
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
	out := make([]string, 0, len(supportedFormatDefinitions))
	for _, definition := range supportedFormatDefinitions {
		out = append(out, definition.Name)
	}
	return out
}

func BuildableFormats() []string {
	out := make([]string, len(buildableFormats))
	copy(out, buildableFormats)
	return out
}

func FormatIsBuildable(raw string) bool {
	format := NormalizeFormat(raw)
	for _, buildable := range buildableFormats {
		if buildable == format {
			return true
		}
	}
	return false
}

func FormatDefinitionFor(raw string) FormatDefinition {
	format := NormalizeFormat(raw)
	for _, definition := range supportedFormatDefinitions {
		if definition.Name == format {
			return definition
		}
	}
	return FormatDefinition{
		Name:                format,
		TargetMainboardSize: 60,
		CopyLimit:           4,
	}
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

	if normalized, ok := formatAliases[strings.ToLower(trimmed)]; ok {
		return normalized
	}
	return trimmed
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
	return FormatDefinitionFor(format).RequiresCommander
}

func FormatTargetMainboardSize(format string) int {
	return FormatDefinitionFor(format).TargetMainboardSize
}

func FormatEnforcesSingleton(format string) bool {
	return FormatDefinitionFor(format).CopyLimit == 1
}

func FormatCopyLimit(format string) int {
	return FormatDefinitionFor(format).CopyLimit
}
