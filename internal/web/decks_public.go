package web

import (
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

const publicDeckBrowsePageSize = 24

type publicDeckCost struct {
	Display string
	Note    string
}

func parsePublicDeckPriceCents(raw string) (int64, bool) {
	cleaned := strings.NewReplacer("$", "", ",", "").Replace(strings.TrimSpace(raw))
	if cleaned == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, false
	}
	cents := math.Round(value * 100)
	if cents > math.MaxInt64 {
		return 0, false
	}
	return int64(cents), true
}

func formatPublicDeckCost(cents int64) string {
	dollars := strconv.FormatInt(cents/100, 10)
	for i := len(dollars) - 3; i > 0; i -= 3 {
		dollars = dollars[:i] + "," + dollars[i:]
	}
	return fmt.Sprintf("$%s.%02d", dollars, cents%100)
}

func buildPublicDeckCost(
	mainboard []decks.DeckCard,
	sideboard []decks.DeckCard,
	commanderName string,
	commander *cards.Card,
	includeCommander bool,
) publicDeckCost {
	var (
		totalCents        int64
		pricedCards       int
		missingPriceCards int
	)
	commanderName = strings.TrimSpace(commanderName)

	addRows := func(rows []decks.DeckCard, detectCommander bool) {
		for _, row := range rows {
			quantity := row.Quantity
			if quantity <= 0 {
				continue
			}
			if detectCommander && commanderName != "" && strings.EqualFold(strings.TrimSpace(row.CardName), commanderName) {
				// Older builders represented the commander with a hidden
				// mainboard row. Its deck-level printing is authoritative.
				quantity--
			}
			if quantity <= 0 {
				continue
			}
			priceCents, ok := parsePublicDeckPriceCents(row.PriceUSD)
			if !ok {
				missingPriceCards += quantity
				continue
			}
			totalCents += priceCents * int64(quantity)
			pricedCards += quantity
		}
	}

	addRows(mainboard, true)
	addRows(sideboard, false)

	if includeCommander && commanderName != "" {
		if commander != nil {
			if priceCents, ok := parsePublicDeckPriceCents(commander.PriceUSD); ok {
				totalCents += priceCents
				pricedCards++
			} else {
				missingPriceCards++
			}
		} else {
			missingPriceCards++
		}
	}

	out := publicDeckCost{Display: formatPublicDeckCost(totalCents)}
	if missingPriceCards == 0 {
		return out
	}
	if pricedCards == 0 {
		out.Display = "—"
	}
	cardLabel := "cards"
	if missingPriceCards == 1 {
		cardLabel = "card"
	}
	if pricedCards == 0 {
		out.Note = fmt.Sprintf("Price unavailable for %d %s.", missingPriceCards, cardLabel)
		return out
	}
	out.Display += "+"
	out.Note = fmt.Sprintf("Minimum known cost; %d %s have no USD price.", missingPriceCards, cardLabel)
	return out
}

func buildPublicDeckSampleHand(
	mainboard []decks.DeckCard,
	commanderName string,
	excludeCommander bool,
	limit int,
	random *rand.Rand,
) []decks.DeckCard {
	if limit <= 0 {
		return nil
	}
	commanderName = strings.TrimSpace(commanderName)
	pool := make([]decks.DeckCard, 0, deckCardQuantityTotal(mainboard))
	for _, row := range mainboard {
		if row.Quantity <= 0 ||
			(excludeCommander && commanderName != "" && strings.EqualFold(strings.TrimSpace(row.CardName), commanderName)) {
			continue
		}
		copyRow := row
		copyRow.Quantity = 1
		for i := 0; i < row.Quantity; i++ {
			pool = append(pool, copyRow)
		}
	}
	if len(pool) == 0 {
		return nil
	}
	if random == nil {
		random = rand.New(rand.NewSource(1))
	}
	random.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})
	if limit > len(pool) {
		limit = len(pool)
	}
	return append([]decks.DeckCard(nil), pool[:limit]...)
}

func publicDeckBrowsePage(raw string) int {
	page, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || page < 1 {
		return 1
	}
	if page > 10000 {
		return 10000
	}
	return page
}

func publicDeckBrowsePath(values url.Values, page int) string {
	query := make(url.Values, len(values))
	for key, items := range values {
		query[key] = append([]string(nil), items...)
	}
	if page > 1 {
		query.Set("page", strconv.Itoa(page))
	} else {
		query.Del("page")
	}
	if encoded := query.Encode(); encoded != "" {
		return "/decks/public?" + encoded
	}
	return "/decks/public"
}

func publicDeckColorNames(colors []string) []string {
	names := map[string]string{
		"W": "White",
		"U": "Blue",
		"B": "Black",
		"R": "Red",
		"G": "Green",
		"C": "Colorless",
	}
	out := make([]string, 0, len(colors))
	for _, color := range colors {
		if name := names[color]; name != "" {
			out = append(out, name)
		}
	}
	return out
}

func publicDeckAppliedFilters(values url.Values, archetypes, colors []string, colorMode string) []publicDeckAppliedFilter {
	out := make([]publicDeckAppliedFilter, 0, 4+len(archetypes))
	copyValues := func() url.Values {
		next := make(url.Values, len(values))
		for key, items := range values {
			next[key] = append([]string(nil), items...)
		}
		return next
	}
	appendFilter := func(label string, next url.Values) {
		out = append(out, publicDeckAppliedFilter{
			Label:      label,
			RemovePath: publicDeckBrowsePath(next, 1),
		})
	}
	add := func(label string, removeKeys ...string) {
		next := copyValues()
		for _, key := range removeKeys {
			next.Del(key)
		}
		appendFilter(label, next)
	}

	if commander := strings.TrimSpace(values.Get("commander")); commander != "" {
		add("Commander: "+commander, "commander")
	}
	if format := strings.TrimSpace(values.Get("format")); format != "" {
		add("Format: "+format, "format")
	}
	if powerBracket := strings.TrimSpace(values.Get("power_bracket")); powerBracket != "" {
		add("Power bracket: "+powerBracket, "power_bracket")
	}
	for _, archetype := range archetypes {
		next := copyValues()
		next.Del("archetype")
		for _, other := range archetypes {
			if other != archetype {
				next.Add("archetype", other)
			}
		}
		appendFilter("Archetype: "+archetype, next)
	}
	if len(colors) > 0 {
		label := "Colors: "
		if len(colors) == 1 && colors[0] == "C" {
			label += "Colorless"
		} else {
			switch decks.NormalizePublicDeckColorMode(colorMode) {
			case "exact":
				label += "exactly "
			case "at_most":
				label += "at most "
			default:
				label += "include "
			}
			label += strings.Join(publicDeckColorNames(colors), " / ")
		}
		add(label, "color", "color_mode")
	}
	return out
}

func (a *App) HandlePublicDecks(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	query := r.URL.Query()
	deckName := strings.TrimSpace(query.Get("deck_name"))
	format := strings.TrimSpace(query.Get("format"))
	commanderName := strings.TrimSpace(query.Get("commander"))
	powerBracket := strings.TrimSpace(query.Get("power_bracket"))
	archetypes := decks.NormalizePublicDeckArchetypes(query["archetype"])
	colorFilters := decks.NormalizePublicDeckColors(query["color"])
	colorMode := decks.NormalizePublicDeckColorMode(query.Get("color_mode"))
	sortMode := decks.NormalizePublicDeckSort(query.Get("sort"))
	page := publicDeckBrowsePage(query.Get("page"))
	selectedFormat := ""
	if format != "" {
		selectedFormat = decks.NormalizeFormat(format)
	}
	selectedPowerBracket := ""
	if powerBracket != "" {
		selectedPowerBracket = decks.NormalizePowerBracket(powerBracket)
	}
	colorSelected := make(map[string]bool, len(colorFilters))
	for _, color := range colorFilters {
		colorSelected[color] = true
	}
	archetypeSelected := make(map[string]bool, len(archetypes))
	for _, archetype := range archetypes {
		archetypeSelected[archetype] = true
	}
	browseQuery := url.Values{}
	if deckName != "" {
		browseQuery.Set("deck_name", deckName)
	}
	if commanderName != "" {
		browseQuery.Set("commander", commanderName)
	}
	if selectedFormat != "" {
		browseQuery.Set("format", selectedFormat)
	}
	if selectedPowerBracket != "" {
		browseQuery.Set("power_bracket", selectedPowerBracket)
	}
	for _, archetype := range archetypes {
		browseQuery.Add("archetype", archetype)
	}
	for _, color := range colorFilters {
		browseQuery.Add("color", color)
	}
	if len(colorFilters) > 0 {
		browseQuery.Set("color_mode", colorMode)
	}
	if sortMode != "recent" {
		browseQuery.Set("sort", sortMode)
	}
	clearQuery := url.Values{}
	if deckName != "" {
		clearQuery.Set("deck_name", deckName)
	}
	if sortMode != "recent" {
		clearQuery.Set("sort", sortMode)
	}
	clearPath := publicDeckBrowsePath(clearQuery, 1)
	appliedFilters := publicDeckAppliedFilters(browseQuery, archetypes, colorFilters, colorMode)

	publicDecks, err := decks.ListPublicDecks(r.Context(), a.DB, decks.PublicDeckFilters{
		DeckName:      deckName,
		CommanderName: commanderName,
		Format:        format,
		PowerBracket:  powerBracket,
		Archetypes:    archetypes,
		ColorIdentity: colorFilters,
		ColorMode:     colorMode,
		Sort:          sortMode,
		Limit:         publicDeckBrowsePageSize + 1,
		Offset:        (page - 1) * publicDeckBrowsePageSize,
	})
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	hasNext := len(publicDecks) > publicDeckBrowsePageSize
	if hasNext {
		publicDecks = publicDecks[:publicDeckBrowsePageSize]
	}
	hasPrevious := page > 1

	items := make([]deckListItem, 0, len(publicDecks))
	commanderNames := make([]string, 0, len(publicDecks))
	userIDs := make([]int64, 0, len(publicDecks))
	for _, d := range publicDecks {
		if name := strings.TrimSpace(d.CommanderName); name != "" {
			commanderNames = append(commanderNames, name)
		}
		if d.UserID > 0 {
			userIDs = append(userIDs, d.UserID)
		}
	}
	commanderCards, err := cards.LookupCardsByNames(r.Context(), a.DB, commanderNames)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	ownersByID, err := account.ListPublicProfilesByIDs(r.Context(), a.DB, userIDs)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	for _, d := range publicDecks {
		item := deckListItem{
			ID:            d.ID,
			OwnerID:       d.UserID,
			Name:          d.Name,
			Description:   d.Description,
			Tags:          d.Tags,
			Format:        d.Format,
			CommanderName: d.CommanderName,
			IsPublic:      d.IsPublic,
			PublicSlug:    d.PublicSlug,
			PowerBracket:  d.PowerBracket,
		}
		if d.PublishedAt != nil {
			item.PublishedLabel = d.PublishedAt.Format("Jan 2, 2006")
		}
		if owner, ok := ownersByID[d.UserID]; ok {
			item.OwnerDisplayName = owner.DisplayName
		}
		if commanderName := strings.TrimSpace(d.CommanderName); commanderName != "" {
			if c, ok := commanderCards[strings.ToLower(commanderName)]; ok {
				applyCommanderCardMetaToDeckItem(&item, c)
				if strings.TrimSpace(d.CommanderPrintID) != "" {
					if selected, selectedErr := cards.GetCardPrintingByID(
						r.Context(),
						a.DB,
						c.OracleID,
						d.CommanderPrintID,
					); selectedErr == nil {
						applyCommanderPrintingMetaToDeckItem(&item, selected)
					}
				}
			}
		}
		items = append(items, item)
	}

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		Data: publicDeckListPageData{
			DeckName:          deckName,
			CommanderName:     commanderName,
			Format:            selectedFormat,
			PowerBracket:      selectedPowerBracket,
			Archetypes:        archetypes,
			ArchetypeSelected: archetypeSelected,
			ColorFilters:      colorFilters,
			ColorSelected:     colorSelected,
			ColorMode:         colorMode,
			Sort:              sortMode,
			ClearPath:         clearPath,
			ActiveFilters:     len(appliedFilters),
			AppliedFilters:    appliedFilters,
			Page:              page,
			HasPrevious:       hasPrevious,
			HasNext:           hasNext,
			PreviousPath:      publicDeckBrowsePath(browseQuery, page-1),
			NextPath:          publicDeckBrowsePath(browseQuery, page+1),
			Items:             items,
		},
	}

	a.Renderer.Render(w, "decks_public", data)
}

func (a *App) HandlePublicDeckShow(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)
	slug := parsePublicDeckSlugFromPath(r)
	if slug == "" {
		a.RenderNotFound(w, r)
		return
	}

	d, err := decks.GetPublicDeckBySlug(r.Context(), a.DB, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			a.RenderNotFound(w, r)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	deckCards, err := decks.ListDeckCards(r.Context(), a.DB, d.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	maybeDeckCards, err := decks.ListDeckMaybeCards(r.Context(), a.DB, d.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	sideboardDeckCards, err := decks.ListDeckSideboardCards(r.Context(), a.DB, d.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	var commanderCard *cards.Card
	if decks.FormatRequiresCommander(d.Format) {
		commanderCard = a.lookupCommanderCardPrinting(r.Context(), d.CommanderName, d.CommanderPrintID)
		if strings.TrimSpace(d.CommanderPrintID) == "" {
			for _, row := range deckCards {
				if row.Quantity <= 0 ||
					!strings.EqualFold(strings.TrimSpace(row.CardName), strings.TrimSpace(d.CommanderName)) ||
					strings.TrimSpace(row.PrintID) == "" {
					continue
				}
				if selected, selectedErr := cards.GetCardPrintingByID(r.Context(), a.DB, row.CardID, row.PrintID); selectedErr == nil {
					commanderCard = selected
				}
				break
			}
		}
	}
	owner, err := account.GetPublicProfileByID(r.Context(), a.DB, d.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			a.RenderNotFound(w, r)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	publishedLabel := ""
	if d.PublishedAt != nil {
		publishedLabel = d.PublishedAt.Format("Jan 2, 2006")
	}
	updatedLabel := ""
	if !d.UpdatedAt.IsZero() {
		updatedLabel = d.UpdatedAt.Format("Jan 2, 2006")
	}
	var colorPips []manaPipView
	colorIdentityName := ""
	if commanderCard != nil {
		colorIdentity := strings.Join(commanderCard.ColorIdentity, ",")
		colorPips = manaPipsForColorIdentity(colorIdentity)
		colorIdentityName = colorCombinationName(colorIdentity)
	}
	cost := buildPublicDeckCost(
		deckCards,
		sideboardDeckCards,
		d.CommanderName,
		commanderCard,
		decks.FormatRequiresCommander(d.Format),
	)
	sampleHand := buildPublicDeckSampleHand(
		deckCards,
		d.CommanderName,
		decks.FormatRequiresCommander(d.Format),
		7,
		rand.New(rand.NewSource(time.Now().UnixNano())),
	)

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		Meta:        buildPublicDeckShareMeta(a.PublicBaseURL, d, commanderCard),
		WideLayout:  true,
		Data: publicDeckPageData{
			Deck:                    d,
			DeckCards:               deckCards,
			SideboardDeckCards:      sideboardDeckCards,
			MaybeDeckCards:          maybeDeckCards,
			Analytics:               computeDeckAnalyticsFromDeckCards(d.Format, d.CommanderName, deckCards),
			Commander:               commanderCard,
			Owner:                   owner,
			ColorPips:               colorPips,
			ColorIdentityName:       colorIdentityName,
			PublishedLabel:          publishedLabel,
			UpdatedLabel:            updatedLabel,
			DeckCostDisplay:         cost.Display,
			DeckCostNote:            cost.Note,
			SampleHand:              sampleHand,
			SampleExcludesCommander: decks.FormatRequiresCommander(d.Format),
		},
	}
	a.Renderer.Render(w, "decks_public_show", data)
}

func buildPublicDeckShareMeta(publicBaseURL string, d *decks.Deck, commander *cards.Card) *PageMeta {
	meta := defaultPageMeta("decks_public_show")
	if meta == nil {
		meta = &PageMeta{}
	}
	if d == nil {
		return meta
	}

	meta.CanonicalURL = absoluteSiteURL(publicBaseURL, "/decks/public/"+url.PathEscape(d.PublicSlug))
	if name := strings.TrimSpace(d.Name); name != "" {
		meta.Title = name
	}
	if description := strings.TrimSpace(d.Description); description != "" {
		meta.Description = truncateShareText(description, 180)
	} else {
		format := strings.TrimSpace(d.Format)
		if format == "" {
			format = "Magic"
		}
		if commanderName := strings.TrimSpace(d.CommanderName); commanderName != "" {
			meta.Description = "Explore " + meta.Title + ", a " + format + " deck led by " + commanderName + ", on ManaTomb."
		} else {
			meta.Description = "Explore " + meta.Title + ", a " + format + " deck shared on ManaTomb."
		}
	}
	if commander != nil {
		meta.ImageURL = strings.TrimSpace(commander.ArtCropURI)
		if meta.ImageURL == "" {
			meta.ImageURL = strings.TrimSpace(commander.ImageURI)
		}
		if name := strings.TrimSpace(commander.Name); name != "" {
			meta.ImageAlt = name + " artwork"
		}
	}
	meta.Type = "article"
	return meta
}

func (a *App) HandlePublicDeckForkPost(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		next := "/decks/public"
		if slug := strings.TrimSpace(r.FormValue("slug")); slug != "" {
			next = "/decks/public/" + url.PathEscape(slug)
		}
		http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	slug := strings.TrimSpace(r.Form.Get("slug"))
	if slug == "" {
		a.RenderNotFound(w, r)
		return
	}

	forked, err := decks.ForkDeckToUser(r.Context(), a.DB, slug, user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			a.RenderNotFound(w, r)
			return
		}
		a.RenderServerError(w, r, err)
		return
	}

	setFlash(w, "Deck copied to your account.")
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(forked.ID, 10), http.StatusSeeOther)
}
