package web

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"manatomb/app/internal/account"
	"manatomb/app/internal/decks"

	"github.com/google/uuid"
)

type ctxKey string

const ctxKeyUser ctxKey = "currentUser"
const sessionCookieName = "mt_session"

type App struct {
	DB       *sql.DB
	Renderer *Renderer
}

type TemplateData struct {
	CurrentUser *account.User
	Data        any
	Flash       string
}

func (a *App) withCurrentUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var currentUser *account.User

		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && cookie.Value != "" {
			if sid, err := uuid.Parse(cookie.Value); err == nil {
				if u, err := account.GetUserBySession(r.Context(), a.DB, sid); err == nil {
					currentUser = u
				}
			}
		}

		ctx := context.WithValue(r.Context(), ctxKeyUser, currentUser)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CurrentUser(r *http.Request) *account.User {
	u, _ := r.Context().Value(ctxKeyUser).(*account.User)
	return u
}

// ===== Handlers =====

func (a *App) HandleHome(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	flash := readFlash(w, r)

	// If not logged in, show a simple landing page
	if user == nil {
		data := TemplateData{
			CurrentUser: nil,
			Data:        nil, // no extra data needed
			Flash:       flash,
		}
		a.Renderer.Render(w, "home", data)
		return
	}

	// Logged-in dashboard: show a few recent decks
	userDecks, err := decks.ListDecksByUser(r.Context(), a.DB, user.ID)
	if err != nil {
		http.Error(w, "could not load decks", http.StatusInternalServerError)
		return
	}

	// Optionally limit to first 5 for the dashboard
	if len(userDecks) > 5 {
		userDecks = userDecks[:5]
	}

	type homeData struct {
		RecentDecks []decks.Deck
	}

	data := TemplateData{
		CurrentUser: user,
		Data: homeData{
			RecentDecks: userDecks,
		},
		Flash: flash,
	}

	a.Renderer.Render(w, "home", data)
}
func (a *App) HandleSignupShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Flash:       flash,
	}
	a.Renderer.Render(w, "signup", data)
}

func (a *App) HandleSignupPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Printf("signup parse form error: %v", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	email := r.Form.Get("email")
	displayName := r.Form.Get("display_name")
	password := r.Form.Get("password")

	log.Printf("signup attempt: email=%s displayName=%s", email, displayName)

	u, err := account.CreateUser(r.Context(), a.DB, email, displayName, password)
	if err != nil {
		log.Printf("create user error: %v", err)
		http.Error(w, "could not create user", http.StatusInternalServerError)
		return
	}

	sess, err := account.CreateSession(r.Context(), a.DB, u.ID, 7*24*time.Hour)
	if err != nil {
		log.Printf("create session error: %v", err)
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sess.ID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // true in prod
	})

	log.Printf("signup success: userID=%d, redirecting to /decks", u.ID)
	setFlash(w, "Account created. Welcome to Mana Tomb!")
	// http.Redirect(w, r, "/", http.StatusSeeOther)
	http.Redirect(w, r, "/decks", http.StatusSeeOther)
}

func (a *App) HandleLoginShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Flash:       flash,
	}
	a.Renderer.Render(w, "login", data)
}

func (a *App) HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	email := r.Form.Get("email")
	password := r.Form.Get("password")

	u, err := account.Authenticate(r.Context(), a.DB, email, password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	sess, err := account.CreateSession(r.Context(), a.DB, u.ID, 7*24*time.Hour)
	if err != nil {
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sess.ID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
	})

	setFlash(w, "Welcome back!")
	http.Redirect(w, r, "/decks", http.StatusSeeOther)
}

func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		if sid, err := uuid.Parse(cookie.Value); err == nil {
			_ = account.DeleteSession(r.Context(), a.DB, sid)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) WithUserMiddleware(next http.Handler) http.Handler {
	return a.withCurrentUser(next)
}
