package web

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type playtestCard struct {
	Name     string `json:"name"`
	ImageURI string `json:"image_uri"`
	TypeLine string `json:"type_line"`
	Qty      int    `json:"qty"`
}

type playtestCommander struct {
	Name     string `json:"name"`
	ImageURI string `json:"image_uri"`
	TypeLine string `json:"type_line"`
}

type playtestData struct {
	Deck          *decks.Deck
	CardsJSON     template.JS // safe because we're injecting JSON we generated
	CommanderJSON template.JS // safe because we're injecting JSON we generated
	AuthNextPath  string
	GuestMode     bool
}

type guestPlaytestCard struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

type guestPlaytestPayload struct {
	CommanderName string              `json:"commander_name"`
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	Cards         []guestPlaytestCard `json:"cards"`
	Sandbox       bool                `json:"sandbox"`
}

func parsePlaytestDeckID(path string) (int64, error) {
	// Accept both:
	//   /decks/playtest/{id}
	//   /decks/{id}/playtest
	var idStr string
	if strings.HasPrefix(path, "/decks/playtest/") {
		idStr = strings.TrimPrefix(path, "/decks/playtest/")
	} else if strings.HasPrefix(path, "/decks/") && strings.HasSuffix(path, "/playtest") {
		idStr = strings.TrimPrefix(path, "/decks/")
		idStr = strings.TrimSuffix(idStr, "/playtest")
	} else {
		return 0, errors.New("invalid playtest path")
	}

	idStr = strings.Trim(idStr, "/")
	deckID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || deckID <= 0 {
		return 0, errors.New("invalid deck id")
	}
	return deckID, nil
}

func normalizeGuestPlaytestCards(cardsIn []guestPlaytestCard) []playtestCard {
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

func deckRowsToPlaytestCards(deckCards []decks.DeckCard) []playtestCard {
	out := make([]playtestCard, 0, len(deckCards))
	for _, dc := range deckCards {
		out = append(out, playtestCard{
			Name:     strings.TrimSpace(dc.CardName),
			ImageURI: strings.TrimSpace(dc.ImageURI),
			TypeLine: strings.TrimSpace(dc.TypeLine),
			Qty:      dc.Quantity,
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
		typeLine := strings.TrimSpace(row.TypeLine)
		if imageURI == "" || typeLine == "" {
			if dbCard, ok := resolvedByName[strings.ToLower(name)]; ok {
				if imageURI == "" {
					imageURI = strings.TrimSpace(dbCard.ImageURI)
				}
				if typeLine == "" {
					typeLine = strings.TrimSpace(dbCard.TypeLine)
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
			if commander.TypeLine == "" {
				commander.TypeLine = typeLine
			}
			continue
		}

		out = append(out, playtestCard{
			Name:     name,
			ImageURI: imageURI,
			TypeLine: typeLine,
			Qty:      row.Qty,
		})
	}

	if commander.Name != "" && (commander.ImageURI == "" || commander.TypeLine == "") {
		if dbCard, ok := resolvedByName[strings.ToLower(commander.Name)]; ok {
			if commander.ImageURI == "" {
				commander.ImageURI = strings.TrimSpace(dbCard.ImageURI)
			}
			if commander.TypeLine == "" {
				commander.TypeLine = strings.TrimSpace(dbCard.TypeLine)
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
	authNextPath string,
	guestMode bool,
) {
	data := TemplateData{
		CurrentUser: user,
		Data: playtestData{
			Deck:          deck,
			CardsJSON:     template.JS(cardsJSON),
			CommanderJSON: template.JS(commanderJSON),
			AuthNextPath:  strings.TrimSpace(authNextPath),
			GuestMode:     guestMode,
		},
		Flash: readFlash(w, r),
		Error: "",
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
		"/decks/"+strconv.FormatInt(d.ID, 10),
		false,
	)
}

func (a *App) HandleDeckPlaytestGuest(w http.ResponseWriter, r *http.Request) {
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

	var in guestPlaytestPayload
	if err := json.Unmarshal([]byte(rawPayload), &in); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	commanderName := strings.TrimSpace(in.CommanderName)

	cardsJSON, commanderJSON, err := a.buildPlaytestPayload(r.Context(), commanderName, normalizeGuestPlaytestCards(in.Cards))
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	deckName := strings.TrimSpace(in.Name)
	if deckName == "" {
		deckName = "Guest Deck"
	}

	fakeDeck := &decks.Deck{
		ID:            0,
		UserID:        0,
		Name:          deckName,
		Description:   strings.TrimSpace(in.Description),
		Format:        "commander",
		CommanderName: commanderName,
	}

	authNextPath := "/decks/guest"
	if in.Sandbox {
		authNextPath = "/decks/guest?sandbox=1&save_guest=1"
	} else if commanderName != "" {
		authNextPath = "/decks/guest?commander_name=" + url.QueryEscape(commanderName) + "&save_guest=1"
	} else {
		authNextPath = "/decks/guest?save_guest=1"
	}

	a.renderPlaytestPage(
		w,
		r,
		CurrentUser(r),
		fakeDeck,
		cardsJSON,
		commanderJSON,
		authNextPath,
		true,
	)
}
