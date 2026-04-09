package web

import "net/http"

func (a *App) HandlePrivacy(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
	}

	a.Renderer.Render(w, "privacy", data)
}

func (a *App) HandleTerms(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
	}

	a.Renderer.Render(w, "terms", data)
}
