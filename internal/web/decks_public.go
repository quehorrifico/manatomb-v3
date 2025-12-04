package web

import (
	"net/http"
)

func (a *App) HandlePublicDecks(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		// Data is unused for now
	}

	a.Renderer.Render(w, "decks_public", data)
}
