// Package forge defines the private, versioned web-to-engine contract.
// It never decides card legality or implements game rules.
package forge

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

const Protocol = "manatomb-forge/v1"
const Pin = "26d8aff87509dde8a9d5017852339382d7ab3e83"

type Card struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}
type Deck struct {
	Name       string `json:"name"`
	Commanders []Card `json:"commanders"`
	Main       []Card `json:"main"`
}
type Create struct {
	Request string `json:"request"`
	Human   Deck   `json:"human"`
	CPU     Deck   `json:"cpu"`
	Default string `json:"default,omitempty"`
}
type Action struct {
	Request  string  `json:"request"`
	Session  string  `json:"session"`
	Revision int64   `json:"revision"`
	Prompt   string  `json:"prompt"`
	Action   string  `json:"action"`
	ID       *int    `json:"id,omitempty"`
	Selected []int   `json:"selected"`
	Value    *int    `json:"value,omitempty"`
	Text     *string `json:"text,omitempty"`
	Amounts  []int   `json:"amounts,omitempty"`
	Seat     string  `json:"seat,omitempty"`
	Phase    string  `json:"phase,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
}
type View struct {
	Protocol         string          `json:"protocol"`
	Forge            string          `json:"forge"`
	Build            string          `json:"build,omitempty"`
	ID               string          `json:"id"`
	Status           string          `json:"status"`
	State            json.RawMessage `json:"state,omitempty"`
	Error            string          `json:"error,omitempty"`
	ExpiresAt        string          `json:"expiresAt"`
	ReconnectSeconds int             `json:"reconnectSeconds"`
}

// Validate checks transport shape only. Forge's pinned catalog and Commander
// conformance checker remain authoritative, including paired commanders.
func (d Deck) Validate() error {
	if len(d.Commanders) < 1 || len(d.Commanders) > 2 {
		return errors.New("select one or two commanders")
	}
	if len(d.Main) > 100 || len(d.Name) > 160 {
		return errors.New("deck exceeds transport limits")
	}
	total := 0
	seen := map[string]bool{}
	for _, group := range [][]Card{d.Commanders, d.Main} {
		for _, c := range group {
			name := strings.TrimSpace(c.Name)
			if name == "" || name != c.Name || len(name) > 200 || strings.ContainsAny(name, "\r\n|[]") || strings.IndexFunc(name, unicode.IsControl) >= 0 {
				return errors.New("invalid card name")
			}
			if c.Quantity < 1 || c.Quantity > 100 {
				return errors.New("invalid card quantity")
			}
			if seen[name] {
				return fmt.Errorf("duplicate entry %q; combine quantities", name)
			}
			seen[name] = true
			total += c.Quantity
		}
	}
	for _, c := range d.Commanders {
		if c.Quantity != 1 {
			return errors.New("each commander must have quantity one")
		}
	}
	if total != 100 {
		return fmt.Errorf("Commander deck must total 100 cards including commanders (got %d)", total)
	}
	return nil
}
func (a Action) Validate() error {
	if a.Action == "setPhaseStop" {
		if len(a.Request) < 8 || len(a.Request) > 80 || len(a.Session) < 8 || len(a.Session) > 80 || a.Revision != 0 || a.Prompt != "" || a.ID != nil || len(a.Selected) > 0 || a.Value != nil || a.Text != nil || len(a.Amounts) > 0 || a.Enabled == nil || (a.Seat != "Human" && a.Seat != "CPU") {
			return errors.New("invalid phase stop preference")
		}
		switch a.Phase {
		case "UNTAP", "UPKEEP", "DRAW", "MAIN1", "COMBAT_BEGIN", "COMBAT_DECLARE_ATTACKERS", "COMBAT_DECLARE_BLOCKERS", "COMBAT_FIRST_STRIKE_DAMAGE", "COMBAT_DAMAGE", "COMBAT_END", "MAIN2", "END_OF_TURN", "CLEANUP":
			return nil
		default:
			return errors.New("invalid phase")
		}
	}
	if a.Seat != "" || a.Phase != "" || a.Enabled != nil {
		return errors.New("phase preference on gameplay action")
	}
	if a.Action == "requestConcede" {
		if len(a.Request) < 8 || len(a.Request) > 80 || len(a.Session) < 8 || len(a.Session) > 80 || a.Revision != 0 || a.Prompt != "" || a.ID != nil || len(a.Selected) > 0 || a.Value != nil || a.Text != nil || len(a.Amounts) > 0 {
			return errors.New("invalid concession intent")
		}
		return nil
	}
	if len(a.Request) < 8 || len(a.Request) > 80 || len(a.Session) > 80 || len(a.Prompt) > 80 || a.Revision < 1 {
		return errors.New("invalid action identity")
	}
	switch a.Action {
	case "ok", "cancel", "card", "player", "mana", "reply", "concede":
	default:
		return errors.New("invalid action")
	}
	if len(a.Selected) > 200 || len(a.Amounts) > 200 || a.Text != nil && len(*a.Text) > 200 {
		return errors.New("action exceeds limits")
	}
	return nil
}
