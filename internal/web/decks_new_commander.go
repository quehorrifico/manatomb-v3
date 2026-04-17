package web

import (
	"net/http"
	"net/url"
	"strings"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
)

type commanderDeckBuilderPageData struct {
	Query       string
	Results     []searchResult
	Recommended []searchResult
}

type commanderDeckBuilderState struct {
	Query string
}

func commanderDeckBuilderPath(state commanderDeckBuilderState) string {
	values := url.Values{}

	if query := strings.TrimSpace(state.Query); query != "" {
		values.Set("q", query)
	}

	path := "/decks/new/commander/"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

func commanderDeckBuilderStateFromValues(values url.Values) commanderDeckBuilderState {
	return commanderDeckBuilderState{
		Query: strings.TrimSpace(values.Get("q")),
	}
}

func (a *App) startCommanderDeck(w http.ResponseWriter, r *http.Request, commanderName string) {
	commanderName = strings.TrimSpace(commanderName)
	if commanderName == "" {
		a.renderDeckCommanderBuilder(w, r, CurrentUser(r), "", "Choose a commander first.", commanderDeckBuilderState{})
		return
	}

	user := CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, deckWorkbenchPath(deckWorkbenchOptions{
			Format:        "Commander",
			CommanderName: commanderName,
			Reset:         true,
		}), http.StatusSeeOther)
		return
	}

	a.createSavedBuilderDeck(w, r, user, "Commander", commanderName)
}

func (a *App) renderDeckCommanderBuilder(
	w http.ResponseWriter,
	r *http.Request,
	user *account.User,
	flash string,
	errMsg string,
	state commanderDeckBuilderState,
) {
	var (
		searchResultsRaw []cards.Card
		recommendedRaw   []cards.Card
	)

	if state.Query != "" {
		found, err := cards.SearchCards(r.Context(), a.DB, cards.CardSearchParams{
			Query:         state.Query,
			CommanderOnly: true,
			Limit:         18,
		})
		if err != nil {
			if errMsg == "" {
				errMsg = "There was a problem searching for commanders. Please try again."
			}
		} else {
			searchResultsRaw = found
		}
	}

	if recommended, err := cards.RandomTopCommanders(r.Context(), a.DB, 24, 1500); err == nil {
		recommendedRaw = recommended
	}

	data := TemplateData{
		CurrentUser: user,
		Flash:       flash,
		Error:       errMsg,
		Data: commanderDeckBuilderPageData{
			Query:       state.Query,
			Results:     buildSearchResults(searchResultsRaw),
			Recommended: buildSearchResults(recommendedRaw),
		},
	}

	a.Renderer.Render(w, "decks_new_commander", data)
}

func (a *App) HandleDeckNewCommanderRedirect(w http.ResponseWriter, r *http.Request) {
	target := "/decks/new/commander/"
	if raw := r.URL.RawQuery; raw != "" {
		target += "?" + raw
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (a *App) HandleDeckNewCommanderShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.renderDeckCommanderBuilder(
		w,
		r,
		CurrentUser(r),
		readFlash(w, r),
		"",
		commanderDeckBuilderStateFromValues(r.URL.Query()),
	)
}
