package web

import (
	"net/http"
	"net/url"
	"strings"
)

func unsafeRequestOriginAllowed(r *http.Request, publicBaseURL string) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	configured, err := url.Parse(strings.TrimSpace(publicBaseURL))
	return err == nil && strings.EqualFold(parsed.Scheme, configured.Scheme) && strings.EqualFold(parsed.Host, configured.Host)
}

// WithSecurityHeadersMiddleware applies a small, broadly compatible security
// baseline. The content policy intentionally limits only framing, form targets,
// document base URLs, and plugins because ManaTomb still has inline page scripts
// and styles that will be migrated to nonce-backed assets separately.
func (a *App) WithSecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			header.Add("Vary", "Cookie")
			if _, err := r.Cookie(sessionCookieName); err == nil {
				header.Set("Cache-Control", "private, no-store")
			}
		}
		header.Set("Content-Security-Policy", "base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'")
		header.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		if a.SessionCookieSecure {
			header.Set("Strict-Transport-Security", "max-age=31536000")
		}
		if !unsafeRequestOriginAllowed(r, a.PublicBaseURL) {
			http.Error(w, "cross-origin request denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
