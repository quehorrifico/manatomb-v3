package web

import (
	"os"
	"strings"
	"testing"

	"manatomb/app/internal/account"
)

func TestSavedDeckPickerUsesFlatThemeAwareRows(t *testing.T) {
	body := renderTemplate(t, "saved_deck_picker", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Deck Builder"},
		Data: cardDetailPageData{SavedDecks: []deckListItem{
			{ID: 7, Name: "Definitely Not Lands", Format: "Commander", CommanderName: "Aesi, Tyrant of Gyre Strait", IsPublic: true},
		}},
	})

	for _, needle := range []string{
		`role="dialog"`,
		`aria-modal="true"`,
		`class="mt-modal mt-saved-deck-picker-modal hidden"`,
		`class="mt-modal-panel mt-saved-deck-picker"`,
		`class="mt-saved-deck-picker__option"`,
		`data-deck-id="7"`,
		`Definitely Not Lands`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("saved deck picker missing %q", needle)
		}
	}
	for _, forbidden := range []string{"text-slate-", "bg-slate-", "border-slate-", "mt-chip", "mt-action-card"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("saved deck picker retained legacy presentation %q", forbidden)
		}
	}
}

func TestSavedDeckPickerScriptKeepsCardNameAfterClosing(t *testing.T) {
	source, err := os.ReadFile("templates/saved_deck_picker_script.html.tmpl")
	if err != nil {
		t.Fatalf("read saved deck picker script: %v", err)
	}
	script := string(source)
	for _, needle := range []string{
		`var cardName = activeCardName;`,
		`closePicker();`,
		`"Added " + cardName + " to " + deckName`,
		`pageStatus.dataset.status`,
		`pickerStatus.dataset.status`,
		`document.body.style.overflow = "hidden";`,
		`pickerSearch.focus({ preventScroll: true });`,
		`previousFocus.focus({ preventScroll: true });`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("saved deck picker script missing %q", needle)
		}
	}
	for _, forbidden := range []string{"border-rose-", "border-emerald-", "border-sky-", "text-rose-", "text-sky-"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("saved deck picker script retained palette class %q", forbidden)
		}
	}
}
