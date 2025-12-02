package web

import (
	"net/http"
	"net/url"
)

const flashCookieName = "flash"

func setFlash(w http.ResponseWriter, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookieName,
		Value:    url.QueryEscape(msg),
		Path:     "/",
		MaxAge:   5, // a few seconds is enough; it will be read on next request
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure: true, // enable this when you're on HTTPS in prod
	})
}

// readFlash reads the flash cookie (if present), deletes it, and returns the message.
func readFlash(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie(flashCookieName)
	if err != nil {
		return ""
	}

	// Delete the cookie
	http.SetCookie(w, &http.Cookie{
		Name:   flashCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	msg, _ := url.QueryUnescape(c.Value)
	return msg
}
