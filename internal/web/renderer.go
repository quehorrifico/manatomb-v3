package web

import (
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
	err := r.tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
