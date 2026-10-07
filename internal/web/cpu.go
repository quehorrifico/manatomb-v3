package web

import (
	"encoding/json"
	"io"
	"manatomb/app/internal/decks"
	"manatomb/app/internal/forge"
	"net/http"
	"strconv"
	"strings"
)

func cpuError(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": text})
}
func (a *App) CPUPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if CurrentUser(r) == nil {
		http.Redirect(w, r, "/login?next=%2Fcpu", 303)
		return
	}
	if a.Forge == nil {
		http.Error(w, "CPU Commander play is not enabled. Your decks and goldfishing remain available.", 503)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	a.Renderer.Render(w, "cpu", TemplateData{CurrentUser: CurrentUser(r), WideLayout: true, HideHeader: true, HideFooter: true, ActiveNav: "extras", Meta: &PageMeta{Title: "Commander vs CPU", Robots: "noindex"}})
}
func (a *App) CPUAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user := CurrentUser(r)
	if user == nil {
		cpuError(w, 401, "Sign in to play against the CPU.")
		return
	}
	if r.URL.Path == "/api/cpu/config" && r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]any{"enabled": a.Forge != nil, "owner": user.ID})
		return
	}
	if a.Forge == nil {
		cpuError(w, 503, "CPU play is disabled; ordinary deck tools remain available.")
		return
	}
	if r.Method != "GET" && r.Header.Get("Origin") != a.PublicBaseURL {
		cpuError(w, 403, "Invalid request origin.")
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/cpu")
	if suffix == "/decks" && r.Method == "GET" {
		list, err := decks.ListDecksByUser(r.Context(), a.DB, user.ID)
		if err != nil {
			cpuError(w, 503, "Cannot load decks.")
			return
		}
		type item struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		result := []item{}
		for _, d := range list {
			if strings.EqualFold(d.Format, "commander") {
				result = append(result, item{d.ID, d.Name})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}
	if strings.HasPrefix(suffix, "/decks/") && r.Method == "GET" {
		id, err := strconv.ParseInt(strings.TrimPrefix(suffix, "/decks/"), 10, 64)
		if err != nil {
			cpuError(w, 400, "Invalid deck.")
			return
		}
		deck, err := decks.GetDeck(r.Context(), a.DB, id, user.ID)
		if err != nil {
			cpuError(w, 404, "Deck not found.")
			return
		}
		cards, err := decks.ListDeckCards(r.Context(), a.DB, id)
		if err != nil {
			cpuError(w, 503, "Cannot load deck cards.")
			return
		}
		snapshot := forge.Deck{Name: deck.Name, Commanders: []forge.Card{{Name: deck.CommanderName, Quantity: 1}}, Main: []forge.Card{}}
		for _, c := range cards {
			if c.CardName != deck.CommanderName {
				snapshot.Main = append(snapshot.Main, forge.Card{Name: c.CardName, Quantity: c.Quantity})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshot)
		return
	}
	if !(strings.HasPrefix(suffix, "/sessions") || r.Method == "GET" && strings.HasPrefix(suffix, "/defaults/")) || strings.ContainsAny(suffix, "?%\\") || strings.Contains(suffix, "..") {
		cpuError(w, 404, "Not found.")
		return
	}
	if r.Method != "GET" && r.Method != "POST" && r.Method != "DELETE" {
		cpuError(w, 405, "Method not allowed.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		cpuError(w, 413, "Request too large.")
		return
	}
	code, response, err := a.Forge.Request(r.Context(), user.ID, r.Method, "/v1"+suffix, body)
	if err != nil {
		cpuError(w, 503, "Forge is unavailable. Your decks are safe; reconnect before retrying a game action.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	w.Write(response)
}
