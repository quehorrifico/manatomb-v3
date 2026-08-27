package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCardSearchQueryCanonicalRedirectOnlyRemovesSubmittedNoise(t *testing.T) {
	canonical := url.Values{
		"q":           {"lightning"},
		"search_mode": {"advanced"},
		"sort":        {"relevance"},
		"sort_dir":    {"desc"},
	}

	if cardSearchQueryNeedsCanonicalRedirect(url.Values{
		"q":           {"lightning"},
		"search_mode": {"advanced"},
	}, canonical) {
		t.Fatal("missing implicit canonical values should not force a redirect")
	}

	for name, raw := range map[string]url.Values{
		"empty field": {
			"q":         {"lightning"},
			"mana_cost": {""},
		},
		"inactive default": {
			"q":          {"lightning"},
			"color_mode": {"includes"},
		},
		"changed value": {
			"q": {"lightning bolt"},
		},
		"unknown field": {
			"q":       {"lightning"},
			"unknown": {"1"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if !cardSearchQueryNeedsCanonicalRedirect(raw, canonical) {
				t.Fatalf("cardSearchQueryNeedsCanonicalRedirect(%v) = false, want true", raw)
			}
		})
	}
}

func TestHandleCardListCanonicalizesInlineFilterSubmissionBeforeQuerying(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest(
		http.MethodGet,
		"/cards?sort=relevance&sort_dir=desc&search_mode=advanced&q=lightning&mana_cost=&text=draw+a+card&color_mode=includes&set=&artist=&layout=&stat=mana_value&stat_op=eq&stat_value=&price_op=eq&price_value=",
		nil,
	)
	rr := httptest.NewRecorder()

	app.HandleCardList(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("HandleCardList() status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	const want = "/cards?q=lightning&search_mode=advanced&sort=relevance&sort_dir=desc&text=draw+a+card"
	if got := rr.Header().Get("Location"); got != want {
		t.Fatalf("HandleCardList() Location = %q, want %q", got, want)
	}
}
