package web

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"manatomb/app/internal/account"
	"manatomb/app/internal/decks"

	"github.com/google/uuid"
)

const ctxKeyUser ctxKey = "currentUser"
const sessionCookieName = "mt_session"

const (
	// A browser-session login still expires server-side if a browser keeps the
	// session cookie alive unusually long. Persistent logins are deliberately
	// bounded rather than acting as permanent credentials.
	defaultSessionTTL           = 24 * time.Hour
	defaultPersistentSessionTTL = 30 * 24 * time.Hour
)

type ctxKey string

type notFoundRecorder struct {
	rw     http.ResponseWriter
	header http.Header
	status int
	buf    bytes.Buffer
}

type App struct {
	DB                  *sql.DB
	Renderer            *Renderer
	SessionCookieSecure bool
	PublicBaseURL       string
	PasswordResetMailer PasswordResetMailer
	TrustedProxyHops    int
	rateLimitsOnce      sync.Once
	rateLimits          *appRateLimiters
	packCatalogMu       sync.Mutex
	packCatalog         *packOpeningCatalogSnapshot
}

type TemplateData struct {
	CurrentUser *account.User
	Data        any
	Meta        *PageMeta
	Flash       string
	Error       string
	ActiveNav   string
	WideLayout  bool
	HideHeader  bool
	HideFooter  bool
	Theme       SiteTheme
	PageID      string
}

type PageMeta struct {
	Title        string
	Description  string
	CanonicalURL string
	ImageURL     string
	ImageAlt     string
	Type         string
	Robots       string
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

// authNextPath sends ordinary authentication to My Decks. The only resumable
// exception is a marked guest-workbench save, whose draft remains in this
// browser until the returned workbench imports it into the signed-in account.
func authNextPath(raw string) string {
	const fallback = "/decks"

	path := normalizeLocalReturnPath(raw, fallback)
	parsed, err := url.Parse(path)
	if err != nil {
		return fallback
	}
	if parsed.Path != "/decks/new/workbench" {
		return fallback
	}

	query := parsed.Query()
	if strings.TrimSpace(query.Get("save_guest")) != "1" {
		return fallback
	}

	sandbox := strings.TrimSpace(query.Get("sandbox")) == "1"
	format := query.Get("format")
	commanderName := query.Get("commander_name")
	commanderPrintID := query.Get("commander_print_id")
	if sandbox {
		format = "Sandbox"
		commanderName = ""
		commanderPrintID = ""
	} else {
		format = defaultDeckFormat(format, commanderName, "")
		if !decks.FormatRequiresCommander(format) {
			commanderName = ""
			commanderPrintID = ""
		}
	}

	return deckWorkbenchPath(deckWorkbenchOptions{
		Format:           format,
		CommanderName:    commanderName,
		CommanderPrintID: commanderPrintID,
		Sandbox:          sandbox,
		SaveWorkbench:    true,
	})
}

type loginPageData struct {
	Email        string
	Next         string
	StaySignedIn bool
}

type signupPageData struct {
	DisplayName  string
	Email        string
	Next         string
	StaySignedIn bool
}

type forgotPasswordPageData struct {
	Email string
}

type resetPasswordPageData struct {
	Token string
}

func resetPasswordURL(publicBaseURL, token string) string {
	return strings.TrimRight(publicBaseURL, "/") + "/reset-password?token=" + url.QueryEscape(token)
}

func wantsPersistentSession(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

func authSessionTTL(staySignedIn bool) time.Duration {
	if staySignedIn {
		return defaultPersistentSessionTTL
	}
	return defaultSessionTTL
}

func sessionCookie(session *account.Session, secure, staySignedIn bool) *http.Cookie {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if staySignedIn {
		cookie.Expires = session.ExpiresAt.UTC()
		cookie.MaxAge = int(session.ExpiresAt.Sub(session.CreatedAt) / time.Second)
	}
	return cookie
}

// ===== Handlers =====

func (a *App) HandleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := CurrentUser(r)
	flash := readFlash(w, r)
	a.Renderer.Render(w, "home", TemplateData{
		CurrentUser: user,
		Flash:       flash,
		HideHeader:  true,
	})
}

func (a *App) HandleSignupShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)

	next := authNextPath(r.URL.Query().Get("next"))

	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Data: signupPageData{
			Next:         next,
			StaySignedIn: true,
		},
		Flash: flash,
		Error: "",
	}

	a.Renderer.Render(w, "signup", data)
}

func (a *App) HandleSignupPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Printf("signup parse form error: %v", err)
		data := TemplateData{
			Data:  signupPageData{StaySignedIn: true},
			Error: "Invalid form submission. Please try again.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	email := strings.TrimSpace(r.Form.Get("email"))
	displayName := strings.TrimSpace(r.Form.Get("display_name"))
	password := r.Form.Get("password")
	staySignedIn := wantsPersistentSession(r.Form.Get("stay_signed_in"))

	next := authNextPath(r.Form.Get("next"))

	// Basic validation
	if displayName == "" || email == "" || password == "" {
		data := TemplateData{
			Data: signupPageData{
				DisplayName:  displayName,
				Email:        email,
				Next:         next,
				StaySignedIn: staySignedIn,
			},
			Error: "Display name, email, and password are required.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	if len(password) < 8 {
		data := TemplateData{
			Data: signupPageData{
				DisplayName:  displayName,
				Email:        email,
				Next:         next,
				StaySignedIn: staySignedIn,
			},
			Error: "Password must be at least 8 characters long.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	u, err := account.CreateUser(r.Context(), a.DB, email, displayName, password)
	if err != nil {
		log.Printf("create user error during signup")
		data := TemplateData{
			Data: signupPageData{
				DisplayName:  displayName,
				Email:        email,
				Next:         next,
				StaySignedIn: staySignedIn,
			},
			Error: "Could not create account. This email may already be in use.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	sess, err := account.CreateSession(r.Context(), a.DB, u.ID, authSessionTTL(staySignedIn))
	if err != nil {
		log.Printf("create session error: %v", err)
		data := TemplateData{
			Data: signupPageData{
				DisplayName:  displayName,
				Email:        email,
				Next:         next,
				StaySignedIn: staySignedIn,
			},
			Error: "Account created, but we couldn't sign you in automatically. Please try signing in.",
		}
		a.Renderer.Render(w, "signup", data)
		return
	}

	http.SetCookie(w, sessionCookie(sess, a.SessionCookieSecure, staySignedIn))

	log.Printf("signup success: userID=%d, redirecting to %s", u.ID, next)
	setFlash(w, "Account created. Welcome to ManaTomb!")
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (a *App) HandleLoginShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)

	next := authNextPath(r.URL.Query().Get("next"))

	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Data: loginPageData{
			Next:         next,
			StaySignedIn: true,
		},
		Flash: flash,
		Error: "",
	}

	a.Renderer.Render(w, "login", data)
}

func (a *App) HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		data := TemplateData{
			Data:  loginPageData{StaySignedIn: true},
			Error: "Invalid form submission. Please try again.",
		}
		a.Renderer.Render(w, "login", data)
		return
	}

	email := strings.TrimSpace(r.Form.Get("email"))
	password := r.Form.Get("password")
	staySignedIn := wantsPersistentSession(r.Form.Get("stay_signed_in"))

	next := authNextPath(r.Form.Get("next"))

	u, err := account.Authenticate(r.Context(), a.DB, email, password)
	if err != nil {
		if !errors.Is(err, account.ErrInvalidCredentials) {
			log.Printf("authenticate error: %v", err)
		}
		data := TemplateData{
			Data: loginPageData{
				Email:        email,
				Next:         next,
				StaySignedIn: staySignedIn,
			},
			Error: "Invalid email or password.",
		}
		a.Renderer.Render(w, "login", data)
		return
	}

	sess, err := account.CreateSession(r.Context(), a.DB, u.ID, authSessionTTL(staySignedIn))
	if err != nil {
		log.Printf("create session error: %v", err)
		data := TemplateData{
			Data: loginPageData{
				Email:        email,
				Next:         next,
				StaySignedIn: staySignedIn,
			},
			Error: "We couldn't sign you in. Please try again.",
		}
		a.Renderer.Render(w, "login", data)
		return
	}

	http.SetCookie(w, sessionCookie(sess, a.SessionCookieSecure, staySignedIn))

	setFlash(w, "Welcome back!")
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (a *App) HandleForgotPasswordShow(w http.ResponseWriter, r *http.Request) {
	flash := readFlash(w, r)
	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Data:        forgotPasswordPageData{},
		Flash:       flash,
		Error:       "",
	}
	a.Renderer.Render(w, "forgot_password", data)
}

func (a *App) HandleForgotPasswordPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		data := TemplateData{
			Data:  forgotPasswordPageData{},
			Error: "Invalid form submission. Please try again.",
		}
		a.Renderer.Render(w, "forgot_password", data)
		return
	}

	email := strings.TrimSpace(r.Form.Get("email"))
	if a.PasswordResetMailer == nil {
		log.Printf("password reset delivery unavailable: SMTP is not configured")
		setFlash(w, "If an account exists for that email, a password reset link will be sent shortly.")
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}

	reset, found, err := account.CreatePasswordResetToken(r.Context(), a.DB, email, time.Hour)
	if err != nil {
		log.Printf("create password reset token error: %v", err)
		data := TemplateData{
			Data:  forgotPasswordPageData{Email: email},
			Error: "Could not start password reset. Please try again.",
		}
		a.Renderer.Render(w, "forgot_password", data)
		return
	}

	if found && reset != nil {
		if err := a.PasswordResetMailer.SendPasswordReset(
			r.Context(),
			reset.Email,
			resetPasswordURL(a.PublicBaseURL, reset.Token),
		); err != nil {
			log.Printf("password reset email delivery failed: userID=%d error=%v", reset.UserID, err)
		}
	}

	setFlash(w, "If an account exists for that email, a password reset link will be sent shortly.")
	http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
}

func (a *App) HandleResetPasswordShow(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Data:        resetPasswordPageData{Token: token},
		Error:       "",
	}
	if token == "" {
		data.Error = "Reset link is missing a token."
	}
	a.Renderer.Render(w, "reset_password", data)
}

func (a *App) HandleResetPasswordPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		data := TemplateData{
			Data:  resetPasswordPageData{},
			Error: "Invalid form submission. Please try again.",
		}
		a.Renderer.Render(w, "reset_password", data)
		return
	}

	token := strings.TrimSpace(r.Form.Get("token"))
	password := r.Form.Get("password")
	confirm := r.Form.Get("password_confirm")
	data := TemplateData{
		CurrentUser: CurrentUser(r),
		Data:        resetPasswordPageData{Token: token},
	}
	if token == "" {
		data.Error = "Reset link is missing a token."
		a.Renderer.Render(w, "reset_password", data)
		return
	}
	if len(password) < 8 {
		data.Error = "Password must be at least 8 characters long."
		a.Renderer.Render(w, "reset_password", data)
		return
	}
	if password != confirm {
		data.Error = "Passwords do not match."
		a.Renderer.Render(w, "reset_password", data)
		return
	}

	if err := account.ResetPasswordWithToken(r.Context(), a.DB, token, password); err != nil {
		if errors.Is(err, account.ErrInvalidResetToken) {
			data.Error = "Reset link is invalid or expired."
			a.Renderer.Render(w, "reset_password", data)
			return
		}
		log.Printf("reset password error: %v", err)
		data.Error = "Could not reset password. Please try again."
		a.Renderer.Render(w, "reset_password", data)
		return
	}

	setFlash(w, "Password updated. Please sign in with your new password.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
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
		Secure:   a.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
