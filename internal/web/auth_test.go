package web

import (
	"net/http"
	"testing"
	"time"

	"manatomb/app/internal/account"

	"github.com/google/uuid"
)

func TestAuthNextPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "blank falls back", raw: "", want: "/decks"},
		{name: "generic local path falls back", raw: "/decks/public", want: "/decks"},
		{name: "generic local query falls back", raw: "/decks/new?format=Commander", want: "/decks"},
		{name: "settings falls back", raw: " /settings ", want: "/decks"},
		{
			name: "guest workbench save resumes",
			raw:  "/decks/new/workbench?save_guest=1",
			want: "/decks/new/workbench?format=Sandbox&save_guest=1",
		},
		{
			name: "commander guest workbench preserves builder selectors",
			raw:  "/decks/new/workbench?save_guest=1&format=Commander&commander_name=Atraxa%2C+Praetors%27+Voice",
			want: "/decks/new/workbench?commander_name=Atraxa%2C+Praetors%27+Voice&format=Commander&save_guest=1",
		},
		{
			name: "commander guest workbench preserves selected printing",
			raw:  "/decks/new/workbench?save_guest=1&format=Commander&commander_name=Atraxa&commander_print_id=223e4567-e89b-12d3-a456-426614174000",
			want: "/decks/new/workbench?commander_name=Atraxa&commander_print_id=223e4567-e89b-12d3-a456-426614174000&format=Commander&save_guest=1",
		},
		{
			name: "other commander formats preserve commander state",
			raw:  "/decks/new/workbench?save_guest=1&format=Historic+Brawl&commander_name=Atraxa&commander_print_id=223e4567-e89b-12d3-a456-426614174000",
			want: "/decks/new/workbench?commander_name=Atraxa&commander_print_id=223e4567-e89b-12d3-a456-426614174000&format=Historic+Brawl&save_guest=1",
		},
		{
			name: "sandbox guest workbench normalizes conflicting selectors",
			raw:  "/decks/new/workbench?save_guest=1&sandbox=1&format=Commander&commander_name=Atraxa&commander_print_id=223e4567-e89b-12d3-a456-426614174000",
			want: "/decks/new/workbench?format=Sandbox&sandbox=1&save_guest=1",
		},
		{
			name: "guest workbench strips reset unknown parameters and fragment",
			raw:  "/decks/new/workbench?save_guest=1&format=Commander&commander_name=Krenko&reset=1&unknown=value#cards",
			want: "/decks/new/workbench?commander_name=Krenko&format=Commander&save_guest=1",
		},
		{name: "workbench without save marker falls back", raw: "/decks/new/workbench?format=Commander", want: "/decks"},
		{name: "workbench with disabled save marker falls back", raw: "/decks/new/workbench?save_guest=0", want: "/decks"},
		{name: "workbench prefix lookalike falls back", raw: "/decks/new/workbench/archive?save_guest=1", want: "/decks"},
		{name: "reject scheme relative", raw: "//evil.example", want: "/decks"},
		{name: "reject absolute url", raw: "https://evil.example", want: "/decks"},
		{name: "reject backslash authority", raw: `/\evil.example`, want: "/decks"},
		{name: "reject login loop", raw: "/login", want: "/decks"},
		{name: "reject signup loop", raw: "/signup?next=%2Flogin", want: "/decks"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := authNextPath(tc.raw); got != tc.want {
				t.Fatalf("authNextPath(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestResetPasswordURLUsesConfiguredPublicBaseURL(t *testing.T) {
	t.Parallel()

	got := resetPasswordURL("https://manatomb.app/", "a token/with spaces")
	want := "https://manatomb.app/reset-password?token=a+token%2Fwith+spaces"
	if got != want {
		t.Fatalf("resetPasswordURL() = %q, want %q", got, want)
	}
}

func TestWantsPersistentSession(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"1", "true", "TRUE", "on", "yes"} {
		if !wantsPersistentSession(raw) {
			t.Errorf("wantsPersistentSession(%q) = false, want true", raw)
		}
	}
	for _, raw := range []string{"", "0", "false", "off", "unexpected"} {
		if wantsPersistentSession(raw) {
			t.Errorf("wantsPersistentSession(%q) = true, want false", raw)
		}
	}
}

func TestAuthSessionTTLsAreBounded(t *testing.T) {
	t.Parallel()

	if got := authSessionTTL(false); got != 24*time.Hour {
		t.Fatalf("authSessionTTL(false) = %s, want 24h", got)
	}
	if got := authSessionTTL(true); got != 30*24*time.Hour {
		t.Fatalf("authSessionTTL(true) = %s, want 30 days", got)
	}
}

func TestSessionCookiePersistenceAndSecurityAttributes(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	session := &account.Session{
		ID:        uuid.MustParse("223e4567-e89b-12d3-a456-426614174000"),
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(defaultPersistentSessionTTL),
	}

	persistent := sessionCookie(session, true, true)
	if persistent.Name != sessionCookieName || persistent.Value != session.ID.String() || persistent.Path != "/" {
		t.Fatalf("persistent session cookie identity = %#v", persistent)
	}
	if !persistent.HttpOnly || !persistent.Secure || persistent.SameSite != http.SameSiteLaxMode {
		t.Fatalf("persistent session cookie lost security attributes: %#v", persistent)
	}
	if persistent.MaxAge != int(defaultPersistentSessionTTL/time.Second) {
		t.Fatalf("persistent session cookie MaxAge = %d, want %d", persistent.MaxAge, int(defaultPersistentSessionTTL/time.Second))
	}
	if !persistent.Expires.Equal(session.ExpiresAt) {
		t.Fatalf("persistent session cookie Expires = %s, want %s", persistent.Expires, session.ExpiresAt)
	}

	browserSession := sessionCookie(session, true, false)
	if browserSession.MaxAge != 0 || !browserSession.Expires.IsZero() {
		t.Fatalf("browser-session cookie unexpectedly persists: %#v", browserSession)
	}
	if !browserSession.HttpOnly || !browserSession.Secure || browserSession.SameSite != http.SameSiteLaxMode {
		t.Fatalf("browser-session cookie lost security attributes: %#v", browserSession)
	}
}
