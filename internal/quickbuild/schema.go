package quickbuild

import (
	"context"
	"database/sql"
)

func EnsureTables(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`
		CREATE TABLE IF NOT EXISTS quick_build_card_features (
			oracle_id UUID PRIMARY KEY REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			roles TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			themes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			strategy_tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			land_tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			mana_tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			curve_bucket SMALLINT NOT NULL DEFAULT 0,
			color_pips JSONB NOT NULL DEFAULT '{}'::jsonb,
			score_flags JSONB NOT NULL DEFAULT '{}'::jsonb,
			rule_version INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS quick_build_commander_overrides (
			oracle_id UUID PRIMARY KEY REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			force_strategy TEXT NOT NULL DEFAULT '',
			force_themes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			bucket_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			notes TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS quick_build_edhrec_scope_cards (
			id BIGSERIAL PRIMARY KEY,
			scope_type TEXT NOT NULL,
			scope_key TEXT NOT NULL,
			oracle_id UUID NOT NULL REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE,
			weight INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (scope_type, scope_key, oracle_id)
		)
		`,
		`CREATE INDEX IF NOT EXISTS idx_quick_build_card_features_roles ON quick_build_card_features USING GIN (roles)`,
		`CREATE INDEX IF NOT EXISTS idx_quick_build_card_features_themes ON quick_build_card_features USING GIN (themes)`,
		`CREATE INDEX IF NOT EXISTS idx_quick_build_card_features_rule_version ON quick_build_card_features (rule_version)`,
		`CREATE INDEX IF NOT EXISTS idx_quick_build_edhrec_scope_cards_lookup ON quick_build_edhrec_scope_cards (scope_type, scope_key, weight DESC)`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
