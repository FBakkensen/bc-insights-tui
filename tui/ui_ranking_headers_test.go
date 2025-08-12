package tui

import (
	"strings"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
)

// Validate that ranking influences ordering: keyword/error keys promoted, long keys later.
func TestUIHeaders_RankingOrdering(t *testing.T) {
	cols := []appinsights.Column{{Name: "timestamp"}, {Name: "message"}, {Name: "customDimensions"}}
	// Build rows with several keys; errorCount and requestId should rank high; longVerboseDescription should fall back
	rows := [][]interface{}{
		{"2025-01-01T00:00:00Z", "m0", map[string]interface{}{"errorCount": 1, "requestId": "abc", "shortFlag": "X", "longVerboseDescription": "" + strings.Repeat("x", 250)}},
		{"2025-01-01T00:01:00Z", "m1", map[string]interface{}{"errorCount": 2, "requestId": "def", "shortFlag": "Y", "longVerboseDescription": "" + strings.Repeat("y", 260)}},
		{"2025-01-01T00:02:00Z", "m2", map[string]interface{}{"errorCount": 3, "requestId": "ghi", "shortFlag": "Z", "longVerboseDescription": "" + strings.Repeat("z", 270)}},
	}
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.cfg.RankEnable = true
	m2Any, _ := m.Update(kqlResultMsg{tableName: "PrimaryResult", columns: cols, rows: rows})
	m2 := m2Any.(model)
	headers := m2.lastDisplayHeaders
	// Expect primaries first
	if len(headers) < 6 || headers[0] != "timestamp" || headers[1] != "message" {
		t.Fatalf("unexpected primaries: %v", headers)
	}
	// Extract custom ordering
	custom := headers[2:]
	// Find indexes
	idxErr, idxReq, idxShort, idxLong := -1, -1, -1, -1
	for i, h := range custom {
		switch h {
		case "errorCount":
			idxErr = i
		case "requestId":
			idxReq = i
		case "shortFlag":
			idxShort = i
		case "longVerboseDescription":
			idxLong = i
		}
	}
	if idxErr == -1 || idxReq == -1 || idxShort == -1 || idxLong == -1 {
		t.Fatalf("missing expected headers: %v", headers)
	}
	if idxErr >= idxShort || idxReq >= idxShort {
		t.Fatalf("expected keyword boosted headers ahead of shortFlag: %v", custom)
	}
	if idxLong <= idxShort {
		t.Fatalf("expected longVerboseDescription (penalized) after shortFlag: %v", custom)
	}
}
