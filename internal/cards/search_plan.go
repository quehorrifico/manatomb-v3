package cards

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type cardSearchSourcePlan struct {
	SelectSQL         string
	CountFromSQL      string
	FilterSQL         string
	Args              []any
	MatchingPrintings bool
}

type cardSearchPageWindow struct {
	Page       int
	PageSize   int
	Offset     int
	Total      int
	TotalPages int
}

func hasPrintingSpecificCardSearchFilters(params CardSearchParams) bool {
	if len(params.PriceFilters) > 0 ||
		params.PriceValue != nil ||
		params.PriceUSDMin != nil ||
		params.PriceUSDMax != nil {
		return true
	}
	if len(normalizeRarityFilters(params.Rarities)) > 0 ||
		normalizeRarityFilter(params.Rarity) != "" {
		return true
	}
	return strings.TrimSpace(params.SetQuery) != "" ||
		strings.TrimSpace(params.ArtistQuery) != ""
}

func withoutPrintingSpecificCardSearchFilters(params CardSearchParams) CardSearchParams {
	params.PriceOperator = ""
	params.PriceValue = nil
	params.PriceFilters = nil
	params.PriceUSDMin = nil
	params.PriceUSDMax = nil
	params.Rarity = ""
	params.Rarities = nil
	params.SetQuery = ""
	params.ArtistQuery = ""
	return params
}

func matchingPrintingPriceSQL(alias string) string {
	return "NULLIF(regexp_replace(COALESCE(" + alias + ".price_usd, ''), '[^0-9.]', '', 'g'), '')::double precision"
}

func buildMatchingPrintingFilters(params CardSearchParams, alias string, startArg int) (string, []any) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	argN := startArg

	priceExpr := matchingPrintingPriceSQL(alias)
	if len(params.PriceFilters) > 0 {
		for _, filter := range params.PriceFilters {
			clauses = append(clauses, priceExpr+" "+cardStatOperatorSQL(filter.Operator)+" $"+fmt.Sprint(argN))
			args = append(args, filter.Value)
			argN++
		}
	} else if params.PriceValue != nil {
		clauses = append(clauses, priceExpr+" "+cardStatOperatorSQL(params.PriceOperator)+" $"+fmt.Sprint(argN))
		args = append(args, *params.PriceValue)
		argN++
	} else {
		if params.PriceUSDMin != nil {
			clauses = append(clauses, priceExpr+" >= $"+fmt.Sprint(argN))
			args = append(args, *params.PriceUSDMin)
			argN++
		}
		if params.PriceUSDMax != nil {
			clauses = append(clauses, priceExpr+" <= $"+fmt.Sprint(argN))
			args = append(args, *params.PriceUSDMax)
			argN++
		}
	}

	rarities := normalizeRarityFilters(params.Rarities)
	if len(rarities) == 0 {
		if rarity := normalizeRarityFilter(params.Rarity); rarity != "" {
			rarities = []string{rarity}
		}
	}
	if len(rarities) == 1 {
		clauses = append(clauses, "lower(COALESCE("+alias+".rarity, '')) = $"+fmt.Sprint(argN))
		args = append(args, rarities[0])
		argN++
	} else if len(rarities) > 1 {
		clauses = append(clauses, "lower(COALESCE("+alias+".rarity, '')) = ANY($"+fmt.Sprint(argN)+"::text[])")
		args = append(args, pq.Array(rarities))
		argN++
	}

	if setQuery := strings.TrimSpace(params.SetQuery); setQuery != "" {
		clauses = append(
			clauses,
			"("+alias+".set_code ILIKE '%' || $"+fmt.Sprint(argN)+" || '%' OR "+
				alias+".set_name ILIKE '%' || $"+fmt.Sprint(argN)+" || '%')",
		)
		args = append(args, setQuery)
		argN++
	}
	if artistQuery := strings.TrimSpace(params.ArtistQuery); artistQuery != "" {
		clauses = append(clauses, alias+".artist ILIKE '%' || $"+fmt.Sprint(argN)+" || '%'")
		args = append(args, artistQuery)
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

func defaultCardSearchFromSQL() string {
	return `
		FROM oracle_cards oc
		LEFT JOIN card_prints cp ON cp.scryfall_id = oc.default_print_id
	`
}

func matchingPrintingCardSearchFromSQL(printingFilterSQL string) string {
	return `
		FROM oracle_cards oc
		JOIN LATERAL (
			SELECT cp_match.*
			FROM card_prints cp_match
			WHERE cp_match.oracle_id = oc.oracle_id` + printingFilterSQL + `
			ORDER BY
				CASE WHEN lower(COALESCE(cp_match.lang, 'en')) = 'en' THEN 0 ELSE 1 END ASC,
				cp_match.released_at DESC NULLS LAST,
				lower(COALESCE(cp_match.set_code, '')) ASC,
				COALESCE(cp_match.collector_number, '') ASC,
				cp_match.scryfall_id ASC
			LIMIT 1
		) cp ON TRUE
	`
}

func matchingPrintingCardSearchSelectSQL(fromSQL string) string {
	return `
		SELECT
			cp.scryfall_id::text AS id,
			oc.oracle_id::text,
			oc.name,
			COALESCE(oc.mana_cost, '') AS mana_cost,
			COALESCE(oc.type_line, '') AS type_line,
			COALESCE(oc.oracle_text, '') AS oracle_text,
			COALESCE(cp.flavor_text, '') AS flavor_text,
			COALESCE(cp.image_uri, '') AS image_uri,
			COALESCE(cp.image_uris->>'art_crop', cp.image_uri, '') AS art_crop_uri,
			COALESCE(oc.colors, ARRAY[]::text[]) AS colors,
			COALESCE(oc.color_identity, ARRAY[]::text[]) AS color_identity,
			COALESCE(oc.cmc, 0) AS cmc,
			COALESCE(oc.power_text, '') AS power_text,
			COALESCE(oc.toughness_text, '') AS toughness_text,
			COALESCE(oc.loyalty_text, '') AS loyalty_text,
			COALESCE(oc.layout, '') AS layout,
			COALESCE(oc.legal_anywhere, true) AS legal_anywhere,
			COALESCE(oc.commander_legal, false) AS commander_legal,
			COALESCE(oc.is_commander_candidate, false) AS is_commander_candidate,
			COALESCE(cp.price_usd, '') AS price_usd,
			COALESCE(cp.artist, '') AS artist,
			COALESCE(oc.edhrec_rank, 0) AS edhrec_rank,
			COALESCE(cp.scryfall_uri, '') AS scryfall_uri,
			COALESCE(cp.set_code, '') AS set_code,
			COALESCE(cp.set_name, '') AS set_name,
			COALESCE(cp.collector_number, '') AS collector_number,
			COALESCE(cp.rarity, '') AS rarity,
			COALESCE(to_char(cp.released_at, 'YYYY-MM-DD'), '') AS released_at,
			COALESCE(cp.lang, 'en') AS lang,
			COALESCE(
				NULLIF(cp.card_faces_json::text, '[]'),
				oc.card_faces::text,
				'[]'
			),
			COALESCE(oc.legalities, '{}'::jsonb)::text AS legalities_json
	` + fromSQL
}

func buildCardSearchSourcePlan(params CardSearchParams, startArg int) cardSearchSourcePlan {
	if !hasPrintingSpecificCardSearchFilters(params) {
		filterSQL, args := buildCardSearchFilters(params, startArg)
		return cardSearchSourcePlan{
			SelectSQL:    canonicalCardSelectSQL(),
			CountFromSQL: defaultCardSearchFromSQL(),
			FilterSQL:    filterSQL,
			Args:         args,
		}
	}

	printingFilterSQL, printingArgs := buildMatchingPrintingFilters(params, "cp_match", startArg)
	oracleParams := withoutPrintingSpecificCardSearchFilters(params)
	oracleFilterSQL, oracleArgs := buildCardSearchFilters(oracleParams, startArg+len(printingArgs))
	fromSQL := matchingPrintingCardSearchFromSQL(printingFilterSQL)

	return cardSearchSourcePlan{
		SelectSQL:         matchingPrintingCardSearchSelectSQL(fromSQL),
		CountFromSQL:      fromSQL,
		FilterSQL:         oracleFilterSQL,
		Args:              append(printingArgs, oracleArgs...),
		MatchingPrintings: true,
	}
}

func normalizeCardSearchPage(page, pageSize, total int) cardSearchPageWindow {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
		if page > totalPages {
			page = totalPages
		}
	} else {
		page = 1
	}
	return cardSearchPageWindow{
		Page:       page,
		PageSize:   pageSize,
		Offset:     (page - 1) * pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}
