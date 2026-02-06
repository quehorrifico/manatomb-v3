package decks

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ParsedDeckItem struct {
	Name string
	Qty  int
}

var (
	reQtyName                   = regexp.MustCompile(`^\s*(\d+)\s*x?\s+(.+?)\s*$`) // supports "1 Sol Ring" and "1x Sol Ring"
	reNameOnly                  = regexp.MustCompile(`^\s*([^#\\/].+?)\s*$`)       // supports "Sol Ring"
	reSideboardPrefix           = regexp.MustCompile(`(?i)^sb:\s*`)
	reTrailingParenSetCollector = regexp.MustCompile(`\s+\([A-Za-z0-9]{2,8}\)\s+\d+[A-Za-z]?\s*$`)
	reTrailingParenSet          = regexp.MustCompile(`\s+\([A-Za-z0-9]{2,8}\)\s*$`)
	reTrailingBracketSetCollect = regexp.MustCompile(`\s+\[[A-Za-z0-9]{2,8}\]\s+\d+[A-Za-z]?\s*$`)
	reTrailingBracketSet        = regexp.MustCompile(`\s+\[[A-Za-z0-9]{2,8}\]\s*$`)
)

func normalizeImportedCardName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}

	// Remove inline comments for lines like:
	// "1 Sol Ring # ramp"
	if idx := strings.Index(name, "#"); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}

	name = reSideboardPrefix.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Drop common export suffixes:
	// "Swords to Plowshares (CMM) 57", "Island [M21]", etc.
	for {
		next := reTrailingParenSetCollector.ReplaceAllString(name, "")
		next = reTrailingBracketSetCollect.ReplaceAllString(next, "")
		next = reTrailingParenSet.ReplaceAllString(next, "")
		next = reTrailingBracketSet.ReplaceAllString(next, "")
		next = strings.TrimSpace(next)
		if next == name {
			break
		}
		name = next
	}

	return strings.TrimSpace(name)
}

// ParseCommanderDecklistText tries to extract a commander and a list of (qty,name) cards.
// Supported formats (best-effort):
// - "Commander: Atraxa, Grand Unifier" then lines like "1 Sol Ring"
// - "Commander" section header then "1 Atraxa, Grand Unifier"
// - Comments starting with # or //
func ParseCommanderDecklistText(input string) (commander string, cardsOut []ParsedDeckItem, err error) {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil, fmt.Errorf("paste a decklist to import")
	}

	var inCommanderSection bool
	scanner := bufio.NewScanner(strings.NewReader(input))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		low := strings.ToLower(line)
		if strings.HasPrefix(low, "#") || strings.HasPrefix(low, "//") {
			continue
		}

		// Section headers
		if strings.HasPrefix(low, "commander:") {
			commander = normalizeImportedCardName(line[len("commander:"):])
			inCommanderSection = true
			continue
		}
		if low == "commander" || strings.HasPrefix(low, "commander ") {
			inCommanderSection = true
			continue
		}
		if low == "mainboard" || low == "main" || low == "deck" {
			inCommanderSection = false
			continue
		}
		if strings.HasPrefix(low, "sideboard") || strings.HasPrefix(low, "maybeboard") {
			inCommanderSection = false
			continue
		}

		m := reQtyName.FindStringSubmatch(line)
		var qty int
		var name string

		if m != nil {
			qty, _ = strconv.Atoi(m[1])
			name = strings.TrimSpace(m[2])
		} else {
			// Allow a bare commander line if we're in commander section and commander not set.
			if inCommanderSection && commander == "" {
				commander = normalizeImportedCardName(line)
				continue
			}

			// If no qty is provided, treat as 1x card name.
			m2 := reNameOnly.FindStringSubmatch(line)
			if m2 == nil {
				continue
			}
			qty = 1
			name = normalizeImportedCardName(m2[1])
		}
		name = normalizeImportedCardName(name)
		if qty <= 0 || name == "" {
			continue
		}

		// If commander not set and we're in commander section, treat first entry as commander.
		if inCommanderSection && commander == "" {
			commander = name
			continue
		}

		cardsOut = append(cardsOut, ParsedDeckItem{Name: name, Qty: qty})
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return "", nil, fmt.Errorf("could not read decklist")
	}

	commander = strings.TrimSpace(commander)
	return commander, cardsOut, nil
}
