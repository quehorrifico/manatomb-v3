package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

const (
	deckImportMaxBodyBytes = 2 << 20
	deckImportMaxRows      = 500
)

type importDraftCardRequest struct {
	Name             string `json:"name"`
	OracleID         string `json:"oracle_id,omitempty"`
	Qty              int    `json:"qty"`
	PreferredPrintID string `json:"preferred_print_id,omitempty"`
	SetCode          string `json:"set_code,omitempty"`
	CollectorNumber  string `json:"collector_number,omitempty"`
}

type importDraftCardMeta struct {
	CardID                string `json:"cardID"`
	CardIDSnake           string `json:"card_id"`
	OracleID              string `json:"oracleID"`
	OracleIDSnake         string `json:"oracle_id"`
	PreferredPrintID      string `json:"preferredPrintID"`
	PreferredPrintIDSnake string `json:"preferred_print_id"`
	PrintID               string `json:"printID"`
	PrintIDSnake          string `json:"print_id"`
	SetCode               string `json:"setCode"`
	SetCodeSnake          string `json:"set_code"`
	CollectorNumber       string `json:"collectorNumber"`
	CollectorNumberSnake  string `json:"collector_number"`
}

func (m importDraftCardMeta) resolvedOracleID() string {
	for _, value := range []string{m.OracleID, m.OracleIDSnake, m.CardID, m.CardIDSnake} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (m importDraftCardMeta) resolvedPrintID() string {
	for _, value := range []string{m.PreferredPrintID, m.PreferredPrintIDSnake, m.PrintID, m.PrintIDSnake} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (m importDraftCardMeta) resolvedSetCode() string {
	if value := strings.TrimSpace(m.SetCode); value != "" {
		return value
	}
	return strings.TrimSpace(m.SetCodeSnake)
}

func (m importDraftCardMeta) resolvedCollectorNumber() string {
	if value := strings.TrimSpace(m.CollectorNumber); value != "" {
		return value
	}
	return strings.TrimSpace(m.CollectorNumberSnake)
}

type importDraftRequest struct {
	CommanderName    string                         `json:"commander_name"`
	CommanderPrintID string                         `json:"commander_print_id"`
	Name             string                         `json:"name"`
	Description      string                         `json:"description"`
	Tags             string                         `json:"tags"`
	Format           string                         `json:"format"`
	Cards            []importDraftCardRequest       `json:"cards"`
	SideboardCards   []importDraftCardRequest       `json:"sideboard_cards"`
	MaybeCards       []importDraftCardRequest       `json:"maybe_cards"`
	CardMeta         map[string]importDraftCardMeta `json:"card_meta"`
}

type importDraftResponse struct {
	DeckID     int64    `json:"deck_id,omitempty"`
	Error      string   `json:"error,omitempty"`
	Unresolved []string `json:"unresolved,omitempty"`
}

type workbenchImportSeedData struct {
	PayloadJSON template.JS
}

type workbenchImportSeedCard struct {
	Name             string                  `json:"name"`
	OracleID         string                  `json:"oracle_id"`
	Qty              int                     `json:"qty"`
	PreferredPrintID string                  `json:"preferred_print_id,omitempty"`
	SetCode          string                  `json:"set_code,omitempty"`
	CollectorNumber  string                  `json:"collector_number,omitempty"`
	Meta             workbenchImportCardMeta `json:"meta"`
}

type workbenchImportCardMeta struct {
	CardID           string  `json:"cardID"`
	Name             string  `json:"name"`
	ManaCost         string  `json:"manaCost,omitempty"`
	TypeLine         string  `json:"typeLine,omitempty"`
	OracleText       string  `json:"oracleText,omitempty"`
	CMC              float64 `json:"cmc"`
	PriceUSD         string  `json:"priceUSD,omitempty"`
	ImageURI         string  `json:"imageURI,omitempty"`
	PreferredPrintID string  `json:"preferredPrintID,omitempty"`
	PrintID          string  `json:"printID,omitempty"`
	SetCode          string  `json:"setCode,omitempty"`
	SetName          string  `json:"setName,omitempty"`
	CollectorNumber  string  `json:"collectorNumber,omitempty"`
	Rarity           string  `json:"rarity,omitempty"`
	ReleasedAt       string  `json:"releasedAt,omitempty"`
	Artist           string  `json:"artist,omitempty"`
}

type workbenchImportPayload struct {
	CommanderName        string                             `json:"commander_name"`
	CommanderPrintID     string                             `json:"commander_print_id,omitempty"`
	Name                 string                             `json:"name"`
	Description          string                             `json:"description"`
	Format               string                             `json:"format"`
	Cards                []workbenchImportSeedCard          `json:"cards"`
	SideboardCards       []workbenchImportSeedCard          `json:"sideboard_cards,omitempty"`
	MaybeCards           []workbenchImportSeedCard          `json:"maybe_cards,omitempty"`
	CommanderCandidates  []string                           `json:"commander_candidates,omitempty"`
	CardMeta             map[string]workbenchImportCardMeta `json:"card_meta,omitempty"`
	ReplaceExistingDraft bool                               `json:"replace_existing_draft,omitempty"`
	ImportWarnings       []string                           `json:"import_warnings,omitempty"`
}

type deckImportReviewItem struct {
	Index            int
	Line             int
	Qty              int
	Name             string
	Board            string
	BoardLabel       string
	SetCode          string
	CollectorNumber  string
	PrintID          string
	CanonicalName    string
	SuggestedName    string
	Status           string
	StatusLabel      string
	StatusDetail     string
	PreferredPrintID string
	PrintingLabel    string
	NeedsAttention   bool
	OracleID         string
	resolvedCard     cards.DBCard
	resolvedPrinting *cards.Card
}

type deckImportPageData struct {
	Step           string
	Decklist       string
	FieldError     string
	Format         string
	Items          []deckImportReviewItem
	Issues         []string
	CanFinalize    bool
	MainCount      int
	SideCount      int
	MaybeCount     int
	CommanderCount int
	TotalCount     int
}

func importBoardLabel(board string) string {
	switch strings.ToLower(strings.TrimSpace(board)) {
	case "commander":
		return "Commander"
	case "side":
		return "Sideboard"
	case "maybe":
		return "Maybeboard"
	default:
		return "Mainboard"
	}
}

func normalizedImportBoard(raw string) decks.ImportBoard {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "commander":
		return decks.ImportBoardCommander
	case "side", "sideboard":
		return decks.ImportBoardSide
	case "maybe", "maybeboard":
		return decks.ImportBoardMaybe
	default:
		return decks.ImportBoardMain
	}
}

func (a *App) renderDeckImport(w http.ResponseWriter, user *account.User, flash, errMsg string, page deckImportPageData) {
	page.FieldError = strings.TrimSpace(errMsg)
	data := TemplateData{
		CurrentUser: user,
		Data:        page,
		Flash:       flash,
		ActiveNav:   "builder",
		PageID:      "decks-import",
	}
	a.Renderer.Render(w, "decks_import", data)
}

func lookupImportedPrintingID(
	ctx context.Context,
	db *sql.DB,
	oracleID, setCode, collectorNumber string,
) (string, error) {
	var matchedPrintID string
	err := db.QueryRowContext(ctx, `
		SELECT cp.scryfall_id::text
		FROM card_prints cp
		WHERE cp.oracle_id = $1::uuid
		  AND lower(cp.set_code) = lower($2)
		  AND ($3 = '' OR lower(cp.collector_number) = lower($3))
		ORDER BY
			(COALESCE(cp.lang, 'en') = 'en') DESC,
			cp.released_at DESC NULLS LAST,
			cp.collector_number,
			cp.scryfall_id
		LIMIT 1
	`, oracleID, setCode, collectorNumber).Scan(&matchedPrintID)
	if errors.Is(err, sql.ErrNoRows) {
		// A set-only annotation is a preference rather than a hard printing
		// requirement. If that set is unavailable locally, keep the card and
		// let the caller use its canonical/default printing instead.
		if collectorNumber == "" {
			return "", nil
		}
		return "", cards.ErrCardNotFound
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(matchedPrintID), nil
}

func applyCanonicalImportedPrinting(item *deckImportReviewItem, card cards.DBCard) {
	if item == nil {
		return
	}
	item.SetCode = strings.ToUpper(strings.TrimSpace(card.SetCode))
	item.CollectorNumber = strings.TrimSpace(card.CollectorNumber)
	item.PrintingLabel = ""
	if item.SetCode != "" {
		item.PrintingLabel = item.SetCode
		if item.CollectorNumber != "" {
			item.PrintingLabel += " · " + item.CollectorNumber
		}
	}
	if !item.NeedsAttention {
		item.StatusDetail = "Default printing"
	}
}

func (a *App) lookupImportedPrinting(
	ctx context.Context,
	card cards.DBCard,
	printID, setCode, collectorNumber string,
) (*cards.Card, error) {
	printID = strings.TrimSpace(printID)
	setCode = strings.TrimSpace(setCode)
	collectorNumber = strings.TrimSpace(collectorNumber)

	if printID != "" {
		printing, err := cards.GetCardPrintingByID(ctx, a.DB, card.OracleID, printID)
		if err != nil {
			return nil, err
		}
		if setCode != "" && !strings.EqualFold(setCode, printing.SetCode) {
			return nil, cards.ErrCardNotFound
		}
		if collectorNumber != "" && !strings.EqualFold(collectorNumber, printing.CollectorNumber) {
			return nil, cards.ErrCardNotFound
		}
		return printing, nil
	}
	if setCode == "" && collectorNumber == "" {
		return nil, nil
	}
	if setCode == "" {
		return nil, fmt.Errorf("a set code is required when a collector number is supplied")
	}

	matchedPrintID, err := lookupImportedPrintingID(ctx, a.DB, card.OracleID, setCode, collectorNumber)
	if err != nil {
		return nil, err
	}
	if matchedPrintID == "" {
		return nil, nil
	}
	return cards.GetCardPrintingByID(ctx, a.DB, card.OracleID, matchedPrintID)
}

func (a *App) buildDeckImportReview(ctx context.Context, parsed decks.ParsedDecklist, rawDecklist string) (deckImportPageData, error) {
	page := deckImportPageData{
		Step:     "review",
		Decklist: rawDecklist,
		Format:   defaultDeckFormat(parsed.Format, "", "import"),
		Items:    make([]deckImportReviewItem, len(parsed.Items)),
	}

	names := make([]string, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		if strings.TrimSpace(item.Name) != "" {
			names = append(names, item.Name)
		}
	}
	exact, err := cards.LookupCardsByNames(ctx, a.DB, names)
	if err != nil {
		return page, err
	}

	missingNames := make([]string, 0)
	for _, name := range names {
		if _, ok := exact[strings.ToLower(strings.TrimSpace(name))]; !ok {
			missingNames = append(missingNames, name)
		}
	}
	suggestions := map[string]cards.NameResolution{}
	if len(missingNames) > 0 {
		suggestions, err = cards.ResolveCardNamesBatch(ctx, a.DB, missingNames, 0.42)
		if err != nil {
			return page, err
		}
	}

	commanderRows := 0
	globalIssues := make([]string, 0)
	seenIssue := make(map[string]struct{})
	addIssue := func(issue string) {
		issue = strings.TrimSpace(issue)
		if issue == "" {
			return
		}
		if _, exists := seenIssue[issue]; exists {
			return
		}
		seenIssue[issue] = struct{}{}
		globalIssues = append(globalIssues, issue)
	}

	for index, parsedItem := range parsed.Items {
		board := normalizedImportBoard(string(parsedItem.Board))
		item := deckImportReviewItem{
			Index:           index,
			Line:            parsedItem.Line,
			Qty:             parsedItem.Qty,
			Name:            strings.TrimSpace(parsedItem.Name),
			Board:           string(board),
			BoardLabel:      importBoardLabel(string(board)),
			SetCode:         strings.ToUpper(strings.TrimSpace(parsedItem.SetCode)),
			CollectorNumber: strings.TrimSpace(parsedItem.CollectorNumber),
			PrintID:         strings.TrimSpace(parsedItem.PrintID),
			Status:          "unresolved",
			StatusLabel:     "Needs review",
			NeedsAttention:  true,
		}

		switch board {
		case decks.ImportBoardCommander:
			commanderRows++
			page.CommanderCount += parsedItem.Qty
		case decks.ImportBoardSide:
			page.SideCount += parsedItem.Qty
		case decks.ImportBoardMaybe:
			page.MaybeCount += parsedItem.Qty
		default:
			page.MainCount += parsedItem.Qty
		}
		page.TotalCount += parsedItem.Qty

		if parsedItem.Qty <= 0 {
			item.StatusDetail = "Enter a quantity greater than zero."
			page.Items[index] = item
			continue
		}
		if item.Name == "" {
			item.StatusDetail = "Enter a card name."
			page.Items[index] = item
			continue
		}

		key := strings.ToLower(item.Name)
		resolved, ok := exact[key]
		if !ok {
			if suggestion, hasSuggestion := suggestions[key]; hasSuggestion && strings.TrimSpace(suggestion.Card.Name) != "" {
				item.Status = "suggestion"
				item.StatusLabel = "Possible match"
				item.StatusDetail = "No exact card-name match was found."
				item.SuggestedName = suggestion.Card.Name
			} else {
				item.StatusDetail = "No matching card was found."
			}
			page.Items[index] = item
			continue
		}

		item.resolvedCard = resolved
		item.OracleID = resolved.OracleID
		item.CanonicalName = resolved.Name
		item.Status = "resolved"
		item.StatusLabel = "Matched"
		item.StatusDetail = "Default printing"
		item.NeedsAttention = false
		if expectedOracleID := strings.TrimSpace(parsedItem.OracleID); expectedOracleID != "" &&
			!strings.EqualFold(expectedOracleID, resolved.OracleID) {
			item.Status = "unresolved"
			item.StatusLabel = "Card changed"
			item.StatusDetail = "The saved card identity no longer matches this name. Review the card before continuing."
			item.NeedsAttention = true
		}

		if board == decks.ImportBoardCommander {
			if parsedItem.Qty != 1 {
				item.Status = "commander"
				item.StatusLabel = "Commander quantity"
				item.StatusDetail = "A commander must have a quantity of one."
				item.NeedsAttention = true
			} else if !isCommanderCandidateAllowed(resolved.IsCommanderCandidate, resolved.TypeLine) {
				item.Status = "commander"
				item.StatusLabel = "Not a commander"
				item.StatusDetail = "This card is not currently recognized as a legal commander candidate."
				item.NeedsAttention = true
			}
		}

		hasPrintingInput := item.PrintID != "" || item.SetCode != "" || item.CollectorNumber != ""
		if hasPrintingInput {
			requestedSetOnly := item.PrintID == "" && item.SetCode != "" && item.CollectorNumber == ""
			printing, printErr := a.lookupImportedPrinting(
				ctx,
				resolved,
				item.PrintID,
				item.SetCode,
				item.CollectorNumber,
			)
			if printErr != nil {
				item.Status = "printing"
				item.StatusLabel = "Printing not found"
				if item.SetCode == "" && item.CollectorNumber != "" {
					item.StatusDetail = "The requested collector number also needs a set code."
				} else {
					item.StatusDetail = "The requested printing could not be matched."
				}
				item.NeedsAttention = true
			} else if printing != nil {
				item.resolvedPrinting = printing
				item.PreferredPrintID = printing.ID
				item.SetCode = strings.ToUpper(printing.SetCode)
				item.CollectorNumber = printing.CollectorNumber
				item.PrintingLabel = strings.ToUpper(printing.SetCode) + " · " + printing.CollectorNumber
				if !item.NeedsAttention {
					item.StatusDetail = item.PrintingLabel
				}
			} else if requestedSetOnly {
				// The requested set is not present in the local printing catalog.
				// Fall back to the canonical card data rather than dropping an
				// otherwise valid row from the imported deck.
				applyCanonicalImportedPrinting(&item, resolved)
			}
		}

		page.Items[index] = item
	}

	if page.Format == "Commander" {
		if commanderRows == 0 {
			addIssue("Commander decks need exactly one commander row.")
		} else if commanderRows > 1 {
			addIssue("ManaTomb currently supports one commander. Move additional commander rows to the mainboard.")
		}
	} else if commanderRows > 0 {
		addIssue("Move the commander row to the mainboard or change the detected format to Commander.")
	}
	if page.TotalCount <= 0 {
		addIssue("The import needs at least one card.")
	}

	type printingKey struct {
		oracleID string
		board    string
	}
	printingByCard := make(map[printingKey]string)
	for index := range page.Items {
		item := &page.Items[index]
		if item.resolvedCard.OracleID == "" || item.Board == string(decks.ImportBoardCommander) {
			continue
		}
		key := printingKey{
			oracleID: strings.ToLower(item.resolvedCard.OracleID),
			board:    item.Board,
		}
		if prior, exists := printingByCard[key]; exists &&
			prior != "" &&
			item.PreferredPrintID != "" &&
			!strings.EqualFold(prior, item.PreferredPrintID) {
			item.Status = "printing"
			item.StatusLabel = "Conflicting printing"
			item.StatusDetail = "One card can use only one printing on each board."
			item.NeedsAttention = true
		}
		if printingByCard[key] == "" && item.PreferredPrintID != "" {
			printingByCard[key] = item.PreferredPrintID
		}
	}

	page.Issues = globalIssues
	page.CanFinalize = len(page.Issues) == 0
	for _, item := range page.Items {
		if item.NeedsAttention {
			page.CanFinalize = false
			break
		}
	}
	return page, nil
}

func parsedDecklistFromReviewForm(r *http.Request) (decks.ParsedDecklist, error) {
	rowCount, err := strconv.Atoi(strings.TrimSpace(r.Form.Get("row_count")))
	if err != nil || rowCount < 1 || rowCount > deckImportMaxRows {
		return decks.ParsedDecklist{}, fmt.Errorf("invalid import review")
	}

	format := defaultDeckFormat(r.Form.Get("format"), "", "import")
	parsed := decks.ParsedDecklist{
		Format: format,
		Items:  make([]decks.ParsedDeckItem, 0, rowCount),
	}
	for index := 0; index < rowCount; index++ {
		suffix := strconv.Itoa(index)
		if strings.TrimSpace(r.Form.Get("row_remove_"+suffix)) == "1" {
			continue
		}
		qty, _ := strconv.Atoi(strings.TrimSpace(r.Form.Get("row_qty_" + suffix)))
		line, _ := strconv.Atoi(strings.TrimSpace(r.Form.Get("row_line_" + suffix)))
		parsed.Items = append(parsed.Items, decks.ParsedDeckItem{
			Name:            strings.TrimSpace(r.Form.Get("row_name_" + suffix)),
			Qty:             qty,
			Board:           normalizedImportBoard(r.Form.Get("row_board_" + suffix)),
			SetCode:         strings.TrimSpace(r.Form.Get("row_set_" + suffix)),
			CollectorNumber: strings.TrimSpace(r.Form.Get("row_collector_" + suffix)),
			PrintID:         strings.TrimSpace(r.Form.Get("row_print_" + suffix)),
			Line:            line,
		})
	}
	return parsed, nil
}

func reviewPersistenceData(page deckImportPageData) (string, string, []decks.ImportedDeckCardInput) {
	commanderName := ""
	commanderPrintID := ""
	items := make([]decks.ImportedDeckCardInput, 0, len(page.Items))
	for _, item := range page.Items {
		if item.Board == string(decks.ImportBoardCommander) {
			commanderName = item.resolvedCard.Name
			commanderPrintID = item.PreferredPrintID
			continue
		}
		items = append(items, decks.ImportedDeckCardInput{
			OracleID:         item.resolvedCard.OracleID,
			Qty:              item.Qty,
			Board:            item.Board,
			PreferredPrintID: item.PreferredPrintID,
		})
	}
	return commanderName, commanderPrintID, items
}

func workbenchMetaForReviewItem(item deckImportReviewItem) workbenchImportCardMeta {
	meta := workbenchImportCardMeta{
		CardID:           item.resolvedCard.OracleID,
		Name:             item.resolvedCard.Name,
		ManaCost:         item.resolvedCard.ManaCost,
		TypeLine:         item.resolvedCard.TypeLine,
		OracleText:       item.resolvedCard.OracleText,
		CMC:              item.resolvedCard.CMC,
		PriceUSD:         item.resolvedCard.PriceUSD,
		ImageURI:         item.resolvedCard.ImageURI,
		PreferredPrintID: item.PreferredPrintID,
		PrintID:          item.PreferredPrintID,
		SetCode:          item.resolvedCard.SetCode,
		SetName:          item.resolvedCard.SetName,
		CollectorNumber:  item.resolvedCard.CollectorNumber,
		Artist:           item.resolvedCard.Artist,
	}
	if printing := item.resolvedPrinting; printing != nil {
		meta.PriceUSD = printing.PriceUSD
		meta.ImageURI = printing.ImageURI
		meta.SetCode = printing.SetCode
		meta.SetName = printing.SetName
		meta.CollectorNumber = printing.CollectorNumber
		meta.Rarity = printing.Rarity
		meta.ReleasedAt = printing.ReleasedAt
		meta.Artist = printing.Artist
	}
	return meta
}

func workbenchPayloadFromImportReview(page deckImportPageData, name, description string, replaceExistingDraft bool) workbenchImportPayload {
	commanderName, commanderPrintID, _ := reviewPersistenceData(page)
	payload := workbenchImportPayload{
		CommanderName:        commanderName,
		CommanderPrintID:     commanderPrintID,
		Name:                 strings.TrimSpace(name),
		Description:          strings.TrimSpace(description),
		Format:               page.Format,
		CardMeta:             make(map[string]workbenchImportCardMeta),
		ReplaceExistingDraft: replaceExistingDraft,
	}
	if payload.Name == "" {
		payload.Name = randomDeckName()
	}
	if commanderName != "" {
		payload.CommanderCandidates = []string{commanderName}
	}

	for _, item := range page.Items {
		canonicalName := item.resolvedCard.Name
		if canonicalName == "" {
			continue
		}
		cardMeta := workbenchMetaForReviewItem(item)
		payload.CardMeta[canonicalName] = cardMeta
		if item.Board == string(decks.ImportBoardCommander) {
			continue
		}
		card := workbenchImportSeedCard{
			Name:             canonicalName,
			OracleID:         item.resolvedCard.OracleID,
			Qty:              item.Qty,
			PreferredPrintID: item.PreferredPrintID,
			SetCode:          item.SetCode,
			CollectorNumber:  item.CollectorNumber,
			Meta:             cardMeta,
		}
		switch item.Board {
		case string(decks.ImportBoardSide):
			payload.SideboardCards = append(payload.SideboardCards, card)
		case string(decks.ImportBoardMaybe):
			payload.MaybeCards = append(payload.MaybeCards, card)
		default:
			payload.Cards = append(payload.Cards, card)
		}
	}
	return payload
}

func importOmissionLabel(item deckImportReviewItem) string {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = "Unnamed row"
	}
	if item.Qty > 1 {
		name = strconv.Itoa(item.Qty) + "x " + name
	}

	context := make([]string, 0, 2)
	if board := strings.TrimSpace(item.BoardLabel); board != "" {
		context = append(context, board)
	}
	if item.Line > 0 {
		context = append(context, "line "+strconv.Itoa(item.Line))
	}
	if len(context) > 0 {
		name += " (" + strings.Join(context, ", ") + ")"
	}

	detail := strings.TrimSpace(item.StatusDetail)
	if detail != "" {
		name += " — " + detail
	}
	return name
}

func appendImportNotes(description string, omissions []string) string {
	description = strings.TrimSpace(description)
	if len(omissions) == 0 {
		return description
	}

	var notes strings.Builder
	notes.WriteString("Import notes\n")
	notes.WriteString("The following decklist entries could not be imported:\n")
	for _, omission := range omissions {
		if omission = strings.TrimSpace(omission); omission != "" {
			notes.WriteString("- ")
			notes.WriteString(omission)
			notes.WriteString("\n")
		}
	}

	importNotes := strings.TrimSpace(notes.String())
	if description == "" {
		return importNotes
	}
	return description + "\n\n" + importNotes
}

func directImportPayloadFromReview(
	page deckImportPageData,
	name, description string,
	replaceExistingDraft bool,
) workbenchImportPayload {
	importable := page
	importable.Items = make([]deckImportReviewItem, 0, len(page.Items))
	omissions := make([]string, 0)
	seenOmission := make(map[string]struct{})
	commanderSelected := false

	addOmission := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, exists := seenOmission[key]; exists {
			return
		}
		seenOmission[key] = struct{}{}
		omissions = append(omissions, value)
	}

	for _, item := range page.Items {
		if item.NeedsAttention ||
			item.Qty <= 0 ||
			strings.TrimSpace(item.resolvedCard.OracleID) == "" ||
			strings.TrimSpace(item.resolvedCard.Name) == "" {
			addOmission(importOmissionLabel(item))
			continue
		}
		if item.Board == string(decks.ImportBoardCommander) {
			if !decks.FormatRequiresCommander(page.Format) {
				omitted := item
				omitted.StatusDetail = "This format does not use a commander."
				addOmission(importOmissionLabel(omitted))
				continue
			}
			if commanderSelected {
				omitted := item
				omitted.StatusDetail = "Only one commander can be imported."
				addOmission(importOmissionLabel(omitted))
				continue
			}
			commanderSelected = true
		}
		importable.Items = append(importable.Items, item)
	}
	for _, issue := range page.Issues {
		addOmission("Deck setup — " + strings.TrimSpace(issue))
	}

	payload := workbenchPayloadFromImportReview(
		importable,
		name,
		appendImportNotes(description, omissions),
		replaceExistingDraft,
	)
	payload.ImportWarnings = omissions
	return payload
}

func importMetaForName(meta map[string]importDraftCardMeta, name string) importDraftCardMeta {
	if len(meta) == 0 {
		return importDraftCardMeta{}
	}
	if exact, ok := meta[name]; ok {
		return exact
	}
	key := strings.ToLower(strings.TrimSpace(name))
	for candidateName, candidate := range meta {
		if strings.ToLower(strings.TrimSpace(candidateName)) == key {
			return candidate
		}
	}
	return importDraftCardMeta{}
}

func applyDraftRequestMetadata(item *decks.ParsedDeckItem, request importDraftCardRequest, meta importDraftCardMeta) {
	if item == nil {
		return
	}
	item.OracleID = firstNonBlank(request.OracleID, meta.resolvedOracleID())
	item.PrintID = strings.TrimSpace(request.PreferredPrintID)
	item.SetCode = strings.TrimSpace(request.SetCode)
	item.CollectorNumber = strings.TrimSpace(request.CollectorNumber)
	if item.PrintID == "" {
		item.PrintID = meta.resolvedPrintID()
	}
	if item.SetCode == "" {
		item.SetCode = meta.resolvedSetCode()
	}
	if item.CollectorNumber == "" {
		item.CollectorNumber = meta.resolvedCollectorNumber()
	}
}

func parsedDecklistFromDraftRequest(req importDraftRequest) decks.ParsedDecklist {
	format := defaultDeckFormat(req.Format, req.CommanderName, "import")
	parsed := decks.ParsedDecklist{Format: format}
	if commander := strings.TrimSpace(req.CommanderName); commander != "" {
		meta := importMetaForName(req.CardMeta, commander)
		parsed.Items = append(parsed.Items, decks.ParsedDeckItem{
			Name:     commander,
			OracleID: meta.resolvedOracleID(),
			Qty:      1,
			Board:    decks.ImportBoardCommander,
			PrintID:  firstNonBlank(req.CommanderPrintID, meta.resolvedPrintID()),
		})
	}

	appendBoard := func(rows []importDraftCardRequest, board decks.ImportBoard) {
		for _, row := range rows {
			item := decks.ParsedDeckItem{
				Name:  strings.TrimSpace(row.Name),
				Qty:   row.Qty,
				Board: board,
			}
			applyDraftRequestMetadata(&item, row, importMetaForName(req.CardMeta, row.Name))
			parsed.Items = append(parsed.Items, item)
		}
	}
	appendBoard(req.Cards, decks.ImportBoardMain)
	appendBoard(req.SideboardCards, decks.ImportBoardSide)
	appendBoard(req.MaybeCards, decks.ImportBoardMaybe)
	return parsed
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func importReviewAttention(page deckImportPageData) []string {
	out := append([]string(nil), page.Issues...)
	seen := make(map[string]struct{}, len(out))
	for _, issue := range out {
		seen[strings.ToLower(issue)] = struct{}{}
	}
	for _, item := range page.Items {
		if !item.NeedsAttention {
			continue
		}
		label := strings.TrimSpace(item.Name)
		if label == "" {
			label = "Unnamed row"
		}
		detail := strings.TrimSpace(item.StatusDetail)
		if detail != "" {
			label += ": " + detail
		}
		key := strings.ToLower(label)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

func writeImportDraftError(w http.ResponseWriter, status int, message string, unresolved []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(importDraftResponse{
		Error:      message,
		Unresolved: unresolved,
	})
}

func writeImportDraftServerError(w http.ResponseWriter, userID int64, err error) {
	log.Printf("deck save failed: user_id=%d error=%v", userID, err)
	writeImportDraftError(w, http.StatusInternalServerError, "We couldn't save this deck. Your draft is still available here, so please try again.", nil)
}

func importDraftHasNoCards(req importDraftRequest) bool {
	return strings.TrimSpace(req.CommanderName) == "" &&
		len(req.Cards) == 0 &&
		len(req.SideboardCards) == 0 &&
		len(req.MaybeCards) == 0
}

// The browser keeps rich display metadata alongside each card. Only a small
// subset is needed to persist a deck, so tolerate extra metadata fields here
// instead of rejecting an otherwise valid, unfinished deck as "invalid data".
func parseImportDraftJSONBody(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

func (a *App) HandleDeckImportDraft(w http.ResponseWriter, r *http.Request) {
	// This endpoint is called by the editor with fetch(), so JSON is much more
	// useful than redirecting to a login page when the session has expired.
	user := CurrentUser(r)
	if user == nil {
		writeImportDraftError(w, http.StatusUnauthorized, "Your session expired. Sign in again, then save your deck.", nil)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, deckImportMaxBodyBytes)
	var req importDraftRequest
	if err := parseImportDraftJSONBody(r, &req); err != nil {
		writeImportDraftError(w, http.StatusBadRequest, "Invalid deck data.", nil)
		return
	}
	if len(req.Cards)+len(req.SideboardCards)+len(req.MaybeCards) > deckImportMaxRows {
		writeImportDraftError(w, http.StatusRequestEntityTooLarge, "That deck contains too many import rows.", nil)
		return
	}
	if importDraftHasNoCards(req) {
		deckName := strings.TrimSpace(req.Name)
		if deckName == "" {
			deckName = randomDeckName()
		}
		format := defaultDeckFormat(req.Format, "", "import")
		deck, err := decks.CreateDeckWithOptions(r.Context(), a.DB, user.ID, decks.DeckInput{
			Name:         deckName,
			Description:  strings.TrimSpace(req.Description),
			Tags:         strings.TrimSpace(req.Tags),
			Format:       format,
			PowerBracket: defaultDeckPowerBracket("", format),
		})
		if err != nil {
			writeImportDraftServerError(w, user.ID, err)
			return
		}
		writeJSON(w, http.StatusCreated, importDraftResponse{DeckID: deck.ID})
		return
	}

	parsed := parsedDecklistFromDraftRequest(req)
	review, err := a.buildDeckImportReview(r.Context(), parsed, "")
	if err != nil {
		writeImportDraftServerError(w, user.ID, err)
		return
	}
	if !review.CanFinalize {
		writeImportDraftError(
			w,
			http.StatusUnprocessableEntity,
			"Every card and printing must be resolved before the deck can be saved.",
			importReviewAttention(review),
		)
		return
	}

	commanderName, commanderPrintID, importedCards := reviewPersistenceData(review)
	format := review.Format
	if !decks.FormatRequiresCommander(format) {
		commanderName = ""
		commanderPrintID = ""
	}
	deckName := strings.TrimSpace(req.Name)
	if deckName == "" {
		deckName = randomDeckName()
	}
	deck, err := decks.CreateImportedDeck(r.Context(), a.DB, user.ID, decks.DeckInput{
		Name:             deckName,
		Description:      strings.TrimSpace(req.Description),
		Tags:             strings.TrimSpace(req.Tags),
		Format:           format,
		CommanderName:    commanderName,
		CommanderPrintID: commanderPrintID,
		PowerBracket:     defaultDeckPowerBracket("", format),
	}, importedCards)
	if err != nil {
		writeImportDraftServerError(w, user.ID, err)
		return
	}

	writeJSON(w, http.StatusCreated, importDraftResponse{DeckID: deck.ID})
}

func (a *App) HandleDeckImportText(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, deckImportMaxBodyBytes)
	if err := r.ParseForm(); err != nil {
		a.renderDeckImport(w, user, readFlash(w, r), "That decklist was too large or could not be read.", deckImportPageData{
			Step: "paste",
		})
		return
	}

	intent := strings.ToLower(strings.TrimSpace(r.Form.Get("intent")))
	source := strings.ToLower(strings.TrimSpace(r.Form.Get("source")))
	rawDecklist := r.Form.Get("decklist")
	if intent == "edit_text" {
		a.renderDeckImport(w, user, readFlash(w, r), "", deckImportPageData{
			Step:     "paste",
			Decklist: rawDecklist,
		})
		return
	}
	var (
		parsed decks.ParsedDecklist
		err    error
	)
	if source == "rows" {
		parsed, err = parsedDecklistFromReviewForm(r)
	} else {
		parsed, err = decks.ParseDecklist(rawDecklist)
	}
	if err != nil {
		a.renderDeckImport(w, user, readFlash(w, r), err.Error(), deckImportPageData{
			Step:     "paste",
			Decklist: rawDecklist,
		})
		return
	}
	if len(parsed.Items) > deckImportMaxRows {
		a.renderDeckImport(w, user, readFlash(w, r), "Import up to 500 card rows at a time.", deckImportPageData{
			Step:     "paste",
			Decklist: rawDecklist,
		})
		return
	}

	review, err := a.buildDeckImportReview(r.Context(), parsed, rawDecklist)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	payload := directImportPayloadFromReview(
		review,
		parsed.Name,
		"",
		strings.TrimSpace(r.Form.Get("replace_existing_draft")) == "1",
	)
	encoded, err := json.Marshal(payload)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	a.Renderer.Render(w, "decks_workbench_import_seed", TemplateData{
		CurrentUser: user,
		ActiveNav:   "builder",
		PageID:      "decks-workbench-import-seed",
		Data: workbenchImportSeedData{
			PayloadJSON: template.JS(encoded),
		},
	})
}
