package web

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"manatomb/app/internal/account"
	"manatomb/app/internal/decks"

	"github.com/google/uuid"
)

const ctxKeyUser ctxKey = "currentUser"
const sessionCookieName = "mt_session"

type ctxKey string

type notFoundRecorder struct {
	rw     http.ResponseWriter
	header http.Header
	status int
	buf    bytes.Buffer
}

type App struct {
	DB       *sql.DB
	Renderer *Renderer
}

type TemplateData struct {
	CurrentUser *account.User
	Data        any
	Flash       string
	Error       string
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
		a.RenderServerError(w, r, err)
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
		Data: struct {
			DisplayName string
			Email       string
		}{},
		Flash: flash,
		Error: "",
	}

	a.Renderer.Render(w, "signup", data)
}

func (a *App) HandleSignupPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Printf("signup parse form error: %v", err)
		data := TemplateData{
			Data: struct {
				DisplayName string
				Email       string
			}{},
			Error: "Invalid form submission. Please try again.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	email := strings.TrimSpace(r.Form.Get("email"))
	displayName := strings.TrimSpace(r.Form.Get("display_name"))
	password := r.Form.Get("password")

	log.Printf("signup attempt: email=%s displayName=%s", email, displayName)

	// Basic validation
	if displayName == "" || email == "" || password == "" {
		data := TemplateData{
			Data: struct {
				DisplayName string
				Email       string
			}{
				DisplayName: displayName,
				Email:       email,
			},
			Error: "Display name, email, and password are required.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	if len(password) < 8 {
		data := TemplateData{
			Data: struct {
				DisplayName string
				Email       string
			}{
				DisplayName: displayName,
				Email:       email,
			},
			Error: "Password must be at least 8 characters long.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	u, err := account.CreateUser(r.Context(), a.DB, email, displayName, password)
	if err != nil {
		log.Printf("create user error: %v", err)
		data := TemplateData{
			Data: struct {
				DisplayName string
				Email       string
			}{
				DisplayName: displayName,
				Email:       email,
			},
			Error: "Could not create account. This email may already be in use.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	sess, err := account.CreateSession(r.Context(), a.DB, u.ID, 7*24*time.Hour)
	if err != nil {
		log.Printf("create session error: %v", err)
		data := TemplateData{
			Data: struct {
				DisplayName string
				Email       string
			}{
				DisplayName: displayName,
				Email:       email,
			},
			Error: "Account created, but we couldn't log you in automatically. Please try logging in.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sess.ID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // set true in prod when behind HTTPS
	})

	log.Printf("signup success: userID=%d, redirecting to /decks", u.ID)
	setFlash(w, "Account created. Welcome to Mana Tomb!")
	http.Redirect(w, r, "/decks", http.StatusSeeOther)
}

func (a *App) HandleLoginShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)

	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Data: struct {
			Email string
		}{},
		Flash: flash,
		Error: "",
	}

	a.Renderer.Render(w, "login", data)
}

func (a *App) HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		data := TemplateData{
			Data: struct {
				Email string
			}{},
			Error: "Invalid form submission. Please try again.",
		}
		a.Renderer.Render(w, "login", data)
		return
	}

	email := strings.TrimSpace(r.Form.Get("email"))
	password := r.Form.Get("password")

	u, err := account.Authenticate(r.Context(), a.DB, email, password)
	if err != nil {
		log.Printf("authenticate error: %v", err)
		data := TemplateData{
			Data: struct {
				Email string
			}{
				Email: email,
			},
			Error: "Invalid email or password.",
		}
		a.Renderer.Render(w, "login", data)
		return
	}

	sess, err := account.CreateSession(r.Context(), a.DB, u.ID, 7*24*time.Hour)
	if err != nil {
		log.Printf("create session error: %v", err)
		data := TemplateData{
			Data: struct {
				Email string
			}{
				Email: email,
			},
			Error: "Could not create session. Please try logging in again.",
		}
		a.Renderer.Render(w, "login", data)
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

// ClearSessionCookie clears the current session in the database (if present)
// and removes the session cookie from the client.
func (a *App) ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
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
}

func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	a.ClearSessionCookie(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) WithUserMiddleware(next http.Handler) http.Handler {
	return a.withCurrentUser(next)
}

func (a *App) RenderNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)

	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Data:        nil,
		Flash:       "",
		Error:       "",
	}

	a.Renderer.Render(w, "not_found", data)
}

func (a *App) RenderServerError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("server error: %v", err)

	w.WriteHeader(http.StatusInternalServerError)

	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Data:        nil,
		Flash:       "",
		Error:       "",
	}

	a.Renderer.Render(w, "error", data)
}

func (a *App) WithRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Log panic + stack trace
				log.Printf("panic: %v\n%s", rec, debug.Stack())

				// Show pretty 500 error page
				a.RenderServerError(w, r, fmt.Errorf("panic: %v", rec))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (r *notFoundRecorder) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *notFoundRecorder) WriteHeader(status int) {
	r.status = status
}

func (r *notFoundRecorder) Write(b []byte) (int, error) {
	return r.buf.Write(b)
}

func (a *App) WithNotFoundMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &notFoundRecorder{
			rw:     w,
			status: http.StatusOK,
		}

		next.ServeHTTP(rec, r)

		// If the wrapped handler (typically the mux) reported a 404,
		// render our pretty not_found page instead of the default text.
		if rec.status == http.StatusNotFound {
			a.RenderNotFound(w, r)
			return
		}

		// Otherwise, copy recorded headers and body through to the real ResponseWriter.
		for k, vv := range rec.Header() {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}

		// If no status was explicitly set, treat it as 200 OK.
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(rec.buf.Bytes())
	})
}
