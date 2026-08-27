package cards

import "strings"

// CardFace represents one face of a multi-faced card (MDFC, split, etc.).
type CardFace struct {
	Name       string   `json:"name"`
	ManaCost   string   `json:"mana_cost"`
	TypeLine   string   `json:"type_line"`
	OracleText string   `json:"oracle_text"`
	FlavorText string   `json:"flavor_text,omitempty"`
	ImageURI   string   `json:"image_uri"`
	ArtCropURI string   `json:"art_crop_uri,omitempty"`
	Artist     string   `json:"artist"`
	Power      string   `json:"power,omitempty"`
	Toughness  string   `json:"toughness,omitempty"`
	Loyalty    string   `json:"loyalty,omitempty"`
	Colors     []string `json:"colors"`
	ColorID    []string `json:"color_identity"`
}

type RelatedPart struct {
	Component string `json:"component"`
	Name      string `json:"name"`
	TypeLine  string `json:"type_line"`
	URI       string `json:"uri"`
}

// Card is the normalized view returned by search/resolve APIs. ID is the
// Scryfall printing ID represented by the row; OracleID identifies the card
// across all of its printings.
type Card struct {
	ID       string
	OracleID string
	Lang     string

	Name       string
	ManaCost   string
	TypeLine   string
	OracleText string
	FlavorText string
	ImageURI   string
	ArtCropURI string
	ReleasedAt string

	Colors               []string
	ColorIdentity        []string
	CMC                  float64
	Power                string
	Toughness            string
	Loyalty              string
	Layout               string
	LegalAnywhere        bool
	CommanderLegal       bool
	Legalities           map[string]string
	IsCommanderCandidate bool

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
	FlavorText string            `json:"flavor_text"`
	ImageURIs  map[string]string `json:"image_uris"`
	ReleasedAt string            `json:"released_at"`
	SetType    string            `json:"set_type"`
	Games      []string          `json:"games"`
	Finishes   []string          `json:"finishes"`

	CollectorNumber string   `json:"collector_number"`
	Rarity          string   `json:"rarity"`
	BorderColor     string   `json:"border_color"`
	Frame           string   `json:"frame"`
	SecurityStamp   string   `json:"security_stamp"`
	FullArt         bool     `json:"full_art"`
	Textless        bool     `json:"textless"`
	Booster         bool     `json:"booster"`
	Digital         bool     `json:"digital"`
	Variation       bool     `json:"variation"`
	FrameEffects    []string `json:"frame_effects"`
	PromoTypes      []string `json:"promo_types"`

	CardFaces []struct {
		Name       string            `json:"name"`
		ManaCost   string            `json:"mana_cost"`
		TypeLine   string            `json:"type_line"`
		OracleText string            `json:"oracle_text"`
		FlavorText string            `json:"flavor_text"`
		ImageURIs  map[string]string `json:"image_uris"`
		Artist     string            `json:"artist"`
		Power      string            `json:"power"`
		Toughness  string            `json:"toughness"`
		Loyalty    string            `json:"loyalty"`
		Colors     []string          `json:"colors"`
		ColorID    []string          `json:"color_identity"`
	} `json:"card_faces"`

	AllParts []RelatedPart `json:"all_parts"`

	Colors        []string          `json:"colors"`
	ColorIdentity []string          `json:"color_identity"`
	CMC           float64           `json:"cmc"`
	Power         string            `json:"power"`
	Toughness     string            `json:"toughness"`
	Loyalty       string            `json:"loyalty"`
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

func preferredArtCropURI(imageURIs map[string]string) string {
	if imageURIs == nil {
		return ""
	}
	if v := strings.TrimSpace(imageURIs["art_crop"]); v != "" {
		return v
	}
	return preferredImageURI(imageURIs)
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

func normalizeLegalities(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.ToLower(strings.TrimSpace(value))
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

// normalizeScryfallCard flattens a raw Scryfall card into one Card row.
func normalizeScryfallCard(sc scryfallCard) Card {
	name := strings.TrimSpace(sc.Name)
	manaCost := strings.TrimSpace(sc.ManaCost)
	typeLine := strings.TrimSpace(sc.TypeLine)
	oracleText := strings.TrimSpace(sc.OracleText)
	flavorText := strings.TrimSpace(sc.FlavorText)
	artist := strings.TrimSpace(sc.Artist)
	colors := sc.Colors
	colorID := sc.ColorIdentity
	power := strings.TrimSpace(sc.Power)
	toughness := strings.TrimSpace(sc.Toughness)
	loyalty := strings.TrimSpace(sc.Loyalty)

	img := preferredImageURI(sc.ImageURIs)
	artCrop := preferredArtCropURI(sc.ImageURIs)
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
		if flavorText == "" {
			flavorText = strings.TrimSpace(face.FlavorText)
		}
		if v := strings.TrimSpace(face.Artist); v != "" {
			artist = v
		}
		if power == "" {
			power = strings.TrimSpace(face.Power)
		}
		if toughness == "" {
			toughness = strings.TrimSpace(face.Toughness)
		}
		if loyalty == "" {
			loyalty = strings.TrimSpace(face.Loyalty)
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
		if faceArtCrop := preferredArtCropURI(face.ImageURIs); faceArtCrop != "" {
			artCrop = faceArtCrop
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
			FlavorText: strings.TrimSpace(face.FlavorText),
			ImageURI:   preferredImageURI(face.ImageURIs),
			ArtCropURI: preferredArtCropURI(face.ImageURIs),
			Artist:     strings.TrimSpace(face.Artist),
			Power:      strings.TrimSpace(face.Power),
			Toughness:  strings.TrimSpace(face.Toughness),
			Loyalty:    strings.TrimSpace(face.Loyalty),
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
		FlavorText:     flavorText,
		ImageURI:       img,
		ArtCropURI:     artCrop,
		ReleasedAt:     strings.TrimSpace(sc.ReleasedAt),
		Colors:         colors,
		ColorIdentity:  colorID,
		CMC:            sc.CMC,
		Power:          power,
		Toughness:      toughness,
		Loyalty:        loyalty,
		Layout:         strings.TrimSpace(sc.Layout),
		LegalAnywhere:  legalAnywhere(sc.Legalities),
		CommanderLegal: commanderLegal,
		Legalities:     normalizeLegalities(sc.Legalities),
		PriceUSD:       preferredPriceUSD(sc),
		Artist:         artist,
		EDHRecRank:     sc.EDHRecRank,
		ScryfallURI:    strings.TrimSpace(sc.ScryfallURI),
		SetCode:        strings.TrimSpace(sc.Set),
		SetName:        strings.TrimSpace(sc.SetName),
		Faces:          faces,
	}
}
