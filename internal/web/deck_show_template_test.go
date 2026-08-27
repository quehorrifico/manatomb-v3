package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"manatomb/app/internal/account"
	"manatomb/app/internal/decks"
)

func TestDeckShowUsesPrimaryDecklistWorkspaceAndSharedAssets(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck: &decks.Deck{
				Name:   "Draft Workspace",
				Format: "Standard",
			},
			WorkbenchMode: true,
		},
	})

	for _, needle := range []string{
		`data-theme="tomb"`,
		`href="/assets/deck_show.css"`,
		`src="/assets/deck_show.js"`,
		`id="deck-editor-heading"`,
		`id="deck-import-warning-toast"`,
		`class="mt-deck-editor__import-toast hidden"`,
		`role="alert"`,
		`id="deck-import-warning-dismiss"`,
		`data-deck-workspace-tab="decklist"`,
		`data-deck-workspace-panel="decklist"`,
		`data-deck-workspace-tab="analysis"`,
		`data-deck-workspace-panel="analysis"`,
		`role="tab"`,
		`tabindex="0">Cards</button>`,
		`id="guest-deck-name-input"`,
		`value="Draft Workspace"`,
		`var initialDeckName = "Draft Workspace"`,
		`id="guest-card-name"`,
		`aria-keyshortcuts="/"`,
		`id="deck-focus-mode-toggle"`,
		`id="deck-focus-mode-label">Full screen</span>`,
		`aria-label="Enter full screen editing mode"`,
		`aria-controls="deck-workspace-decklist"`,
		`id="deck-focus-mode-help" class="sr-only"`,
		`Press Escape to exit.`,
		`id="deck-card-entry-row"`,
		`id="deck-commander-trigger"`,
		`aria-label="Open commander actions"`,
		`class="sr-only">Search cards to add</label>`,
		`placeholder="Search cards to add to Main"`,
		`class="mt-deck-editor__decklist-toolbar"`,
		`class="mt-deck-editor__board-meta"`,
		`id="deck-undo"`,
		`id="deck-undo-action">Undo</button>`,
		`id="deck-display-options"`,
		`id="deck-analytics-status"`,
		`aria-label="Deck statistics"`,
		`id="deck-analysis-playtest"`,
		`data-deck-playtest`,
		`class="mt-deck-editor__analysis-overview-grid"`,
		`class="mt-deck-editor__analysis-section mt-deck-editor__notes-panel"`,
		`class="mt-deck-editor__notes-box"`,
		`class="mt-deck-editor__selected-tags mt-deck-editor__tag-list"`,
		`class="mt-deck-editor__analysis-composition-grid"`,
		`class="mt-power-summary-row"`,
		`id="deck-power-confidence"`,
		`id="deck-stat-explorer"`,
		`aria-controls="deck-analytics-card-list-panel" aria-expanded="false"`,
		`class="mt-deck-editor__mana-profile"`,
		`data-mana-profile-bar="pips"`,
		`data-mana-profile-bar="sources"`,
		`id="deck-mana-locked-panel"`,
		`class="mt-kicker">Locked in</p>`,
		`function renderManaProfiles(analytics)`,
		`function toggleManaLockedPanel(kind, symbol)`,
		`renderManaProfiles(a);`,
		`id="deck-analytics-new-hand-slot"`,
		`class="mt-deck-editor__sample-refresh"`,
		`aria-controls="deck-analytics-example-hand"`,
		`id="deck-analytics-example-hand-status"`,
		`data-example-hand-card`,
		`exampleHandList.insertBefore(tile.node, exampleHandFreshSlot || null);`,
		`drawExampleHand(true);`,
		`function syncDeckDisplayOptions(rows)`,
		`<h2 class="sr-only">Cards</h2>`,
		`id="deck-empty-state-title"`,
		`id="deck-quick-build-copy"`,
		`id="deck-overview-format"`,
		`<option value="Standard" selected>Standard</option>`,
		`var chip = document.createElement('button');`,
		`chip.className = tone.editor_chip + ' mt-deck-editor__tag-action';`,
		`chip.setAttribute('aria-label', 'Remove ' + tagValue + ' tag');`,
		`removeMark.setAttribute('aria-hidden', 'true');`,
		`removeTag(tagValue);`,
		`return storedGroupMode ? normalizeDeckGroupMode(storedGroupMode) : 'type';`,
		`return storedSortMode ? normalizeDeckSortMode(storedSortMode) : 'mv';`,
		`return storedViewMode ? normalizeDeckViewMode(storedViewMode) : freshView;`,
		`if (!(grouped && (normalized === 'stacks' || normalized === 'text')))`,
		`var minColumnWidth = textLayout ? 304 : 196;`,
		`if (!Number.isFinite(availableWidth) || availableWidth < 1)`,
		`listEl.setAttribute('data-stack-layout-pending', '1');`,
		`deckStackResizeObserver = new window.ResizeObserver(function ()`,
		`window.mtDeckRefreshCardLayout = scheduleGroupedStackLayoutLock;`,
		`if (viewMode === 'stacks' && normalizeDeckGroupMode(deckGroupMode) !== 'none')`,
		`Start your Standard deck`,
		`Start with any card`,
		`function showDeckUndo(message, action)`,
		`var lines = ['Name: ' + deckName, 'Format: ' + format, ''];`,
		`value += ' {scryfall:' + printID + '}'`,
		`'Print ID', 'Deck Name', 'Format'`,
		`action', 'undo_card_change'`,
		`config.vertical ? ' mt-deck-action-cluster--vertical'`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck editor missing %q", needle)
		}
	}

	if got := strings.Count(body, "<main"); got != 1 {
		t.Fatalf("deck editor rendered %d main elements, want the single shared layout main", got)
	}
	if got := strings.Count(body, `data-deck-workspace-tab="`); got != 2 {
		t.Fatalf("deck editor rendered %d workspace tabs, want Cards and Analysis", got)
	}
	if got := strings.Count(body, `data-deck-workspace-panel="`); got != 2 {
		t.Fatalf("deck editor rendered %d workspace panels, want Cards and Analysis", got)
	}
	if addIndex, tabIndex := strings.Index(body, `id="guest-card-name"`), strings.Index(body, `data-deck-workspace-panel="analysis"`); addIndex == -1 || tabIndex == -1 || addIndex > tabIndex {
		t.Fatal("deck editor no longer prioritizes card entry before analysis")
	}
	cardsPanelIndex := strings.Index(body, `<section id="deck-workspace-decklist"`)
	formatIndex := strings.Index(body, `id="deck-overview-format"`)
	analysisPanelIndex := strings.Index(body, `<section id="deck-workspace-analysis"`)
	if cardsPanelIndex == -1 || formatIndex == -1 || analysisPanelIndex == -1 || !(cardsPanelIndex < formatIndex && formatIndex < analysisPanelIndex) {
		t.Fatal("format selector must live inside the Cards workspace")
	}
	playtestIndex := strings.Index(body, `id="deck-analysis-playtest"`)
	if playtestIndex == -1 || !(cardsPanelIndex < playtestIndex && playtestIndex < analysisPanelIndex) {
		t.Fatal("Playtest must replace deck progress inside the Cards toolbar")
	}
	statsIndex := strings.Index(body, `class="mt-deck-editor__stats"`)
	handIndex := strings.Index(body, `id="deck-analytics-example-hand"`)
	notesIndex := strings.Index(body, `id="deck-overview-notes"`)
	selectedTagsIndex := strings.Index(body, `id="deck-overview-tags-list"`)
	curveIndex := strings.Index(body, `<h3>Mana curve</h3>`)
	typeMixIndex := strings.Index(body, `<h3>Type mix</h3>`)
	if statsIndex == -1 || handIndex == -1 || notesIndex == -1 || selectedTagsIndex == -1 ||
		curveIndex == -1 || typeMixIndex == -1 ||
		!(analysisPanelIndex < statsIndex && statsIndex < handIndex && handIndex < notesIndex &&
			notesIndex < selectedTagsIndex && selectedTagsIndex < curveIndex && curveIndex < typeMixIndex) {
		t.Fatal("Analysis workspace content is no longer ordered as stats, hand, notes/tags, mana curve, and type mix")
	}
	cardsIndex := strings.Index(body, `<section id="deck-workspace-decklist"`)
	commanderIndex := strings.Index(body, `id="deck-overview-layout"`)
	addIndex := strings.Index(body, `id="deck-add-card-form"`)
	if cardsIndex == -1 || commanderIndex == -1 || addIndex == -1 || !(cardsIndex < commanderIndex && commanderIndex < addIndex) {
		t.Fatal("commander identity must stay at the top of the Cards workspace before card entry")
	}
	for _, id := range []string{
		"deck-overview-layout",
		"deck-overview-side",
		"deck-commander-panel",
		"deck-commander-trigger",
		"deck-top-commander-art-wrap",
		"deck-top-commander-art",
		"deck-top-commander-placeholder",
		"deck-commander-name",
		"deck-commander-flip",
		"deck-commander-menu-slot",
	} {
		if got := strings.Count(body, `id="`+id+`"`); got != 1 {
			t.Fatalf("commander hook %q rendered %d times, want 1", id, got)
		}
	}
	for _, forbidden := range []string{
		"Local draft",
		"Draft saved on this device.",
		"Save this draft and continue anywhere.",
		`id="guest-panel-login-to-save"`,
		`id="guest-panel-signup-to-save"`,
		`id="deck-add-submit"`,
		`data-add-card-board`,
		`>Add a card <kbd>`,
		`requestFullscreen`,
		`data-deck-workspace-tab="details"`,
		`data-deck-workspace-panel="details"`,
		`About this deck`,
		`<h2>Details</h2>`,
		`<p class="mt-kicker">Explore</p>`,
		`<h2>Analysis</h2>`,
		`None selected.`,
		`textContent = 'Live'`,
		`id="guest-playtest"`,
		`id="deck-format-progress"`,
		`var deckFormatTargets = {`,
		`function syncDeckFormatProgress(d, mainCount)`,
		`id="deck-power-summary"`,
		`id="deck-power-game-changers"`,
		`id="deck-power-land-denial"`,
		`id="deck-power-extra-turns"`,
		`id="deck-power-combos"`,
		`id="deck-power-signals-list"`,
		`Bracket pressure`,
		`id="deck-power-pressure"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("deck editor retained removed header copy/control %q", forbidden)
		}
	}
	if got := strings.Count(body, `data-deck-playtest`); got != 1 {
		t.Fatalf("deck editor rendered %d Playtest actions, want one in the Cards toolbar", got)
	}
}

func TestDeckShowCardSuggestionsUseReadableAccessibleOptions(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck:          &decks.Deck{Name: "Search Workspace", Format: "Sandbox"},
			WorkbenchMode: true,
		},
	})

	for _, needle := range []string{
		`role="combobox"`,
		`aria-autocomplete="list"`,
		`aria-controls="deck-card-suggestions"`,
		`aria-label="Card suggestions"`,
		`role="listbox"`,
		`button.className = 'mt-deck-editor__suggestion';`,
		`button.dataset.cardName = name;`,
		`button.setAttribute('role', 'option');`,
		`button.setAttribute('tabindex', '-1');`,
		`nameEl.className = 'mt-deck-editor__suggestion-name';`,
		`items[activeIndex].scrollIntoView({ block: 'nearest' });`,
		`&limit=8`,
		`}, 160);`,
		`submitCard(items[activeIndex].dataset.cardName || '');`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck card autocomplete is missing %q", needle)
		}
	}
	for _, forbidden := range []string{
		`&limit=0`,
		`submitCard(items[activeIndex].textContent || '')`,
		`mt-deck-editor__suggestion mt-deck-editor__suggestion-name`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("deck card autocomplete retained obsolete behavior %q", forbidden)
		}
	}
}

func TestDeckShowMergesBoardPrintingMetadataWithFullCardMetadata(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck:          &decks.Deck{Name: "Imported Workspace", Format: "Sandbox"},
			WorkbenchMode: true,
		},
	})

	for _, needle := range []string{
		`function draftRawMetaForCard(metaByName, cardName)`,
		`var baseMeta = draftMetaForCard(d, cardName);`,
		`var boardRawMeta = draftRawMetaForCard(boardMeta, cardName);`,
		`var printingMeta = normalizeDraftCardMeta(boardRawMeta);`,
		`return mergeDraftCardMeta(baseMeta, printingMeta, boardRawMeta);`,
		`'typeLine',`,
		`'imageURI',`,
		`if (normalizeName(overrideMeta[field] || ''))`,
		`if (Object.prototype.hasOwnProperty.call(raw, 'cmc'))`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck editor metadata merge is missing %q", needle)
		}
	}

	for _, obsolete := range []string{
		`var direct = normalizeDraftCardMeta(boardMeta[cardName]);`,
		`if (direct) return direct;`,
	} {
		if strings.Contains(body, obsolete) {
			t.Fatalf("sparse board printing metadata can still replace full card metadata via %q", obsolete)
		}
	}
}

func TestDeckShowOffersCurrentBuildableFormatsOnly(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck:          &decks.Deck{Name: "Formats", Format: "Sandbox"},
			WorkbenchMode: true,
		},
	})

	for _, format := range decks.BuildableFormats() {
		if !strings.Contains(body, `option value="`+format+`"`) {
			t.Errorf("deck editor is missing buildable format %q", format)
		}
	}
	for _, format := range decks.SupportedFormats() {
		if decks.FormatIsBuildable(format) {
			continue
		}
		if strings.Contains(body, `option value="`+format+`"`) {
			t.Errorf("deck editor offered known but not-yet-buildable format %q", format)
		}
	}
	for _, needle := range []string{
		`var supportedDeckFormats = `,
		`function formatUsesCommander(value)`,
		`window.mtDeckFormatUsesCommander = formatUsesCommander`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck editor format normalizer missing %q", needle)
		}
	}
}

func TestDeckShowDisplayControlsStayPredictableAndAccessible(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		Data: deckPageData{
			Deck:          &decks.Deck{Name: "Display controls", Format: "Sandbox"},
			WorkbenchMode: true,
		},
	})

	for _, needle := range []string{
		`<option value="none">No grouping</option>`,
		`<option value="category">Card role</option>`,
		`<option value="alphabet">First letter</option>`,
		`<option value="alphabet">Name</option>`,
		`<option value="mv">Mana value</option>`,
		`<option value="price">Price</option>`,
		`<option value="text">Text list</option>`,
		`<option value="grid">Card grid</option>`,
		`id="deck-sort-mode-label" for="deck-sort-mode"`,
		`id="deck-sort-direction"`,
		`aria-label="Sort direction: ascending"`,
		`aria-pressed="false"`,
		`aria-controls="deck-cards-list"`,
		`var deckSortDirectionKey = 'manatomb.deckEditor.sortDirection';`,
		`var deckSortDirection = loadDeckSortDirection();`,
		`function syncDeckSortDirectionControl()`,
		`direction: deckSortDirection`,
		`stacksOption.disabled = ungrouped;`,
		`persistDeckViewMode('grid');`,
		`img.loading = 'lazy';`,
		`img.decoding = 'async';`,
		`img.draggable = false;`,
		`if (!rawQty) {`,
		`totalHeight += extraSpace;`,
		`scheduleGroupedStackLayoutLock();`,
		`var moveScope = hoverScope;`,
		`container.addEventListener('focusout'`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("deck display controls missing %q", needle)
		}
	}

	for _, forbidden := range []string{
		`focus:ring-sky-400`,
		`var moveScope = listEl && listEl.contains(container) ? listEl : hoverScope;`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("deck display controls retained obsolete behavior %q", forbidden)
		}
	}
	if got := strings.Count(body, `deckSortDirectionButton.addEventListener('click'`); got != 2 {
		t.Fatalf("deck editor rendered %d sort-direction listeners, want saved and guest handlers", got)
	}
}

func TestDeckShowPreservesLegacyFormatWithoutOfferingIt(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		CurrentUser: &account.User{ID: 7},
		Data: deckPageData{
			Deck: &decks.Deck{
				ID:     42,
				UserID: 7,
				Name:   "Existing Modern Deck",
				Format: "Modern",
			},
		},
	})

	if !strings.Contains(body, `<option value="Modern" selected disabled>Modern (existing format)</option>`) {
		t.Fatal("legacy deck format should remain visible without being offered for new selection")
	}
	if strings.Contains(body, `<option value="Pioneer"`) {
		t.Fatal("legacy deck should not expose other non-buildable formats")
	}
}

func TestSavedDeckActionsUseAccessibleDialog(t *testing.T) {
	body := renderTemplate(t, "deck_show", TemplateData{
		CurrentUser: &account.User{ID: 7},
		Data: deckPageData{
			Deck: &decks.Deck{
				ID:     42,
				UserID: 7,
				Name:   "Saved Workspace",
				Format: "Modern",
			},
		},
	})

	for _, needle := range []string{
		`id="deck-menu-open"`,
		`aria-haspopup="dialog"`,
		`aria-expanded="false">Deck actions</button>`,
		`id="deck-menu-overlay"`,
		`role="dialog"`,
		`aria-modal="true"`,
		`aria-labelledby="deck-menu-title"`,
		`deckMenuOverlay.addEventListener('keydown'`,
		`if (event.key === 'Escape')`,
		`deckMenuReturnFocus.focus()`,
		`if (deckMenuTitle) deckMenuTitle.textContent = d.name || 'Deck';`,
		`href="/decks/playtest/42"`,
		`id="deck-analysis-playtest"`,
		`data-deck-playtest>Playtest</a>`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("saved editor deck-actions dialog missing %q", needle)
		}
	}
	if got := strings.Count(body, `data-deck-playtest`); got != 1 {
		t.Fatalf("saved deck editor rendered %d Playtest actions, want one in the Cards toolbar", got)
	}
	for _, forbidden := range []string{
		`id="deck-menu-name-save"`,
		`data-analytics-stat="nonlands"`,
		`viewButton.textContent = 'View';`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("saved editor retained deprecated interaction %q", forbidden)
		}
	}
}

func TestDeckShowStylesUseThemeRolesAndDisableMotion(t *testing.T) {
	withRendererRoot(t)
	body, err := os.ReadFile(filepath.Join("internal", "web", "assets", "deck_show.css"))
	if err != nil {
		t.Fatalf("read deck editor stylesheet: %v", err)
	}
	css := string(body)

	for _, needle := range []string{
		`body[data-page="deck-show"] .mt-deck-editor`,
		`width: min(100%, 104rem)`,
		`.mt-deck-editor__views`,
		`.mt-deck-editor__card-entry-row`,
		`.mt-deck-editor__add-row`,
		`.mt-deck-editor__focus-toggle`,
		`.mt-deck-editor--focus-mode`,
		`.mt-deck-focus-mode .mt-site-header`,
		`.mt-deck-focus-mode #site-footer`,
		`min-height: 100dvh`,
		`.mt-deck-editor__decklist-toolbar`,
		`.mt-deck-editor__board-meta`,
		`.mt-deck-editor__sort-control`,
		`.mt-deck-editor__sort-direction`,
		`.mt-deck-editor__undo`,
		`.mt-deck-editor__quick-build`,
		`.mt-deck-editor__tag-list:empty`,
		`.mt-deck-editor__tag-action:focus-visible`,
		`.mt-deck-editor__notes-box`,
		`.mt-deck-editor__selected-tags`,
		`.mt-deck-editor__import-toast`,
		`#deck-card-suggestions`,
		`.mt-deck-editor__suggestion-name`,
		`.mt-deck-editor__suggestion[aria-selected="true"]`,
		`.mt-deck-editor__analysis-overview-grid`,
		`.mt-deck-editor__analysis-composition-grid`,
		`.mt-power-summary-row`,
		`.mt-deck-editor__stat-explorer`,
		`.mt-deck-editor__mana-profile-grid`,
		`.mt-deck-editor__mana-segment--w`,
		`.mt-deck-editor__mana-locked`,
		`.mt-deck-editor__sample-refresh`,
		`.mt-deck-editor__sample-refresh-icon`,
		`.mt-deck-view-root--text:not(.mt-deck-view-root--grouped)`,
		`.mt-deck-view-root--stacks:not(.mt-deck-view-root--grouped)`,
		`.mt-deck-view-root--grid.mt-deck-view-root--grouped`,
		`.mt-deck-view-root--text.mt-deck-view-root--grouped[data-stack-layout-locked="1"]`,
		`.mt-deck-view-section--stack-active`,
		`.mt-deck-art-card__media:focus-visible`,
		`.mt-deck-table__group-row td`,
		`@media (max-width: 559px)`,
		`@media (max-width: 430px)`,
		`var(--mt-bg)`,
		`var(--mt-surface)`,
		`var(--mt-text)`,
		`animation: none !important`,
		`transition: none !important`,
		`.mt-deck-action-cluster--vertical`,
		`top: -0.42rem`,
		`.mt-deck-editor__feedback > .mt-deck-editor__inline-status:not(:empty)`,
	} {
		if !strings.Contains(css, needle) {
			t.Errorf("deck editor stylesheet missing %q", needle)
		}
	}
	if strings.Contains(css, `.mt-power-pressure`) {
		t.Error("deck editor stylesheet retained the removed Bracket pressure field")
	}
	for _, forbidden := range []string{"rgba(", "rgb("} {
		if strings.Contains(css, forbidden) {
			t.Errorf("deck editor stylesheet hardcodes a color with %q", forbidden)
		}
	}
	for _, forbidden := range []string{`.mt-deck-editor__format-progress`} {
		if strings.Contains(css, forbidden) {
			t.Errorf("deck editor stylesheet retained removed layout rule %q", forbidden)
		}
	}
}
