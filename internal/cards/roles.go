package cards

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"sort"
	"time"

	"github.com/lib/pq"
)

// Broad roles inherit all descendant tags, such as tutor-creature -> tutor.
var RoleKeys = []string{"ramp", "draw", "tutor", "removal", "board_wipe", "counterspell", "protection", "blink", "recursion", "tokens", "sacrifice", "graveyard", "stax", "combo", "threat", "utility", "lands"}
var roleTagRoots = map[string]string{
	"ramp": "ramp", "draw": "draw", "tutor": "tutor", "removal": "removal", "sweeper": "board_wipe", "counterspell": "counterspell", "protection": "protection", "flicker": "blink", "recursion": "recursion", "reanimate": "recursion", "repeatable-token-generator": "tokens", "creates-token-of-a-card": "tokens", "sacrifice-outlet": "sacrifice", "mill": "graveyard", "surveil": "graveyard", "hatebear": "stax", "tax": "stax",
}

func ValidRole(role string) bool {
	for _, key := range RoleKeys {
		if key == role {
			return true
		}
	}
	return role == ""
}

type oracleRoleTag struct {
	ID        string   `json:"id"`
	Slug      string   `json:"slug"`
	ParentIDs []string `json:"parent_ids"`
	Taggings  []struct {
		OracleID string `json:"oracle_id"`
	} `json:"taggings"`
}

func decodeOracleRoleTags(reader io.Reader) (map[string][]string, error) {
	dec, cleanup, err := newBulkJSONDecoderFromReader(reader)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	tags := map[string]oracleRoleTag{}
	for {
		var tag oracleRoleTag
		ok, err := dec.DecodeNext(&tag)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if tag.ID != "" {
			tags[tag.ID] = tag
		}
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("empty Oracle Tags dataset")
	}
	memo := map[string]map[string]bool{}
	var ancestors func(string, map[string]bool) map[string]bool
	ancestors = func(id string, visiting map[string]bool) map[string]bool {
		if cached, ok := memo[id]; ok {
			return cached
		}
		if visiting[id] {
			return nil
		}
		visiting[id] = true
		tag := tags[id]
		roles := map[string]bool{}
		if role := roleTagRoots[tag.Slug]; role != "" {
			roles[role] = true
		}
		for _, parent := range tag.ParentIDs {
			for role := range ancestors(parent, visiting) {
				roles[role] = true
			}
		}
		delete(visiting, id)
		memo[id] = roles
		return roles
	}
	byOracle := map[string]map[string]bool{}
	for id, tag := range tags {
		roles := ancestors(id, map[string]bool{})
		if len(roles) == 0 {
			continue
		}
		for _, tagging := range tag.Taggings {
			if tagging.OracleID == "" {
				continue
			}
			if byOracle[tagging.OracleID] == nil {
				byOracle[tagging.OracleID] = map[string]bool{}
			}
			for role := range roles {
				byOracle[tagging.OracleID][role] = true
			}
		}
	}
	out := map[string][]string{}
	for id, roles := range byOracle {
		for _, key := range RoleKeys {
			if roles[key] {
				out[id] = append(out[id], key)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Oracle Tags dataset has no recognized role assignments")
	}
	return out, nil
}

func EnsureRoleTables(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS card_role_tags(oracle_id UUID PRIMARY KEY REFERENCES oracle_cards(oracle_id) ON DELETE CASCADE, roles TEXT[] NOT NULL);
 CREATE TABLE IF NOT EXISTS card_role_sync_state(id SMALLINT PRIMARY KEY CHECK(id=1),last_success_at TIMESTAMPTZ,source_updated_at TIMESTAMPTZ);`)
	return err
}

func SyncOracleRoleTags(ctx context.Context, db *sql.DB) error {
	descriptor, err := fetchBulkDescriptor(ctx, "oracle_tags")
	if err != nil {
		return err
	}
	response, err := doScryfallRequest(ctx, "GET", descriptor.DownloadURI, "Oracle Tags download")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return scryfallStatusError("Oracle Tags download", response)
	}
	roles, err := decodeOracleRoleTags(response.Body)
	if err != nil {
		return err
	}
	return replaceOracleRoleTags(ctx, db, roles, descriptor.UpdatedAt)
}

func replaceOracleRoleTags(ctx context.Context, db *sql.DB, roles map[string][]string, updatedAt time.Time) error {
	if len(roles) == 0 {
		return fmt.Errorf("refusing to replace roles with an empty snapshot")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(91342818)`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE role_tags_stage(oracle_id UUID PRIMARY KEY,roles TEXT[]) ON COMMIT DROP`); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("role_tags_stage", "oracle_id", "roles"))
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(roles))
	for id := range roles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := stmt.ExecContext(ctx, id, pq.Array(roles[id])); err != nil {
			stmt.Close()
			return err
		}
	}
	if _, err := stmt.ExecContext(ctx); err != nil {
		stmt.Close()
		return err
	}
	if err := stmt.Close(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO card_role_tags(oracle_id,roles) SELECT s.oracle_id,s.roles FROM role_tags_stage s JOIN oracle_cards oc USING(oracle_id) ON CONFLICT(oracle_id) DO UPDATE SET roles=EXCLUDED.roles;
 DELETE FROM card_role_tags t WHERE NOT EXISTS(SELECT 1 FROM role_tags_stage s WHERE s.oracle_id=t.oracle_id);`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO card_role_sync_state(id,last_success_at,source_updated_at) VALUES(1,NOW(),$1) ON CONFLICT(id) DO UPDATE SET last_success_at=NOW(),source_updated_at=EXCLUDED.source_updated_at`, updatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

// Runs independently of a full card download so existing installations acquire
// tags at startup. A failed refresh retains the last good snapshot.
func StartOracleRoleSyncLoop(db *sql.DB) {
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			var due bool
			err := db.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM card_role_sync_state WHERE id=1 AND last_success_at>NOW()-INTERVAL '24 hours')`).Scan(&due)
			if err == nil && due {
				err = SyncOracleRoleTags(ctx, db)
			}
			cancel()
			if err != nil {
				log.Printf("Oracle role tag sync failed; keeping cached roles: %v", err)
			}
			time.Sleep(time.Hour)
		}
	}()
}
