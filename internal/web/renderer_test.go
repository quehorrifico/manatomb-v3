package web

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"manatomb/app/internal/decks"
)

func TestRendererParsesTemplates(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	if renderer := NewRenderer(); renderer == nil {
		t.Fatal("NewRenderer returned nil")
	}
}

func TestDeckShowTemplateIncludesCardDetailModal(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:   "Inline Card View",
				Format: "Commander",
			},
			WorkbenchMode: true,
		},
	})

	body := rec.Body.String()
	if !strings.Contains(body, `id="card-detail-modal"`) {
		t.Fatalf("deck_show template did not render the card detail modal shell: %s", body)
	}
	if !strings.Contains(body, `data-field="open-page-link"`) {
		t.Fatalf("deck_show template did not render the full-page modal action: %s", body)
	}
	if !strings.Contains(body, `window.mtCardDetailModal =`) {
		t.Fatalf("deck_show template did not render the shared card detail modal script: %s", body)
	}
}

func TestDeckShowTemplateSeedsGuestWorkbenchCommanderState(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:          "Guest Commander",
				Format:        "Commander",
				CommanderName: "Atraxa, Grand Unifier",
			},
			WorkspaceState: workspaceDeckState{
				Name:          "New Guest Deck",
				Format:        "Commander",
				CommanderName: "Atraxa, Grand Unifier",
				Cards:         map[string]int{},
				MaybeCards:    map[string]int{},
				CardMeta:      map[string]workspaceCardMeta{},
			},
			WorkbenchMode: true,
		},
	})

	body := rec.Body.String()
	if !strings.Contains(body, `<option value="Commander" selected>Commander</option>`) {
		t.Fatalf("deck_show template did not select Commander for guest workbench: %s", body)
	}
	if !strings.Contains(body, `"format":"Commander"`) {
		t.Fatalf("deck_show template did not seed guest workspace format: %s", body)
	}
	if !strings.Contains(body, `"commanderName":"Atraxa, Grand Unifier"`) {
		t.Fatalf("deck_show template did not seed guest workspace commander: %s", body)
	}
}

func TestDeckShowTemplateIncludesGuestSaveAuthCTAs(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	rec := httptest.NewRecorder()
	NewRenderer().Render(rec, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:   "Guest Save",
				Format: "Commander",
			},
			WorkbenchMode: true,
		},
	})

	body := rec.Body.String()
	if !strings.Contains(body, `id="guest-save-auth-panel"`) {
		t.Fatalf("deck_show template did not render guest save auth panel: %s", body)
	}
}
