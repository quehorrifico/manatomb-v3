package quickbuild

import "strings"

const FeatureRuleVersion = 1

type Request struct {
	CommanderName string
	Seed          int64
}

type CandidateCard struct {
	OracleID             string
	Name                 string
	ManaCost             string
	TypeLine             string
	OracleText           string
	AllPartsJSON         string
	ImageURI             string
	PriceUSD             string
	ColorIdentity        []string
	CMC                  float64
	EDHRecRank           int
	CommanderLegal       bool
	IsCommanderCandidate bool

	Roles        []string
	Themes       []string
	StrategyTags []string
	LandTags     []string
	ManaTags     []string
	CurveBucket  int
	ColorPips    map[string]int
	ScoreFlags   map[string]bool
}

type CommanderOverride struct {
	ForceStrategy   string
	ForceThemes     []string
	BucketOverrides map[string]int
	Enabled         bool
	Notes           string
}

type BucketSpec struct {
	Name         string
	Count        int
	Roles        []string
	Themes       []string
	StrategyTags []string
	ScoreFlags   []string
	MaxCMC       float64
}

type Profile struct {
	Strategy         string
	PrimaryTheme     string
	Themes           []string
	Tribe            string
	LandCount        int
	RampCount        int
	DrawCount        int
	InteractionCount int
	WipeCount        int
	ProtectionCount  int
	UtilityCount     int
	SynergyCount     int
	Explanation      string
}

type BuiltCard struct {
	Card   CandidateCard
	Qty    int
	Bucket string
}

type Summary struct {
	Strategy      string         `json:"strategy"`
	PrimaryTheme  string         `json:"primary_theme"`
	Themes        []string       `json:"themes"`
	BucketCounts  map[string]int `json:"bucket_counts"`
	Explanation   string         `json:"explanation"`
	Seed          int64          `json:"seed"`
	RepairActions []string       `json:"repair_actions,omitempty"`
	FallbackNotes []string       `json:"fallback_notes,omitempty"`
	LandMix       map[string]int `json:"land_mix,omitempty"`
}

type Result struct {
	Commander CandidateCard
	Cards     []BuiltCard
	Summary   Summary
}

func normalizeLabel(raw string) string {
	return strings.TrimSpace(raw)
}
