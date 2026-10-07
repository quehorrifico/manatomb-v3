package web

import "net/http"

// HandleExtras is the public discovery page for games and playtesting.
func (a *App) HandleExtras(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/extras" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.Renderer.Render(w, "extras", TemplateData{CurrentUser: CurrentUser(r), ActiveNav: "extras"})
}
