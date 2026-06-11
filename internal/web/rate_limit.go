package web

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateLimitBucket struct {
	start time.Time
	count int
}

type fixedWindowRateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	buckets map[string]rateLimitBucket
}

type appRateLimiters struct {
	global         *fixedWindowRateLimiter
	authentication *fixedWindowRateLimiter
	forgotPassword *fixedWindowRateLimiter
	packOpening    *fixedWindowRateLimiter
	games          *fixedWindowRateLimiter
}

func newFixedWindowRateLimiter(limit int, window time.Duration) *fixedWindowRateLimiter {
	return &fixedWindowRateLimiter{
		window:  window,
		limit:   limit,
		buckets: make(map[string]rateLimitBucket),
	}
}

func (l *fixedWindowRateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket := l.buckets[key]
	if bucket.start.IsZero() || now.Sub(bucket.start) >= l.window {
		l.buckets[key] = rateLimitBucket{start: now, count: 1}
		return true, 0
	}
	if bucket.count >= l.limit {
		return false, l.window - now.Sub(bucket.start)
	}
	bucket.count++
	l.buckets[key] = bucket

	if len(l.buckets) > 10000 {
		for bucketKey, candidate := range l.buckets {
			if now.Sub(candidate.start) >= l.window {
				delete(l.buckets, bucketKey)
			}
		}
		for bucketKey := range l.buckets {
			if len(l.buckets) <= 10000 {
				break
			}
			delete(l.buckets, bucketKey)
		}
	}
	return true, 0
}

func requestClientIP(r *http.Request, trustedProxyHops int) string {
	if trustedProxyHops > 0 {
		forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if len(forwarded) >= trustedProxyHops {
			candidate := strings.TrimSpace(forwarded[len(forwarded)-trustedProxyHops])
			if net.ParseIP(candidate) != nil {
				return candidate
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func enforceRateLimit(w http.ResponseWriter, limiter *fixedWindowRateLimiter, key string) bool {
	allowed, retryAfter := limiter.allow(key, time.Now())
	if allowed {
		return true
	}
	seconds := int(retryAfter.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	http.Error(w, "too many requests", http.StatusTooManyRequests)
	return false
}

func (a *App) WithRateLimitMiddleware(next http.Handler) http.Handler {
	a.rateLimitsOnce.Do(func() {
		a.rateLimits = &appRateLimiters{
			global:         newFixedWindowRateLimiter(240, time.Minute),
			authentication: newFixedWindowRateLimiter(20, 15*time.Minute),
			forgotPassword: newFixedWindowRateLimiter(5, 15*time.Minute),
			packOpening:    newFixedWindowRateLimiter(30, time.Minute),
			games:          newFixedWindowRateLimiter(120, time.Minute),
		}
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/assets/") {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := requestClientIP(r, a.TrustedProxyHops)
		if !enforceRateLimit(w, a.rateLimits.global, clientIP) {
			return
		}
		if r.Method == http.MethodPost &&
			(r.URL.Path == "/login" || r.URL.Path == "/signup" || r.URL.Path == "/reset-password") &&
			!enforceRateLimit(w, a.rateLimits.authentication, clientIP) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/forgot-password" &&
			!enforceRateLimit(w, a.rateLimits.forgotPassword, clientIP) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/games/pack-opening" &&
			!enforceRateLimit(w, a.rateLimits.packOpening, clientIP) {
			return
		}
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/games/") &&
			!enforceRateLimit(w, a.rateLimits.games, clientIP) {
			return
		}
		next.ServeHTTP(w, r)
	})
}
