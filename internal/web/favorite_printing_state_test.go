package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
)

func TestApplyPrintingFavoriteStatusTracksSelectedAndOtherPrintings(t *testing.T) {
	t.Parallel()

	const (
		oracleID  = "123e4567-e89b-12d3-a456-426614174000"
		selected  = "223e4567-e89b-12d3-a456-426614174000"
		alternate = "323e4567-e89b-12d3-a456-426614174000"
	)
	page := buildCardDetailPageData(cards.Card{
		ID:       selected,
		OracleID: oracleID,
		Name:     "Favorite Test",
	}, []cards.Card{
		{ID: selected, OracleID: oracleID, Name: "Favorite Test"},
		{ID: alternate, OracleID: oracleID, Name: "Favorite Test"},
	})

	applyPrintingFavoriteStatus(&page, map[string]bool{strings.ToUpper(alternate): true})
	if page.FavoritePrintingCount != 1 || page.OtherFavoritePrintingCount != 1 {
		t.Fatalf("favorite counts = %d total, %d other; want 1 and 1", page.FavoritePrintingCount, page.OtherFavoritePrintingCount)
	}
	if page.SelectedPrintingIsFavorited {
		t.Fatal("selected printing unexpectedly marked as favorited")
	}
	if page.Printings[0].IsFavorited || !page.Printings[1].IsFavorited {
		t.Fatalf("printing favorite state = %#v, want only alternate favorited", page.Printings)
	}

	applyPrintingFavoriteStatus(&page, map[string]bool{selected: true, alternate: true})
	if !page.SelectedPrintingIsFavorited || page.FavoritePrintingCount != 2 || page.OtherFavoritePrintingCount != 1 {
		t.Fatalf("selected favorite state = selected %v, %d total, %d other; want true, 2, 1", page.SelectedPrintingIsFavorited, page.FavoritePrintingCount, page.OtherFavoritePrintingCount)
	}

	applyPrintingFavoriteStatus(&page, nil)
	if page.SelectedPrintingIsFavorited || page.FavoritePrintingCount != 0 || page.OtherFavoritePrintingCount != 0 {
		t.Fatalf("empty favorite state did not reset aggregate fields: %#v", page)
	}
	if page.Printings[0].IsFavorited || page.Printings[1].IsFavorited {
		t.Fatalf("empty favorite state did not reset printings: %#v", page.Printings)
	}
}

func TestCardShowExplainsCrossPrintingFavoriteAndMarksGallery(t *testing.T) {
	const (
		oracleID  = "123e4567-e89b-12d3-a456-426614174000"
		selected  = "223e4567-e89b-12d3-a456-426614174000"
		alternate = "323e4567-e89b-12d3-a456-426614174000"
	)
	page := buildCardDetailPageData(cards.Card{
		ID:         selected,
		OracleID:   oracleID,
		Name:       "Favorite Test",
		TypeLine:   "Artifact",
		OracleText: "Test text.",
	}, []cards.Card{
		{ID: selected, OracleID: oracleID, Name: "Favorite Test", SetName: "Current Set"},
		{ID: alternate, OracleID: oracleID, Name: "Favorite Test", SetName: "Favorite Set"},
	})
	applyPrintingFavoriteStatus(&page, map[string]bool{alternate: true})

	body := renderTemplate(t, "card_show", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Collector"},
		Data:        page,
	})
	for _, needle := range []string{
		`data-card-favorite-context`,
		`You favorited another printing.`,
		`class="mt-printing-tile__favorite"`,
		`(isFavorited ? ", favorited" : "")`,
		`name="favorite_action" value="favorite"`,
		`var favoriteEndpoint = trimValue(form.getAttribute("action"))`,
		`window.fetch(favoriteEndpoint`,
		`"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8"`,
		`var favoriteBody = new URLSearchParams();`,
		`body: favoriteBody`,
		`printing.IsFavorited = !!payload.favorited`,
		`favoritePrintingCount: Number(`,
		`form.submit();`,
		`var profileArtMenus = Array.from(document.querySelectorAll("[data-card-profile-art-menu]"));`,
		`if (menu.open && !menu.contains(event.target)) menu.open = false;`,
		`if (event.key === "Escape") closeProfileArtMenus();`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("signed-in card page missing cross-printing favorite behavior %q", needle)
		}
	}

	applyPrintingFavoriteStatus(&page, map[string]bool{selected: true, alternate: true})
	body = renderTemplate(t, "card_show", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Collector"},
		Data:        page,
	})
	for _, needle := range []string{
		`value="unfavorite" data-card-favorite-action`,
		`class="mt-heart-button is-loved"`,
		`Remove this printing from favorites`,
		`This printing and 1 other are in your favorites.`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("selected favorite state missing %q", needle)
		}
	}
}

func TestCardFavoriteSubmissionAvoidsFirefoxFormActionClobbering(t *testing.T) {
	page := buildCardDetailPageData(cards.Card{
		ID:       "223e4567-e89b-12d3-a456-426614174000",
		OracleID: "123e4567-e89b-12d3-a456-426614174000",
		Name:     "Favorite Test",
	}, nil)
	body := renderTemplate(t, "card_show", TemplateData{
		CurrentUser: &account.User{ID: 42, DisplayName: "Collector"},
		Data:        page,
	})

	if strings.Contains(body, `window.fetch(form.action`) {
		t.Fatal("card page still reads the clobberable HTMLFormElement.action property")
	}
	if !strings.Contains(body, `name="favorite_action"`) {
		t.Fatal("card favorite form should use the non-clobbering favorite_action field")
	}
	if strings.Contains(body, `name="action" data-card-favorite-action`) {
		t.Fatal("card favorite form retained the field name that shadows form.action")
	}
}

func TestFavoritePrintingActionPrefersNewFieldAndSupportsLegacyClients(t *testing.T) {
	t.Parallel()

	if got := favoritePrintingAction(url.Values{
		"favorite_action": {"unfavorite"},
		"action":          {"favorite"},
	}); got != "unfavorite" {
		t.Fatalf("new favorite action = %q, want unfavorite", got)
	}
	if got := favoritePrintingAction(url.Values{"action": {" UnFavorite "}}); got != "unfavorite" {
		t.Fatalf("legacy favorite action = %q, want unfavorite", got)
	}
}

func TestParseCardPrintingFavoriteFormAcceptsURLencodedAndMultipartBodies(t *testing.T) {
	t.Parallel()

	const (
		printingID = "223e4567-e89b-12d3-a456-426614174000"
		returnTo   = "/cards/view/123e4567-e89b-12d3-a456-426614174000"
	)
	wantValues := url.Values{
		"scryfall_id":     {printingID},
		"favorite_action": {"favorite"},
		"return_to":       {returnTo},
	}

	urlEncodedRequest := httptest.NewRequest(
		http.MethodPost,
		"/cards/favorites/printing",
		strings.NewReader(wantValues.Encode()),
	)
	urlEncodedRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := parseCardPrintingFavoriteForm(urlEncodedRequest); err != nil {
		t.Fatalf("parse URL-encoded favorite form: %v", err)
	}
	assertFavoriteFormValues(t, urlEncodedRequest, wantValues)

	var multipartBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBody)
	for key, values := range wantValues {
		for _, value := range values {
			if err := multipartWriter.WriteField(key, value); err != nil {
				t.Fatalf("write multipart field %q: %v", key, err)
			}
		}
	}
	if err := multipartWriter.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	multipartRequest := httptest.NewRequest(
		http.MethodPost,
		"/cards/favorites/printing",
		&multipartBody,
	)
	multipartRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	if err := parseCardPrintingFavoriteForm(multipartRequest); err != nil {
		t.Fatalf("parse multipart favorite form: %v", err)
	}
	assertFavoriteFormValues(t, multipartRequest, wantValues)
}

func TestParseCardPrintingFavoriteFormRejectsMultipartWithoutBoundary(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/cards/favorites/printing", strings.NewReader("not-a-form"))
	req.Header.Set("Content-Type", "multipart/form-data")
	if err := parseCardPrintingFavoriteForm(req); err == nil {
		t.Fatal("multipart favorite form without a boundary unexpectedly parsed")
	}
}

func assertFavoriteFormValues(t *testing.T, req *http.Request, want url.Values) {
	t.Helper()
	for key, values := range want {
		if got := req.Form[key]; len(got) != len(values) || strings.Join(got, "\x00") != strings.Join(values, "\x00") {
			t.Errorf("favorite form %q = %#v, want %#v", key, got, values)
		}
	}
}

func TestFavoriteJSONResponseContract(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/cards/favorites/printing", nil)
	req.Header.Set("Accept", "text/html, application/json")
	if !wantsFavoriteJSON(req) {
		t.Fatal("application/json Accept header was not detected")
	}

	rec := httptest.NewRecorder()
	writeFavoriteJSON(rec, http.StatusOK, cardPrintingFavoriteResponse{
		ScryfallID: "223e4567-e89b-12d3-a456-426614174000",
		Favorited:  true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("favorite JSON status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("favorite JSON Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("favorite JSON Cache-Control = %q", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"favorited":true`) {
		t.Fatalf("favorite JSON body = %q", body)
	}
}
