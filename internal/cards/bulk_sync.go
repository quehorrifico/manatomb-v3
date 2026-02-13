package cards

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

const (
	scryfallBulkListURL     = "https://api.scryfall.com/bulk-data"
	defaultBulkSyncInterval = 24 * time.Hour
	cardSyncAdvisoryLockKey = int64(91342817)
)

var ErrCardSyncInProgress = errors.New("card sync already running")

type CardBulkSyncOptions struct {
	MaxRows int
}

type CardBulkSyncResult struct {
	ImportedCards     int
	ImportedPrintings int
	SourceUpdatedAt   time.Time
	OracleDownloadURI string
	PrintsDownloadURI string
}

type scryfallBulkDescriptor struct {
	Object      string    `json:"object"`
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	UpdatedAt   time.Time `json:"updated_at"`
	DownloadURI string    `json:"download_uri"`
}

type scryfallBulkListResponse struct {
	Data []scryfallBulkDescriptor `json:"data"`
}

type oracleBulkRow struct {
	OracleID             string
	Name                 string
	ManaCost             string
	CMC                  float64
	TypeLine             string
	OracleText           string
	Colors               []string
	ColorIdentity        []string
	Layout               string
	CardFacesJSON        string
	CommanderLegal       bool
	IsCommanderCandidate bool
	EDHRecRank           int
}

type printBulkRow struct {
	ScryfallID      string
	OracleID        string
	Name            string
	SetCode         string
	CollectorNumber string
	Lang            string
	ReleasedAt      sql.NullTime
	ImageURIsJSON   string
	ImageURI        string
	CardFacesJSON   string
	SetName         string
	Rarity          string
	Artist          string
	PriceUSD        string
	ScryfallURI     string
}

func normalizeCardBulkSyncOptions(options CardBulkSyncOptions) CardBulkSyncOptions {
	if options.MaxRows < 0 {
		options.MaxRows = 0
	}
	return options
}

func formatSyncDuration(d time.Duration) string {
	return d.Round(10 * time.Millisecond).String()
}

func parseReleaseDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func supportsPaper(games []string) bool {
	if len(games) == 0 {
		return false
	}
	for _, g := range games {
		if strings.EqualFold(strings.TrimSpace(g), "paper") {
			return true
		}
	}
	return false
}

func isLegendaryCreatureType(typeLine string) bool {
	typeLine = strings.ToLower(strings.TrimSpace(typeLine))
	if typeLine == "" {
		return false
	}
	return strings.Contains(typeLine, "legendary") && strings.Contains(typeLine, "creature")
}

func hasCommanderText(oracleText string) bool {
	oracleText = strings.ToLower(strings.TrimSpace(oracleText))
	if oracleText == "" {
		return false
	}
	return strings.Contains(oracleText, "can be your commander")
}

func faceMatchesCommanderCandidateRule(typeLine, oracleText string) bool {
	return isLegendaryCreatureType(typeLine) || hasCommanderText(oracleText)
}

func isCommanderCandidate(sc scryfallCard) bool {
	if sc.Legalities == nil || !strings.EqualFold(strings.TrimSpace(sc.Legalities["commander"]), "legal") {
		return false
	}
	if !supportsPaper(sc.Games) {
		return false
	}

	if faceMatchesCommanderCandidateRule(sc.TypeLine, sc.OracleText) {
		return true
	}
	for _, face := range sc.CardFaces {
		if faceMatchesCommanderCandidateRule(face.TypeLine, face.OracleText) {
			return true
		}
	}
	return false
}

func shouldExcludeToken(sc scryfallCard, c Card) bool {
	if strings.EqualFold(strings.TrimSpace(sc.SetType), "token") {
		return true
	}
	typeLine := strings.ToLower(strings.TrimSpace(c.TypeLine))
	return strings.Contains(typeLine, "token")
}

func shouldIncludeOracleCard(sc scryfallCard, c Card) bool {
	if strings.TrimSpace(c.OracleID) == "" || strings.TrimSpace(c.Name) == "" {
		return false
	}
	if !supportsPaper(sc.Games) {
		return false
	}
	if shouldExcludeToken(sc, c) {
		return false
	}
	return true
}

func shouldIncludePrint(sc scryfallCard, c Card) bool {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.OracleID) == "" || strings.TrimSpace(c.Name) == "" {
		return false
	}
	if !supportsPaper(sc.Games) {
		return false
	}
	if shouldExcludeToken(sc, c) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(sc.Lang), "en") {
		return false
	}
	return true
}

func fetchBulkDescriptor(ctx context.Context, wantedType string) (scryfallBulkDescriptor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scryfallBulkListURL, nil)
	if err != nil {
		return scryfallBulkDescriptor{}, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return scryfallBulkDescriptor{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return scryfallBulkDescriptor{}, fmt.Errorf("bulk descriptor list request failed: status %d", resp.StatusCode)
	}

	var list scryfallBulkListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return scryfallBulkDescriptor{}, err
	}

	wantedType = strings.TrimSpace(wantedType)
	for _, descriptor := range list.Data {
		if strings.EqualFold(strings.TrimSpace(descriptor.Type), wantedType) {
			if strings.TrimSpace(descriptor.DownloadURI) == "" {
				return scryfallBulkDescriptor{}, fmt.Errorf("missing download_uri for bulk type %s", wantedType)
			}
			return descriptor, nil
		}
	}
	return scryfallBulkDescriptor{}, fmt.Errorf("bulk descriptor not found for type %s", wantedType)
}

func newBulkJSONDecoder(ctx context.Context, downloadURI string) (*json.Decoder, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURI, nil)
	if err != nil {
		return nil, nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		cleanup()
		return nil, nil, fmt.Errorf("bulk download failed: status %d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	startTok, err := dec.Token()
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	delim, ok := startTok.(json.Delim)
	if !ok || delim != '[' {
		cleanup()
		return nil, nil, errors.New("unexpected bulk payload format: expected JSON array")
	}
	return dec, cleanup, nil
}

func decodeOracleRows(ctx context.Context, downloadURI string, maxRows int) ([]oracleBulkRow, error) {
	dec, cleanup, err := newBulkJSONDecoder(ctx, downloadURI)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	rows := make([]oracleBulkRow, 0, 40000)
	for dec.More() {
		var raw scryfallCard
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}

		card := normalizeScryfallCard(raw)
		if !shouldIncludeOracleCard(raw, card) {
			continue
		}

		facesJSON := "[]"
		if len(card.Faces) > 0 {
			if b, err := json.Marshal(card.Faces); err == nil {
				facesJSON = string(b)
			}
		}

		rows = append(rows, oracleBulkRow{
			OracleID:             card.OracleID,
			Name:                 card.Name,
			ManaCost:             card.ManaCost,
			CMC:                  card.CMC,
			TypeLine:             card.TypeLine,
			OracleText:           card.OracleText,
			Colors:               nonNilStrings(card.Colors),
			ColorIdentity:        nonNilStrings(card.ColorIdentity),
			Layout:               card.Layout,
			CardFacesJSON:        facesJSON,
			CommanderLegal:       card.CommanderLegal,
			IsCommanderCandidate: isCommanderCandidate(raw),
			EDHRecRank:           card.EDHRecRank,
		})

		if maxRows > 0 && len(rows) >= maxRows {
			break
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		left := strings.ToLower(rows[i].Name)
		right := strings.ToLower(rows[j].Name)
		if left != right {
			return left < right
		}
		return strings.ToLower(rows[i].OracleID) < strings.ToLower(rows[j].OracleID)
	})
	return rows, nil
}

func decodePrintRows(ctx context.Context, downloadURI string, maxRows int) ([]printBulkRow, error) {
	dec, cleanup, err := newBulkJSONDecoder(ctx, downloadURI)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	rows := make([]printBulkRow, 0, 50000)
	for dec.More() {
		var raw scryfallCard
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}

		card := normalizeScryfallCard(raw)
		if !shouldIncludePrint(raw, card) {
			continue
		}

		releasedAt, hasRelease := parseReleaseDate(raw.ReleasedAt)
		imageURIsJSON := "{}"
		if raw.ImageURIs != nil {
			if b, err := json.Marshal(raw.ImageURIs); err == nil {
				imageURIsJSON = string(b)
			}
		}
		cardFacesJSON := "[]"
		if len(card.Faces) > 0 {
			if b, err := json.Marshal(card.Faces); err == nil {
				cardFacesJSON = string(b)
			}
		}

		rows = append(rows, printBulkRow{
			ScryfallID:      card.ID,
			OracleID:        card.OracleID,
			Name:            card.Name,
			SetCode:         strings.ToLower(strings.TrimSpace(card.SetCode)),
			CollectorNumber: strings.TrimSpace(raw.CollectorNumber),
			Lang:            "en",
			ReleasedAt: sql.NullTime{
				Time:  releasedAt,
				Valid: hasRelease,
			},
			ImageURIsJSON: imageURIsJSON,
			ImageURI:      card.ImageURI,
			CardFacesJSON: cardFacesJSON,
			SetName:       card.SetName,
			Rarity:        strings.TrimSpace(raw.Rarity),
			Artist:        card.Artist,
			PriceUSD:      card.PriceUSD,
			ScryfallURI:   card.ScryfallURI,
		})

		if maxRows > 0 && len(rows) >= maxRows {
			break
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		leftOracle := strings.ToLower(rows[i].OracleID)
		rightOracle := strings.ToLower(rows[j].OracleID)
		if leftOracle != rightOracle {
			return leftOracle < rightOracle
		}
		leftDateValid := rows[i].ReleasedAt.Valid
		rightDateValid := rows[j].ReleasedAt.Valid
		if leftDateValid != rightDateValid {
			return leftDateValid
		}
		if leftDateValid && !rows[i].ReleasedAt.Time.Equal(rows[j].ReleasedAt.Time) {
			return rows[i].ReleasedAt.Time.After(rows[j].ReleasedAt.Time)
		}
		leftSet := strings.ToLower(rows[i].SetCode)
		rightSet := strings.ToLower(rows[j].SetCode)
		if leftSet != rightSet {
			return leftSet < rightSet
		}
		return strings.ToLower(rows[i].CollectorNumber) < strings.ToLower(rows[j].CollectorNumber)
	})
	return rows, nil
}

func withCardSyncLock(ctx context.Context, db *sql.DB, fn func() error) error {
	var locked bool
	if err := db.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, cardSyncAdvisoryLockKey).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return ErrCardSyncInProgress
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, cardSyncAdvisoryLockKey)
	}()
	return fn()
}

func markCardSyncFailure(ctx context.Context, db *sql.DB, err error) {
	if err == nil {
		return
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		msg = "unknown sync error"
	}
	_, _ = db.ExecContext(ctx, `
		UPDATE card_sync_state
		SET last_attempt_at = NOW(), last_error = $1
		WHERE id = 1
	`, msg)
}

func applyBulkRows(
	ctx context.Context,
	db *sql.DB,
	oracleRows []oracleBulkRow,
	printRows []printBulkRow,
	sourceUpdatedAt time.Time,
) error {
	logger := log.Default()
	applyStart := time.Now()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	logger.Printf("cards sync phase: begin transaction")

	if _, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE oracle_cards_sync_stage (
			oracle_id UUID NOT NULL,
			name TEXT NOT NULL,
			mana_cost TEXT,
			cmc DOUBLE PRECISION,
			type_line TEXT,
			oracle_text TEXT,
			colors TEXT[] NOT NULL,
			color_identity TEXT[] NOT NULL,
			layout TEXT,
			card_faces JSONB,
			commander_legal BOOLEAN,
			is_commander_candidate BOOLEAN,
			edhrec_rank INTEGER
		) ON COMMIT DROP;
	`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE card_prints_sync_stage (
			scryfall_id UUID NOT NULL,
			oracle_id UUID NOT NULL,
			name TEXT NOT NULL,
			set_code TEXT NOT NULL,
			collector_number TEXT NOT NULL,
			lang TEXT NOT NULL,
			released_at DATE,
			image_uris JSONB,
			image_uri TEXT,
			card_faces_json JSONB,
			set_name TEXT,
			rarity TEXT,
			artist TEXT,
			price_usd TEXT,
			scryfall_uri TEXT
		) ON COMMIT DROP;
	`); err != nil {
		return err
	}

	oracleCopyStmt, err := tx.PrepareContext(ctx, pq.CopyIn(
		"oracle_cards_sync_stage",
		"oracle_id",
		"name",
		"mana_cost",
		"cmc",
		"type_line",
		"oracle_text",
		"colors",
		"color_identity",
		"layout",
		"card_faces",
		"commander_legal",
		"is_commander_candidate",
		"edhrec_rank",
	))
	if err != nil {
		return err
	}
	for _, row := range oracleRows {
		if _, err := oracleCopyStmt.Exec(
			row.OracleID,
			row.Name,
			row.ManaCost,
			row.CMC,
			row.TypeLine,
			row.OracleText,
			pq.Array(nonNilStrings(row.Colors)),
			pq.Array(nonNilStrings(row.ColorIdentity)),
			row.Layout,
			row.CardFacesJSON,
			row.CommanderLegal,
			row.IsCommanderCandidate,
			row.EDHRecRank,
		); err != nil {
			_ = oracleCopyStmt.Close()
			return err
		}
	}
	if _, err := oracleCopyStmt.Exec(); err != nil {
		_ = oracleCopyStmt.Close()
		return err
	}
	if err := oracleCopyStmt.Close(); err != nil {
		return err
	}

	printCopyStmt, err := tx.PrepareContext(ctx, pq.CopyIn(
		"card_prints_sync_stage",
		"scryfall_id",
		"oracle_id",
		"name",
		"set_code",
		"collector_number",
		"lang",
		"released_at",
		"image_uris",
		"image_uri",
		"card_faces_json",
		"set_name",
		"rarity",
		"artist",
		"price_usd",
		"scryfall_uri",
	))
	if err != nil {
		return err
	}
	for _, row := range printRows {
		if _, err := printCopyStmt.Exec(
			row.ScryfallID,
			row.OracleID,
			row.Name,
			row.SetCode,
			row.CollectorNumber,
			row.Lang,
			row.ReleasedAt,
			row.ImageURIsJSON,
			row.ImageURI,
			row.CardFacesJSON,
			row.SetName,
			row.Rarity,
			row.Artist,
			row.PriceUSD,
			row.ScryfallURI,
		); err != nil {
			_ = printCopyStmt.Close()
			return err
		}
	}
	if _, err := printCopyStmt.Exec(); err != nil {
		_ = printCopyStmt.Close()
		return err
	}
	if err := printCopyStmt.Close(); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX oracle_cards_sync_stage_oracle_idx ON oracle_cards_sync_stage (oracle_id);`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX card_prints_sync_stage_print_idx ON card_prints_sync_stage (scryfall_id);`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ANALYZE oracle_cards_sync_stage;`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ANALYZE card_prints_sync_stage;`); err != nil {
		return err
	}

	oracleUpsertRes, err := tx.ExecContext(ctx, `
		INSERT INTO oracle_cards (
			oracle_id,
			name,
			mana_cost,
			cmc,
			type_line,
			oracle_text,
			colors,
			color_identity,
			layout,
			card_faces,
			commander_legal,
			is_commander_candidate,
			edhrec_rank
		)
		SELECT
			s.oracle_id,
			s.name,
			s.mana_cost,
			s.cmc,
			s.type_line,
			s.oracle_text,
			s.colors,
			s.color_identity,
			s.layout,
			s.card_faces,
			s.commander_legal,
			s.is_commander_candidate,
			s.edhrec_rank
		FROM oracle_cards_sync_stage s
		ON CONFLICT (oracle_id) DO UPDATE
		SET
			name = EXCLUDED.name,
			mana_cost = EXCLUDED.mana_cost,
			cmc = EXCLUDED.cmc,
			type_line = EXCLUDED.type_line,
			oracle_text = EXCLUDED.oracle_text,
			colors = EXCLUDED.colors,
			color_identity = EXCLUDED.color_identity,
			layout = EXCLUDED.layout,
			card_faces = EXCLUDED.card_faces,
			commander_legal = EXCLUDED.commander_legal,
			is_commander_candidate = EXCLUDED.is_commander_candidate,
			edhrec_rank = EXCLUDED.edhrec_rank
	`)
	if err != nil {
		return err
	}
	oracleUpserted, _ := oracleUpsertRes.RowsAffected()
	logger.Printf("cards sync phase: upserted oracle cards=%d", oracleUpserted)

	printUpsertRes, err := tx.ExecContext(ctx, `
		INSERT INTO card_prints (
			scryfall_id,
			oracle_id,
			name,
			set_code,
			collector_number,
			lang,
			released_at,
			image_uris,
			image_uri,
			card_faces_json,
			set_name,
			rarity,
			artist,
			price_usd,
			scryfall_uri
		)
		SELECT
			s.scryfall_id,
			s.oracle_id,
			s.name,
			s.set_code,
			s.collector_number,
			s.lang,
			s.released_at,
			s.image_uris,
			s.image_uri,
			s.card_faces_json,
			s.set_name,
			s.rarity,
			s.artist,
			s.price_usd,
			s.scryfall_uri
		FROM card_prints_sync_stage s
		ON CONFLICT (scryfall_id) DO UPDATE
		SET
			oracle_id = EXCLUDED.oracle_id,
			name = EXCLUDED.name,
			set_code = EXCLUDED.set_code,
			collector_number = EXCLUDED.collector_number,
			lang = EXCLUDED.lang,
			released_at = EXCLUDED.released_at,
			image_uris = EXCLUDED.image_uris,
			image_uri = EXCLUDED.image_uri,
			card_faces_json = EXCLUDED.card_faces_json,
			set_name = EXCLUDED.set_name,
			rarity = EXCLUDED.rarity,
			artist = EXCLUDED.artist,
			price_usd = EXCLUDED.price_usd,
			scryfall_uri = EXCLUDED.scryfall_uri
	`)
	if err != nil {
		return err
	}
	printUpserted, _ := printUpsertRes.RowsAffected()
	logger.Printf("cards sync phase: upserted printings=%d", printUpserted)

	// Stale printings are removed unless explicitly selected as preferred prints.
	printDeleteRes, err := tx.ExecContext(ctx, `
		DELETE FROM card_prints cp
		WHERE NOT EXISTS (
			SELECT 1
			FROM card_prints_sync_stage s
			WHERE s.scryfall_id = cp.scryfall_id
		)
		AND NOT EXISTS (
			SELECT 1
			FROM deck_cards dc
			WHERE dc.preferred_print_id = cp.scryfall_id
		)
	`)
	if err != nil {
		return err
	}
	printDeleted, _ := printDeleteRes.RowsAffected()
	logger.Printf("cards sync phase: deleted stale printings=%d", printDeleted)

	// Stale canonical cards are removed unless still referenced by decks.
	oracleDeleteRes, err := tx.ExecContext(ctx, `
		DELETE FROM oracle_cards oc
		WHERE NOT EXISTS (
			SELECT 1
			FROM oracle_cards_sync_stage s
			WHERE s.oracle_id = oc.oracle_id
		)
		AND NOT EXISTS (
			SELECT 1
			FROM deck_cards dc
			WHERE dc.oracle_id = oc.oracle_id
		)
	`)
	if err != nil {
		return err
	}
	oracleDeleted, _ := oracleDeleteRes.RowsAffected()
	logger.Printf("cards sync phase: deleted stale oracle cards=%d", oracleDeleted)

	// Cache the newest English printing for each canonical oracle card.
	if _, err := tx.ExecContext(ctx, `
		WITH newest AS (
			SELECT DISTINCT ON (oracle_id)
				oracle_id,
				scryfall_id
			FROM card_prints
			WHERE lower(lang) = 'en'
			ORDER BY oracle_id, released_at DESC NULLS LAST, set_code ASC, collector_number ASC, scryfall_id ASC
		)
		UPDATE oracle_cards oc
		SET default_print_id = n.scryfall_id
		FROM newest n
		WHERE oc.oracle_id = n.oracle_id
		  AND oc.default_print_id IS DISTINCT FROM n.scryfall_id
	`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE oracle_cards oc
		SET default_print_id = NULL
		WHERE oc.default_print_id IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM card_prints cp
			WHERE cp.scryfall_id = oc.default_print_id
		)
	`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE oracle_cards oc
		SET
			default_image_uri = COALESCE(cp.image_uri, ''),
			default_price_usd = COALESCE(cp.price_usd, ''),
			default_artist = COALESCE(cp.artist, ''),
			default_set_code = COALESCE(cp.set_code, ''),
			default_set_name = COALESCE(cp.set_name, ''),
			default_released_at = cp.released_at,
			default_scryfall_uri = COALESCE(cp.scryfall_uri, '')
		FROM card_prints cp
		WHERE cp.scryfall_id = oc.default_print_id
	`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE oracle_cards
		SET
			default_image_uri = '',
			default_price_usd = '',
			default_artist = '',
			default_set_code = '',
			default_set_name = '',
			default_released_at = NULL,
			default_scryfall_uri = ''
		WHERE default_print_id IS NULL
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE card_sync_state
		SET
			last_attempt_at = NOW(),
			last_success_at = NOW(),
			source_updated_at = $1,
			last_error = '',
			card_count = (SELECT COUNT(*) FROM oracle_cards)
		WHERE id = 1
	`, sourceUpdatedAt); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	logger.Printf("cards sync phase: commit complete (apply total %s)", formatSyncDuration(time.Since(applyStart)))
	return nil
}

func SyncCardsFromScryfallBulk(ctx context.Context, db *sql.DB, options CardBulkSyncOptions) (*CardBulkSyncResult, error) {
	logger := log.Default()
	totalStart := time.Now()
	var result CardBulkSyncResult
	options = normalizeCardBulkSyncOptions(options)

	logger.Printf("cards sync phase: sync requested")
	if options.MaxRows > 0 {
		logger.Printf("cards sync phase: limited mode enabled (max_rows=%d)", options.MaxRows)
	}

	err := withCardSyncLock(ctx, db, func() error {
		logger.Printf("cards sync phase: advisory lock acquired")
		_, _ = db.ExecContext(ctx, `
			UPDATE card_sync_state
			SET last_attempt_at = NOW(), last_error = ''
			WHERE id = 1
		`)

		oracleDescriptor, err := fetchBulkDescriptor(ctx, "oracle_cards")
		if err != nil {
			return err
		}
		printsDescriptor, err := fetchBulkDescriptor(ctx, "all_cards")
		if err != nil {
			return err
		}

		sourceUpdatedAt := oracleDescriptor.UpdatedAt
		if printsDescriptor.UpdatedAt.After(sourceUpdatedAt) {
			sourceUpdatedAt = printsDescriptor.UpdatedAt
		}
		logger.Printf(
			"cards sync phase: fetched descriptors (oracle=%s prints=%s)",
			oracleDescriptor.UpdatedAt.UTC().Format(time.RFC3339),
			printsDescriptor.UpdatedAt.UTC().Format(time.RFC3339),
		)

		oracleRows, err := decodeOracleRows(ctx, oracleDescriptor.DownloadURI, options.MaxRows)
		if err != nil {
			return err
		}
		logger.Printf("cards sync phase: decoded oracle rows=%d", len(oracleRows))

		printLimit := options.MaxRows
		if options.MaxRows > 0 {
			// Limited canonical mode still needs complete print coverage for sampled
			// oracle ids so version dropdown/art/price remain coherent.
			printLimit = 0
		}
		printRows, err := decodePrintRows(ctx, printsDescriptor.DownloadURI, printLimit)
		if err != nil {
			return err
		}
		logger.Printf("cards sync phase: decoded print rows=%d", len(printRows))

		// In limited mode, oracle/default streams can diverge on which records are
		// sampled first. Keep only printings whose oracle_id exists in the sampled
		// canonical set so FK inserts remain valid.
		oracleIDs := make(map[string]struct{}, len(oracleRows))
		for _, row := range oracleRows {
			id := strings.ToLower(strings.TrimSpace(row.OracleID))
			if id == "" {
				continue
			}
			oracleIDs[id] = struct{}{}
		}
		filteredPrints := make([]printBulkRow, 0, len(printRows))
		for _, row := range printRows {
			id := strings.ToLower(strings.TrimSpace(row.OracleID))
			if id == "" {
				continue
			}
			if _, ok := oracleIDs[id]; !ok {
				continue
			}
			filteredPrints = append(filteredPrints, row)
		}
		if len(filteredPrints) != len(printRows) {
			logger.Printf(
				"cards sync phase: filtered print rows to match oracle set (%d -> %d)",
				len(printRows),
				len(filteredPrints),
			)
		}
		printRows = filteredPrints

		if err := applyBulkRows(ctx, db, oracleRows, printRows, sourceUpdatedAt); err != nil {
			return err
		}

		result = CardBulkSyncResult{
			ImportedCards:     len(oracleRows),
			ImportedPrintings: len(printRows),
			SourceUpdatedAt:   sourceUpdatedAt,
			OracleDownloadURI: oracleDescriptor.DownloadURI,
			PrintsDownloadURI: printsDescriptor.DownloadURI,
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCardSyncInProgress) {
			logger.Printf("cards sync phase: sync skipped because another run holds lock")
		} else {
			markCardSyncFailure(ctx, db, err)
		}
		return nil, err
	}

	logger.Printf(
		"cards sync phase: sync total duration %s (oracle=%d printings=%d)",
		formatSyncDuration(time.Since(totalStart)),
		result.ImportedCards,
		result.ImportedPrintings,
	)
	return &result, nil
}

func CardSyncDue(ctx context.Context, db *sql.DB, maxAge time.Duration) (bool, error) {
	if maxAge <= 0 {
		maxAge = defaultBulkSyncInterval
	}

	var cardsCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM oracle_cards`).Scan(&cardsCount); err != nil {
		return false, err
	}
	if cardsCount == 0 {
		return true, nil
	}

	var lastSuccess sql.NullTime
	err := db.QueryRowContext(ctx, `
		SELECT last_success_at
		FROM card_sync_state
		WHERE id = 1
	`).Scan(&lastSuccess)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !lastSuccess.Valid {
		return true, nil
	}
	return time.Since(lastSuccess.Time) >= maxAge, nil
}

func StartCardBulkSyncLoop(db *sql.DB, interval time.Duration, logger *log.Logger, options CardBulkSyncOptions) {
	if interval <= 0 {
		interval = defaultBulkSyncInterval
	}
	if logger == nil {
		logger = log.Default()
	}
	options = normalizeCardBulkSyncOptions(options)
	checkEvery := time.Hour
	if interval < checkEvery {
		checkEvery = interval
	}

	go func() {
		ticker := time.NewTicker(checkEvery)
		defer ticker.Stop()

		for range ticker.C {
			checkCtx, cancelCheck := context.WithTimeout(context.Background(), 30*time.Second)
			due, dueErr := CardSyncDue(checkCtx, db, interval)
			cancelCheck()
			if dueErr != nil {
				logger.Printf("cards bulk sync due-check failed: %v", dueErr)
				continue
			}
			if !due {
				continue
			}

			syncCtx, cancelSync := context.WithTimeout(context.Background(), 2*time.Hour)
			result, err := SyncCardsFromScryfallBulk(syncCtx, db, options)
			cancelSync()
			if err != nil {
				if errors.Is(err, ErrCardSyncInProgress) {
					logger.Printf("cards bulk sync skipped: already running")
					continue
				}
				logger.Printf("cards bulk sync failed: %v", err)
				continue
			}
			logger.Printf(
				"cards bulk sync complete: %d canonical cards, %d printings (source updated %s)",
				result.ImportedCards,
				result.ImportedPrintings,
				result.SourceUpdatedAt.UTC().Format(time.RFC3339),
			)
		}
	}()
}
