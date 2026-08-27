package cards

import (
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

const (
	scryfallBulkListURL     = "https://api.scryfall.com/bulk-data"
	scryfallAPIAccept       = "application/json;q=0.9,*/*;q=0.8"
	scryfallAPIUserAgent    = "ManaTomb/1.0 (+https://github.com/zeusborrego/manatomb-v3)"
	scryfallHeaderTimeout   = 90 * time.Second
	scryfallRetryAttempts   = 4
	scryfallRetryDelay      = 3 * time.Second
	scryfallMaxRetryDelay   = 30 * time.Second
	defaultBulkSyncInterval = 24 * time.Hour
	cardSyncAdvisoryLockKey = int64(91342817)
	cardSyncDataVersion     = 7
	staleCardPrintDeleteSQL = `
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
		AND NOT EXISTS (
			SELECT 1
			FROM decks d
			WHERE d.commander_print_id = cp.scryfall_id
		)
	`
	staleOracleCardDeleteSQL = `
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
		AND NOT EXISTS (
			SELECT 1
			FROM decks d
			JOIN card_prints cp
			  ON cp.scryfall_id = d.commander_print_id
			WHERE cp.oracle_id = oc.oracle_id
		)
	`
)

var ErrCardSyncInProgress = errors.New("card sync already running")

var scryfallHTTPClient = newScryfallHTTPClient()

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
	Object           string    `json:"object"`
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	UpdatedAt        time.Time `json:"updated_at"`
	DownloadURI      string    `json:"download_uri"`
	JSONLDownloadURI string    `json:"jsonl_download_uri"`
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
	FlavorText           string
	Colors               []string
	ColorIdentity        []string
	PowerText            string
	ToughnessText        string
	LoyaltyText          string
	PowerValue           sql.NullFloat64
	ToughnessValue       sql.NullFloat64
	LoyaltyValue         sql.NullFloat64
	Layout               string
	CardFacesJSON        string
	AllPartsJSON         string
	LegalAnywhere        bool
	CommanderLegal       bool
	LegalitiesJSON       string
	IsCommanderCandidate bool
	EDHRecRank           int
}

type printBulkRow struct {
	ScryfallID       string
	OracleID         string
	Name             string
	SetCode          string
	SetType          string
	CollectorNumber  string
	Lang             string
	ReleasedAt       sql.NullTime
	FlavorText       string
	ImageURIsJSON    string
	ImageURI         string
	CardFacesJSON    string
	FinishesJSON     string
	FrameEffectsJSON string
	PromoTypesJSON   string
	SetName          string
	Rarity           string
	BorderColor      string
	Frame            string
	SecurityStamp    string
	FullArt          bool
	Textless         bool
	Booster          bool
	Digital          bool
	Variation        bool
	Artist           string
	PriceUSD         string
	PriceUSDNonfoil  string
	PriceUSDFoil     string
	PriceUSDEtched   string
	ScryfallURI      string
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

func stringSliceJSON(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	b, err := json.Marshal(nonNilStrings(values))
	if err != nil {
		return "[]"
	}
	return string(b)
}

func stringMapJSON(values map[string]string) string {
	if len(values) == 0 {
		return "{}"
	}
	b, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func parseNumericCardStat(raw string) sql.NullFloat64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sql.NullFloat64{}
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: value, Valid: true}
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

func legalAnywhere(legalities map[string]string) bool {
	for _, status := range legalities {
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "legal", "restricted":
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

func shouldIncludeOracleCard(sc scryfallCard, c Card) bool {
	if strings.TrimSpace(c.OracleID) == "" || strings.TrimSpace(c.Name) == "" {
		return false
	}
	if !supportsPaper(sc.Games) {
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
	if !strings.EqualFold(strings.TrimSpace(sc.Lang), "en") && !isHobbitEternalDwarvishPrint(sc) {
		return false
	}
	return true
}

func isHobbitEternalDwarvishPrint(sc scryfallCard) bool {
	if !strings.EqualFold(strings.TrimSpace(sc.Lang), "dw") ||
		!strings.EqualFold(strings.TrimSpace(sc.Set), "hoc") {
		return false
	}

	collectorNumber, err := strconv.Atoi(strings.TrimSpace(sc.CollectorNumber))
	return err == nil && collectorNumber >= 93 && collectorNumber <= 97
}

func newScryfallRequest(ctx context.Context, method, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", scryfallAPIUserAgent)
	req.Header.Set("Accept", scryfallAPIAccept)
	return req, nil
}

func newScryfallHTTPClient() *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok || base == nil {
		return &http.Client{Timeout: scryfallHeaderTimeout}
	}

	transport := base.Clone()
	transport.ResponseHeaderTimeout = scryfallHeaderTimeout
	return &http.Client{Transport: transport}
}

func shouldRetryScryfallStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	if secs, err := time.ParseDuration(value + "s"); err == nil && secs > 0 {
		return secs, true
	}

	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delay := time.Until(when)
	if !now.IsZero() {
		delay = when.Sub(now)
	}
	if delay <= 0 {
		return 0, false
	}
	return delay, true
}

func retryDelayForAttempt(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if retryAfter, ok := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
			if retryAfter > scryfallMaxRetryDelay {
				return scryfallMaxRetryDelay
			}
			return retryAfter
		}
	}

	delay := time.Duration(attempt) * scryfallRetryDelay
	if delay > scryfallMaxRetryDelay {
		return scryfallMaxRetryDelay
	}
	return delay
}

func doScryfallRequest(ctx context.Context, method, rawURL, label string) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= scryfallRetryAttempts; attempt++ {
		req, err := newScryfallRequest(ctx, method, rawURL)
		if err != nil {
			return nil, err
		}

		resp, err := scryfallHTTPClient.Do(req)
		if err == nil && !shouldRetryScryfallStatus(resp.StatusCode) {
			return resp, nil
		}

		if err == nil {
			if attempt == scryfallRetryAttempts {
				return resp, nil
			}
			lastErr = scryfallStatusError(label, resp)
			resp.Body.Close()
		} else {
			lastErr = err
		}
		if ctx.Err() != nil || attempt == scryfallRetryAttempts {
			break
		}

		delay := retryDelayForAttempt(resp, attempt)
		log.Printf(
			"cards sync phase: retrying %s (%d/%d) after error: %v",
			label,
			attempt+1,
			scryfallRetryAttempts,
			lastErr,
		)

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, lastErr
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func scryfallStatusError(label string, resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("%s failed: empty response", label)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	msg := strings.TrimSpace(string(body))
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))

	if resp.StatusCode == http.StatusServiceUnavailable {
		lowerMsg := strings.ToLower(msg)
		if strings.Contains(lowerMsg, "offline for maintenance") || strings.Contains(lowerMsg, "errors.scryfall.com/503h.html") {
			if retryAfter, ok := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
				return fmt.Errorf(
					"%s failed: Scryfall is temporarily offline for maintenance (503); retry in about %s",
					label,
					retryAfter.Round(time.Second),
				)
			}
			return fmt.Errorf("%s failed: Scryfall is temporarily offline for maintenance (503); try again later", label)
		}
	}

	if msg == "" {
		return fmt.Errorf("%s failed: status %d", label, resp.StatusCode)
	}
	if strings.Contains(contentType, "text/html") {
		return fmt.Errorf("%s failed: status %d (HTML error response omitted)", label, resp.StatusCode)
	}
	return fmt.Errorf("%s failed: status %d: %s", label, resp.StatusCode, msg)
}

func fetchBulkDescriptor(ctx context.Context, wantedType string) (scryfallBulkDescriptor, error) {
	resp, err := doScryfallRequest(ctx, http.MethodGet, scryfallBulkListURL, "bulk descriptor list request")
	if err != nil {
		return scryfallBulkDescriptor{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return scryfallBulkDescriptor{}, scryfallStatusError("bulk descriptor list request", resp)
	}

	return decodeBulkDescriptor(resp.Body, wantedType)
}

func decodeBulkDescriptor(reader io.Reader, wantedType string) (scryfallBulkDescriptor, error) {
	var list scryfallBulkListResponse
	if err := json.NewDecoder(reader).Decode(&list); err != nil {
		return scryfallBulkDescriptor{}, err
	}
	wantedType = strings.TrimSpace(wantedType)
	for _, descriptor := range list.Data {
		if strings.EqualFold(strings.TrimSpace(descriptor.Type), wantedType) {
			descriptor.DownloadURI = strings.TrimSpace(descriptor.DownloadURI)
			if descriptor.DownloadURI == "" {
				descriptor.DownloadURI = strings.TrimSpace(descriptor.JSONLDownloadURI)
			}
			if descriptor.DownloadURI == "" {
				return scryfallBulkDescriptor{}, fmt.Errorf(
					"missing download_uri and jsonl_download_uri for bulk type %s",
					wantedType,
				)
			}
			return descriptor, nil
		}
	}
	return scryfallBulkDescriptor{}, fmt.Errorf("bulk descriptor not found for type %s", wantedType)
}

type bulkJSONDecoder struct {
	decoder *json.Decoder
	array   bool
}

func (d *bulkJSONDecoder) DecodeNext(destination any) (bool, error) {
	if d.array && !d.decoder.More() {
		endToken, err := d.decoder.Token()
		if err != nil {
			return false, err
		}
		if delim, ok := endToken.(json.Delim); !ok || delim != ']' {
			return false, errors.New("unexpected bulk payload format: expected closing JSON array")
		}
		return false, nil
	}

	if err := d.decoder.Decode(destination); err != nil {
		if !d.array && errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func newBulkJSONDecoder(ctx context.Context, downloadURI string) (*bulkJSONDecoder, func(), error) {
	resp, err := doScryfallRequest(ctx, http.MethodGet, downloadURI, "bulk download request")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = resp.Body.Close() }
	if resp.StatusCode != http.StatusOK {
		cleanup()
		return nil, nil, scryfallStatusError("bulk download request", resp)
	}

	bulkDecoder, decoderCleanup, err := newBulkJSONDecoderFromReader(resp.Body)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	cleanup = func() {
		decoderCleanup()
		_ = resp.Body.Close()
	}
	return bulkDecoder, cleanup, nil
}

func newBulkJSONDecoderFromReader(payload io.Reader) (*bulkJSONDecoder, func(), error) {
	cleanup := func() {}
	buffered := bufio.NewReader(payload)
	reader := io.Reader(buffered)
	var gzipReader *gzip.Reader
	if header, peekErr := buffered.Peek(2); peekErr == nil && len(header) == 2 && header[0] == 0x1f && header[1] == 0x8b {
		var err error
		gzipReader, err = gzip.NewReader(buffered)
		if err != nil {
			return nil, nil, fmt.Errorf("open gzip bulk payload: %w", err)
		}
		reader = gzipReader
		cleanup = func() { _ = gzipReader.Close() }
	}

	jsonReader := bufio.NewReader(reader)
	firstByte, err := firstBulkJSONByte(jsonReader)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("read bulk payload: %w", err)
	}
	if firstByte != '[' && firstByte != '{' {
		cleanup()
		return nil, nil, errors.New("unexpected bulk payload format: expected JSON array or JSON Lines objects")
	}

	decoder := json.NewDecoder(jsonReader)
	bulkDecoder := &bulkJSONDecoder{decoder: decoder, array: firstByte == '['}
	if bulkDecoder.array {
		startToken, tokenErr := decoder.Token()
		if tokenErr != nil {
			cleanup()
			return nil, nil, tokenErr
		}
		if delim, ok := startToken.(json.Delim); !ok || delim != '[' {
			cleanup()
			return nil, nil, errors.New("unexpected bulk payload format: expected JSON array")
		}
	}
	return bulkDecoder, cleanup, nil
}

func firstBulkJSONByte(reader *bufio.Reader) (byte, error) {
	for {
		value, err := reader.Peek(1)
		if err != nil {
			return 0, err
		}
		switch value[0] {
		case ' ', '\t', '\r', '\n':
			if _, err := reader.ReadByte(); err != nil {
				return 0, err
			}
		default:
			return value[0], nil
		}
	}
}

func decodeOracleRows(ctx context.Context, downloadURI string, maxRows int) ([]oracleBulkRow, error) {
	dec, cleanup, err := newBulkJSONDecoder(ctx, downloadURI)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return decodeOracleRowsFromDecoder(dec, maxRows)
}

func decodeOracleRowsFromDecoder(dec *bulkJSONDecoder, maxRows int) ([]oracleBulkRow, error) {
	rows := make([]oracleBulkRow, 0, 40000)
	for {
		var raw scryfallCard
		more, err := dec.DecodeNext(&raw)
		if err != nil {
			return nil, err
		}
		if !more {
			break
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
		allPartsJSON := "[]"
		if len(raw.AllParts) > 0 {
			if b, err := json.Marshal(raw.AllParts); err == nil {
				allPartsJSON = string(b)
			}
		}

		rows = append(rows, oracleBulkRow{
			OracleID:             card.OracleID,
			Name:                 card.Name,
			ManaCost:             card.ManaCost,
			CMC:                  card.CMC,
			TypeLine:             card.TypeLine,
			OracleText:           card.OracleText,
			FlavorText:           card.FlavorText,
			Colors:               nonNilStrings(card.Colors),
			ColorIdentity:        nonNilStrings(card.ColorIdentity),
			PowerText:            strings.TrimSpace(card.Power),
			ToughnessText:        strings.TrimSpace(card.Toughness),
			LoyaltyText:          strings.TrimSpace(card.Loyalty),
			PowerValue:           parseNumericCardStat(card.Power),
			ToughnessValue:       parseNumericCardStat(card.Toughness),
			LoyaltyValue:         parseNumericCardStat(card.Loyalty),
			Layout:               card.Layout,
			CardFacesJSON:        facesJSON,
			AllPartsJSON:         allPartsJSON,
			LegalAnywhere:        legalAnywhere(raw.Legalities),
			CommanderLegal:       card.CommanderLegal,
			LegalitiesJSON:       stringMapJSON(card.Legalities),
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
	return decodePrintRowsFromDecoder(dec, maxRows)
}

func decodePrintRowsFromDecoder(dec *bulkJSONDecoder, maxRows int) ([]printBulkRow, error) {
	rows := make([]printBulkRow, 0, 50000)
	for {
		var raw scryfallCard
		more, err := dec.DecodeNext(&raw)
		if err != nil {
			return nil, err
		}
		if !more {
			break
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
		finishesJSON := stringSliceJSON(raw.Finishes)
		frameEffectsJSON := stringSliceJSON(raw.FrameEffects)
		promoTypesJSON := stringSliceJSON(raw.PromoTypes)

		rows = append(rows, printBulkRow{
			ScryfallID:      card.ID,
			OracleID:        card.OracleID,
			Name:            card.Name,
			SetCode:         strings.ToLower(strings.TrimSpace(card.SetCode)),
			SetType:         strings.ToLower(strings.TrimSpace(raw.SetType)),
			CollectorNumber: strings.TrimSpace(raw.CollectorNumber),
			Lang:            strings.TrimSpace(card.Lang),
			ReleasedAt: sql.NullTime{
				Time:  releasedAt,
				Valid: hasRelease,
			},
			FlavorText:       card.FlavorText,
			ImageURIsJSON:    imageURIsJSON,
			ImageURI:         card.ImageURI,
			CardFacesJSON:    cardFacesJSON,
			FinishesJSON:     finishesJSON,
			FrameEffectsJSON: frameEffectsJSON,
			PromoTypesJSON:   promoTypesJSON,
			SetName:          card.SetName,
			Rarity:           strings.TrimSpace(raw.Rarity),
			BorderColor:      strings.TrimSpace(raw.BorderColor),
			Frame:            strings.TrimSpace(raw.Frame),
			SecurityStamp:    strings.TrimSpace(raw.SecurityStamp),
			FullArt:          raw.FullArt,
			Textless:         raw.Textless,
			Booster:          raw.Booster,
			Digital:          raw.Digital,
			Variation:        raw.Variation,
			Artist:           card.Artist,
			PriceUSD:         card.PriceUSD,
			PriceUSDNonfoil:  strings.TrimSpace(raw.Prices.USD),
			PriceUSDFoil:     strings.TrimSpace(raw.Prices.USDFoil),
			PriceUSDEtched:   strings.TrimSpace(raw.Prices.USDEtched),
			ScryfallURI:      card.ScryfallURI,
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
	deleteStale bool,
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
			flavor_text TEXT,
			colors TEXT[] NOT NULL,
			color_identity TEXT[] NOT NULL,
			power_text TEXT,
			toughness_text TEXT,
			loyalty_text TEXT,
			power_value DOUBLE PRECISION,
			toughness_value DOUBLE PRECISION,
			loyalty_value DOUBLE PRECISION,
			layout TEXT,
			card_faces JSONB,
			all_parts JSONB NOT NULL DEFAULT '[]'::jsonb,
			legal_anywhere BOOLEAN,
			commander_legal BOOLEAN,
			legalities JSONB,
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
			set_type TEXT NOT NULL,
			collector_number TEXT NOT NULL,
			lang TEXT NOT NULL,
			released_at DATE,
			flavor_text TEXT,
			image_uris JSONB,
			image_uri TEXT,
			card_faces_json JSONB,
			finishes_json JSONB,
			frame_effects_json JSONB,
			promo_types_json JSONB,
			set_name TEXT,
			rarity TEXT,
			border_color TEXT,
			frame TEXT,
			security_stamp TEXT,
			full_art BOOLEAN,
			textless BOOLEAN,
			booster BOOLEAN,
			digital BOOLEAN,
			variation BOOLEAN,
			artist TEXT,
			price_usd TEXT,
			price_usd_nonfoil TEXT,
			price_usd_foil TEXT,
			price_usd_etched TEXT,
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
		"flavor_text",
		"colors",
		"color_identity",
		"power_text",
		"toughness_text",
		"loyalty_text",
		"power_value",
		"toughness_value",
		"loyalty_value",
		"layout",
		"card_faces",
		"all_parts",
		"legal_anywhere",
		"commander_legal",
		"legalities",
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
			row.FlavorText,
			pq.Array(nonNilStrings(row.Colors)),
			pq.Array(nonNilStrings(row.ColorIdentity)),
			row.PowerText,
			row.ToughnessText,
			row.LoyaltyText,
			row.PowerValue,
			row.ToughnessValue,
			row.LoyaltyValue,
			row.Layout,
			row.CardFacesJSON,
			row.AllPartsJSON,
			row.LegalAnywhere,
			row.CommanderLegal,
			row.LegalitiesJSON,
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
		"set_type",
		"collector_number",
		"lang",
		"released_at",
		"flavor_text",
		"image_uris",
		"image_uri",
		"card_faces_json",
		"finishes_json",
		"frame_effects_json",
		"promo_types_json",
		"set_name",
		"rarity",
		"border_color",
		"frame",
		"security_stamp",
		"full_art",
		"textless",
		"booster",
		"digital",
		"variation",
		"artist",
		"price_usd",
		"price_usd_nonfoil",
		"price_usd_foil",
		"price_usd_etched",
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
			row.SetType,
			row.CollectorNumber,
			row.Lang,
			row.ReleasedAt,
			row.FlavorText,
			row.ImageURIsJSON,
			row.ImageURI,
			row.CardFacesJSON,
			row.FinishesJSON,
			row.FrameEffectsJSON,
			row.PromoTypesJSON,
			row.SetName,
			row.Rarity,
			row.BorderColor,
			row.Frame,
			row.SecurityStamp,
			row.FullArt,
			row.Textless,
			row.Booster,
			row.Digital,
			row.Variation,
			row.Artist,
			row.PriceUSD,
			row.PriceUSDNonfoil,
			row.PriceUSDFoil,
			row.PriceUSDEtched,
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
			flavor_text,
			colors,
			color_identity,
			power_text,
			toughness_text,
			loyalty_text,
			power_value,
			toughness_value,
			loyalty_value,
			layout,
			card_faces,
			all_parts,
			legal_anywhere,
			commander_legal,
			legalities,
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
			s.flavor_text,
			s.colors,
			s.color_identity,
			s.power_text,
			s.toughness_text,
			s.loyalty_text,
			s.power_value,
			s.toughness_value,
			s.loyalty_value,
			s.layout,
			s.card_faces,
			s.all_parts,
			s.legal_anywhere,
			s.commander_legal,
			s.legalities,
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
			flavor_text = EXCLUDED.flavor_text,
			colors = EXCLUDED.colors,
			color_identity = EXCLUDED.color_identity,
			power_text = EXCLUDED.power_text,
			toughness_text = EXCLUDED.toughness_text,
			loyalty_text = EXCLUDED.loyalty_text,
			power_value = EXCLUDED.power_value,
			toughness_value = EXCLUDED.toughness_value,
			loyalty_value = EXCLUDED.loyalty_value,
			layout = EXCLUDED.layout,
			card_faces = EXCLUDED.card_faces,
			all_parts = EXCLUDED.all_parts,
			legal_anywhere = EXCLUDED.legal_anywhere,
			commander_legal = EXCLUDED.commander_legal,
			legalities = EXCLUDED.legalities,
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
			set_type,
			collector_number,
			lang,
			released_at,
			flavor_text,
			image_uris,
			image_uri,
			card_faces_json,
			finishes_json,
			frame_effects_json,
			promo_types_json,
			set_name,
			rarity,
			border_color,
			frame,
			security_stamp,
			full_art,
			textless,
			booster,
			digital,
			variation,
			artist,
			price_usd,
			price_usd_nonfoil,
			price_usd_foil,
			price_usd_etched,
			scryfall_uri
		)
		SELECT
			s.scryfall_id,
			s.oracle_id,
			s.name,
			s.set_code,
			s.set_type,
			s.collector_number,
			s.lang,
			s.released_at,
			s.flavor_text,
			s.image_uris,
			s.image_uri,
			s.card_faces_json,
			s.finishes_json,
			s.frame_effects_json,
			s.promo_types_json,
			s.set_name,
			s.rarity,
			s.border_color,
			s.frame,
			s.security_stamp,
			s.full_art,
			s.textless,
			s.booster,
			s.digital,
			s.variation,
			s.artist,
			s.price_usd,
			s.price_usd_nonfoil,
			s.price_usd_foil,
			s.price_usd_etched,
			s.scryfall_uri
		FROM card_prints_sync_stage s
		ON CONFLICT (scryfall_id) DO UPDATE
		SET
			oracle_id = EXCLUDED.oracle_id,
			name = EXCLUDED.name,
			set_code = EXCLUDED.set_code,
			set_type = EXCLUDED.set_type,
			collector_number = EXCLUDED.collector_number,
			lang = EXCLUDED.lang,
			released_at = EXCLUDED.released_at,
			flavor_text = EXCLUDED.flavor_text,
			image_uris = EXCLUDED.image_uris,
			image_uri = EXCLUDED.image_uri,
			card_faces_json = EXCLUDED.card_faces_json,
			finishes_json = EXCLUDED.finishes_json,
			frame_effects_json = EXCLUDED.frame_effects_json,
			promo_types_json = EXCLUDED.promo_types_json,
			set_name = EXCLUDED.set_name,
			rarity = EXCLUDED.rarity,
			border_color = EXCLUDED.border_color,
			frame = EXCLUDED.frame,
			security_stamp = EXCLUDED.security_stamp,
			full_art = EXCLUDED.full_art,
			textless = EXCLUDED.textless,
			booster = EXCLUDED.booster,
			digital = EXCLUDED.digital,
			variation = EXCLUDED.variation,
			artist = EXCLUDED.artist,
			price_usd = EXCLUDED.price_usd,
			price_usd_nonfoil = EXCLUDED.price_usd_nonfoil,
			price_usd_foil = EXCLUDED.price_usd_foil,
			price_usd_etched = EXCLUDED.price_usd_etched,
			scryfall_uri = EXCLUDED.scryfall_uri
	`)
	if err != nil {
		return err
	}
	printUpserted, _ := printUpsertRes.RowsAffected()
	logger.Printf("cards sync phase: upserted printings=%d", printUpserted)

	if deleteStale {
		// Stale printings are removed unless explicitly selected by a deck card
		// or by the deck's commander slot.
		printDeleteRes, err := tx.ExecContext(ctx, staleCardPrintDeleteSQL)
		if err != nil {
			return err
		}
		printDeleted, _ := printDeleteRes.RowsAffected()
		logger.Printf("cards sync phase: deleted stale printings=%d", printDeleted)

		// Stale canonical cards are removed unless still referenced by decks.
		oracleDeleteRes, err := tx.ExecContext(ctx, staleOracleCardDeleteSQL)
		if err != nil {
			return err
		}
		oracleDeleted, _ := oracleDeleteRes.RowsAffected()
		logger.Printf("cards sync phase: deleted stale oracle cards=%d", oracleDeleted)
	} else {
		logger.Printf("cards sync phase: limited mode preserves rows outside the sampled data")
	}

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
				source_updated_at = CASE WHEN $3 THEN $1 ELSE source_updated_at END,
				last_error = '',
				card_count = (SELECT COUNT(*) FROM oracle_cards),
				data_version = CASE WHEN $3 THEN $2 ELSE data_version END
			WHERE id = 1
		`, sourceUpdatedAt, cardSyncDataVersion, deleteStale); err != nil {
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

		if err := applyBulkRows(ctx, db, oracleRows, printRows, sourceUpdatedAt, options.MaxRows == 0); err != nil {
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

	var (
		lastSuccess sql.NullTime
		dataVersion int
	)
	err := db.QueryRowContext(ctx, `
		SELECT last_success_at, COALESCE(data_version, 0)
		FROM card_sync_state
		WHERE id = 1
	`).Scan(&lastSuccess, &dataVersion)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if dataVersion < cardSyncDataVersion {
		return true, nil
	}
	if !lastSuccess.Valid {
		return true, nil
	}
	return time.Since(lastSuccess.Time) >= maxAge, nil
}

func runCardBulkSyncOnce(db *sql.DB, logger *log.Logger, options CardBulkSyncOptions, reason string) {
	if logger == nil {
		logger = log.Default()
	}
	syncCtx, cancelSync := context.WithTimeout(context.Background(), 2*time.Hour)
	result, err := SyncCardsFromScryfallBulk(syncCtx, db, options)
	cancelSync()
	if err != nil {
		if errors.Is(err, ErrCardSyncInProgress) {
			logger.Printf("cards bulk sync skipped (%s): already running", reason)
			return
		}
		logger.Printf("cards bulk sync failed (%s): %v", reason, err)
		return
	}
	logger.Printf(
		"cards bulk sync complete (%s): %d canonical cards, %d printings (source updated %s)",
		reason,
		result.ImportedCards,
		result.ImportedPrintings,
		result.SourceUpdatedAt.UTC().Format(time.RFC3339),
	)
}

func StartCardBulkSyncLoop(db *sql.DB, interval time.Duration, runOnStart bool, logger *log.Logger, options CardBulkSyncOptions) {
	if interval <= 0 {
		interval = defaultBulkSyncInterval
	}
	if logger == nil {
		logger = log.Default()
	}
	options = normalizeCardBulkSyncOptions(options)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		if runOnStart {
			logger.Printf("cards bulk sync: starting immediate sync because startup sync was requested")
			runCardBulkSyncOnce(db, logger, options, "startup")
		}

		for range ticker.C {
			runCardBulkSyncOnce(db, logger, options, "scheduled")
		}
	}()
}
