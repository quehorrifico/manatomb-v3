package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

func (a *App) HandleCardPrintingFavoritePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		setFlash(w, "Invalid favorite request.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	returnTo := normalizeLocalReturnPath(r.Form.Get("return_to"), "/")
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(returnTo), http.StatusSeeOther)
		return
	}

	returnTo = normalizeLocalReturnPath(r.Form.Get("return_to"), userProfilePath(user.ID))
	scryfallID := strings.TrimSpace(r.Form.Get("scryfall_id"))
	if _, err := uuid.Parse(scryfallID); err != nil {
		setFlash(w, "Choose a valid card printing to favorite.")
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
		return
	}

	action := strings.ToLower(strings.TrimSpace(r.Form.Get("action")))
	favorite := action != "unfavorite"
	if err := setFavoritePrinting(r.Context(), a.DB, user.ID, scryfallID, favorite); err != nil {
		setFlash(w, "Could not update favorite printing.")
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
		return
	}

	if favorite {
		setFlash(w, "Loved card version")
	} else {
		setFlash(w, "Favorite art removed.")
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}
