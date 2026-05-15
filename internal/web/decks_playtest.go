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
	Name       string `json:"name"`
	ImageURI   string `json:"image_uri"`
	ManaCost   string `json:"mana_cost"`
	TypeLine   string `json:"type_line"`
	OracleText string `json:"oracle_text"`
	Qty        int    `json:"qty"`
}

type playtestCommander struct {
	Name       string `json:"name"`
	ImageURI   string `json:"image_uri"`
	ManaCost   string `json:"mana_cost"`
	TypeLine   string `json:"type_line"`
	OracleText string `json:"oracle_text"`
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
	Name                 string  `json:"name,omitempty"`
	ManaCost             string  `json:"manaCost,omitempty"`
	TypeLine             string  `json:"typeLine,omitempty"`
	OracleText           string  `json:"oracleText,omitempty"`
	CMC                  float64 `json:"cmc,omitempty"`
	PriceUSD             string  `json:"priceUSD,omitempty"`
	ImageURI             string  `json:"imageURI,omitempty"`
	IsCommanderCandidate bool    `json:"isCommanderCandidate,omitempty"`
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

func normalizeWorkbenchPlaytestCards(cardsIn []workbenchPlaytestCard) []playtestCard {
	merged := make(map[string]playtestCard, len(cardsIn))
	order := make([]string, 0, len(cardsIn))

	for _, row := range cardsIn {
		name := strings.TrimSpace(row.Name)
		if name == "" || row.Qty <= 0 {
			continue
		}

		key := strings.ToLower(name)
		if existing, ok := merged[key]; ok {
			existing.Qty += row.Qty
			merged[key] = existing
			continue
		}

		merged[key] = playtestCard{
			Name: name,
			Qty:  row.Qty,
		}
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
		meta.PriceUSD = strings.TrimSpace(meta.PriceUSD)
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
			Name:       strings.TrimSpace(dc.CardName),
			ImageURI:   strings.TrimSpace(dc.ImageURI),
			TypeLine:   strings.TrimSpace(dc.TypeLine),
			OracleText: strings.TrimSpace(dc.OracleText),
			Qty:        dc.Quantity,
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
		if strings.TrimSpace(row.ImageURI) == "" || strings.TrimSpace(row.TypeLine) == "" {
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

		imageURI := strings.TrimSpace(row.ImageURI)
		manaCost := strings.TrimSpace(row.ManaCost)
		typeLine := strings.TrimSpace(row.TypeLine)
		oracleText := strings.TrimSpace(row.OracleText)
		if imageURI == "" || manaCost == "" || typeLine == "" || oracleText == "" {
			if dbCard, ok := resolvedByName[strings.ToLower(name)]; ok {
				if imageURI == "" {
					imageURI = strings.TrimSpace(dbCard.ImageURI)
				}
				if manaCost == "" {
					manaCost = strings.TrimSpace(dbCard.ManaCost)
				}
				if typeLine == "" {
					typeLine = strings.TrimSpace(dbCard.TypeLine)
				}
				if oracleText == "" {
					oracleText = strings.TrimSpace(dbCard.OracleText)
				}
			}
		}

		if strings.EqualFold(name, commanderName) {
			if commander.Name == "" {
				commander.Name = name
			}
			if commander.ImageURI == "" {
				commander.ImageURI = imageURI
			}
			if commander.ManaCost == "" {
				commander.ManaCost = manaCost
			}
			if commander.TypeLine == "" {
				commander.TypeLine = typeLine
			}
			if commander.OracleText == "" {
				commander.OracleText = oracleText
			}
			continue
		}

		out = append(out, playtestCard{
			Name:       name,
			ImageURI:   imageURI,
			ManaCost:   manaCost,
			TypeLine:   typeLine,
			OracleText: oracleText,
			Qty:        row.Qty,
		})
	}

	if commander.Name != "" && (commander.ImageURI == "" || commander.ManaCost == "" || commander.TypeLine == "" || commander.OracleText == "") {
		if dbCard, ok := resolvedByName[strings.ToLower(commander.Name)]; ok {
			if commander.ImageURI == "" {
				commander.ImageURI = strings.TrimSpace(dbCard.ImageURI)
			}
			if commander.ManaCost == "" {
				commander.ManaCost = strings.TrimSpace(dbCard.ManaCost)
			}
			if commander.TypeLine == "" {
				commander.TypeLine = strings.TrimSpace(dbCard.TypeLine)
			}
			if commander.OracleText == "" {
				commander.OracleText = strings.TrimSpace(dbCard.OracleText)
			}
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

	cardsJSON, commanderJSON, err := a.buildPlaytestPayload(r.Context(), commanderName, normalizeWorkbenchPlaytestCards(in.Cards))
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
