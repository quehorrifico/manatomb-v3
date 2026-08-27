package web

import (
	"encoding/json"
	"log"
	"mime"
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
	if err := parseCardPrintingFavoriteForm(r); err != nil {
		if wantsFavoriteJSON(r) {
			writeFavoriteJSON(w, http.StatusBadRequest, cardPrintingFavoriteResponse{Error: "Invalid favorite request."})
			return
		}
		setFlash(w, "Invalid favorite request.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	returnTo := normalizeLocalReturnPath(r.Form.Get("return_to"), "/")
	user := CurrentUser(r)
	if user == nil {
		if wantsFavoriteJSON(r) {
			writeFavoriteJSON(w, http.StatusUnauthorized, cardPrintingFavoriteResponse{
				Error:    "Sign in to update favorites.",
				LoginURL: "/login?next=" + url.QueryEscape(returnTo),
			})
			return
		}
		http.Redirect(w, r, "/login?next="+url.QueryEscape(returnTo), http.StatusSeeOther)
		return
	}

	returnTo = normalizeLocalReturnPath(r.Form.Get("return_to"), userProfilePath(user.ID))
	scryfallID := strings.TrimSpace(r.Form.Get("scryfall_id"))
	if _, err := uuid.Parse(scryfallID); err != nil {
		if wantsFavoriteJSON(r) {
			writeFavoriteJSON(w, http.StatusBadRequest, cardPrintingFavoriteResponse{Error: "Choose a valid card printing to favorite."})
			return
		}
		setFlash(w, "Choose a valid card printing to favorite.")
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
		return
	}

	action := favoritePrintingAction(r.Form)
	favorite := action != "unfavorite"
	if err := setFavoritePrinting(r.Context(), a.DB, user.ID, scryfallID, favorite); err != nil {
		log.Printf("favorite printing update failed: user_id=%d scryfall_id=%s favorite=%t error=%v", user.ID, scryfallID, favorite, err)
		if wantsFavoriteJSON(r) {
			writeFavoriteJSON(w, http.StatusInternalServerError, cardPrintingFavoriteResponse{Error: "Could not update favorites. Try again."})
			return
		}
		setFlash(w, "Could not update favorite printing.")
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
		return
	}
	if wantsFavoriteJSON(r) {
		writeFavoriteJSON(w, http.StatusOK, cardPrintingFavoriteResponse{
			ScryfallID: scryfallID,
			Favorited:  favorite,
		})
		return
	}

	if favorite {
		setFlash(w, "Printing added to favorites.")
	} else {
		setFlash(w, "Printing removed from favorites.")
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func favoritePrintingAction(values url.Values) string {
	action := strings.TrimSpace(values.Get("favorite_action"))
	if action == "" {
		// Keep accepting the original field name for non-JavaScript clients and
		// already-open pages. New forms avoid `name="action"` because that name
		// shadows HTMLFormElement.action in Firefox.
		action = values.Get("action")
	}
	return strings.ToLower(strings.TrimSpace(action))
}

const maxCardPrintingFavoriteMultipartMemory = 64 << 10

// parseCardPrintingFavoriteForm accepts both a browser's normal URL-encoded
// form submission and FormData's multipart encoding. Request.ParseForm alone
// deliberately does not read multipart bodies.
func parseCardPrintingFavoriteForm(r *http.Request) error {
	if r == nil {
		return http.ErrMissingBoundary
	}

	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	mediaType, _, mediaTypeErr := mime.ParseMediaType(contentType)
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		if mediaTypeErr != nil {
			return mediaTypeErr
		}
		if !strings.EqualFold(mediaType, "multipart/form-data") {
			return http.ErrNotMultipart
		}
		return r.ParseMultipartForm(maxCardPrintingFavoriteMultipartMemory)
	}

	return r.ParseForm()
}

type cardPrintingFavoriteResponse struct {
	ScryfallID string `json:"scryfall_id,omitempty"`
	Favorited  bool   `json:"favorited"`
	Error      string `json:"error,omitempty"`
	LoginURL   string `json:"login_url,omitempty"`
}

func wantsFavoriteJSON(r *http.Request) bool {
	return r != nil && strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json")
}

func writeFavoriteJSON(w http.ResponseWriter, status int, response cardPrintingFavoriteResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
