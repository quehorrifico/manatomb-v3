package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedCardDetailModalUsesSemanticTombStructure(t *testing.T) {
	withRendererRoot(t)

	templateBody, err := os.ReadFile(filepath.Join("internal", "web", "templates", "card_detail_modal.html.tmpl"))
	if err != nil {
		t.Fatalf("read shared card detail modal: %v", err)
	}
	modal := string(templateBody)
	for _, needle := range []string{
		`aria-labelledby="card-detail-modal-title"`,
		`class="mt-modal-panel mt-card-detail-modal__panel" tabindex="-1"`,
		`class="mt-card-detail-modal__close"`,
		`aria-label="Close card details"`,
		`class="mt-card-detail-modal__layout"`,
		`data-field="image-fallback"`,
		`class="mt-card-detail-modal__title-row"`,
		`class="mt-card-detail-modal__oracle"`,
		`class="mt-card-detail-modal__facts"`,
		`class="mt-card-detail-modal__printing"`,
		`data-field="printing-grid"`,
		`role="listbox"`,
		`View card page`,
	} {
		if !strings.Contains(modal, needle) {
			t.Fatalf("shared card detail modal missing %q", needle)
		}
	}
	for _, forbidden := range []string{"text-slate-", "shadow-slate-", "Cost:", "Open Full Page"} {
		if strings.Contains(modal, forbidden) {
			t.Fatalf("shared card detail modal contains legacy styling or wording %q", forbidden)
		}
	}
}

func TestSharedCardDetailModalUsesThemeTokensAndAccessibleBehavior(t *testing.T) {
	withRendererRoot(t)

	themeBody, err := os.ReadFile(filepath.Join("internal", "web", "assets", "theme.css"))
	if err != nil {
		t.Fatalf("read shared theme stylesheet: %v", err)
	}
	theme := string(themeBody)
	componentIndex := strings.Index(theme, "/* Shared card details */")
	if componentIndex == -1 {
		t.Fatal("shared theme stylesheet is missing the card detail component")
	}
	component := theme[componentIndex:]
	for _, needle := range []string{
		`#card-detail-modal.mt-modal`,
		`.mt-card-detail-modal__panel.mt-modal-panel`,
		`width: min(100%, 56rem);`,
		`grid-template-columns: minmax(13rem, 18rem) minmax(0, 1fr);`,
		`background: var(--mt-surface);`,
		`color: var(--mt-text);`,
		`outline: 2px solid var(--mt-focus);`,
		`box-shadow: var(--mt-shadow-card);`,
		`@media (max-width: 719px)`,
		`max-width: min(68vw, 16rem);`,
		`#card-detail-modal .mt-card-detail-modal__printing-grid`,
		`grid-template-columns: repeat(3, minmax(0, 1fr));`,
		`grid-auto-rows: 12.75rem;`,
		`width: min(100%, 30rem);`,
		`height: 13.9rem;`,
		`gap: 0.8rem;`,
		`overflow-y: auto;`,
		`overscroll-behavior: contain;`,
		`#card-detail-modal .mt-card-detail-modal__printing-choice[aria-selected="true"] img`,
		`box-shadow: 0 0 0 2px var(--mt-accent-strong);`,
		`object-fit: contain;`,
	} {
		if !strings.Contains(component, needle) {
			t.Fatalf("shared card detail theme missing %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "@keyframes"} {
		if strings.Contains(component, forbidden) {
			t.Fatalf("shared card detail component contains hardcoded color or motion %q", forbidden)
		}
	}

	scriptBody, err := os.ReadFile(filepath.Join("internal", "web", "templates", "card_detail_modal_script.html.tmpl"))
	if err != nil {
		t.Fatalf("read shared card detail modal script: %v", err)
	}
	script := string(scriptBody)
	for _, needle := range []string{
		`modalRequestToken++`,
		`imageEl.alt = name ? ('Card image for ' + name)`,
		`setName + ' (' + setCode + ')'`,
		`collectorNumber`,
		`modalPanelEl.setAttribute('aria-busy', 'true')`,
		`closeButtonEl.focus({ preventScroll: true })`,
		`event.key !== 'Tab'`,
		`last.focus()`,
		`first.focus()`,
		`data-printing-index`,
		`Change version`,
		`currentVersionContext.onSelect`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("shared card detail script missing %q", needle)
		}
	}
}
