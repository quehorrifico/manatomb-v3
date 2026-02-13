package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type importDraftRequest struct {
	CommanderName string `json:"commander_name"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Cards         []struct {
		Name string `json:"name"`
		Qty  int    `json:"qty"`
	} `json:"cards"`
	MaybeCards []struct {
		Name string `json:"name"`
		Qty  int    `json:"qty"`
	} `json:"maybe_cards"`
}

type importDraftResponse struct {
	DeckID int64 `json:"deck_id"`
}

type guestImportSeedData struct {
	CommanderName string
	PayloadJSON   template.JS
}

type guestImportSeedCard struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

type guestImportPayload struct {
	CommanderName       string                `json:"commander_name"`
	Name                string                `json:"name"`
	Description         string                `json:"description"`
	Cards               []guestImportSeedCard `json:"cards"`
	MaybeCards          []guestImportSeedCard `json:"maybe_cards,omitempty"`
	CommanderCandidates []string              `json:"commander_candidates,omitempty"`
}

type resolvedImportCard struct {
	OracleID string
	Name     string
	Qty      int
}

func (a *App) HandleDeckImportDraft(w http.ResponseWriter, r *http.Request) {
	user := a.currentUserOrRedirect(w, r)
	if user == nil {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req importDraftRequest
	if err := parseJSONBody(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	commander := strings.TrimSpace(req.CommanderName)
	name := strings.TrimSpace(req.Name)
	desc := strings.TrimSpace(req.Description)

	if name == "" {
		if commander != "" {
			name = commander
		} else {
			name = "Imported Deck"
		}
	}

	// Create the saved deck
	d, err := decks.CreateDeck(r.Context(), a.DB, user.ID, name, desc, commander)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	allImportNames := make([]string, 0, len(req.Cards)+len(req.MaybeCards))
	for _, item := range req.Cards {
		allImportNames = append(allImportNames, item.Name)
	}
	for _, item := range req.MaybeCards {
		allImportNames = append(allImportNames, item.Name)
	}
	resolvedImportCards, err := cards.ResolveCardNamesBatch(r.Context(), a.DB, allImportNames, 0.40)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	// Add cards; skip unknown cards instead of failing the whole import
	for _, item := range req.Cards {
		cardName := strings.TrimSpace(item.Name)
		qty := item.Qty
		if cardName == "" || qty <= 0 {
			continue
		}

		resolution, ok := resolvedImportCards[strings.ToLower(cardName)]
		if !ok {
			continue
		}

		_ = decks.AddCard(r.Context(), a.DB, d.ID, resolution.Card.OracleID, qty) // best-effort
	}

	// Add maybeboard cards (optional for guest draft imports).
	// Skip unknown cards instead of failing the whole import.
	maybeByName := map[string]string{}
	existingMaybe, err := decks.ListDeckMaybeCards(r.Context(), a.DB, d.ID)
	if err == nil {
		for _, rec := range existingMaybe {
			name := strings.ToLower(strings.TrimSpace(rec.CardName))
			if name == "" {
				continue
			}
			maybeByName[name] = rec.CardID
		}
	}

	for _, item := range req.MaybeCards {
		cardName := strings.TrimSpace(item.Name)
		qty := item.Qty
		if cardName == "" || qty <= 0 {
			continue
		}

		resolution, ok := resolvedImportCards[strings.ToLower(cardName)]
		if !ok {
			continue
		}

		targetCardID := resolution.Card.OracleID
		key := strings.ToLower(strings.TrimSpace(resolution.Card.Name))
		if key == "" {
			key = strings.ToLower(cardName)
		}
		if existingID, ok := maybeByName[key]; ok && existingID != "" {
			targetCardID = existingID
		}

		if err := decks.AddMaybeCard(r.Context(), a.DB, d.ID, targetCardID, qty); err == nil {
			maybeByName[key] = targetCardID
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(importDraftResponse{DeckID: d.ID})
}

func (a *App) HandleDeckImportText(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	decklist := r.Form.Get("decklist")

	commanderName, items, err := decks.ParseCommanderDecklistText(decklist)
	if err != nil {
		a.renderDeckNew(w, user, readFlash(w, r), err.Error(), deckNewPageData{
			Mode:            "import",
			ImportText:      decklist,
			ImportUnmatched: nil,
		})
		return
	}

	lookupNames := make([]string, 0, len(items)+1)
	if strings.TrimSpace(commanderName) != "" {
		lookupNames = append(lookupNames, commanderName)
	}
	for _, it := range items {
		if strings.TrimSpace(it.Name) == "" || it.Qty <= 0 {
			continue
		}
		lookupNames = append(lookupNames, it.Name)
	}

	resolvedExact, err := cards.LookupCardsByNames(r.Context(), a.DB, lookupNames)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	missing := make([]string, 0)
	missingSeen := make(map[string]struct{})
	trackMissing := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		key := strings.ToLower(raw)
		if _, ok := missingSeen[key]; ok {
			return
		}
		missingSeen[key] = struct{}{}
		missing = append(missing, raw)
	}

	if commander := strings.TrimSpace(commanderName); commander != "" {
		key := strings.ToLower(commander)
		if _, ok := resolvedExact[key]; !ok {
			trackMissing(commander)
		}
	}
	for _, it := range items {
		cardName := strings.TrimSpace(it.Name)
		if cardName == "" || it.Qty <= 0 {
			continue
		}
		key := strings.ToLower(cardName)
		if _, ok := resolvedExact[key]; !ok {
			trackMissing(cardName)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		a.renderDeckNew(w, user, readFlash(w, r), "Import failed. Every card name must match exactly before continuing.", deckNewPageData{
			Mode:            "import",
			ImportText:      decklist,
			ImportUnmatched: missing,
		})
		return
	}

	canonicalCommander := strings.TrimSpace(commanderName)
	if canonicalCommander != "" {
		canonicalCommander = resolvedExact[strings.ToLower(canonicalCommander)].Name
	}

	resolvedByOracleID := make(map[string]*resolvedImportCard, len(items))
	resolvedCardMeta := make(map[string]cards.DBCard, len(items))
	total := 0
	for _, it := range items {
		cardName := strings.TrimSpace(it.Name)
		if cardName == "" || it.Qty <= 0 {
			continue
		}

		card := resolvedExact[strings.ToLower(cardName)]
		if rec, ok := resolvedByOracleID[card.OracleID]; ok {
			rec.Qty += it.Qty
		} else {
			resolvedByOracleID[card.OracleID] = &resolvedImportCard{
				OracleID: card.OracleID,
				Name:     card.Name,
				Qty:      it.Qty,
			}
			resolvedCardMeta[card.OracleID] = card
		}
		total += it.Qty
	}

	// If the commander came from a dedicated "Commander:" line and is not
	// already present in the imported deck rows, include one copy so users can
	// explicitly choose it from commander candidates after import.
	if canonicalCommander != "" {
		if commanderCard, ok := resolvedExact[strings.ToLower(canonicalCommander)]; ok {
			if _, exists := resolvedByOracleID[commanderCard.OracleID]; !exists {
				resolvedByOracleID[commanderCard.OracleID] = &resolvedImportCard{
					OracleID: commanderCard.OracleID,
					Name:     commanderCard.Name,
					Qty:      1,
				}
				resolvedCardMeta[commanderCard.OracleID] = commanderCard
			}
		}
	}

	if total == 0 {
		a.renderDeckNew(w, user, readFlash(w, r), "Could not import any cards. Please check the names and try again.", deckNewPageData{
			Mode:            "import",
			ImportText:      decklist,
			ImportUnmatched: nil,
		})
		return
	}

	resolvedCards := make([]resolvedImportCard, 0, len(resolvedByOracleID))
	for _, rec := range resolvedByOracleID {
		resolvedCards = append(resolvedCards, *rec)
	}
	sort.Slice(resolvedCards, func(i, j int) bool {
		return resolvedCards[i].Name < resolvedCards[j].Name
	})

	commanderCandidates := make([]string, 0)
	candidateSeen := make(map[string]struct{})
	for _, rec := range resolvedCards {
		cardRec := resolvedCardMeta[rec.OracleID]
		if !cardRec.IsCommanderCandidate {
			continue
		}

		candidateName := strings.TrimSpace(cardRec.Name)
		if candidateName == "" {
			continue
		}

		key := strings.ToLower(candidateName)
		if _, ok := candidateSeen[key]; ok {
			continue
		}
		candidateSeen[key] = struct{}{}
		commanderCandidates = append(commanderCandidates, candidateName)
	}
	sort.Strings(commanderCandidates)

	deckName := strings.TrimSpace(canonicalCommander)
	if deckName == "" {
		deckName = "Imported Deck"
	}

	if user == nil {
		guestCards := make([]guestImportSeedCard, 0, len(resolvedCards))
		for _, rec := range resolvedCards {
			guestCards = append(guestCards, guestImportSeedCard{
				Name: rec.Name,
				Qty:  rec.Qty,
			})
		}

		payload := guestImportPayload{
			CommanderName:       "",
			Name:                deckName,
			Description:         "",
			Cards:               guestCards,
			CommanderCandidates: commanderCandidates,
		}

		b, err := json.Marshal(payload)
		if err != nil {
			a.RenderServerError(w, r, err)
			return
		}

		if len(commanderCandidates) > 0 {
			setFlash(w, "Deck imported. Choose your commander from eligible imported cards.")
		} else {
			setFlash(w, "Deck imported into your guest deck.")
		}

		data := TemplateData{
			CurrentUser: user,
			Data: guestImportSeedData{
				CommanderName: "",
				PayloadJSON:   template.JS(b),
			},
		}
		a.Renderer.Render(w, "decks_guest_import_seed", data)
		return
	}

	d, err := decks.CreateDeck(r.Context(), a.DB, user.ID, deckName, "", "")
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	persisted := 0
	for _, rec := range resolvedCards {
		if err := decks.AddCard(r.Context(), a.DB, d.ID, rec.OracleID, rec.Qty); err == nil {
			persisted += rec.Qty
		}
	}

	if persisted == 0 {
		_ = decks.DeleteDeck(r.Context(), a.DB, d.ID)
		a.renderDeckNew(w, user, readFlash(w, r), "Could not import any cards. Please check the names and try again.", deckNewPageData{
			Mode:            "import",
			ImportText:      decklist,
			ImportUnmatched: nil,
		})
		return
	}

	if len(commanderCandidates) > 0 {
		setFlash(w, "Deck imported. Choose your commander from eligible cards in the deck.")
	} else {
		setFlash(w, "Deck imported!")
	}
	http.Redirect(w, r, "/decks/"+strconv.FormatInt(d.ID, 10), http.StatusSeeOther)
}
