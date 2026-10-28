package web

import (
	"net/http"
	"strings"

	"github.com/lib/pq"
	"manatomb/app/internal/cards"
)

func printingWorkspaceMeta(card *cards.Card) workspaceCardMeta {
	return workspaceCardMeta{CardID: card.OracleID, Name: card.Name, ManaCost: card.ManaCost, TypeLine: card.TypeLine, OracleText: card.OracleText, CMC: card.CMC, PriceUSD: card.PriceUSD, ImageURI: card.ImageURI, PreferredPrintID: card.ID, PrintID: card.ID, SetCode: card.SetCode, SetName: card.SetName, CollectorNumber: card.CollectorNumber, Rarity: card.Rarity, ReleasedAt: card.ReleasedAt, Artist: card.Artist, Faces: card.Faces, IsCommanderCandidate: card.IsCommanderCandidate}
}

func (a *App) HandleCheapestPrintings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Names []string `json:"names"`
	}
	if err := parseJSONBody(r, &req); err != nil || len(req.Names) > 500 {
		http.Error(w, "Supply up to 500 card names.", http.StatusBadRequest)
		return
	}
	byName, err := cards.LookupCardsByNames(r.Context(), a.DB, req.Names)
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	ids := make([]string, 0, len(byName))
	for _, card := range byName {
		ids = append(ids, card.OracleID)
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT DISTINCT ON (oracle_id) oracle_id::text,scryfall_id::text FROM card_prints WHERE oracle_id=ANY($1::uuid[]) AND NOT digital AND price_usd ~ '^[0-9]+([.][0-9]+)?$' ORDER BY oracle_id,(CASE WHEN price_usd ~ '^[0-9]+([.][0-9]+)?$' THEN price_usd::numeric END),scryfall_id`, pq.Array(ids))
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	selected := map[string]string{}
	for rows.Next() {
		var oracle, print string
		if err := rows.Scan(&oracle, &print); err != nil {
			rows.Close()
			a.RenderServerError(w, r, err)
			return
		}
		selected[oracle] = print
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		a.RenderServerError(w, r, err)
		return
	}
	result := map[string]workspaceCardMeta{}
	for _, card := range byName {
		if printID := selected[card.OracleID]; printID != "" {
			printing, err := cards.GetCardPrintingByID(r.Context(), a.DB, card.OracleID, printID)
			if err != nil {
				a.RenderServerError(w, r, err)
				return
			}
			result[strings.ToLower(card.Name)] = printingWorkspaceMeta(printing)
		}
	}
	writeJSON(w, http.StatusOK, result)
}
