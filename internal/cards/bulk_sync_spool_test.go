package cards

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const spoolFixture = `[
 {"id":"a1","oracle_id":"a","name":"A Card","lang":"en","games":["paper"],"released_at":"2026-01-01","prices":{"usd":"1.23"},"image_uris":{"normal":"https://example.test/a.jpg"},"finishes":["nonfoil","foil"]},
 {"id":"a2","oracle_id":"a","name":"A Card","lang":"en","games":["paper"],"card_faces":[{"name":"Front","oracle_text":"Front rules","image_uris":{"normal":"https://example.test/front.jpg"}},{"name":"Back","oracle_text":"Back rules"}]},
 {"id":"b1","oracle_id":"b","name":"B Card","lang":"en","games":["paper"]},
 {"id":"digital","oracle_id":"c","name":"Digital Card","lang":"en","games":["arena"]},
 {"id":"orphan","oracle_id":"missing","name":"Orphan","lang":"en","games":["paper"]}
]`

func TestPaperPrintSpoolPreservesVersionsAndMetadata(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	decode := func() *bulkJSONDecoder {
		d, cleanup, err := newBulkJSONDecoderFromReader(strings.NewReader(spoolFixture))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(cleanup)
		return d
	}
	oracles := []oracleBulkRow{{OracleID: "a", Name: "A Card"}, {OracleID: "b", Name: "B Card"}, {OracleID: "c", Name: "Digital Card"}}
	spool, err := spoolPaperPrints(context.Background(), decode(), oracles)
	if err != nil {
		t.Fatal(err)
	}
	defer spool.close()
	if spool.count != 3 {
		t.Fatalf("stored %d rows, want 3", spool.count)
	}
	legacy, err := decodePrintRowsFromDecoder(decode(), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{0, 1} {
		wantOracles, wantPrints := selectPaperBulkRows(oracles, legacy, limit)
		gotOracles, ids := selectPaperOracles(oracles, spool.paperIDs, limit)
		got := make(map[string]printBulkRow)
		err := spool.walk(context.Background(), ids, func(row printBulkRow) error { got[row.ScryfallID] = row; return nil })
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotOracles, wantOracles) || len(got) != len(wantPrints) {
			t.Fatalf("selection differs at limit %d", limit)
		}
		for _, row := range wantPrints {
			if !reflect.DeepEqual(got[row.ScryfallID], row) {
				t.Fatalf("printing metadata changed: %+v", got[row.ScryfallID])
			}
		}
	}
	name := spool.file.Name()
	spool.close()
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("temporary printing file retained: %v", err)
	}
}

func TestPaperPrintSpoolCleansUpFailedDownloads(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		cancel        bool
	}{
		{"truncated", `[{"id":"p","oracle_id":"a","name":"Card","games":["paper"]},`, false},
		{"cancelled", spoolFixture, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("TMPDIR", dir)
			dec, cleanup, err := newBulkJSONDecoderFromReader(strings.NewReader(tc.payload))
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			spool, err := spoolPaperPrints(ctx, dec, []oracleBulkRow{{OracleID: "a"}})
			if err == nil || spool != nil {
				t.Fatal("failed download returned a usable spool")
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("failed download retained files: %v %v", files, err)
			}
		})
	}
}

func TestPaperPrintSpoolPropagatesCopyFailure(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dec, cleanup, err := newBulkJSONDecoderFromReader(strings.NewReader(spoolFixture))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	spool, err := spoolPaperPrints(context.Background(), dec, []oracleBulkRow{{OracleID: "a"}})
	if err != nil {
		t.Fatal(err)
	}
	defer spool.close()
	want := errors.New("database COPY failed")
	err = spool.walk(context.Background(), spool.paperIDs, func(printBulkRow) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("COPY error lost: %v", err)
	}
}

// Feed a large catalog without first allocating a large fixture string. Run
// with -benchtime=1x to compare retained printing rows with the production spool.
type repeatedPrintReader struct {
	record            string
	remaining, offset int
}

func (r *repeatedPrintReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.record[r.offset:])
	r.offset += n
	if r.offset == len(r.record) {
		r.offset = 0
		r.remaining--
	}
	return n, nil
}

func BenchmarkPaperPrintingMemory(b *testing.B) {
	const count = 100000
	record := `{"id":"print","oracle_id":"oracle","name":"Card","games":["paper"],"lang":"en","image_uris":{"normal":"https://example.test/` + strings.Repeat("art", 300) + `"},"flavor_text":"` + strings.Repeat("flavor ", 100) + `"}` + "\n"
	for _, mode := range []string{"retained", "spooled"} {
		b.Run(mode, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				runtime.GC()
				dec, cleanup, err := newBulkJSONDecoderFromReader(&repeatedPrintReader{record: record, remaining: count})
				if err != nil {
					b.Fatal(err)
				}
				var rows []printBulkRow
				var spool *paperPrintSpool
				if mode == "retained" {
					rows, err = decodePrintRowsFromDecoder(dec, 0)
				} else {
					spool, err = spoolPaperPrints(context.Background(), dec, []oracleBulkRow{{OracleID: "oracle"}})
				}
				if err != nil {
					b.Fatal(err)
				}
				runtime.GC()
				var stats runtime.MemStats
				runtime.ReadMemStats(&stats)
				b.ReportMetric(float64(stats.HeapAlloc)/(1<<20), "retained-MiB")
				if mode == "retained" {
					if len(rows) != count {
						b.Fatal("row loss")
					}
					runtime.KeepAlive(rows)
				} else {
					if spool.count != count {
						b.Fatal("row loss")
					}
					spool.close()
				}
				cleanup()
			}
		})
	}
}
