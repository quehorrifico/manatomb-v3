package cards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// CardFace represents one face of a multi-faced card (MDFC, split, etc.).
type CardFace struct {
	Name       string   `json:"name"`
	ManaCost   string   `json:"mana_cost"`
	TypeLine   string   `json:"type_line"`
	OracleText string   `json:"oracle_text"`
	ImageURI   string   `json:"image_uri"`
	Artist     string   `json:"artist"`
	Colors     []string `json:"colors"`
	ColorID    []string `json:"color_identity"`
}

// Card is the normalized view of a card used by ManaTomb. It hides the
// complexity of different Scryfall layouts (normal, MDFC, split, etc.) and
// exposes a single "primary face" plus commander-relevant metadata.
type Card struct {
	ID       string
	OracleID string

	Name       string
	ManaCost   string
	TypeLine   string
	OracleText string
	ImageURI   string

	Colors         []string
	ColorIdentity  []string
	CMC            float64
	Layout         string
	CommanderLegal bool

	// Extra metadata used by the UI (not required to be persisted).
	PriceUSD   string
	Artist     string
	EDHRecRank int

	ScryfallURI string
	SetCode     string
	SetName     string

	// Faces contains all faces for MDFC/similar layouts. For normal cards this will be empty.
	Faces []CardFace `json:"faces"`
}

type scryfallCard struct {
	ID       string `json:"id"`
	OracleID string `json:"oracle_id"`

	Name       string            `json:"name"`
	ManaCost   string            `json:"mana_cost"`
	TypeLine   string            `json:"type_line"`
	OracleText string            `json:"oracle_text"`
	ImageURIs  map[string]string `json:"image_uris"`

	// MDFC / modal double-faced cards
	CardFaces []struct {
		Name       string            `json:"name"`
		ManaCost   string            `json:"mana_cost"`
		TypeLine   string            `json:"type_line"`
		OracleText string            `json:"oracle_text"`
		ImageURIs  map[string]string `json:"image_uris"`
		Artist     string            `json:"artist"`
		Colors     []string          `json:"colors"`
		ColorID    []string          `json:"color_identity"`
	} `json:"card_faces"`

	Colors        []string          `json:"colors"`
	ColorIdentity []string          `json:"color_identity"`
	CMC           float64           `json:"cmc"`
	Layout        string            `json:"layout"`
	Legalities    map[string]string `json:"legalities"`

	Prices struct {
		USD       string `json:"usd"`
		USDFoil   string `json:"usd_foil"`
		USDEtched string `json:"usd_etched"`
	} `json:"prices"`

	Artist     string `json:"artist"`
	EDHRecRank int    `json:"edhrec_rank"`

	ScryfallURI string `json:"scryfall_uri"`
	Set         string `json:"set"`
	SetName     string `json:"set_name"`
}

// normalizeScryfallCard flattens a raw Scryfall card (which may be MDFC or
// other layouts) into a single Card value that the rest of the app can use.
func normalizeScryfallCard(sc scryfallCard) Card {
	// Start with top-level fields.
	name := sc.Name
	manaCost := sc.ManaCost
	typeLine := sc.TypeLine
	oracleText := sc.OracleText
	artist := sc.Artist
	colors := sc.Colors
	colorID := sc.ColorIdentity

	// Pick an image URI, preferring primary face for MDFCs.
	img := ""
	if len(sc.CardFaces) > 0 {
		face := sc.CardFaces[0]
		if face.Name != "" {
			name = face.Name
		}
		if face.ManaCost != "" {
			manaCost = face.ManaCost
		}
		if face.TypeLine != "" {
			typeLine = face.TypeLine
		}
		if face.OracleText != "" {
			oracleText = face.OracleText
		}
		if face.Artist != "" {
			artist = face.Artist
		}
		if len(face.Colors) > 0 {
			colors = face.Colors
		}
		if len(face.ColorID) > 0 {
			colorID = face.ColorID
		}
		if face.ImageURIs != nil {
			img = face.ImageURIs["normal"]
			if img == "" {
				img = face.ImageURIs["large"]
			}
			if img == "" {
				img = face.ImageURIs["small"]
			}
		}
	}

	// If we didn't get an image from faces, try the top-level image_uris.
	if img == "" && sc.ImageURIs != nil {
		img = sc.ImageURIs["normal"]
		if img == "" {
			img = sc.ImageURIs["large"]
		}
		if img == "" {
			img = sc.ImageURIs["small"]
		}
	}

	// Prefer non-foil USD, then fallback to other prices.
	price := sc.Prices.USD
	if price == "" {
		price = sc.Prices.USDFoil
	}
	if price == "" {
		price = sc.Prices.USDEtched
	}

	commanderLegal := false
	if sc.Legalities != nil && sc.Legalities["commander"] == "legal" {
		commanderLegal = true
	}

	// Build faces slice for MDFC and similar layouts.
	faces := make([]CardFace, 0, len(sc.CardFaces))
	for _, f := range sc.CardFaces {
		faceImg := ""
		if f.ImageURIs != nil {
			faceImg = f.ImageURIs["normal"]
			if faceImg == "" {
				faceImg = f.ImageURIs["large"]
			}
			if faceImg == "" {
				faceImg = f.ImageURIs["small"]
			}
		}

		faces = append(faces, CardFace{
			Name:       f.Name,
			ManaCost:   f.ManaCost,
			TypeLine:   f.TypeLine,
			OracleText: f.OracleText,
			ImageURI:   faceImg,
			Artist:     f.Artist,
			Colors:     f.Colors,
			ColorID:    f.ColorID,
		})
	}

	return Card{
		ID:             sc.ID,
		OracleID:       sc.OracleID,
		Name:           name,
		ManaCost:       manaCost,
		TypeLine:       typeLine,
		OracleText:     oracleText,
		ImageURI:       img,
		Colors:         colors,
		ColorIdentity:  colorID,
		CMC:            sc.CMC,
		Layout:         sc.Layout,
		CommanderLegal: commanderLegal,
		PriceUSD:       price,
		Artist:         artist,
		EDHRecRank:     sc.EDHRecRank,
		ScryfallURI:    sc.ScryfallURI,
		SetCode:        sc.Set,
		SetName:        sc.SetName,
		Faces:          faces,
	}
}

type ScryfallClient struct {
	httpClient *http.Client
}

func NewScryfallClient() *ScryfallClient {
	return &ScryfallClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *ScryfallClient) SearchByName(ctx context.Context, q string) ([]Card, error) {
	endpoint := "https://api.scryfall.com/cards/search"
	values := url.Values{}
	values.Set("q", q)
	u := endpoint + "?" + values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the body so we can see whether this is a normal list or an error.
	var body struct {
		Object   string         `json:"object"`
		Code     string         `json:"code"`
		Status   int            `json:"status"`
		Data     []scryfallCard `json:"data"`
		Details  string         `json:"details"`
		Warnings []string       `json:"warnings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	// If Scryfall returns an error object, branch on the code:
	if body.Object == "error" {
		// Typical "no cards matched your search query" case
		if body.Code == "not_found" || resp.StatusCode == http.StatusNotFound {
			return []Card{}, nil
		}

		// Bad query, rate limit, etc. – this is a real error.
		return nil, fmt.Errorf("scryfall error (%s): %s", body.Code, body.Details)
	}

	// Normal case: zero or more results.
	out := make([]Card, 0, len(body.Data))
	for _, sc := range body.Data {
		out = append(out, normalizeScryfallCard(sc))
	}
	return out, nil
}
