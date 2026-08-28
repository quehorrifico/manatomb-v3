package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"manatomb/app/internal/decks"
)

const (
	projectRepoURL   = "https://github.com/quehorrifico/manatomb-v3"
	projectIssuesURL = "https://github.com/quehorrifico/manatomb-v3/issues"
)

//go:embed templates/*.html.tmpl
var rendererTemplates embed.FS

type Renderer struct {
	tmpl          *template.Template
	publicBaseURL string
}

type deckTagTheme struct {
	Public       string `json:"public"`
	EditorChip   string `json:"editor_chip"`
	EditorRemove string `json:"editor_remove"`
	Button       string `json:"button"`
	ButtonActive string `json:"button_active"`
}

func deckTagThemeFor(tag string) deckTagTheme {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "aggro":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-red-500/50 bg-red-500/18 px-2.5 py-1 text-xs font-medium text-red-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-red-500/50 bg-red-500/18 px-3 py-1 text-sm font-medium text-red-100",
			EditorRemove: "text-red-200/80 hover:text-red-50",
			Button:       "mt-btn mt-btn--xs border border-red-500/50 bg-red-500/15 !text-red-100 hover:bg-red-500/25",
			ButtonActive: "mt-btn mt-btn--xs border border-red-400/80 bg-red-500/35 !text-red-50 shadow-[0_0_0_1px_rgba(248,113,113,0.4)]",
		}
	case "midrange":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-green-500/50 bg-green-500/18 px-2.5 py-1 text-xs font-medium text-green-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-green-500/50 bg-green-500/18 px-3 py-1 text-sm font-medium text-green-100",
			EditorRemove: "text-green-200/80 hover:text-green-50",
			Button:       "mt-btn mt-btn--xs border border-green-500/50 bg-green-500/15 !text-green-100 hover:bg-green-500/25",
			ButtonActive: "mt-btn mt-btn--xs border border-green-400/80 bg-green-500/35 !text-green-50 shadow-[0_0_0_1px_rgba(74,222,128,0.4)]",
		}
	case "control":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-sky-500/50 bg-sky-500/18 px-2.5 py-1 text-xs font-medium text-sky-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-sky-500/50 bg-sky-500/18 px-3 py-1 text-sm font-medium text-sky-100",
			EditorRemove: "text-sky-200/80 hover:text-sky-50",
			Button:       "mt-btn mt-btn--xs border border-sky-500/50 bg-sky-500/15 !text-sky-100 hover:bg-sky-500/25",
			ButtonActive: "mt-btn mt-btn--xs border border-sky-400/80 bg-sky-500/35 !text-sky-50 shadow-[0_0_0_1px_rgba(56,189,248,0.4)]",
		}
	case "combo":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-purple-500/50 bg-purple-500/18 px-2.5 py-1 text-xs font-medium text-purple-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-purple-500/50 bg-purple-500/18 px-3 py-1 text-sm font-medium text-purple-100",
			EditorRemove: "text-purple-200/80 hover:text-purple-50",
			Button:       "mt-btn mt-btn--xs border border-purple-500/50 bg-purple-500/15 !text-purple-100 hover:bg-purple-500/25",
			ButtonActive: "mt-btn mt-btn--xs border border-purple-400/80 bg-purple-500/35 !text-purple-50 shadow-[0_0_0_1px_rgba(192,132,252,0.4)]",
		}
	case "ramp":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-lime-500/50 bg-lime-500/18 px-2.5 py-1 text-xs font-medium text-lime-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-lime-500/50 bg-lime-500/18 px-3 py-1 text-sm font-medium text-lime-100",
			EditorRemove: "text-lime-200/80 hover:text-lime-50",
			Button:       "mt-btn mt-btn--xs border border-lime-500/50 bg-lime-500/15 !text-lime-100 hover:bg-lime-500/25",
			ButtonActive: "mt-btn mt-btn--xs border border-lime-400/80 bg-lime-500/35 !text-lime-50 shadow-[0_0_0_1px_rgba(163,230,53,0.4)]",
		}
	case "stax":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-slate-500/50 bg-slate-500/18 px-2.5 py-1 text-xs font-medium text-slate-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-slate-500/50 bg-slate-500/18 px-3 py-1 text-sm font-medium text-slate-100",
			EditorRemove: "text-slate-200/80 hover:text-slate-50",
			Button:       "mt-btn mt-btn--xs border border-slate-500/50 bg-slate-500/15 !text-slate-100 hover:bg-slate-500/25",
			ButtonActive: "mt-btn mt-btn--xs border border-slate-300/80 bg-slate-500/35 !text-slate-50 shadow-[0_0_0_1px_rgba(148,163,184,0.35)]",
		}
	case "aristocrats":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-zinc-700/70 bg-black/45 px-2.5 py-1 text-xs font-medium text-zinc-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-zinc-700/70 bg-black/45 px-3 py-1 text-sm font-medium text-zinc-100",
			EditorRemove: "text-zinc-300/80 hover:text-zinc-50",
			Button:       "mt-btn mt-btn--xs border border-zinc-700/70 bg-black/35 !text-zinc-100 hover:bg-black/55",
			ButtonActive: "mt-btn mt-btn--xs border border-zinc-400/80 bg-zinc-900 !text-zinc-50 shadow-[0_0_0_1px_rgba(161,161,170,0.35)]",
		}
	case "spellslinger":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-orange-500/50 bg-orange-500/18 px-2.5 py-1 text-xs font-medium text-orange-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-orange-500/50 bg-orange-500/18 px-3 py-1 text-sm font-medium text-orange-100",
			EditorRemove: "text-orange-200/80 hover:text-orange-50",
			Button:       "mt-btn mt-btn--xs border border-orange-500/50 bg-orange-500/15 !text-orange-100 hover:bg-orange-500/25",
			ButtonActive: "mt-btn mt-btn--xs border border-orange-400/80 bg-orange-500/35 !text-orange-50 shadow-[0_0_0_1px_rgba(251,146,60,0.4)]",
		}
	case "tokens":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-slate-200/80 bg-white/90 px-2.5 py-1 text-xs font-medium text-slate-900",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-slate-200/80 bg-white/90 px-3 py-1 text-sm font-medium text-slate-900",
			EditorRemove: "text-slate-600 hover:text-slate-950",
			Button:       "mt-btn mt-btn--xs border border-slate-200/80 bg-white/90 !text-slate-900 hover:bg-white",
			ButtonActive: "mt-btn mt-btn--xs border border-white bg-white !text-slate-950 shadow-[0_0_0_1px_rgba(255,255,255,0.35)]",
		}
	case "reanimator":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-violet-700/70 bg-violet-950/65 px-2.5 py-1 text-xs font-medium text-violet-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-violet-700/70 bg-violet-950/65 px-3 py-1 text-sm font-medium text-violet-100",
			EditorRemove: "text-violet-300/80 hover:text-violet-50",
			Button:       "mt-btn mt-btn--xs border border-violet-700/70 bg-violet-950/45 !text-violet-100 hover:bg-violet-900/65",
			ButtonActive: "mt-btn mt-btn--xs border border-violet-400/80 bg-violet-800/80 !text-violet-50 shadow-[0_0_0_1px_rgba(167,139,250,0.35)]",
		}
	case "voltron":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-yellow-500/60 bg-yellow-500/18 px-2.5 py-1 text-xs font-medium text-yellow-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-yellow-500/60 bg-yellow-500/18 px-3 py-1 text-sm font-medium text-yellow-100",
			EditorRemove: "text-yellow-200/80 hover:text-yellow-50",
			Button:       "mt-btn mt-btn--xs border border-yellow-500/60 bg-yellow-500/15 !text-yellow-100 hover:bg-yellow-500/25",
			ButtonActive: "mt-btn mt-btn--xs border border-yellow-300/80 bg-yellow-500/35 !text-yellow-50 shadow-[0_0_0_1px_rgba(253,224,71,0.35)]",
		}
	case "tribal":
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-stone-500/60 bg-stone-700/35 px-2.5 py-1 text-xs font-medium text-stone-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-stone-500/60 bg-stone-700/35 px-3 py-1 text-sm font-medium text-stone-100",
			EditorRemove: "text-stone-200/80 hover:text-stone-50",
			Button:       "mt-btn mt-btn--xs border border-stone-500/60 bg-stone-700/25 !text-stone-100 hover:bg-stone-700/40",
			ButtonActive: "mt-btn mt-btn--xs border border-stone-300/80 bg-stone-700/60 !text-stone-50 shadow-[0_0_0_1px_rgba(214,211,209,0.3)]",
		}
	default:
		return deckTagTheme{
			Public:       "inline-flex items-center rounded-full border border-slate-600 bg-slate-800/80 px-2.5 py-1 text-xs font-medium text-slate-100",
			EditorChip:   "inline-flex items-center gap-2 rounded-full border border-slate-600 bg-slate-800/80 px-3 py-1 text-sm font-medium text-slate-100",
			EditorRemove: "text-slate-300/80 hover:text-slate-50",
			Button:       "mt-btn mt-btn--xs border border-slate-600 bg-slate-800/80 !text-slate-100 hover:bg-slate-700/90",
			ButtonActive: "mt-btn mt-btn--xs border border-slate-300/80 bg-slate-700 !text-slate-50",
		}
	}
}

func publicTagChipClass(tag string) string {
	return deckTagThemeFor(tag).Public
}

func deckTagButtonClass(tag string) string {
	return deckTagThemeFor(tag).Button
}

func deckTagThemeMap() map[string]deckTagTheme {
	out := make(map[string]deckTagTheme, len(decks.SupportedDeckTags()))
	for _, tag := range decks.SupportedDeckTags() {
		out[tag] = deckTagThemeFor(tag)
	}
	return out
}

func NewRenderer(publicBaseURLs ...string) *Renderer {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"buildableFormats":          decks.BuildableFormats,
		"formatIsBuildable":         decks.FormatIsBuildable,
		"supportedFormats":          decks.SupportedFormats,
		"supportedPowerBrackets":    decks.SupportedPowerBrackets,
		"supportedDeckTags":         decks.SupportedDeckTags,
		"splitTags":                 decks.SplitTags,
		"formatRequiresCommander":   decks.FormatRequiresCommander,
		"formatTargetMainboardSize": decks.FormatTargetMainboardSize,
		"toJSON": func(v any) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(string(b))
		},
		"currentYear": func() int {
			return time.Now().Year()
		},
		"projectRepoURL": func() string {
			return projectRepoURL
		},
		"projectIssuesURL": func() string {
			return projectIssuesURL
		},
		"siteVersion": func() string {
			return currentSiteVersion
		},
		"userProfilePath":    userProfilePath,
		"publicTagChipClass": publicTagChipClass,
		"deckTagButtonClass": deckTagButtonClass,
		"deckTagThemeMap":    deckTagThemeMap,
	}).ParseFS(rendererTemplates, "templates/*.html.tmpl")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}
	publicBaseURL := ""
	if len(publicBaseURLs) > 0 {
		publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURLs[0]), "/")
	}
	return &Renderer{tmpl: tmpl, publicBaseURL: publicBaseURL}
}

func defaultActiveNav(name string, data TemplateData) string {
	switch name {
	case "decks_new", "decks_new_commander", "decks_import", "decks_workbench_import_seed":
		return "builder"
	case "deck_show":
		if page, ok := data.Data.(deckPageData); ok && page.WorkbenchMode {
			return "builder"
		}
		return "my-decks"
	case "deck_playtest":
		if data.CurrentUser == nil {
			return "builder"
		}
		return "my-decks"
	case "decks_list":
		return "my-decks"
	case "decks_public", "decks_public_show":
		return "public-decks"
	case "cards_list", "cards_search", "card_show", "commanders_search":
		return "cards"
	case "profile_show":
		return "profile"
	case "settings":
		return "settings"
	case "login", "signup", "forgot_password", "reset_password":
		return "login"
	default:
		return ""
	}
}

func setTemplateDefaults(name string, page *TemplateData) {
	setTemplateDefaultsWithPublicBaseURL(name, page, "")
}

func setTemplateDefaultsWithPublicBaseURL(name string, page *TemplateData, publicBaseURL string) {
	if page.ActiveNav == "" {
		page.ActiveNav = defaultActiveNav(name, *page)
	}
	if name == "cards_list" || name == "deck_show" || name == "decks_new_commander" || name == "decks_public_show" || name == "guess_card" || name == "spellify" || name == "pack_opening" {
		page.WideLayout = true
	}
	if page.Meta == nil {
		page.Meta = defaultPageMeta(name)
	}
	applyPageSEODefaults(name, page, publicBaseURL)
	page.Theme = resolvedSiteTheme(name, *page)
	page.PageID = resolvedPageID(name, *page)
}

func applyPageSEODefaults(name string, page *TemplateData, publicBaseURL string) {
	if page.Meta == nil {
		return
	}
	meta := *page.Meta
	page.Meta = &meta
	if strings.TrimSpace(meta.Robots) == "" {
		meta.Robots = defaultRobotsDirective(name)
	}
	if strings.TrimSpace(meta.CanonicalURL) == "" {
		if path := defaultCanonicalPath(name); path != "" {
			meta.CanonicalURL = absoluteSiteURL(publicBaseURL, path)
		}
	}
	if strings.TrimSpace(meta.Type) == "" {
		meta.Type = "website"
	}
	if strings.TrimSpace(meta.ImageURL) == "" && !strings.HasPrefix(meta.Robots, "noindex") {
		meta.ImageURL = absoluteSiteURL(publicBaseURL, "/assets/manatomb-square-logo.png")
		if meta.ImageURL != "" && strings.TrimSpace(meta.ImageAlt) == "" {
			meta.ImageAlt = "ManaTomb logo"
		}
	}
}

func defaultCanonicalPath(name string) string {
	switch name {
	case "home":
		return "/"
	case "cards_search":
		return "/cards/search"
	case "decks_public":
		return "/decks/public"
	case "guess_card":
		return "/games/guess-card"
	case "spellify":
		return "/games/spellify"
	case "pack_opening":
		return "/games/pack-opening"
	case "changelog":
		return "/changelog"
	case "privacy":
		return "/privacy"
	case "terms":
		return "/terms"
	default:
		return ""
	}
}

func defaultRobotsDirective(name string) string {
	switch name {
	case "decks_list", "deck_show", "deck_playtest", "settings",
		"login", "signup", "forgot_password", "reset_password",
		"cards_list", "decks_new", "decks_new_commander", "commanders_search",
		"decks_import", "decks_workbench_import_seed", "rules_home":
		return "noindex,follow"
	case "not_found", "error":
		return "noindex,nofollow"
	default:
		return "index,follow"
	}
}

func defaultPageMeta(name string) *PageMeta {
	switch name {
	case "home":
		return &PageMeta{
			Title:       "ManaTomb",
			Description: "Search Magic cards, build decks, browse public lists, and play ManaTomb games.",
		}
	case "decks_list":
		return &PageMeta{
			Title:       "My Decks",
			Description: "Open, organize, and continue building your saved Magic decks.",
		}
	case "deck_show":
		return &PageMeta{
			Title:       "Deck Editor",
			Description: "Build, organize, analyze, and playtest a Magic deck.",
		}
	case "deck_playtest":
		return &PageMeta{
			Title:       "Deck Playtest",
			Description: "Playtest a Magic deck in an interactive tabletop workspace.",
		}
	case "decks_public":
		return &PageMeta{
			Title:       "Browse Public Decks",
			Description: "Search and explore public Magic decks shared by ManaTomb players.",
		}
	case "settings":
		return &PageMeta{
			Title:       "Settings",
			Description: "Manage your ManaTomb profile, appearance, and account security.",
		}
	case "profile_show":
		return &PageMeta{
			Title:       "Player Profile",
			Description: "View a ManaTomb player profile, public decks, favorite printings, and achievements.",
		}
	case "login":
		return &PageMeta{
			Title:       "Sign In",
			Description: "Sign in to ManaTomb to access your decks.",
		}
	case "signup":
		return &PageMeta{
			Title:       "Create an Account",
			Description: "Create a ManaTomb account to save and manage your decks.",
		}
	case "forgot_password":
		return &PageMeta{
			Title:       "Reset Your Password",
			Description: "Request a secure password reset link for your ManaTomb account.",
		}
	case "reset_password":
		return &PageMeta{
			Title:       "Choose a New Password",
			Description: "Choose a new password for your ManaTomb account.",
		}
	case "cards_search":
		return &PageMeta{
			Title:       "Advanced Card Search",
			Description: "Find Magic cards by name, color identity, type, printing details, stats, legality, and price.",
		}
	case "cards_list":
		return &PageMeta{
			Title:       "Card Search Results",
			Description: "Browse Magic cards matching your search.",
		}
	case "decks_new":
		return &PageMeta{
			Title:       "Build a Deck",
			Description: "Start a Commander deck, open a blank Sandbox deck, or import an existing decklist.",
		}
	case "decks_new_commander":
		return &PageMeta{
			Title:       "Choose a Commander",
			Description: "Search eligible Magic cards or choose a popular commander to start a deck.",
		}
	case "commanders_search":
		return &PageMeta{
			Title:       "Choose a Commander",
			Description: "Search eligible commander cards and choose an exact printing for your deck.",
		}
	case "decks_import":
		return &PageMeta{
			Title:       "Import a Decklist",
			Description: "Import a Magic: The Gathering decklist and continue editing it in ManaTomb.",
		}
	case "decks_workbench_import_seed":
		return &PageMeta{
			Title:       "Preparing Deck Import",
			Description: "Preparing an imported decklist for the ManaTomb deck editor.",
		}
	case "guess_card":
		return &PageMeta{
			Title:       "Guess the Card",
			Description: "Ask questions, reveal clues, and identify the hidden Magic card.",
		}
	case "spellify":
		return &PageMeta{
			Title:       "Tombscript",
			Description: "Reveal characters across a hidden Magic card, then name it before your clues run out.",
		}
	case "pack_opening":
		return &PageMeta{
			Title:       "Pack Crack",
			Description: "Choose a Magic set, slide open a simulated booster, and reveal every pull.",
		}
	case "changelog":
		return &PageMeta{
			Title:       "Changelog",
			Description: "See what is new in ManaTomb, from major feature releases to smaller improvements and fixes.",
		}
	case "privacy":
		return &PageMeta{
			Title:       "Privacy Notice",
			Description: "Learn what ManaTomb stores, which services it uses, and the controls available to you.",
		}
	case "terms":
		return &PageMeta{
			Title:       "Terms of Use",
			Description: "Read the concise terms for using the free, non-commercial ManaTomb project.",
		}
	case "rules_home":
		return &PageMeta{
			Title:       "Rules",
			Description: "ManaTomb rules reference.",
		}
	case "not_found":
		return &PageMeta{
			Title:       "Page Not Found",
			Description: "The requested ManaTomb page could not be found.",
		}
	case "error":
		return &PageMeta{
			Title:       "Something Went Wrong",
			Description: "ManaTomb could not complete the request.",
		}
	default:
		return nil
	}
}

func applyTemplateDefaults(name string, data any) any {
	return applyTemplateDefaultsWithPublicBaseURL(name, data, "")
}

func applyTemplateDefaultsWithPublicBaseURL(name string, data any, publicBaseURL string) any {
	switch page := data.(type) {
	case TemplateData:
		setTemplateDefaultsWithPublicBaseURL(name, &page, publicBaseURL)
		return page
	case *TemplateData:
		if page != nil {
			setTemplateDefaultsWithPublicBaseURL(name, page, publicBaseURL)
		}
	}
	return data
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	data = applyTemplateDefaultsWithPublicBaseURL(name, data, r.publicBaseURL)

	// Render into a buffer first so we don't write partial HTML and then
	// attempt to write an error response.
	if err := r.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template error (%s): %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	// Ensure browsers render the response as HTML.
	// Only set this if nothing else already has (e.g. file downloads).
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}

	_, _ = w.Write(buf.Bytes())
}
