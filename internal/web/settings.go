package web

import (
	"net/http"
	"regexp"
	"strings"

	"manatomb/app/internal/account"
)

type settingsFormData struct {
	DisplayName string
	Email       string
}

var displayNameRegex = regexp.MustCompile(`^[a-zA-Z0-9 .,_'-]{1,32}$`)

func isValidDisplayName(name string) bool {
	return displayNameRegex.MatchString(name)
}

// GET /settings
func (a *App) HandleSettingsShow(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	flash := readFlash(w, r)

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		Data: settingsFormData{
			DisplayName: user.DisplayName,
			Email:       user.Email,
		},
	}

	a.Renderer.Render(w, "settings", data)
}

// POST /settings (different actions based on hidden "action" field)
func (a *App) HandleSettingsPost(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		setFlash(w, "Invalid form submission.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	action := r.Form.Get("action")

	switch action {
	case "update_profile":
		displayName := strings.TrimSpace(r.Form.Get("display_name"))

		if displayName == "" {
			setFlash(w, "Display name is required.")
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}

		if !isValidDisplayName(displayName) {
			setFlash(w, "Please choose a simpler display name (letters, numbers, spaces, basic punctuation).")
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}

		if err := account.UpdateProfile(r.Context(), a.DB, user.ID, displayName, user.Email); err != nil {
			setFlash(w, "Could not update profile.")
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}

		setFlash(w, "Profile updated.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return

	case "change_password":
		current := r.Form.Get("current_password")
		newPW := r.Form.Get("new_password")
		confirm := r.Form.Get("confirm_password")

		if newPW == "" || confirm == "" {
			setFlash(w, "New password and confirmation are required.")
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}
		if newPW != confirm {
			setFlash(w, "New password and confirmation do not match.")
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}

		if err := account.ChangePassword(r.Context(), a.DB, user.ID, current, newPW); err != nil {
			setFlash(w, "Could not change password. Check your current password.")
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}

		setFlash(w, "Password changed.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return

	case "delete_account":
		// You can optionally require a confirm string or password here later.
		if err := account.DeleteAccount(r.Context(), a.DB, user.ID); err != nil {
			setFlash(w, "Could not delete account.")
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}

		// Reuse logout behavior: clear session cookie
		a.ClearSessionCookie(w, r)
		setFlash(w, "Account deleted.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return

	default:
		setFlash(w, "Unknown action.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
}
