package cards

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestRoleSnapshotReplacementPostgres(t *testing.T) {
	url := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	const oracle = "00000000-0000-0000-0000-000000000071"
	if _, err := db.Exec(`CREATE TEMP TABLE oracle_cards(oracle_id UUID PRIMARY KEY);
 CREATE TEMP TABLE card_role_tags(oracle_id UUID PRIMARY KEY,roles TEXT[]);
 CREATE TEMP TABLE card_role_sync_state(id SMALLINT PRIMARY KEY,last_success_at TIMESTAMPTZ,source_updated_at TIMESTAMPTZ);
 INSERT INTO oracle_cards VALUES('` + oracle + `');`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := replaceOracleRoleTags(ctx, db, map[string][]string{oracle: {"ramp", "tutor"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	var roles string
	if err := db.QueryRow(`SELECT array_to_string(roles,',') FROM card_role_tags WHERE oracle_id=$1`, oracle).Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if roles != "ramp,tutor" {
		t.Fatalf("wrong roles: %s", roles)
	}
	if err := replaceOracleRoleTags(ctx, db, map[string][]string{"bad-uuid": {"draw"}}, time.Now()); err == nil {
		t.Fatal("invalid snapshot succeeded")
	}
	if err := db.QueryRow(`SELECT array_to_string(roles,',') FROM card_role_tags WHERE oracle_id=$1`, oracle).Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if roles != "ramp,tutor" {
		t.Fatal("failed refresh destroyed cached roles")
	}
	if fixture := os.Getenv("MANATOMB_ROLE_FIXTURE"); fixture != "" {
		f, err := os.Open(fixture)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		decoded, err := decodeOracleRoleTags(f)
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded) < 1000 {
			t.Fatalf("unexpectedly sparse real dataset: %d cards", len(decoded))
		}
	}
}
