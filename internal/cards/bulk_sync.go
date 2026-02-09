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
)

const (
	scryfallAllCardsBulkURL = "https://api.scryfall.com/bulk-data/all_cards"
	defaultBulkSyncInterval = 24 * time.Hour
	cardSyncAdvisoryLockKey = int64(91342817)
)

var ErrCardSyncInProgress = errors.New("card sync already running")

type CardBulkSyncResult struct {
	ImportedCards   int
	SourceUpdatedAt time.Time
	DownloadURI     string
}

type scryfallBulkDescriptor struct {
	Object      string    `json:"object"`
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	UpdatedAt   time.Time `json:"updated_at"`
	DownloadURI string    `json:"download_uri"`
}

type bulkCardRow struct {
	Name           string
	ManaCost       string
	TypeLine       string
	OracleText     string
	ImageURI       string
	CardFacesJSON  string
	Colors         string
	ColorIdentity  string
	CMC            float64
	Layout         string
	CommanderLegal bool
	PriceUSD       string
	Artist         string
	EDHRecRank     int
	ScryfallURI    string
	SetCode        string
	SetName        string
	ScryfallID     string
	OracleID       string
}

type stagedBulkCard struct {
	Row         bulkCardRow
	ReleasedAt  time.Time
	HasReleased bool
}

func csvString(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
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

func supportsPaper(games []string) bool {
	if len(games) == 0 {
		return true
	}
	for _, g := range games {
		if strings.EqualFold(strings.TrimSpace(g), "paper") {
			return true
		}
	}
	return false
}

func shouldIncludeBulkCard(sc scryfallCard, c Card) bool {
	if strings.TrimSpace(c.Name) == "" {
		return false
	}
	if !supportsPaper(sc.Games) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(sc.SetType), "token") {
		return false
	}

	typeLine := strings.ToLower(strings.TrimSpace(c.TypeLine))
	if strings.Contains(typeLine, "token") {
		return false
	}
	return true
}

func bulkCardRowFromCard(c Card) bulkCardRow {
	facesJSON := "[]"
	if len(c.Faces) > 0 {
		if b, err := json.Marshal(c.Faces); err == nil {
			facesJSON = string(b)
		}
	}

	return bulkCardRow{
		Name:           c.Name,
		ManaCost:       c.ManaCost,
		TypeLine:       c.TypeLine,
		OracleText:     c.OracleText,
		ImageURI:       c.ImageURI,
		CardFacesJSON:  facesJSON,
		Colors:         csvString(c.Colors),
		ColorIdentity:  csvString(c.ColorIdentity),
		CMC:            c.CMC,
		Layout:         c.Layout,
		CommanderLegal: c.CommanderLegal,
		PriceUSD:       c.PriceUSD,
		Artist:         c.Artist,
		EDHRecRank:     c.EDHRecRank,
		ScryfallURI:    c.ScryfallURI,
		SetCode:        c.SetCode,
		SetName:        c.SetName,
		ScryfallID:     c.ID,
		OracleID:       c.OracleID,
	}
}

func shouldReplaceStagedCard(existing, candidate stagedBulkCard) bool {
	if candidate.HasReleased && !existing.HasReleased {
		return true
	}
	if candidate.HasReleased && existing.HasReleased && candidate.ReleasedAt.After(existing.ReleasedAt) {
		return true
	}
	if existing.Row.ImageURI == "" && candidate.Row.ImageURI != "" {
		return true
	}
	if existing.Row.PriceUSD == "" && candidate.Row.PriceUSD != "" {
		return true
	}
	if !existing.Row.CommanderLegal && candidate.Row.CommanderLegal {
		return true
	}
	if len(candidate.Row.OracleText) > len(existing.Row.OracleText) {
		return true
	}
	return false
}

func fetchBulkDescriptor(ctx context.Context) (scryfallBulkDescriptor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scryfallAllCardsBulkURL, nil)
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
		return scryfallBulkDescriptor{}, fmt.Errorf("bulk descriptor request failed: status %d", resp.StatusCode)
	}

	var descriptor scryfallBulkDescriptor
	if err := json.NewDecoder(resp.Body).Decode(&descriptor); err != nil {
		return scryfallBulkDescriptor{}, err
	}
	if strings.TrimSpace(descriptor.DownloadURI) == "" {
		return scryfallBulkDescriptor{}, errors.New("missing download_uri in scryfall bulk descriptor")
	}
	return descriptor, nil
}

func downloadBulkRows(ctx context.Context, downloadURI string) ([]bulkCardRow, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURI, nil)
	if err != nil {
		return nil, err
	}

	// Bulk file can be large; rely on request context for cancellation.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bulk download failed: status %d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	startTok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := startTok.(json.Delim)
	if !ok || delim != '[' {
		return nil, errors.New("unexpected bulk payload format: expected JSON array")
	}

	byName := make(map[string]stagedBulkCard, 70000)
	for dec.More() {
		var raw scryfallCard
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}

		card := normalizeScryfallCard(raw)
		if !shouldIncludeBulkCard(raw, card) {
			continue
		}

		row := bulkCardRowFromCard(card)
		key := strings.ToLower(strings.TrimSpace(row.Name))
		if key == "" {
			continue
		}

		releasedAt, hasRelease := parseReleaseDate(raw.ReleasedAt)
		candidate := stagedBulkCard{
			Row:         row,
			ReleasedAt:  releasedAt,
			HasReleased: hasRelease,
		}
		existing, exists := byName[key]
		if !exists || shouldReplaceStagedCard(existing, candidate) {
			byName[key] = candidate
		}
	}

	endTok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	endDelim, ok := endTok.(json.Delim)
	if !ok || endDelim != ']' {
		return nil, errors.New("unexpected bulk payload format: missing JSON array end")
	}

	rows := make([]bulkCardRow, 0, len(byName))
	for _, staged := range byName {
		rows = append(rows, staged.Row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
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

func applyBulkRows(ctx context.Context, db *sql.DB, rows []bulkCardRow, sourceUpdatedAt time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE cards_sync_stage (
			name TEXT NOT NULL,
			mana_cost TEXT,
			type_line TEXT,
			oracle_text TEXT,
			image_uri TEXT,
			card_faces_json JSONB,
			colors TEXT,
			color_identity TEXT,
			cmc DOUBLE PRECISION,
			layout TEXT,
			commander_legal BOOLEAN,
			price_usd TEXT,
			artist TEXT,
			edhrec_rank INTEGER,
			scryfall_uri TEXT,
			set_code TEXT,
			set_name TEXT,
			scryfall_id TEXT,
			oracle_id TEXT
		) ON COMMIT DROP;
	`); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO cards_sync_stage (
			name, mana_cost, type_line, oracle_text, image_uri, card_faces_json,
			colors, color_identity, cmc, layout, commander_legal, price_usd,
			artist, edhrec_rank, scryfall_uri, set_code, set_name, scryfall_id, oracle_id
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19
		)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, row := range rows {
		if _, err := stmt.ExecContext(
			ctx,
			row.Name,
			row.ManaCost,
			row.TypeLine,
			row.OracleText,
			row.ImageURI,
			row.CardFacesJSON,
			row.Colors,
			row.ColorIdentity,
			row.CMC,
			row.Layout,
			row.CommanderLegal,
			row.PriceUSD,
			row.Artist,
			row.EDHRecRank,
			row.ScryfallURI,
			row.SetCode,
			row.SetName,
			row.ScryfallID,
			row.OracleID,
		); err != nil {
			return err
		}
	}

	// First, match on oracle_id when present so preexisting rows keep stable numeric IDs.
	if _, err := tx.ExecContext(ctx, `
		UPDATE cards c
		SET
			name = s.name,
			mana_cost = s.mana_cost,
			type_line = s.type_line,
			oracle_text = s.oracle_text,
			image_uri = s.image_uri,
			card_faces_json = s.card_faces_json,
			colors = s.colors,
			color_identity = s.color_identity,
			cmc = s.cmc,
			layout = s.layout,
			commander_legal = s.commander_legal,
			price_usd = s.price_usd,
			artist = s.artist,
			edhrec_rank = s.edhrec_rank,
			scryfall_uri = s.scryfall_uri,
			set_code = s.set_code,
			set_name = s.set_name,
			scryfall_id = s.scryfall_id,
			oracle_id = s.oracle_id
		FROM cards_sync_stage s
		WHERE c.oracle_id <> ''
		  AND s.oracle_id <> ''
		  AND c.oracle_id = s.oracle_id
	`); err != nil {
		return err
	}

	// Then, fallback to case-insensitive name matches (covers rows lacking oracle_id).
	if _, err := tx.ExecContext(ctx, `
		UPDATE cards c
		SET
			name = s.name,
			mana_cost = s.mana_cost,
			type_line = s.type_line,
			oracle_text = s.oracle_text,
			image_uri = s.image_uri,
			card_faces_json = s.card_faces_json,
			colors = s.colors,
			color_identity = s.color_identity,
			cmc = s.cmc,
			layout = s.layout,
			commander_legal = s.commander_legal,
			price_usd = s.price_usd,
			artist = s.artist,
			edhrec_rank = s.edhrec_rank,
			scryfall_uri = s.scryfall_uri,
			set_code = s.set_code,
			set_name = s.set_name,
			scryfall_id = s.scryfall_id,
			oracle_id = s.oracle_id
		FROM cards_sync_stage s
		WHERE lower(c.name) = lower(s.name)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO cards (
			name, mana_cost, type_line, oracle_text, image_uri, card_faces_json,
			colors, color_identity, cmc, layout, commander_legal, price_usd,
			artist, edhrec_rank, scryfall_uri, set_code, set_name, scryfall_id, oracle_id
		)
		SELECT
			s.name, s.mana_cost, s.type_line, s.oracle_text, s.image_uri, s.card_faces_json,
			s.colors, s.color_identity, s.cmc, s.layout, s.commander_legal, s.price_usd,
			s.artist, s.edhrec_rank, s.scryfall_uri, s.set_code, s.set_name, s.scryfall_id, s.oracle_id
		FROM cards_sync_stage s
		WHERE NOT EXISTS (
			SELECT 1
			FROM cards c
			WHERE
				(s.oracle_id <> '' AND c.oracle_id = s.oracle_id)
				OR lower(c.name) = lower(s.name)
		)
	`); err != nil {
		return err
	}

	// Keep rows that are still referenced by decks to avoid breaking existing deck_card links.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM cards c
		WHERE NOT EXISTS (
			SELECT 1
			FROM cards_sync_stage s
			WHERE
				(c.oracle_id <> '' AND s.oracle_id <> '' AND s.oracle_id = c.oracle_id)
				OR lower(s.name) = lower(c.name)
		)
		AND NOT EXISTS (
			SELECT 1 FROM deck_cards dc WHERE dc.card_id = c.id
		)
		AND NOT EXISTS (
			SELECT 1 FROM deck_maybe_cards dmc WHERE dmc.card_id = c.id
		)
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
			card_count = (SELECT COUNT(*) FROM cards)
		WHERE id = 1
	`, sourceUpdatedAt); err != nil {
		return err
	}

	return tx.Commit()
}

func SyncCardsFromScryfallBulk(ctx context.Context, db *sql.DB) (*CardBulkSyncResult, error) {
	var result CardBulkSyncResult

	err := withCardSyncLock(ctx, db, func() error {
		_, _ = db.ExecContext(ctx, `
			UPDATE card_sync_state
			SET last_attempt_at = NOW(), last_error = ''
			WHERE id = 1
		`)

		descriptor, err := fetchBulkDescriptor(ctx)
		if err != nil {
			return err
		}

		rows, err := downloadBulkRows(ctx, descriptor.DownloadURI)
		if err != nil {
			return err
		}

		if err := applyBulkRows(ctx, db, rows, descriptor.UpdatedAt); err != nil {
			return err
		}

		result = CardBulkSyncResult{
			ImportedCards:   len(rows),
			SourceUpdatedAt: descriptor.UpdatedAt,
			DownloadURI:     descriptor.DownloadURI,
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, ErrCardSyncInProgress) {
			markCardSyncFailure(ctx, db, err)
		}
		return nil, err
	}
	return &result, nil
}

func CardSyncDue(ctx context.Context, db *sql.DB, maxAge time.Duration) (bool, error) {
	if maxAge <= 0 {
		maxAge = defaultBulkSyncInterval
	}

	var cardsCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards`).Scan(&cardsCount); err != nil {
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

func StartCardBulkSyncLoop(db *sql.DB, interval time.Duration, logger *log.Logger) {
	if interval <= 0 {
		interval = defaultBulkSyncInterval
	}
	if logger == nil {
		logger = log.Default()
	}
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
			result, err := SyncCardsFromScryfallBulk(syncCtx, db)
			cancelSync()
			if err != nil {
				if errors.Is(err, ErrCardSyncInProgress) {
					logger.Printf("cards bulk sync skipped: already running")
					continue
				}
				logger.Printf("cards bulk sync failed: %v", err)
				continue
			}
			logger.Printf("cards bulk sync complete: %d cards (source updated %s)", result.ImportedCards, result.SourceUpdatedAt.UTC().Format(time.RFC3339))
		}
	}()
}
