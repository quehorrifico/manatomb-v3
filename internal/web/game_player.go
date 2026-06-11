package web

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

const guestGameCookieName = "mt_guest_game"

type gamePlayer struct {
	UserID  int64
	GuestID string
}

func (p gamePlayer) IsGuest() bool {
	return p.UserID <= 0
}

func (p gamePlayer) userIDValue() any {
	if p.UserID > 0 {
		return p.UserID
	}
	return nil
}

func (p gamePlayer) guestIDValue() any {
	if p.GuestID != "" {
		return p.GuestID
	}
	return nil
}

func gamePlayerFromGame(userID int64, guestID string) gamePlayer {
	return gamePlayer{UserID: userID, GuestID: guestID}
}

func (a *App) gamePlayer(w http.ResponseWriter, r *http.Request) gamePlayer {
	if user := CurrentUser(r); user != nil {
		return gamePlayer{UserID: user.ID}
	}
	if cookie, err := r.Cookie(guestGameCookieName); err == nil {
		if id, err := uuid.Parse(cookie.Value); err == nil {
			return gamePlayer{GuestID: id.String()}
		}
	}

	guestID := uuid.NewString()
	http.SetCookie(w, &http.Cookie{
		Name:     guestGameCookieName,
		Value:    guestID,
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   a.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return gamePlayer{GuestID: guestID}
}
