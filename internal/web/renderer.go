package web

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

type Renderer struct {
	tmpl *template.Template
}

func NewRenderer() *Renderer {
	pattern := filepath.Join("internal", "web", "templates", "*.html.tmpl")
	tmpl, err := template.ParseGlob(pattern)
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
