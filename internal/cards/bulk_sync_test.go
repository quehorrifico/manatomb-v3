package cards

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDecodeBulkDescriptorAcceptsCurrentJSONLDownloadURI(t *testing.T) {
	t.Parallel()

	descriptor, err := decodeBulkDescriptor(strings.NewReader(`{
		"object":"list",
		"data":[{
			"object":"bulk_data",
			"type":"oracle_cards",
			"updated_at":"2026-08-12T09:01:56.331+00:00",
			"jsonl_download_uri":"https://data.scryfall.io/oracle-cards/current.jsonl.gz"
		}]
	}`), "oracle_cards")
	if err != nil {
		t.Fatalf("decodeBulkDescriptor() error = %v", err)
	}
	if descriptor.DownloadURI != "https://data.scryfall.io/oracle-cards/current.jsonl.gz" {
		t.Fatalf("DownloadURI = %q, want current JSONL URI", descriptor.DownloadURI)
	}
}

func TestDecodeBulkDescriptorRetainsLegacyDownloadURI(t *testing.T) {
	t.Parallel()

	descriptor, err := decodeBulkDescriptor(strings.NewReader(`{
		"data":[{
			"type":"all_cards",
			"updated_at":"2026-08-12T09:18:35Z",
			"download_uri":"https://data.scryfall.io/all-cards/legacy.json"
		}]
	}`), "all_cards")
	if err != nil {
		t.Fatalf("decodeBulkDescriptor() error = %v", err)
	}
	if descriptor.DownloadURI != "https://data.scryfall.io/all-cards/legacy.json" {
		t.Fatalf("DownloadURI = %q, want legacy URI", descriptor.DownloadURI)
	}
}

func TestDecodeOracleRowsAcceptsGzipJSONLines(t *testing.T) {
	t.Parallel()

	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	for _, line := range []string{
		`{"id":"11111111-1111-1111-1111-111111111111","oracle_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","name":"First Card","lang":"en","games":["paper"],"set":"tst","set_name":"Test Set","legalities":{"commander":"legal"}}`,
		`{"id":"22222222-2222-2222-2222-222222222222","oracle_id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","name":"Second Card","lang":"en","games":["paper"],"set":"tst","set_name":"Test Set","legalities":{"vintage":"restricted"}}`,
	} {
		if _, err := gzipWriter.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("write gzip fixture: %v", err)
		}
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip fixture: %v", err)
	}

	decoder, cleanup, err := newBulkJSONDecoderFromReader(bytes.NewReader(compressed.Bytes()))
	if err != nil {
		t.Fatalf("newBulkJSONDecoderFromReader() error = %v", err)
	}
	defer cleanup()
	rows, err := decodeOracleRowsFromDecoder(decoder, 0)
	if err != nil {
		t.Fatalf("decodeOracleRows() error = %v", err)
	}
	if len(rows) != 2 || rows[0].Name != "First Card" || rows[1].Name != "Second Card" {
		t.Fatalf("decoded rows = %#v, want both JSON Lines records", rows)
	}
}

func TestDecodeOracleRowsRetainsLegacyJSONArraySupport(t *testing.T) {
	t.Parallel()

	decoder, cleanup, err := newBulkJSONDecoderFromReader(strings.NewReader(
		`[{"id":"11111111-1111-1111-1111-111111111111","oracle_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","name":"Legacy Card","lang":"en","games":["paper"]}]`,
	))
	if err != nil {
		t.Fatalf("newBulkJSONDecoderFromReader() error = %v", err)
	}
	defer cleanup()
	rows, err := decodeOracleRowsFromDecoder(decoder, 0)
	if err != nil {
		t.Fatalf("decodeOracleRows() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "Legacy Card" {
		t.Fatalf("decoded rows = %#v, want legacy array record", rows)
	}
}

func TestShouldIncludePrintAllowsOnlyHobbitEternalDwarvishPrints(t *testing.T) {
	t.Parallel()

	base := scryfallCard{
		ID:       "11111111-1111-1111-1111-111111111111",
		OracleID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Name:     "Dwarven Warriors",
		Lang:     "dw",
		Set:      "hoc",
		Games:    []string{"paper"},
	}

	for _, collectorNumber := range []string{"93", "94", "95", "96", "97"} {
		card := base
		card.CollectorNumber = collectorNumber
		if !shouldIncludePrint(card, normalizeScryfallCard(card)) {
			t.Fatalf("shouldIncludePrint() rejected HOC %s in Dwarvish", collectorNumber)
		}
	}

	for _, test := range []struct {
		name            string
		lang            string
		setCode         string
		collectorNumber string
	}{
		{name: "other HOC collector number", lang: "dw", setCode: "hoc", collectorNumber: "92"},
		{name: "later HOC collector number", lang: "dw", setCode: "hoc", collectorNumber: "98"},
		{name: "other set", lang: "dw", setCode: "hob", collectorNumber: "93"},
		{name: "other non-English language", lang: "de", setCode: "hoc", collectorNumber: "93"},
	} {
		t.Run(test.name, func(t *testing.T) {
			card := base
			card.Lang = test.lang
			card.Set = test.setCode
			card.CollectorNumber = test.collectorNumber
			if shouldIncludePrint(card, normalizeScryfallCard(card)) {
				t.Fatalf("shouldIncludePrint() accepted lang=%q set=%q collector=%q", test.lang, test.setCode, test.collectorNumber)
			}
		})
	}
}

func TestDecodePrintRowsStoresActualDwarvishLanguage(t *testing.T) {
	t.Parallel()

	decoder, cleanup, err := newBulkJSONDecoderFromReader(strings.NewReader(`[
		{"id":"11111111-1111-1111-1111-111111111111","oracle_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","name":"English Card","lang":"en","games":["paper"],"set":"hob","collector_number":"1"},
		{"id":"22222222-2222-2222-2222-222222222222","oracle_id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","name":"Dwarven Warriors","lang":"dw","games":["paper"],"set":"hoc","collector_number":"93"},
		{"id":"33333333-3333-3333-3333-333333333333","oracle_id":"cccccccc-cccc-cccc-cccc-cccccccccccc","name":"Excluded Dwarvish Card","lang":"dw","games":["paper"],"set":"hoc","collector_number":"98"}
	]`))
	if err != nil {
		t.Fatalf("newBulkJSONDecoderFromReader() error = %v", err)
	}
	defer cleanup()

	rows, err := decodePrintRowsFromDecoder(decoder, 0)
	if err != nil {
		t.Fatalf("decodePrintRowsFromDecoder() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}

	for _, row := range rows {
		if row.CollectorNumber == "93" {
			if row.Lang != "dw" {
				t.Fatalf("Dwarvish row language = %q, want dw", row.Lang)
			}
			return
		}
	}
	t.Fatal("Dwarvish HOC 93 row was not decoded")
}

func TestParseRetryAfterSeconds(t *testing.T) {
	t.Parallel()

	got, ok := parseRetryAfter("120", time.Date(2026, time.April, 8, 12, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("parseRetryAfter() ok = false, want true")
	}
	if got != 120*time.Second {
		t.Fatalf("parseRetryAfter() = %v, want %v", got, 120*time.Second)
	}
}

func TestScryfallStatusErrorMaintenanceHTML(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header: http.Header{
			"Content-Type": []string{"text/html; charset=utf-8"},
		},
		Body: io.NopCloser(strings.NewReader(`<!DOCTYPE html><title>Offline for Maintenance</title>`)),
	}

	err := scryfallStatusError("bulk descriptor list request", resp)
	if err == nil {
		t.Fatal("scryfallStatusError() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "temporarily offline for maintenance") {
		t.Fatalf("scryfallStatusError() = %q, want maintenance message", err.Error())
	}
	if strings.Contains(strings.ToLower(err.Error()), "<html") {
		t.Fatalf("scryfallStatusError() leaked HTML body: %q", err.Error())
	}
}

func TestNormalizeScryfallCardPreservesFullLegalities(t *testing.T) {
	t.Parallel()

	card := normalizeScryfallCard(scryfallCard{
		Legalities: map[string]string{
			"Commander": "LEGAL",
			"vintage":   "restricted",
			"standard":  "not_legal",
			"legacy":    "banned",
		},
	})
	for format, want := range map[string]string{
		"commander": "legal",
		"vintage":   "restricted",
		"standard":  "not_legal",
		"legacy":    "banned",
	} {
		if got := card.Legalities[format]; got != want {
			t.Fatalf("normalizeScryfallCard().Legalities[%q] = %q, want %q", format, got, want)
		}
	}
}

func TestCardSyncDataVersionIncludesLegalityMap(t *testing.T) {
	t.Parallel()

	if cardSyncDataVersion < 4 {
		t.Fatalf("cardSyncDataVersion = %d, want at least 4 for legality-map backfill", cardSyncDataVersion)
	}
}

func TestCardSyncDataVersionIncludesFinishPrices(t *testing.T) {
	t.Parallel()

	if cardSyncDataVersion < 6 {
		t.Fatalf("cardSyncDataVersion = %d, want at least 6 for finish-price backfill", cardSyncDataVersion)
	}
}

func TestCardSyncDataVersionIncludesHobbitDwarvishPrints(t *testing.T) {
	t.Parallel()

	if cardSyncDataVersion < 7 {
		t.Fatalf("cardSyncDataVersion = %d, want at least 7 for Hobbit Dwarvish-print backfill", cardSyncDataVersion)
	}
}

func TestStaleCardPrintCleanupPreservesDeckSelections(t *testing.T) {
	t.Parallel()

	for _, reference := range []string{
		"dc.preferred_print_id = cp.scryfall_id",
		"d.commander_print_id = cp.scryfall_id",
	} {
		if !strings.Contains(staleCardPrintDeleteSQL, reference) {
			t.Fatalf("stale card-print cleanup does not preserve %q", reference)
		}
	}
}

func TestStaleOracleCleanupPreservesDeckCommanderSelection(t *testing.T) {
	t.Parallel()

	for _, reference := range []string{
		"cp.scryfall_id = d.commander_print_id",
		"cp.oracle_id = oc.oracle_id",
	} {
		if !strings.Contains(staleOracleCardDeleteSQL, reference) {
			t.Fatalf("stale oracle cleanup does not preserve %q", reference)
		}
	}
}
