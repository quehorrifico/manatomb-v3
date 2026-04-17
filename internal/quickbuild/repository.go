package quickbuild

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ResolveCommander(ctx context.Context, name string) (CandidateCard, CommanderOverride, error) {
	card, err := r.lookupCardByName(ctx, name)
	if err != nil {
		return CandidateCard{}, CommanderOverride{}, err
	}

	override, err := r.lookupCommanderOverride(ctx, card.OracleID)
	if err != nil && err != sql.ErrNoRows {
		return CandidateCard{}, CommanderOverride{}, err
	}
	if err == nil && override.Enabled {
		return card, override, nil
	}
	return card, CommanderOverride{}, nil
}

func (r *Repository) CandidatePool(ctx context.Context, commander CandidateCard) ([]CandidateCard, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, ''),
			COALESCE(oc.type_line, ''),
			COALESCE(oc.oracle_text, ''),
			COALESCE(oc.all_parts::text, '[]'),
			COALESCE(oc.default_image_uri, ''),
			COALESCE(oc.default_price_usd, ''),
			COALESCE(oc.color_identity, ARRAY[]::text[]),
			COALESCE(oc.cmc, 0),
			COALESCE(oc.edhrec_rank, 0),
			COALESCE(oc.commander_legal, FALSE),
			COALESCE(oc.is_commander_candidate, FALSE),
			COALESCE(qbf.roles, ARRAY[]::text[]),
			COALESCE(qbf.themes, ARRAY[]::text[]),
			COALESCE(qbf.strategy_tags, ARRAY[]::text[]),
			COALESCE(qbf.land_tags, ARRAY[]::text[]),
			COALESCE(qbf.mana_tags, ARRAY[]::text[]),
			COALESCE(qbf.curve_bucket, 0),
			COALESCE(qbf.color_pips, '{}'::jsonb)::text,
			COALESCE(qbf.score_flags, '{}'::jsonb)::text,
			COALESCE(qbf.rule_version, 0)
		FROM oracle_cards oc
		LEFT JOIN quick_build_card_features qbf
		  ON qbf.oracle_id = oc.oracle_id
		WHERE oc.commander_legal = TRUE
		  AND COALESCE(oc.layout, '') <> 'token'
		  AND COALESCE(oc.layout, '') <> 'double_faced_token'
		  AND lower(btrim(COALESCE(oc.type_line, ''))) <> 'card'
		  AND oc.color_identity <@ $1::text[]
		ORDER BY
			CASE WHEN COALESCE(oc.edhrec_rank, 0) > 0 THEN 0 ELSE 1 END ASC,
			COALESCE(oc.edhrec_rank, 999999) ASC,
			oc.name ASC
	`, pq.Array(commander.ColorIdentity))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pool := make([]CandidateCard, 0, 1024)
	for rows.Next() {
		card, err := scanCandidateCard(rows)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(strings.TrimSpace(card.Name), strings.TrimSpace(commander.Name)) {
			continue
		}
		if card.OracleID == commander.OracleID {
			continue
		}
		if card.CommanderLegal != true {
			continue
		}
		if !featuresFresh(card) {
			card = classifyCard(card)
		}
		pool = append(pool, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *Repository) CacheFeatures(ctx context.Context, cards []CandidateCard) error {
	if len(cards) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO quick_build_card_features (
			oracle_id,
			roles,
			themes,
			strategy_tags,
			land_tags,
			mana_tags,
			curve_bucket,
			color_pips,
			score_flags,
			rule_version,
			updated_at
		)
		VALUES (
			$1::uuid,
			$2::text[],
			$3::text[],
			$4::text[],
			$5::text[],
			$6::text[],
			$7,
			$8::jsonb,
			$9::jsonb,
			$10,
			NOW()
		)
		ON CONFLICT (oracle_id) DO UPDATE SET
			roles = EXCLUDED.roles,
			themes = EXCLUDED.themes,
			strategy_tags = EXCLUDED.strategy_tags,
			land_tags = EXCLUDED.land_tags,
			mana_tags = EXCLUDED.mana_tags,
			curve_bucket = EXCLUDED.curve_bucket,
			color_pips = EXCLUDED.color_pips,
			score_flags = EXCLUDED.score_flags,
			rule_version = EXCLUDED.rule_version,
			updated_at = NOW()
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	seen := map[string]struct{}{}
	for _, card := range cards {
		if card.OracleID == "" {
			continue
		}
		if _, ok := seen[card.OracleID]; ok {
			continue
		}
		seen[card.OracleID] = struct{}{}

		colorPipsJSON, err := json.Marshal(card.ColorPips)
		if err != nil {
			return err
		}
		scoreFlagsJSON, err := json.Marshal(card.ScoreFlags)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(
			ctx,
			card.OracleID,
			pq.Array(card.Roles),
			pq.Array(card.Themes),
			pq.Array(card.StrategyTags),
			pq.Array(card.LandTags),
			pq.Array(card.ManaTags),
			card.CurveBucket,
			string(colorPipsJSON),
			string(scoreFlagsJSON),
			FeatureRuleVersion,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) lookupCardByName(ctx context.Context, name string) (CandidateCard, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, ''),
			COALESCE(oc.type_line, ''),
			COALESCE(oc.oracle_text, ''),
			COALESCE(oc.all_parts::text, '[]'),
			COALESCE(oc.default_image_uri, ''),
			COALESCE(oc.default_price_usd, ''),
			COALESCE(oc.color_identity, ARRAY[]::text[]),
			COALESCE(oc.cmc, 0),
			COALESCE(oc.edhrec_rank, 0),
			COALESCE(oc.commander_legal, FALSE),
			COALESCE(oc.is_commander_candidate, FALSE),
			COALESCE(qbf.roles, ARRAY[]::text[]),
			COALESCE(qbf.themes, ARRAY[]::text[]),
			COALESCE(qbf.strategy_tags, ARRAY[]::text[]),
			COALESCE(qbf.land_tags, ARRAY[]::text[]),
			COALESCE(qbf.mana_tags, ARRAY[]::text[]),
			COALESCE(qbf.curve_bucket, 0),
			COALESCE(qbf.color_pips, '{}'::jsonb)::text,
			COALESCE(qbf.score_flags, '{}'::jsonb)::text,
			COALESCE(qbf.rule_version, 0)
		FROM oracle_cards oc
		LEFT JOIN quick_build_card_features qbf
		  ON qbf.oracle_id = oc.oracle_id
		WHERE oc.name_search = normalize_card_name($1)
		ORDER BY COALESCE(oc.edhrec_rank, 999999) ASC, oc.name ASC
		LIMIT 1
	`, strings.TrimSpace(name))
	if err != nil {
		return CandidateCard{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return CandidateCard{}, sql.ErrNoRows
	}

	card, err := scanCandidateCard(rows)
	if err != nil {
		return CandidateCard{}, err
	}
	if !featuresFresh(card) {
		card = classifyCard(card)
	}
	return card, nil
}

func (r *Repository) lookupCommanderOverride(ctx context.Context, oracleID string) (CommanderOverride, error) {
	var (
		forceStrategy string
		forceThemes   []string
		enabled       bool
		notes         string
		rawJSON       string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(force_strategy, ''),
			COALESCE(force_themes, ARRAY[]::text[]),
			COALESCE(enabled, TRUE),
			COALESCE(notes, ''),
			COALESCE(bucket_overrides, '{}'::jsonb)::text
		FROM quick_build_commander_overrides
		WHERE oracle_id = $1::uuid
	`, strings.TrimSpace(oracleID)).Scan(
		&forceStrategy,
		pq.Array(&forceThemes),
		&enabled,
		&notes,
		&rawJSON,
	)
	if err != nil {
		return CommanderOverride{}, err
	}

	override := CommanderOverride{
		ForceStrategy:   strings.TrimSpace(forceStrategy),
		ForceThemes:     normalizeTextSlice(forceThemes),
		BucketOverrides: map[string]int{},
		Enabled:         enabled,
		Notes:           strings.TrimSpace(notes),
	}
	_ = json.Unmarshal([]byte(rawJSON), &override.BucketOverrides)
	return override, nil
}

func scanCandidateCard(scanner interface{ Scan(dest ...any) error }) (CandidateCard, error) {
	var (
		card         CandidateCard
		colorID      []string
		roles        []string
		themes       []string
		strategyTags []string
		landTags     []string
		manaTags     []string
		colorPipsRaw string
		scoreRaw     string
		ruleVersion  int
	)

	if err := scanner.Scan(
		&card.OracleID,
		&card.Name,
		&card.ManaCost,
		&card.TypeLine,
		&card.OracleText,
		&card.AllPartsJSON,
		&card.ImageURI,
		&card.PriceUSD,
		pq.Array(&colorID),
		&card.CMC,
		&card.EDHRecRank,
		&card.CommanderLegal,
		&card.IsCommanderCandidate,
		pq.Array(&roles),
		pq.Array(&themes),
		pq.Array(&strategyTags),
		pq.Array(&landTags),
		pq.Array(&manaTags),
		&card.CurveBucket,
		&colorPipsRaw,
		&scoreRaw,
		&ruleVersion,
	); err != nil {
		return CandidateCard{}, err
	}

	card.ColorIdentity = normalizeTextSlice(colorID)
	card.Roles = normalizeTextSlice(roles)
	card.Themes = normalizeTextSlice(themes)
	card.StrategyTags = normalizeTextSlice(strategyTags)
	card.LandTags = normalizeTextSlice(landTags)
	card.ManaTags = normalizeTextSlice(manaTags)
	card.ColorPips = map[string]int{}
	card.ScoreFlags = map[string]bool{}
	_ = json.Unmarshal([]byte(colorPipsRaw), &card.ColorPips)
	_ = json.Unmarshal([]byte(scoreRaw), &card.ScoreFlags)
	if ruleVersion != FeatureRuleVersion {
		card.ScoreFlags["features_stale"] = true
	}
	return card, nil
}

func featuresFresh(card CandidateCard) bool {
	if card.ScoreFlags == nil {
		return false
	}
	return !card.ScoreFlags["features_stale"]
}

func normalizeTextSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
