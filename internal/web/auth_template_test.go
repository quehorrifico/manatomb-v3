package web

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readAuthCSS(t *testing.T) string {
	t.Helper()
	withRendererRoot(t)

	body, err := os.ReadFile(filepath.Join("internal", "web", "assets", "auth.css"))
	if err != nil {
		t.Fatalf("read auth stylesheet: %v", err)
	}
	return string(body)
}

func TestAuthTemplatesUseSharedMinimalShell(t *testing.T) {
	tests := []struct {
		name     string
		template string
		pageID   string
		title    string
		data     any
		source   string
	}{
		{
			name:     "sign in",
			template: "login",
			pageID:   "login",
			title:    "Sign In",
			data:     loginPageData{Next: "/decks"},
			source:   "login.html.tmpl",
		},
		{
			name:     "sign up",
			template: "signup",
			pageID:   "signup",
			title:    "Create an Account",
			data:     signupPageData{Next: "/decks"},
			source:   "signup.html.tmpl",
		},
		{
			name:     "forgot password",
			template: "forgot_password",
			pageID:   "forgot-password",
			title:    "Reset Your Password",
			data:     forgotPasswordPageData{},
			source:   "forgot_password.html.tmpl",
		},
		{
			name:     "reset password",
			template: "reset_password",
			pageID:   "reset-password",
			title:    "Choose a New Password",
			data:     resetPasswordPageData{Token: "reset-token"},
			source:   "reset_password.html.tmpl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := renderTemplate(t, tt.template, TemplateData{Data: tt.data})
			htmlTag := renderedOpeningTag(t, body, "html")
			bodyTag := renderedOpeningTag(t, body, "body")

			for _, needle := range []string{
				`data-theme="tomb"`,
				`href="/assets/auth.css"`,
				`class="mt-page-shell `,
				`class="mt-auth-page"`,
				`class="mt-auth-form"`,
				`<title>` + tt.title + ` | ManaTomb</title>`,
				`mt-site-account-action" aria-current="page"`,
				`<footer id="site-footer"`,
			} {
				if !strings.Contains(body, needle) {
					t.Fatalf("%s page missing %q: %s", tt.template, needle, body)
				}
			}
			if !strings.Contains(htmlTag, `data-theme="tomb"`) {
				t.Fatalf("%s did not receive the Tomb palette: %s", tt.template, htmlTag)
			}
			if !strings.Contains(bodyTag, `data-page="`+tt.pageID+`"`) {
				t.Fatalf("%s did not receive page ID %q: %s", tt.template, tt.pageID, bodyTag)
			}
			if got := strings.Count(body, "<main"); got != 1 {
				t.Fatalf("%s rendered %d main landmarks, want one", tt.template, got)
			}
			if got := strings.Count(body, "<h1"); got != 1 {
				t.Fatalf("%s rendered %d h1 elements, want one", tt.template, got)
			}

			source, err := os.ReadFile(filepath.Join("internal", "web", "templates", tt.source))
			if err != nil {
				t.Fatalf("read %s: %v", tt.source, err)
			}
			for _, forbidden := range []string{
				`<main`,
				`mt-panel`,
				`mt-kicker`,
				`text-slate-`,
				`text-sky-`,
				`>Account</p>`,
				`>Account recovery</p>`,
				`>Home</a>`,
			} {
				if strings.Contains(string(source), forbidden) {
					t.Fatalf("%s still contains legacy auth treatment %q", tt.source, forbidden)
				}
			}
		})
	}
}

func TestAuthShowUsesMyDecksUnlessSavingGuestWorkbench(t *testing.T) {
	withRendererRoot(t)
	app := &App{Renderer: NewRenderer()}
	builderReturn := "/decks/new/workbench?save_guest=1&format=Commander&commander_name=Krenko&reset=1"
	wantBuilderReturn := "/decks/new/workbench?commander_name=Krenko&format=Commander&save_guest=1"

	tests := []struct {
		name     string
		path     string
		handle   func(http.ResponseWriter, *http.Request)
		want     string
		switchTo string
	}{
		{
			name:     "sign in defaults to My Decks",
			path:     "/login?next=%2Fsettings",
			handle:   app.HandleLoginShow,
			want:     "/decks",
			switchTo: "/signup",
		},
		{
			name:     "sign up defaults to My Decks",
			path:     "/signup?next=%2Fgames%2Fspellify",
			handle:   app.HandleSignupShow,
			want:     "/decks",
			switchTo: "/login",
		},
		{
			name:     "sign in keeps guest builder save",
			path:     "/login?next=" + url.QueryEscape(builderReturn),
			handle:   app.HandleLoginShow,
			want:     wantBuilderReturn,
			switchTo: "/signup",
		},
		{
			name:     "sign up keeps guest builder save",
			path:     "/signup?next=" + url.QueryEscape(builderReturn),
			handle:   app.HandleSignupShow,
			want:     wantBuilderReturn,
			switchTo: "/login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			tt.handle(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s returned status %d: %s", tt.path, rec.Code, rec.Body.String())
			}

			body := rec.Body.String()
			hiddenNext := `name="next" value="` + html.EscapeString(tt.want) + `"`
			if !strings.Contains(body, hiddenNext) {
				t.Fatalf("%s did not render sanitized next %q: %s", tt.path, hiddenNext, body)
			}
			switchLink := `href="` + tt.switchTo + `?next=` + url.QueryEscape(tt.want) + `"`
			if !strings.Contains(body, switchLink) {
				t.Fatalf("%s did not preserve sanitized next across auth forms: %s", tt.path, body)
			}
			if !strings.Contains(body, `name="stay_signed_in"`) || !strings.Contains(body, "checked") {
				t.Fatalf("%s did not default to a persistent session: %s", tt.path, body)
			}
		})
	}
}

func TestLoginTemplateUsesPasswordManagerSemantics(t *testing.T) {
	body := renderTemplate(t, "login", TemplateData{
		Data: loginPageData{
			Email: "brewer@example.com",
			Next:  "/decks",
		},
	})

	for _, needle := range []string{
		`<label for="login-email" class="mt-field-label">Email</label>`,
		`id="login-email"`,
		`value="brewer@example.com"`,
		`autocomplete="email"`,
		`autocapitalize="none"`,
		`spellcheck="false"`,
		`<label for="login-password" class="mt-field-label">Password</label>`,
		`id="login-password"`,
		`autocomplete="current-password"`,
		`href="/forgot-password" class="mt-auth-link">Forgot password?</a>`,
		`name="next" value="/decks"`,
		`href="/signup?next=%2Fdecks"`,
		`>Sign In</button>`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("login page missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, "autofocus") {
		t.Fatalf("login page unexpectedly forces focus: %s", body)
	}
}

func TestAuthTemplatesOfferPersistentLoginCheckedByDefault(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     any
		inputID  string
	}{
		{
			name:     "login",
			template: "login",
			data:     loginPageData{Next: "/decks", StaySignedIn: true},
			inputID:  "login-stay-signed-in",
		},
		{
			name:     "signup",
			template: "signup",
			data:     signupPageData{Next: "/decks", StaySignedIn: true},
			inputID:  "signup-stay-signed-in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := renderTemplate(t, tt.template, TemplateData{Data: tt.data})
			inputStart := strings.Index(body, `id="`+tt.inputID+`"`)
			if inputStart == -1 {
				t.Fatalf("%s page is missing stay-signed-in input: %s", tt.template, body)
			}
			inputEnd := strings.Index(body[inputStart:], ">")
			if inputEnd == -1 {
				t.Fatalf("%s page has malformed stay-signed-in input: %s", tt.template, body)
			}
			input := body[inputStart : inputStart+inputEnd]
			for _, needle := range []string{`type="checkbox"`, `name="stay_signed_in"`, `value="1"`, "checked"} {
				if !strings.Contains(input, needle) {
					t.Fatalf("%s stay-signed-in input missing %q: %s", tt.template, needle, input)
				}
			}
			for _, copy := range []string{"Stay signed in on this device", "For 30 days. Uncheck this on a shared device."} {
				if !strings.Contains(body, copy) {
					t.Fatalf("%s page missing persistent-login copy %q: %s", tt.template, copy, body)
				}
			}
		})
	}
}

func TestSignupTemplateExplainsAccountRequirements(t *testing.T) {
	body := renderTemplate(t, "signup", TemplateData{
		Data: signupPageData{
			DisplayName:  "Deck Brewer",
			Email:        "brewer@example.com",
			Next:         "/decks",
			StaySignedIn: true,
		},
	})

	for _, needle := range []string{
		`autocomplete="nickname"`,
		`autocomplete="email"`,
		`autocomplete="new-password"`,
		`minlength="8"`,
		`aria-describedby="signup-password-hint"`,
		`id="signup-password-hint" class="mt-auth-hint">Use at least 8 characters.</p>`,
		`href="/login?next=%2Fdecks"`,
		`href="/terms" class="mt-auth-link">Terms of Use</a>`,
		`href="/privacy" class="mt-auth-link">Privacy Notice</a>`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("signup page missing %q: %s", needle, body)
		}
	}
	if strings.Contains(body, "autofocus") {
		t.Fatalf("signup page unexpectedly forces focus: %s", body)
	}
}

func TestPasswordRecoveryKeepsTokensOutOfVisibleFields(t *testing.T) {
	forgotPassword := renderTemplate(t, "forgot_password", TemplateData{
		Data: forgotPasswordPageData{Email: "brewer@example.com"},
	})
	for _, needle := range []string{
		`value="brewer@example.com"`,
		`autocomplete="email"`,
		`autocapitalize="none"`,
		`>Send Reset Link</button>`,
	} {
		if !strings.Contains(forgotPassword, needle) {
			t.Fatalf("forgot-password page missing %q: %s", needle, forgotPassword)
		}
	}

	withToken := renderTemplate(t, "reset_password", TemplateData{
		Data: resetPasswordPageData{Token: "secret-reset-token"},
	})
	for _, needle := range []string{
		`<input type="hidden" name="token" value="secret-reset-token">`,
		`autocomplete="new-password"`,
		`minlength="8"`,
		`id="reset-password-hint" class="mt-auth-hint">Use at least 8 characters.</p>`,
	} {
		if !strings.Contains(withToken, needle) {
			t.Fatalf("reset-password page missing %q: %s", needle, withToken)
		}
	}
	if strings.Contains(withToken, `id="reset-password-token"`) {
		t.Fatalf("reset-password page exposes a supplied token in a visible field: %s", withToken)
	}

	withoutToken := renderTemplate(t, "reset_password", TemplateData{
		Data: resetPasswordPageData{},
	})
	for _, needle := range []string{
		`id="reset-password-token"`,
		`name="token"`,
		`autocomplete="off"`,
		`Paste the token from your password reset email.`,
	} {
		if !strings.Contains(withoutToken, needle) {
			t.Fatalf("manual reset-token form missing %q: %s", needle, withoutToken)
		}
	}
	if strings.Contains(withoutToken, "autofocus") {
		t.Fatalf("reset-password page unexpectedly forces focus: %s", withoutToken)
	}
}

func TestAuthErrorIsAccessibleAndThemeDriven(t *testing.T) {
	body := renderTemplate(t, "login", TemplateData{
		Data:  loginPageData{Email: "brewer@example.com", Next: "/decks"},
		Error: "Invalid email or password.",
	})

	for _, needle := range []string{
		`id="site-error" class="mt-flash mt-flash--error" role="alert"`,
		`class="mt-auth-form" aria-describedby="site-error"`,
		`value="brewer@example.com"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("accessible auth error missing %q: %s", needle, body)
		}
	}

	alertStart := strings.Index(body, `id="site-error"`)
	if alertStart == -1 {
		t.Fatalf("could not find auth error alert: %s", body)
	}
	alertEnd := strings.Index(body[alertStart:], `</div>`)
	if alertEnd == -1 {
		t.Fatalf("could not isolate auth error alert: %s", body)
	}
	alert := body[alertStart : alertStart+alertEnd]
	for _, forbidden := range []string{"red-", "rgb(", "rgba(", "#"} {
		if strings.Contains(alert, forbidden) {
			t.Fatalf("auth error alert hardcodes %q: %s", forbidden, alert)
		}
	}
}

func TestAuthStylesAreFlatTokenDrivenAndMotionless(t *testing.T) {
	css := readAuthCSS(t)

	for _, needle := range []string{
		`.mt-auth-page {`,
		`max-width: 30rem;`,
		`.mt-auth-form {`,
		`border-top: 1px solid var(--mt-border-subtle);`,
		`.mt-auth-page .mt-field-label {`,
		`font-size: 0.75rem;`,
		`color: var(--mt-text-soft);`,
		`color: var(--mt-accent-text);`,
		`.mt-auth-session-choice {`,
		`accent-color: var(--mt-accent);`,
		`animation: none;`,
		`transition: none;`,
		`transform: none;`,
	} {
		if !strings.Contains(css, needle) {
			t.Fatalf("auth stylesheet missing %q", needle)
		}
	}
	for _, forbidden := range []string{"rgb(", "rgba(", "#", "box-shadow:", "@keyframes", "prefers-reduced-motion"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("auth stylesheet contains non-token or motion treatment %q", forbidden)
		}
	}
}

func TestLoginHeadingIsCentered(t *testing.T) {
	css := readAuthCSS(t)
	headingRule := renderedCSSRule(t, css, `body[data-page="login"] .mt-auth-heading`)
	if !strings.Contains(headingRule, "text-align: center;") {
		t.Fatalf("login heading is not centered: %s", headingRule)
	}
	copyRule := renderedCSSRule(t, css, `body[data-page="login"] .mt-auth-copy`)
	for _, needle := range []string{"margin-right: auto;", "margin-left: auto;"} {
		if !strings.Contains(copyRule, needle) {
			t.Fatalf("centered login copy rule missing %q: %s", needle, copyRule)
		}
	}
}
