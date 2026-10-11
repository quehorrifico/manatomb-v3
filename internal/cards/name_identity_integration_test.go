package cards

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
)

// This requires an explicitly supplied disposable database. All cards are temporary.
func TestCardNameIdentityPostgres(t *testing.T) {
	url := os.Getenv("MANATOMB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if err = EnsureCardsTable(ctx, db); err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
 CREATE TEMP TABLE oracle_cards (LIKE public.oracle_cards INCLUDING DEFAULTS INCLUDING GENERATED);
 CREATE TEMP TABLE card_prints (LIKE public.card_prints INCLUDING DEFAULTS INCLUDING GENERATED);
 INSERT INTO oracle_cards (oracle_id,name,layout,legal_anywhere,edhrec_rank) VALUES
 ('00000000-0000-0000-0000-000000000001','Rampant Growth','normal',true,100),
 ('00000000-0000-0000-0000-000000000002','Rampant, Growth','normal',false,1),
 ('00000000-0000-0000-0000-000000000003','Llanowar Elves','token',false,1),
 ('00000000-0000-0000-0000-000000000004','Llanowar Elves','normal',true,100),
 ('00000000-0000-0000-0000-000000000005','Æther Gust','normal',true,100);
 `)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ input, want string }{
		{"Rampant Growth", "00000000-0000-0000-0000-000000000001"},
		{" RAMPANT GROWTH ", "00000000-0000-0000-0000-000000000001"},
		{"Rampant, Growth", "00000000-0000-0000-0000-000000000002"},
		{"Llanowar Elves", "00000000-0000-0000-0000-000000000004"},
		{"Aether Gust", "00000000-0000-0000-0000-000000000005"},
	} {
		card, err := GetCardByName(ctx, db, tc.input)
		if err != nil || card.OracleID != tc.want {
			t.Fatalf("GetCardByName(%q): %+v %v", tc.input, card, err)
		}
		ensured, err := EnsureCardByName(ctx, db, tc.input)
		if err != nil || ensured.OracleID != tc.want {
			t.Fatalf("EnsureCardByName(%q): %+v %v", tc.input, ensured, err)
		}
		searchWant := tc.want
		// Search suggestions intentionally exclude playtest cards.
		if tc.input == "Rampant, Growth" {
			searchWant = "00000000-0000-0000-0000-000000000001"
		}
		found, err := SearchCards(ctx, db, CardSearchParams{Query: tc.input, Limit: 1})
		if err != nil || len(found) != 1 || found[0].OracleID != searchWant {
			t.Fatalf("SearchCards(%q): %+v %v", tc.input, found, err)
		}
	}
	names := []string{"Rampant Growth", "Rampant, Growth", "Aether Gust", "Llanowar Elves"}
	lookup, err := LookupCardsByNames(ctx, db, names)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveCardNamesBatch(ctx, db, names, 0.4)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		key := strings.ToLower(name)
		if lookup[key].Name != name && name != "Aether Gust" {
			t.Fatalf("batch lookup changed %q to %q", name, lookup[key].Name)
		}
		if resolved[key].Card.OracleID != lookup[key].OracleID || !resolved[key].Exact {
			t.Fatalf("batch resolution disagrees for %q: %+v", name, resolved[key])
		}
	}
	// Fuzzy fallbacks still work after preserving punctuation in batch input keys.
	fuzzy, err := ResolveCardNamesBatch(ctx, db, []string{"Rampant Growt"}, 0.4)
	if err != nil || fuzzy["rampant growt"].Card.Name != "Rampant Growth" || fuzzy["rampant growt"].Exact {
		t.Fatalf("fuzzy fallback: %+v %v", fuzzy, err)
	}
}
