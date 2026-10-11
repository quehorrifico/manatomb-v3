package cards

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// Keep only canonical identities in memory. Printing artwork, faces, flavor
// text and prices are much larger and are replayed from a temporary file into
// PostgreSQL COPY. The file is removed on success and every error path.
type paperPrintSpool struct {
	file     *os.File
	paperIDs map[string]struct{}
	count    int
}

func bulkOracleKey(id string) string { return strings.ToLower(strings.TrimSpace(id)) }

func selectPaperOracles(rows []oracleBulkRow, paperIDs map[string]struct{}, limit int) ([]oracleBulkRow, map[string]struct{}) {
	selected := make([]oracleBulkRow, 0, len(rows))
	ids := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		id := bulkOracleKey(row.OracleID)
		if _, ok := paperIDs[id]; !ok {
			continue
		}
		selected = append(selected, row)
		ids[id] = struct{}{}
		if limit > 0 && len(selected) >= limit {
			break
		}
	}
	return selected, ids
}

func spoolPaperPrints(ctx context.Context, dec *bulkJSONDecoder, oracles []oracleBulkRow) (*paperPrintSpool, error) {
	file, err := os.CreateTemp("", "manatomb-card-prints-*.jsonl")
	if err != nil {
		return nil, err
	}
	spool := &paperPrintSpool{file: file, paperIDs: make(map[string]struct{}, len(oracles))}
	ok := false
	defer func() {
		if !ok {
			spool.close()
		}
	}()
	canonical := make(map[string]struct{}, len(oracles))
	for _, row := range oracles {
		canonical[bulkOracleKey(row.OracleID)] = struct{}{}
	}
	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	err = walkPrintRows(dec, 0, func(row printBulkRow) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		id := bulkOracleKey(row.OracleID)
		if _, exists := canonical[id]; !exists {
			return nil
		}
		if err := encoder.Encode(row); err != nil {
			return err
		}
		spool.paperIDs[id] = struct{}{}
		spool.count++
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := writer.Flush(); err != nil {
		return nil, err
	}
	ok = true
	return spool, nil
}

func (s *paperPrintSpool) close() {
	_ = s.file.Close()
	_ = os.Remove(s.file.Name())
}

func (s *paperPrintSpool) walk(ctx context.Context, ids map[string]struct{}, visit func(printBulkRow) error) error {
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	decoder := json.NewDecoder(bufio.NewReader(s.file))
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var row printBulkRow
		if err := decoder.Decode(&row); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if _, ok := ids[bulkOracleKey(row.OracleID)]; !ok {
			continue
		}
		if err := visit(row); err != nil {
			return err
		}
	}
}
