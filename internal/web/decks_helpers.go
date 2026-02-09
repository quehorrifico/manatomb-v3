package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/decks"
)

// deckListItem is a lightweight view model for rendering the "My Decks" page.
// It includes the core deck fields plus an optional CommanderImageURI for UI use.
type deckListItem struct {
	ID                int64
	Name              string
	Description       string
	CommanderName     string
	CommanderImageURI string
}

type deckNewPageData struct {
	Mode          string
	CommanderName string
	Name          string
	Description   string
	ImportText    string
}

func normalizeDeckBuilderMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "scratch", "commander":
		return "commander"
	case "sandbox":
		return "sandbox"
	case "import":
		return "import"
	default:
		return ""
	}
}

func (a *App) renderDeckNew(w http.ResponseWriter, user *account.User, flash, errMsg string, page deckNewPageData) {
	data := TemplateData{
		CurrentUser: user,
		Data:        page,
		Flash:       flash,
		Error:       errMsg,
	}
	a.Renderer.Render(w, "decks_new", data)
}

// currentUserOrRedirect ensures there is a logged-in user, otherwise redirects
// to the login page and returns nil.
func (a *App) currentUserOrRedirect(w http.ResponseWriter, r *http.Request) *account.User {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}
	return user
}

// parseDeckIDFromPath extracts the deck ID from a path like /decks/{id}.
func parseDeckIDFromPath(r *http.Request) (int64, error) {
	const prefix = "/decks/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return 0, fmt.Errorf("invalid deck path")
	}
	idStr := strings.TrimPrefix(r.URL.Path, prefix)
	if idStr == "" {
		return 0, fmt.Errorf("missing deck id in path")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid deck id")
	}
	return id, nil
}

// parseDeckIDFromForm extracts the deck ID from a submitted form (field "id").
func parseDeckIDFromForm(r *http.Request) (int64, error) {
	idStr := strings.TrimSpace(r.Form.Get("id"))
	if idStr == "" {
		return 0, errors.New("missing deck id")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid deck id")
	}
	return id, nil
}

func isCommanderEligible(typeLine, oracleText string) bool {
	tl := strings.ToLower(typeLine)
	ot := strings.ToLower(oracleText)

	// Explicit override text on some cards
	if strings.Contains(ot, "can be your commander") {
		return true
	}

	// Classic: Legendary Creature
	if strings.Contains(tl, "legendary") && strings.Contains(tl, "creature") {
		return true
	}

	return false
}

func parseJSONBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func visibleDeckCardCount(deckCards []decks.DeckCard, commanderName string) int {
	total := 0
	commanderName = strings.TrimSpace(commanderName)

	for _, dc := range deckCards {
		name := strings.TrimSpace(dc.CardName)
		if name == "" || dc.Quantity <= 0 {
			continue
		}
		if commanderName != "" && strings.EqualFold(name, commanderName) {
			continue
		}
		total += dc.Quantity
	}

	return total
}

func deckCardQuantityTotal(cards []decks.DeckCard) int {
	total := 0
	for _, dc := range cards {
		if dc.Quantity <= 0 {
			continue
		}
		total += dc.Quantity
	}
	return total
}
