package web

import (
	"encoding/json"
	"fmt"
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
	CommanderCandidates []string              `json:"commander_candidates,omitempty"`
}

type resolvedImportCard struct {
	CardID int64
	Name   string
	Qty    int
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

	// Add cards; skip unknown cards instead of failing the whole import
	for _, item := range req.Cards {
		cardName := strings.TrimSpace(item.Name)
		qty := item.Qty
		if cardName == "" || qty <= 0 {
			continue
		}

		card, err := cards.EnsureCardByName(r.Context(), a.DB, cardName)
		if err != nil {
			continue
		}

		_ = decks.AddCard(r.Context(), a.DB, d.ID, card.ID, qty) // best-effort
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(importDraftResponse{DeckID: d.ID})
}

func buildImportWarningMessage(unknown []string) string {
	if len(unknown) == 0 {
		return ""
	}

	total := len(unknown)
	display := unknown
	if total > 10 {
		display = unknown[:10]
	}

	msg := fmt.Sprintf("Imported with warnings: could not find %d card(s): %s", total, strings.Join(display, ", "))
	if total > 10 {
		msg += " ..."
	}
	return msg
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
			Mode:       "import",
			ImportText: decklist,
		})
		return
	}

	deckName := strings.TrimSpace(commanderName)
	if deckName == "" {
		deckName = "Imported Deck"
	}

	// Guest imports should not depend on Scryfall/network lookups.
	// We preserve parsed card names exactly as typed and seed localStorage.
	if user == nil {
		merged := make(map[string]guestImportSeedCard, len(items))
		order := make([]string, 0, len(items))
		total := 0

		for _, it := range items {
			name := strings.TrimSpace(it.Name)
			if name == "" || it.Qty <= 0 {
				continue
			}
			total += it.Qty

			key := strings.ToLower(name)
			if existing, ok := merged[key]; ok {
				existing.Qty += it.Qty
				merged[key] = existing
				continue
			}

			merged[key] = guestImportSeedCard{
				Name: name,
				Qty:  it.Qty,
			}
			order = append(order, key)
		}

		if total == 0 {
			a.renderDeckNew(w, user, readFlash(w, r), "Could not import any cards. Please check the names and try again.", deckNewPageData{
				Mode:       "import",
				ImportText: decklist,
			})
			return
		}

		guestCards := make([]guestImportSeedCard, 0, len(order))
		for _, key := range order {
			guestCards = append(guestCards, merged[key])
		}
		sort.Slice(guestCards, func(i, j int) bool {
			return guestCards[i].Name < guestCards[j].Name
		})

		guestCommander := strings.TrimSpace(commanderName)
		commanderCandidates := make([]string, 0)
		candidateSeen := make(map[string]struct{})

		// Best-effort candidate detection for guest mode.
		// We never fail import if lookup/network misses.
		for _, rec := range guestCards {
			cardRec, err := cards.EnsureCardByName(r.Context(), a.DB, rec.Name)
			if err != nil {
				continue
			}
			if !isCommanderEligible(cardRec.TypeLine, cardRec.OracleText) {
				continue
			}

			candidateName := strings.TrimSpace(cardRec.Name)
			if candidateName == "" {
				candidateName = strings.TrimSpace(rec.Name)
			}
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

		payload := guestImportPayload{
			CommanderName:       guestCommander,
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

		if guestCommander == "" && len(commanderCandidates) > 0 {
			setFlash(w, "Deck imported. Choose your commander from eligible imported cards.")
		} else {
			setFlash(w, "Deck imported into your guest deck.")
		}

		data := TemplateData{
			CurrentUser: user,
			Data: guestImportSeedData{
				CommanderName: guestCommander,
				PayloadJSON:   template.JS(b),
			},
		}
		a.Renderer.Render(w, "decks_guest_import_seed", data)
		return
	}

	unknown := make([]string, 0)
	resolvedByID := make(map[int64]*resolvedImportCard)
	added := 0

	for _, it := range items {
		cn := strings.TrimSpace(it.Name)
		if cn == "" || it.Qty <= 0 {
			continue
		}

		card, err := cards.EnsureCardByName(r.Context(), a.DB, cn)
		if err != nil {
			unknown = append(unknown, cn)
			continue
		}

		rec, ok := resolvedByID[card.ID]
		if !ok {
			resolvedByID[card.ID] = &resolvedImportCard{
				CardID: card.ID,
				Name:   card.Name,
				Qty:    it.Qty,
			}
		} else {
			rec.Qty += it.Qty
		}
		added += it.Qty
	}

	if added == 0 {
		a.renderDeckNew(w, user, readFlash(w, r), "Could not import any cards. Please check the names and try again.", deckNewPageData{
			Mode:       "import",
			ImportText: decklist,
		})
		return
	}

	resolvedCards := make([]resolvedImportCard, 0, len(resolvedByID))
	for _, rec := range resolvedByID {
		resolvedCards = append(resolvedCards, *rec)
	}
	sort.Slice(resolvedCards, func(i, j int) bool {
		return resolvedCards[i].Name < resolvedCards[j].Name
	})

	warningMsg := buildImportWarningMessage(unknown)

	d, err := decks.CreateDeck(r.Context(), a.DB, user.ID, deckName, "", commanderName)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}

	persisted := 0
	for _, rec := range resolvedCards {
		if err := decks.AddCard(r.Context(), a.DB, d.ID, rec.CardID, rec.Qty); err == nil {
			persisted += rec.Qty
		}
	}

	if persisted == 0 {
		_ = decks.DeleteDeck(r.Context(), a.DB, d.ID)
		a.renderDeckNew(w, user, readFlash(w, r), "Could not import any cards. Please check the names and try again.", deckNewPageData{
			Mode:       "import",
			ImportText: decklist,
		})
		return
	}

	if warningMsg != "" {
		setFlash(w, warningMsg)
	} else {
		setFlash(w, "Deck imported!")
	}

	http.Redirect(w, r, "/decks/"+strconv.FormatInt(d.ID, 10), http.StatusSeeOther)
}
