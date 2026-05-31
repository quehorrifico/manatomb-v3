package web

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type playtestCard struct {
	Name            string            `json:"name"`
	ImageURI        string            `json:"image_uri"`
	ManaCost        string            `json:"mana_cost"`
	ManaValue       float64           `json:"mana_value"`
	Colors          playtestColorList `json:"colors,omitempty"`
	ColorIdentity   playtestColorList `json:"color_identity,omitempty"`
	TypeLine        string            `json:"type_line"`
	OracleText      string            `json:"oracle_text"`
	FlavorText      string            `json:"flavor_text"`
	PriceUSD        string            `json:"price_usd"`
	Artist          string            `json:"artist"`
	SetCode         string            `json:"set_code"`
	SetName         string            `json:"set_name"`
	CollectorNumber string            `json:"collector_number"`
	Qty             int               `json:"qty"`
}

type playtestCommander struct {
	Name            string            `json:"name"`
	ImageURI        string            `json:"image_uri"`
	ManaCost        string            `json:"mana_cost"`
	ManaValue       float64           `json:"mana_value"`
	Colors          playtestColorList `json:"colors,omitempty"`
	ColorIdentity   playtestColorList `json:"color_identity,omitempty"`
	TypeLine        string            `json:"type_line"`
	OracleText      string            `json:"oracle_text"`
	FlavorText      string            `json:"flavor_text"`
	PriceUSD        string            `json:"price_usd"`
	Artist          string            `json:"artist"`
	SetCode         string            `json:"set_code"`
	SetName         string            `json:"set_name"`
	CollectorNumber string            `json:"collector_number"`
}

type playtestData struct {
	Deck          *decks.Deck
	CardsJSON     template.JS // safe because we're injecting JSON we generated
	CommanderJSON template.JS // safe because we're injecting JSON we generated
	WorkbenchJSON template.JS // safe because we're injecting JSON we generated
	AuthNextPath  string
	WorkbenchMode bool
}

type workbenchPlaytestCard struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

type workbenchPlaytestCardMeta struct {
	Name                 string            `json:"name,omitempty"`
	ManaCost             string            `json:"manaCost,omitempty"`
	TypeLine             string            `json:"typeLine,omitempty"`
	OracleText           string            `json:"oracleText,omitempty"`
	FlavorText           string            `json:"flavorText,omitempty"`
	CMC                  float64           `json:"cmc,omitempty"`
	Colors               playtestColorList `json:"colors,omitempty"`
	ColorIdentity        playtestColorList `json:"colorIdentity,omitempty"`
	PriceUSD             string            `json:"priceUSD,omitempty"`
	SetCode              string            `json:"setCode,omitempty"`
	SetName              string            `json:"setName,omitempty"`
	CollectorNumber      string            `json:"collectorNumber,omitempty"`
	Artist               string            `json:"artist,omitempty"`
	ImageURI             string            `json:"imageURI,omitempty"`
	IsCommanderCandidate bool              `json:"isCommanderCandidate,omitempty"`
}

type workbenchPlaytestPayload struct {
	CommanderName       string                               `json:"commander_name"`
	Format              string                               `json:"format"`
	Name                string                               `json:"name"`
	Description         string                               `json:"description"`
	Tags                string                               `json:"tags"`
	Cards               []workbenchPlaytestCard              `json:"cards"`
	SideboardCards      []workbenchPlaytestCard              `json:"sideboard_cards"`
	MaybeCards          []workbenchPlaytestCard              `json:"maybe_cards"`
	CommanderCandidates []string                             `json:"commander_candidates"`
	CardMeta            map[string]workbenchPlaytestCardMeta `json:"card_meta"`
	Sandbox             bool                                 `json:"sandbox"`
}

type workbenchDraftSeed struct {
	CommanderName       string                               `json:"commanderName"`
	Format              string                               `json:"format"`
	Name                string                               `json:"name"`
	Description         string                               `json:"description"`
	Tags                string                               `json:"tags"`
	Cards               map[string]int                       `json:"cards"`
	SideboardCards      map[string]int                       `json:"sideboardCards"`
	MaybeCards          map[string]int                       `json:"maybeCards"`
	CardMeta            map[string]workbenchPlaytestCardMeta `json:"cardMeta"`
	CommanderCandidates []string                             `json:"commanderCandidates"`
	Sandbox             bool                                 `json:"sandbox"`
}

func parsePlaytestDeckID(path string) (int64, error) {
	const prefix = "/decks/playtest/"
	if !strings.HasPrefix(path, prefix) {
		return 0, errors.New("invalid playtest path")
	}

	idStr := strings.TrimPrefix(path, prefix)
	idStr = strings.Trim(idStr, "/")
	deckID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || deckID <= 0 {
		return 0, errors.New("invalid deck id")
	}
	return deckID, nil
}

type playtestColorList []string

func (l *playtestColorList) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*l = nil
		return nil
	}

	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*l = normalizePlaytestColorList(values)
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*l = splitPlaytestColorList(value)
	return nil
}

func normalizePlaytestColorList(values []string) playtestColorList {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make(playtestColorList, 0, len(values))
	for _, raw := range values {
		value := strings.ToUpper(strings.TrimSpace(raw))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func splitPlaytestColorList(raw string) playtestColorList {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	return normalizePlaytestColorList(parts)
}

func mergePlaytestCardMeta(card playtestCard, meta workbenchPlaytestCardMeta) playtestCard {
	if card.ImageURI == "" {
		card.ImageURI = strings.TrimSpace(meta.ImageURI)
	}
	if card.ManaCost == "" {
		card.ManaCost = strings.TrimSpace(meta.ManaCost)
	}
	if card.ManaValue == 0 && meta.CMC != 0 {
		card.ManaValue = meta.CMC
	}
	if len(card.Colors) == 0 {
		card.Colors = normalizePlaytestColorList(meta.Colors)
	}
	if len(card.ColorIdentity) == 0 {
		card.ColorIdentity = normalizePlaytestColorList(meta.ColorIdentity)
	}
	if card.TypeLine == "" {
		card.TypeLine = strings.TrimSpace(meta.TypeLine)
	}
	if card.OracleText == "" {
		card.OracleText = strings.TrimSpace(meta.OracleText)
	}
	if card.FlavorText == "" {
		card.FlavorText = strings.TrimSpace(meta.FlavorText)
	}
	if card.PriceUSD == "" {
		card.PriceUSD = strings.TrimSpace(meta.PriceUSD)
	}
	if card.Artist == "" {
		card.Artist = strings.TrimSpace(meta.Artist)
	}
	if card.SetCode == "" {
		card.SetCode = strings.TrimSpace(meta.SetCode)
	}
	if card.SetName == "" {
		card.SetName = strings.TrimSpace(meta.SetName)
	}
	if card.CollectorNumber == "" {
		card.CollectorNumber = strings.TrimSpace(meta.CollectorNumber)
	}
	return card
}

func playtestCardNeedsLookup(card playtestCard) bool {
	return strings.TrimSpace(card.ImageURI) == "" ||
		strings.TrimSpace(card.ManaCost) == "" ||
		card.ManaValue == 0 ||
		len(card.Colors) == 0 ||
		len(card.ColorIdentity) == 0 ||
		strings.TrimSpace(card.TypeLine) == "" ||
		strings.TrimSpace(card.OracleText) == "" ||
		strings.TrimSpace(card.FlavorText) == "" ||
		strings.TrimSpace(card.PriceUSD) == "" ||
		strings.TrimSpace(card.Artist) == "" ||
		strings.TrimSpace(card.SetCode) == "" ||
		strings.TrimSpace(card.SetName) == "" ||
		strings.TrimSpace(card.CollectorNumber) == ""
}

func hydratePlaytestCardFromDB(card playtestCard, dbCard cards.DBCard) playtestCard {
	if card.ImageURI == "" {
		card.ImageURI = strings.TrimSpace(dbCard.ImageURI)
	}
	if card.ManaCost == "" {
		card.ManaCost = strings.TrimSpace(dbCard.ManaCost)
	}
	if card.ManaValue == 0 && dbCard.CMC != 0 {
		card.ManaValue = dbCard.CMC
	}
	if len(card.Colors) == 0 {
		card.Colors = splitPlaytestColorList(dbCard.Colors)
	}
	if len(card.ColorIdentity) == 0 {
		card.ColorIdentity = splitPlaytestColorList(dbCard.ColorIdentity)
	}
	if card.TypeLine == "" {
		card.TypeLine = strings.TrimSpace(dbCard.TypeLine)
	}
	if card.OracleText == "" {
		card.OracleText = strings.TrimSpace(dbCard.OracleText)
	}
	if card.FlavorText == "" {
		card.FlavorText = strings.TrimSpace(dbCard.FlavorText)
	}
	if card.PriceUSD == "" {
		card.PriceUSD = strings.TrimSpace(dbCard.PriceUSD)
	}
	if card.Artist == "" {
		card.Artist = strings.TrimSpace(dbCard.Artist)
	}
	if card.SetCode == "" {
		card.SetCode = strings.TrimSpace(dbCard.SetCode)
	}
	if card.SetName == "" {
		card.SetName = strings.TrimSpace(dbCard.SetName)
	}
	if card.CollectorNumber == "" {
		card.CollectorNumber = strings.TrimSpace(dbCard.CollectorNumber)
	}
	return card
}

func hydratePlaytestCommanderFromDB(commander playtestCommander, dbCard cards.DBCard) playtestCommander {
	card := hydratePlaytestCardFromDB(playtestCard{
		Name:            commander.Name,
		ImageURI:        commander.ImageURI,
		ManaCost:        commander.ManaCost,
		ManaValue:       commander.ManaValue,
		Colors:          commander.Colors,
		ColorIdentity:   commander.ColorIdentity,
		TypeLine:        commander.TypeLine,
		OracleText:      commander.OracleText,
		FlavorText:      commander.FlavorText,
		PriceUSD:        commander.PriceUSD,
		Artist:          commander.Artist,
		SetCode:         commander.SetCode,
		SetName:         commander.SetName,
		CollectorNumber: commander.CollectorNumber,
	}, dbCard)
	return playtestCommander{
		Name:            card.Name,
		ImageURI:        card.ImageURI,
		ManaCost:        card.ManaCost,
		ManaValue:       card.ManaValue,
		Colors:          card.Colors,
		ColorIdentity:   card.ColorIdentity,
		TypeLine:        card.TypeLine,
		OracleText:      card.OracleText,
		FlavorText:      card.FlavorText,
		PriceUSD:        card.PriceUSD,
		Artist:          card.Artist,
		SetCode:         card.SetCode,
		SetName:         card.SetName,
		CollectorNumber: card.CollectorNumber,
	}
}

func normalizeWorkbenchPlaytestCards(cardsIn []workbenchPlaytestCard, cardMeta map[string]workbenchPlaytestCardMeta) []playtestCard {
	metaByName := normalizeWorkbenchCardMeta(cardMeta)
	metaByKey := make(map[string]workbenchPlaytestCardMeta, len(metaByName))
	for rawName, meta := range metaByName {
		if key := strings.ToLower(strings.TrimSpace(rawName)); key != "" {
			metaByKey[key] = meta
		}
		if key := strings.ToLower(strings.TrimSpace(meta.Name)); key != "" {
			metaByKey[key] = meta
		}
	}

	merged := make(map[string]playtestCard, len(cardsIn))
	order := make([]string, 0, len(cardsIn))

	for _, row := range cardsIn {
		name := strings.TrimSpace(row.Name)
		if name == "" || row.Qty <= 0 {
			continue
		}

		meta := metaByKey[strings.ToLower(name)]
		cardName := name
		if metaName := strings.TrimSpace(meta.Name); metaName != "" {
			cardName = metaName
		}

		key := strings.ToLower(name)
		if existing, ok := merged[key]; ok {
			existing.Qty += row.Qty
			merged[key] = mergePlaytestCardMeta(existing, meta)
			continue
		}

		merged[key] = mergePlaytestCardMeta(playtestCard{
			Name: cardName,
			Qty:  row.Qty,
		}, meta)
		order = append(order, key)
	}

	out := make([]playtestCard, 0, len(order))
	for _, key := range order {
		out = append(out, merged[key])
	}
	return out
}

func normalizeWorkbenchCardCounts(cardsIn []workbenchPlaytestCard) map[string]int {
	out := make(map[string]int, len(cardsIn))
	for _, row := range cardsIn {
		name := strings.TrimSpace(row.Name)
		if name == "" || row.Qty <= 0 {
			continue
		}
		out[name] += row.Qty
	}
	return out
}

func normalizeWorkbenchCardMeta(cardMeta map[string]workbenchPlaytestCardMeta) map[string]workbenchPlaytestCardMeta {
	if len(cardMeta) == 0 {
		return map[string]workbenchPlaytestCardMeta{}
	}

	out := make(map[string]workbenchPlaytestCardMeta, len(cardMeta))
	for rawName, meta := range cardMeta {
		name := strings.TrimSpace(rawName)
		if name == "" {
			name = strings.TrimSpace(meta.Name)
		}
		if name == "" {
			continue
		}
		meta.Name = strings.TrimSpace(meta.Name)
		if meta.Name == "" {
			meta.Name = name
		}
		meta.ManaCost = strings.TrimSpace(meta.ManaCost)
		meta.TypeLine = strings.TrimSpace(meta.TypeLine)
		meta.OracleText = strings.TrimSpace(meta.OracleText)
		meta.FlavorText = strings.TrimSpace(meta.FlavorText)
		meta.Colors = normalizePlaytestColorList(meta.Colors)
		meta.ColorIdentity = normalizePlaytestColorList(meta.ColorIdentity)
		meta.PriceUSD = strings.TrimSpace(meta.PriceUSD)
		meta.SetCode = strings.TrimSpace(meta.SetCode)
		meta.SetName = strings.TrimSpace(meta.SetName)
		meta.CollectorNumber = strings.TrimSpace(meta.CollectorNumber)
		meta.Artist = strings.TrimSpace(meta.Artist)
		meta.ImageURI = strings.TrimSpace(meta.ImageURI)
		out[name] = meta
	}
	return out
}

func normalizeWorkbenchCommanderCandidates(candidates []string, commanderName string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(candidates)+1)

	for _, raw := range candidates {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}

	commanderName = strings.TrimSpace(commanderName)
	if commanderName != "" {
		key := strings.ToLower(commanderName)
		if !seen[key] {
			out = append(out, commanderName)
		}
	}

	return out
}

func normalizeWorkbenchDraftSeed(in workbenchPlaytestPayload) workbenchDraftSeed {
	commanderName := strings.TrimSpace(in.CommanderName)
	mode := ""
	if in.Sandbox {
		mode = "sandbox"
	}
	format := defaultDeckFormat(in.Format, commanderName, mode)
	if format != "Commander" {
		commanderName = ""
	}

	return workbenchDraftSeed{
		CommanderName:       commanderName,
		Format:              format,
		Name:                "New Guest Deck",
		Description:         strings.TrimSpace(in.Description),
		Tags:                strings.TrimSpace(in.Tags),
		Cards:               normalizeWorkbenchCardCounts(in.Cards),
		SideboardCards:      normalizeWorkbenchCardCounts(in.SideboardCards),
		MaybeCards:          normalizeWorkbenchCardCounts(in.MaybeCards),
		CardMeta:            normalizeWorkbenchCardMeta(in.CardMeta),
		CommanderCandidates: normalizeWorkbenchCommanderCandidates(in.CommanderCandidates, commanderName),
		Sandbox:             in.Sandbox,
	}
}

func deckRowsToPlaytestCards(deckCards []decks.DeckCard) []playtestCard {
	out := make([]playtestCard, 0, len(deckCards))
	for _, dc := range deckCards {
		out = append(out, playtestCard{
			Name:            strings.TrimSpace(dc.CardName),
			ImageURI:        strings.TrimSpace(dc.ImageURI),
			ManaCost:        strings.TrimSpace(dc.ManaCost),
			ManaValue:       dc.CMC,
			Colors:          splitPlaytestColorList(dc.Colors),
			ColorIdentity:   splitPlaytestColorList(dc.ColorIdentity),
			TypeLine:        strings.TrimSpace(dc.TypeLine),
			OracleText:      strings.TrimSpace(dc.OracleText),
			FlavorText:      strings.TrimSpace(dc.FlavorText),
			PriceUSD:        strings.TrimSpace(dc.PriceUSD),
			Artist:          strings.TrimSpace(dc.Artist),
			SetCode:         strings.TrimSpace(dc.SetCode),
			SetName:         strings.TrimSpace(dc.SetName),
			CollectorNumber: strings.TrimSpace(dc.CollectorNumber),
			Qty:             dc.Quantity,
		})
	}
	return out
}

func (a *App) buildPlaytestPayload(ctx context.Context, commanderName string, rows []playtestCard) ([]byte, []byte, error) {
	commanderName = strings.TrimSpace(commanderName)
	commander := playtestCommander{Name: commanderName}
	out := make([]playtestCard, 0, len(rows))
	missingNames := make([]string, 0, len(rows)+1)

	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" || row.Qty <= 0 {
			continue
		}
		row.Name = name
		if playtestCardNeedsLookup(row) {
			missingNames = append(missingNames, name)
		}
	}
	if commanderName != "" {
		missingNames = append(missingNames, commanderName)
	}

	resolvedByName, err := cards.LookupCardsByNames(ctx, a.DB, missingNames)
	if err != nil {
		return nil, nil, err
	}

	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" || row.Qty <= 0 {
			continue
		}

		row.Name = name
		row.ImageURI = strings.TrimSpace(row.ImageURI)
		row.ManaCost = strings.TrimSpace(row.ManaCost)
		row.TypeLine = strings.TrimSpace(row.TypeLine)
		row.OracleText = strings.TrimSpace(row.OracleText)
		row.FlavorText = strings.TrimSpace(row.FlavorText)
		row.PriceUSD = strings.TrimSpace(row.PriceUSD)
		row.Artist = strings.TrimSpace(row.Artist)
		row.SetCode = strings.TrimSpace(row.SetCode)
		row.SetName = strings.TrimSpace(row.SetName)
		row.CollectorNumber = strings.TrimSpace(row.CollectorNumber)
		row.Colors = normalizePlaytestColorList(row.Colors)
		row.ColorIdentity = normalizePlaytestColorList(row.ColorIdentity)
		if playtestCardNeedsLookup(row) {
			if dbCard, ok := resolvedByName[strings.ToLower(name)]; ok {
				row = hydratePlaytestCardFromDB(row, dbCard)
			}
		}

		if strings.EqualFold(name, commanderName) {
			if commander.Name == "" {
				commander.Name = name
			}
			if commander.ImageURI == "" {
				commander.ImageURI = row.ImageURI
			}
			if commander.ManaCost == "" {
				commander.ManaCost = row.ManaCost
			}
			if commander.ManaValue == 0 {
				commander.ManaValue = row.ManaValue
			}
			if len(commander.Colors) == 0 {
				commander.Colors = row.Colors
			}
			if len(commander.ColorIdentity) == 0 {
				commander.ColorIdentity = row.ColorIdentity
			}
			if commander.TypeLine == "" {
				commander.TypeLine = row.TypeLine
			}
			if commander.OracleText == "" {
				commander.OracleText = row.OracleText
			}
			if commander.FlavorText == "" {
				commander.FlavorText = row.FlavorText
			}
			if commander.PriceUSD == "" {
				commander.PriceUSD = row.PriceUSD
			}
			if commander.Artist == "" {
				commander.Artist = row.Artist
			}
			if commander.SetCode == "" {
				commander.SetCode = row.SetCode
			}
			if commander.SetName == "" {
				commander.SetName = row.SetName
			}
			if commander.CollectorNumber == "" {
				commander.CollectorNumber = row.CollectorNumber
			}
			continue
		}

		out = append(out, row)
	}

	if commander.Name != "" {
		if dbCard, ok := resolvedByName[strings.ToLower(commander.Name)]; ok {
			commander = hydratePlaytestCommanderFromDB(commander, dbCard)
		}
	}

	cardsJSON, err := json.Marshal(out)
	if err != nil {
		return nil, nil, err
	}

	var commanderPayload any
	if commander.Name != "" {
		commanderPayload = commander
	}
	commanderJSON, err := json.Marshal(commanderPayload)
	if err != nil {
		return nil, nil, err
	}

	return cardsJSON, commanderJSON, nil
}

func (a *App) renderPlaytestPage(
	w http.ResponseWriter,
	r *http.Request,
	user *account.User,
	deck *decks.Deck,
	cardsJSON []byte,
	commanderJSON []byte,
	workbenchJSON []byte,
	authNextPath string,
	workbenchMode bool,
) {
	data := TemplateData{
		CurrentUser: user,
		Data: playtestData{
			Deck:          deck,
			CardsJSON:     template.JS(cardsJSON),
			CommanderJSON: template.JS(commanderJSON),
			WorkbenchJSON: template.JS(workbenchJSON),
			AuthNextPath:  strings.TrimSpace(authNextPath),
			WorkbenchMode: workbenchMode,
		},
		Flash:      readFlash(w, r),
		Error:      "",
		WideLayout: true,
		HideHeader: true,
		HideFooter: true,
	}

	a.Renderer.Render(w, "deck_playtest", data)
}

func (a *App) HandleDeckPlaytest(w http.ResponseWriter, r *http.Request) {
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}

	deckID, err := parsePlaytestDeckID(r.URL.Path)
	if err != nil {
		a.RenderNotFound(w, r)
		return
	}

	d, err := decks.GetDeck(r.Context(), a.DB, deckID, user.ID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	if d.UserID != user.ID {
		// Don't leak existence of other users' decks
		a.RenderNotFound(w, r)
		return
	}

	deckCards, err := decks.ListDeckCards(r.Context(), a.DB, deckID)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	cardsJSON, commanderJSON, err := a.buildPlaytestPayload(r.Context(), d.CommanderName, deckRowsToPlaytestCards(deckCards))
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	a.renderPlaytestPage(
		w,
		r,
		user,
		d,
		cardsJSON,
		commanderJSON,
		nil,
		"/decks/"+strconv.FormatInt(d.ID, 10),
		false,
	)
}

func (a *App) HandleDeckWorkbenchPlaytest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/decks/new", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	rawPayload := strings.TrimSpace(r.Form.Get("payload"))
	if rawPayload == "" {
		http.Redirect(w, r, "/decks/new", http.StatusSeeOther)
		return
	}

	var in workbenchPlaytestPayload
	if err := json.Unmarshal([]byte(rawPayload), &in); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	workbenchDraft := normalizeWorkbenchDraftSeed(in)
	workbenchJSON, err := json.Marshal(workbenchDraft)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	commanderName := workbenchDraft.CommanderName

	cardsJSON, commanderJSON, err := a.buildPlaytestPayload(r.Context(), commanderName, normalizeWorkbenchPlaytestCards(in.Cards, in.CardMeta))
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	deckName := workbenchDraft.Name
	if strings.TrimSpace(deckName) == "" {
		deckName = "New Guest Deck"
	}

	fakeDeck := &decks.Deck{
		ID:            0,
		UserID:        0,
		Name:          deckName,
		Description:   workbenchDraft.Description,
		Tags:          workbenchDraft.Tags,
		Format:        workbenchDraft.Format,
		CommanderName: commanderName,
	}

	authNextPath := deckWorkbenchPath(deckWorkbenchOptions{
		Format:  workbenchDraft.Format,
		Sandbox: workbenchDraft.Sandbox,
	})

	a.renderPlaytestPage(
		w,
		r,
		CurrentUser(r),
		fakeDeck,
		cardsJSON,
		commanderJSON,
		workbenchJSON,
		authNextPath,
		true,
	)
}
