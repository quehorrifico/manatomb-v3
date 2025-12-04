package web

import (
	"net/http"
)

func (a *App) HandleRulesHome(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		// Data/Error unused for now
	}

	a.Renderer.Render(w, "rules_home", data)
}
