package cards

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
)

// Uses an explicitly supplied disposable local database; never application config.
func TestBatchNormalizationPostgres(t *testing.T) {
	url := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set MANATOMB_TEST_DATABASE_URL to a disposable local PostgreSQL database")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
 CREATE EXTENSION IF NOT EXISTS unaccent;
 CREATE OR REPLACE FUNCTION normalize_card_name(input text) RETURNS text LANGUAGE SQL IMMUTABLE AS $$
 SELECT trim(regexp_replace(lower(unaccent(COALESCE(input,''))), '[^a-z0-9]+', ' ', 'g')) $$;
 SET search_path TO pg_temp, public;
 CREATE TEMP TABLE oracle_cards (
 oracle_id uuid, name text, name_search text GENERATED ALWAYS AS (normalize_card_name(name)) STORED,
 mana_cost text, type_line text, oracle_text text, flavor_text text, all_parts jsonb,
 default_image_uri text, colors text[], color_identity text[], cmc float8, default_price_usd text,
 default_artist text, default_set_code text, default_set_name text, is_commander_candidate boolean,
 legal_anywhere boolean, layout text, edhrec_rank integer, default_print_id uuid);
 CREATE TEMP TABLE card_prints(scryfall_id uuid, flavor_text text, image_uris jsonb, collector_number text);
 INSERT INTO oracle_cards (oracle_id,name,is_commander_candidate,default_print_id) VALUES
 ('00000000-0000-0000-0000-000000000001','Surtr, Fiery Jötun',true,'be38985c-f7a9-4885-9035-09390b4b754a'),
 ('00000000-0000-0000-0000-000000000002','Kaalia of the Vast',true,'819a21b1-ec52-4c83-85b4-491600b2e434'),
 ('00000000-0000-0000-0000-000000000003','Æther Gust',false,null);
 INSERT INTO card_prints(scryfall_id,collector_number) VALUES
 ('be38985c-f7a9-4885-9035-09390b4b754a','surtr-print'),('819a21b1-ec52-4c83-85b4-491600b2e434','kaalia-print');
 `)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []string{"Surtr, Fiery Jötun", " SURTR, FIERY JOTUN ", "Surtr, Fiery Jo\u0308tun", "Kaalia of the Vast", "KAALIA OF THE VAST", "Æther Gust", "Aether Gust", "not an actual card"}
	got, err := LookupCardsByNames(ctx, db, inputs)
	if err != nil {
		t.Fatal(err)
	}
	for i, input := range inputs[:len(inputs)-1] {
		card, ok := got[strings.ToLower(strings.TrimSpace(input))]
		if !ok {
			t.Fatalf("missing %q", input)
		}
		switch {
		case i < 3:
			if card.Name != "Surtr, Fiery Jötun" || !card.IsCommanderCandidate || card.CollectorNumber != "surtr-print" {
				t.Fatalf("wrong Surtr identity: %+v", card)
			}
		case i < 5:
			if card.Name != "Kaalia of the Vast" || !card.IsCommanderCandidate || card.CollectorNumber != "kaalia-print" {
				t.Fatalf("wrong Kaalia identity: %+v", card)
			}
		default:
			if card.Name != "Æther Gust" || card.IsCommanderCandidate {
				t.Fatalf("wrong noncommander: %+v", card)
			}
		}
	}
	if _, ok := got["not an actual card"]; ok {
		t.Fatal("invalid card unexpectedly resolved")
	}
}
