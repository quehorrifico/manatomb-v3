package cards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Card struct {
	Name       string `json:"name"`
	ManaCost   string `json:"mana_cost"`
	TypeLine   string `json:"type_line"`
	OracleText string `json:"oracle_text"`
	ImageURI   string `json:"image_uris_normal"`
}

type scryfallCard struct {
	Name       string            `json:"name"`
	ManaCost   string            `json:"mana_cost"`
	TypeLine   string            `json:"type_line"`
	OracleText string            `json:"oracle_text"`
	ImageURIs  map[string]string `json:"image_uris"`
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

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("scryfall error: %s", resp.Status)
	}

	var body struct {
		Data []scryfallCard `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]Card, 0, len(body.Data))
	for _, sc := range body.Data {
		img := sc.ImageURIs["normal"]
		out = append(out, Card{
			Name:       sc.Name,
			ManaCost:   sc.ManaCost,
			TypeLine:   sc.TypeLine,
			OracleText: sc.OracleText,
			ImageURI:   img,
		})
	}
	return out, nil
}
