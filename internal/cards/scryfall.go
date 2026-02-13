package cards

import "strings"

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

// Card is the normalized view returned by search/resolve APIs.
// For canonical search rows, ID matches OracleID. For print rows, ID may be a
// Scryfall printing id.
type Card struct {
	ID       string
	OracleID string
	Lang     string

	Name       string
	ManaCost   string
	TypeLine   string
	OracleText string
	ImageURI   string
	ReleasedAt string

	Colors         []string
	ColorIdentity  []string
	CMC            float64
	Layout         string
	CommanderLegal bool

	PriceUSD   string
	Artist     string
	EDHRecRank int

	ScryfallURI     string
	SetCode         string
	SetName         string
	CollectorNumber string
	Rarity          string

	// Faces contains all faces for MDFC/similar layouts.
	Faces []CardFace `json:"faces"`
}

type scryfallCard struct {
	ID       string `json:"id"`
	OracleID string `json:"oracle_id"`
	Lang     string `json:"lang"`

	Name       string            `json:"name"`
	ManaCost   string            `json:"mana_cost"`
	TypeLine   string            `json:"type_line"`
	OracleText string            `json:"oracle_text"`
	ImageURIs  map[string]string `json:"image_uris"`
	ReleasedAt string            `json:"released_at"`
	SetType    string            `json:"set_type"`
	Games      []string          `json:"games"`

	CollectorNumber string `json:"collector_number"`
	Rarity          string `json:"rarity"`

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

func preferredImageURI(imageURIs map[string]string) string {
	if imageURIs == nil {
		return ""
	}
	if v := strings.TrimSpace(imageURIs["normal"]); v != "" {
		return v
	}
	if v := strings.TrimSpace(imageURIs["large"]); v != "" {
		return v
	}
	if v := strings.TrimSpace(imageURIs["small"]); v != "" {
		return v
	}
	return ""
}

func preferredPriceUSD(sc scryfallCard) string {
	if v := strings.TrimSpace(sc.Prices.USD); v != "" {
		return v
	}
	if v := strings.TrimSpace(sc.Prices.USDFoil); v != "" {
		return v
	}
	return strings.TrimSpace(sc.Prices.USDEtched)
}

// normalizeScryfallCard flattens a raw Scryfall card into one Card row.
func normalizeScryfallCard(sc scryfallCard) Card {
	name := strings.TrimSpace(sc.Name)
	manaCost := strings.TrimSpace(sc.ManaCost)
	typeLine := strings.TrimSpace(sc.TypeLine)
	oracleText := strings.TrimSpace(sc.OracleText)
	artist := strings.TrimSpace(sc.Artist)
	colors := sc.Colors
	colorID := sc.ColorIdentity

	img := preferredImageURI(sc.ImageURIs)
	if len(sc.CardFaces) > 0 {
		face := sc.CardFaces[0]
		if v := strings.TrimSpace(face.ManaCost); v != "" {
			manaCost = v
		}
		if v := strings.TrimSpace(face.TypeLine); v != "" {
			typeLine = v
		}
		if v := strings.TrimSpace(face.OracleText); v != "" {
			oracleText = v
		}
		if v := strings.TrimSpace(face.Artist); v != "" {
			artist = v
		}
		if len(face.Colors) > 0 {
			colors = face.Colors
		}
		if len(face.ColorID) > 0 {
			colorID = face.ColorID
		}
		if faceImage := preferredImageURI(face.ImageURIs); faceImage != "" {
			img = faceImage
		}
	}

	commanderLegal := false
	if sc.Legalities != nil && strings.EqualFold(strings.TrimSpace(sc.Legalities["commander"]), "legal") {
		commanderLegal = true
	}

	faces := make([]CardFace, 0, len(sc.CardFaces))
	for _, face := range sc.CardFaces {
		faces = append(faces, CardFace{
			Name:       strings.TrimSpace(face.Name),
			ManaCost:   strings.TrimSpace(face.ManaCost),
			TypeLine:   strings.TrimSpace(face.TypeLine),
			OracleText: strings.TrimSpace(face.OracleText),
			ImageURI:   preferredImageURI(face.ImageURIs),
			Artist:     strings.TrimSpace(face.Artist),
			Colors:     face.Colors,
			ColorID:    face.ColorID,
		})
	}

	return Card{
		ID:             strings.TrimSpace(sc.ID),
		OracleID:       strings.TrimSpace(sc.OracleID),
		Lang:           strings.TrimSpace(sc.Lang),
		Name:           name,
		ManaCost:       manaCost,
		TypeLine:       typeLine,
		OracleText:     oracleText,
		ImageURI:       img,
		ReleasedAt:     strings.TrimSpace(sc.ReleasedAt),
		Colors:         colors,
		ColorIdentity:  colorID,
		CMC:            sc.CMC,
		Layout:         strings.TrimSpace(sc.Layout),
		CommanderLegal: commanderLegal,
		PriceUSD:       preferredPriceUSD(sc),
		Artist:         artist,
		EDHRecRank:     sc.EDHRecRank,
		ScryfallURI:    strings.TrimSpace(sc.ScryfallURI),
		SetCode:        strings.TrimSpace(sc.Set),
		SetName:        strings.TrimSpace(sc.SetName),
		Faces:          faces,
	}
}
