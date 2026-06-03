package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"manatomb/app/internal/decks"
)

const (
	projectRepoURL   = "https://github.com/quehorrifico/manatomb-v3"
	projectIssuesURL = "https://github.com/quehorrifico/manatomb-v3/issues"
)

type Renderer struct {
	tmpl *template.Template
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

func NewRenderer() *Renderer {
	pattern := filepath.Join("internal", "web", "templates", "*.html.tmpl")
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"supportedFormats":        decks.SupportedFormats,
		"supportedPowerBrackets":  decks.SupportedPowerBrackets,
		"supportedDeckTags":       decks.SupportedDeckTags,
		"splitTags":               decks.SplitTags,
		"formatRequiresCommander": decks.FormatRequiresCommander,
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
		"userProfilePath":    userProfilePath,
		"publicTagChipClass": publicTagChipClass,
		"deckTagButtonClass": deckTagButtonClass,
		"deckTagThemeMap":    deckTagThemeMap,
	}).ParseGlob(pattern)
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}
	return &Renderer{tmpl: tmpl}
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer

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
