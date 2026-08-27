package decks

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type ImportBoard string

const (
	ImportBoardCommander ImportBoard = "commander"
	ImportBoardMain      ImportBoard = "main"
	ImportBoardSide      ImportBoard = "side"
	ImportBoardMaybe     ImportBoard = "maybe"
)

// ParsedDeckItem is one user-supplied decklist row. Printing metadata remains
// attached to the row so import review can validate it before anything is
// saved.
type ParsedDeckItem struct {
	Name            string
	OracleID        string
	Qty             int
	Board           ImportBoard
	SetCode         string
	CollectorNumber string
	PrintID         string
	Line            int
}

type ParsedDecklist struct {
	Name   string
	Items  []ParsedDeckItem
	Format string
}

var (
	reQtyName            = regexp.MustCompile(`^\s*(\d+)\s*[xX]?\s+(.+?)\s*$`)
	reNameQty            = regexp.MustCompile(`^\s*(.+?)\s+[xX]\s*(\d+)\s*$`)
	reNameOnly           = regexp.MustCompile(`^\s*([^#].+?)\s*$`)
	reSideboardPrefix    = regexp.MustCompile(`(?i)^sb:\s*`)
	rePrintingID         = regexp.MustCompile(`(?i)\s+\{(?:scryfall:)?([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\}\s*$`)
	reUUID               = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	reSetAndCollector    = regexp.MustCompile(`\s+[\(\[]([A-Za-z0-9]{2,8})[\)\]]\s+([^\s]+)\s*(?:\*[A-Za-z]+\*)?\s*$`)
	reSetWithoutCollect  = regexp.MustCompile(`\s+[\(\[]([A-Za-z0-9]{2,8})[\)\]]\s*$`)
	reTrailingInlineNote = regexp.MustCompile(`\s+\^[^^]+\^\s*$`)
	reImportSection      = regexp.MustCompile(`(?i)^\s*(commander(?:s)?|deck|main|mainboard|maindeck|side|sideboard|maybe|maybeboard|considering)(?:\s*\(\s*\d+\s*(?:cards?)?\s*\))?\s*:?\s*$`)
	reCountedSection     = regexp.MustCompile(`(?i)^\s*(?:commander(?:s)?|deck|main|mainboard|maindeck|side|sideboard|maybe|maybeboard|considering)\s*\(\s*\d+\s*(?:cards?)?\s*\)\s*:?\s*$`)
)

func normalizeImportBoard(raw ImportBoard) ImportBoard {
	switch strings.ToLower(strings.TrimSpace(string(raw))) {
	case "commander", "commanders":
		return ImportBoardCommander
	case "side", "sideboard":
		return ImportBoardSide
	case "maybe", "maybeboard", "considering":
		return ImportBoardMaybe
	default:
		return ImportBoardMain
	}
}

func importSectionHeader(line string) (ImportBoard, bool) {
	match := reImportSection.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}

	switch strings.ToLower(strings.TrimSpace(match[1])) {
	case "commander", "commanders":
		return ImportBoardCommander, true
	case "deck", "main", "mainboard", "maindeck":
		return ImportBoardMain, true
	case "side", "sideboard":
		return ImportBoardSide, true
	case "maybe", "maybeboard", "considering":
		return ImportBoardMaybe, true
	default:
		return "", false
	}
}

func isImportMetadataLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(lower, "name:") || strings.HasPrefix(lower, "format:") || strings.HasPrefix(lower, "commander:")
}

// exportedDeckTitleLine recognizes ManaTomb's text-export envelope without
// guessing that the first row in an ordinary quantity-free decklist is a title.
// ManaTomb exports place one title before metadata and/or a counted board
// heading, so a counted heading acts as the format signature.
func exportedDeckTitleLine(input string) (string, int) {
	lines := strings.Split(input, "\n")
	first := -1
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		first = index
		break
	}
	if first < 0 || isImportMetadataLine(lines[first]) {
		return "", -1
	}

	sawMetadata := false
	for index := first + 1; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if isImportMetadataLine(line) {
			sawMetadata = true
			continue
		}
		if reCountedSection.MatchString(line) {
			firstLine := strings.TrimSpace(lines[first])
			if sawMetadata || !reImportSection.MatchString(firstLine) {
				return firstLine, first + 1
			}
		}
		break
	}
	return "", -1
}

func stripInlineImportComment(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	// A hash preceded by whitespace is a decklist comment. This avoids
	// truncating unusual card names that may contain punctuation.
	if idx := strings.Index(value, " #"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return reTrailingInlineNote.ReplaceAllString(value, "")
}

func parseImportedCardIdentity(raw string) (name, setCode, collectorNumber, printID string) {
	name = stripInlineImportComment(raw)
	if name == "" {
		return "", "", "", ""
	}

	if match := rePrintingID.FindStringSubmatch(name); match != nil {
		printID = strings.ToLower(strings.TrimSpace(match[1]))
		name = strings.TrimSpace(rePrintingID.ReplaceAllString(name, ""))
	}
	if match := reSetAndCollector.FindStringSubmatch(name); match != nil {
		setCode = strings.ToUpper(strings.TrimSpace(match[1]))
		collectorNumber = strings.TrimSpace(match[2])
		name = strings.TrimSpace(reSetAndCollector.ReplaceAllString(name, ""))
	} else if match := reSetWithoutCollect.FindStringSubmatch(name); match != nil {
		setCode = strings.ToUpper(strings.TrimSpace(match[1]))
		name = strings.TrimSpace(reSetWithoutCollect.ReplaceAllString(name, ""))
	}

	return strings.TrimSpace(name), setCode, collectorNumber, printID
}

func parseImportedDeckRow(line string, board ImportBoard, lineNumber int) (ParsedDeckItem, bool) {
	board = normalizeImportBoard(board)
	if reSideboardPrefix.MatchString(line) {
		board = ImportBoardSide
		line = reSideboardPrefix.ReplaceAllString(line, "")
	}

	var qty int
	var identity string
	if match := reQtyName.FindStringSubmatch(line); match != nil {
		qty, _ = strconv.Atoi(match[1])
		identity = strings.TrimSpace(match[2])
	} else if match := reNameQty.FindStringSubmatch(line); match != nil {
		qty, _ = strconv.Atoi(match[2])
		identity = strings.TrimSpace(match[1])
	} else if match := reNameOnly.FindStringSubmatch(line); match != nil {
		qty = 1
		identity = strings.TrimSpace(match[1])
	}

	name, setCode, collectorNumber, printID := parseImportedCardIdentity(identity)
	if name == "" {
		return ParsedDeckItem{}, false
	}
	return ParsedDeckItem{
		Name:            name,
		Qty:             qty,
		Board:           board,
		SetCode:         setCode,
		CollectorNumber: collectorNumber,
		PrintID:         printID,
		Line:            lineNumber,
	}, true
}

func finishParsedDecklist(result ParsedDecklist) (ParsedDecklist, error) {
	if len(result.Items) == 0 {
		return ParsedDecklist{}, fmt.Errorf("no card rows were found in that decklist")
	}

	if result.Format == "" {
		result.Format = "Sandbox"
		for _, item := range result.Items {
			if item.Board == ImportBoardCommander {
				result.Format = "Commander"
				break
			}
		}
	}
	return result, nil
}

func normalizeImportFormat(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	format := NormalizeFormat(raw)
	if FormatIsBuildable(format) {
		return format
	}
	return ""
}

func normalizeCSVHeader(raw string) string {
	raw = strings.TrimPrefix(raw, "\ufeff")
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
}

func decklistCSVColumns(record []string) (map[string]int, bool) {
	columns := make(map[string]int, len(record))
	for index, value := range record {
		columns[normalizeCSVHeader(value)] = index
	}
	_, hasBoard := columns["board"]
	_, hasQuantity := columns["quantity"]
	_, hasName := columns["name"]
	return columns, hasBoard && hasQuantity && hasName
}

func csvValue(record []string, columns map[string]int, names ...string) string {
	for _, name := range names {
		if index, ok := columns[name]; ok && index >= 0 && index < len(record) {
			return strings.TrimSpace(record[index])
		}
	}
	return ""
}

func isDecklistCSV(input string) bool {
	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(input)))
	reader.FieldsPerRecord = -1
	record, err := reader.Read()
	if err != nil {
		return false
	}
	_, ok := decklistCSVColumns(record)
	return ok
}

func parseDecklistCSV(input string) (ParsedDecklist, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(input)))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return ParsedDecklist{}, fmt.Errorf("could not read decklist CSV")
	}
	columns, ok := decklistCSVColumns(header)
	if !ok {
		return ParsedDecklist{}, fmt.Errorf("decklist CSV must include Board, Quantity, and Name columns")
	}

	result := ParsedDecklist{}
	recordNumber := 1
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		recordNumber++
		if readErr != nil {
			return ParsedDecklist{}, fmt.Errorf("could not read decklist CSV row %d", recordNumber)
		}
		if result.Name == "" {
			result.Name = csvValue(record, columns, "deck name")
		}
		if result.Format == "" {
			result.Format = normalizeImportFormat(csvValue(record, columns, "format"))
		}

		name := csvValue(record, columns, "name")
		if name == "" {
			continue
		}
		quantityValue := csvValue(record, columns, "quantity")
		quantity, quantityErr := strconv.Atoi(quantityValue)
		if quantityErr != nil {
			return ParsedDecklist{}, fmt.Errorf("decklist CSV row %d has an invalid quantity", recordNumber)
		}

		printID := strings.ToLower(csvValue(record, columns, "print id"))
		if printID != "" && !reUUID.MatchString(printID) {
			return ParsedDecklist{}, fmt.Errorf("decklist CSV row %d has an invalid Print ID", recordNumber)
		}

		result.Items = append(result.Items, ParsedDeckItem{
			Name:            name,
			Qty:             quantity,
			Board:           normalizeImportBoard(ImportBoard(csvValue(record, columns, "board"))),
			SetCode:         strings.ToUpper(csvValue(record, columns, "set", "set code")),
			CollectorNumber: csvValue(record, columns, "collector number", "collector"),
			PrintID:         printID,
			Line:            recordNumber,
		})
	}

	return finishParsedDecklist(result)
}

func parseDecklistPlainText(input string) (ParsedDecklist, error) {
	title, titleLine := exportedDeckTitleLine(input)
	result := ParsedDecklist{Name: title}
	currentBoard := ImportBoardMain
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || lineNumber == titleLine {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "#") || strings.HasPrefix(lower, "//") {
			continue
		}
		if strings.HasPrefix(lower, "format:") {
			result.Format = normalizeImportFormat(line[len("format:"):])
			continue
		}
		if strings.HasPrefix(lower, "name:") {
			result.Name = strings.TrimSpace(line[len("name:"):])
			continue
		}

		if strings.HasPrefix(lower, "commander:") {
			value := strings.TrimSpace(line[len("commander:"):])
			if value == "" {
				currentBoard = ImportBoardCommander
				continue
			}
			if item, ok := parseImportedDeckRow(value, ImportBoardCommander, lineNumber); ok {
				result.Items = append(result.Items, item)
			}
			currentBoard = ImportBoardMain
			continue
		}
		if board, ok := importSectionHeader(line); ok {
			currentBoard = board
			continue
		}

		if item, ok := parseImportedDeckRow(line, currentBoard, lineNumber); ok {
			result.Items = append(result.Items, item)
			if currentBoard == ImportBoardCommander {
				currentBoard = ImportBoardMain
			}
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return ParsedDecklist{}, fmt.Errorf("could not read decklist")
	}
	return finishParsedDecklist(result)
}

// ParseDecklist auto-detects ManaTomb CSV exports and otherwise parses common
// Arena, Moxfield, Archidekt, and plain-text shapes. It does not resolve card
// names; resolution remains a separate review step.
func ParseDecklist(input string) (ParsedDecklist, error) {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.TrimSpace(input)
	if input == "" {
		return ParsedDecklist{}, fmt.Errorf("paste a decklist to import")
	}
	if isDecklistCSV(input) {
		return parseDecklistCSV(input)
	}
	return parseDecklistPlainText(input)
}

// ParseDecklistText remains source-compatible with existing callers while now
// accepting either pasted plain text or a pasted ManaTomb CSV export.
func ParseDecklistText(input string) (ParsedDecklist, error) {
	return ParseDecklist(input)
}

// ParseCommanderDecklistText is retained for callers that only need the old
// commander-plus-cards shape. New import code should use ParseDecklistText.
func ParseCommanderDecklistText(input string) (commander string, cardsOut []ParsedDeckItem, err error) {
	parsed, err := ParseDecklistText(input)
	if err != nil {
		return "", nil, err
	}

	for _, item := range parsed.Items {
		if item.Board == ImportBoardCommander && commander == "" {
			commander = item.Name
			continue
		}
		cardsOut = append(cardsOut, item)
	}
	return strings.TrimSpace(commander), cardsOut, nil
}
