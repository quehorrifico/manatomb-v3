package web

import (
	"context"
	"strings"

	"github.com/lib/pq"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/decks"
)

type cardRoleAssignment struct {
	Roles       []string `json:"roles"`
	PrimaryRole string   `json:"primaryRole,omitempty"`
	Source      string   `json:"source"`
}

func (a *App) loadRoleAssignments(ctx context.Context, deckID int64, names []string) (map[string]cardRoleAssignment, error) {
	out := map[string]cardRoleAssignment{}
	if len(names) == 0 {
		return out, nil
	}
	rows, err := a.DB.QueryContext(ctx, `
 SELECT DISTINCT ON (oc.name_search) oc.name,COALESCE(oc.type_line,''),COALESCE(oc.oracle_text,''),oc.cmc,COALESCE(t.roles,ARRAY[]::text[]),COALESCE(o.role,'')
 FROM oracle_cards oc LEFT JOIN card_role_tags t USING(oracle_id)
 LEFT JOIN deck_card_role_overrides o ON o.oracle_id=oc.oracle_id AND o.deck_id=$2
 WHERE oc.name_search IN(SELECT normalize_card_name(n) FROM unnest($1::text[]) n)
 ORDER BY oc.name_search,COALESCE(oc.legal_anywhere,TRUE) DESC,
 CASE WHEN lower(COALESCE(oc.layout,'')) IN ('token','double_faced_token') THEN 1 ELSE 0 END,
 COALESCE(oc.edhrec_rank,2147483647),oc.name,oc.oracle_id
 `, pq.Array(names), deckID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, typ, oracle, override string
		var cmc float64
		var roles []string
		if err := rows.Scan(&name, &typ, &oracle, &cmc, pq.Array(&roles), &override); err != nil {
			return nil, err
		}
		source := "scryfall"
		if len(roles) == 0 {
			roles = classifyDeckRoleCategories(name, typ, oracle, cmc)
			source = "estimated"
		} else if strings.Contains(strings.ToLower(typ), "land") {
			roles = append([]string{"lands"}, roles...)
		}
		out[strings.ToLower(name)] = cardRoleAssignment{Roles: roles, PrimaryRole: override, Source: source}
	}
	return out, rows.Err()
}

func (a *App) enrichDeckRoles(ctx context.Context, deckID int64, groups ...[]decks.DeckCard) error {
	names := []string{}
	for _, group := range groups {
		for _, card := range group {
			names = append(names, card.CardName)
		}
	}
	assignments, err := a.loadRoleAssignments(ctx, deckID, names)
	if err != nil {
		return err
	}
	for _, group := range groups {
		for i := range group {
			info := assignments[strings.ToLower(group[i].CardName)]
			group[i].Roles = info.Roles
			group[i].RoleOverride = info.PrimaryRole
			group[i].RoleSource = info.Source
		}
	}
	return nil
}

func normalizedRoleOverride(value string) string {
	value = strings.TrimSpace(value)
	if cards.ValidRole(value) {
		return value
	}
	return ""
}
