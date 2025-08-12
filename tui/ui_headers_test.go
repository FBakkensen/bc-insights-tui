package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
)

const (
	hdrTimestamp = "timestamp"
	hdrMessage   = "message"
)

// Build a KQL result where customDimensions vary across rows to ensure unioning works
func TestHeaders_BuildFromAllRows_UnionKeys(t *testing.T) {
	cols := []appinsights.Column{{Name: "timestamp"}, {Name: "message"}, {Name: "customDimensions"}}
	rows := [][]interface{}{
		{"2025-01-01T00:00:00Z", "a", `{"k1":"v1","k2":"v2"}`},
		{"2025-01-01T00:01:00Z", "b", map[string]interface{}{"k2": "v22", "k3": "v3"}},
	}
	headers := buildHeadersForAllRows(cols, rows)
	if len(headers) != 5 { // timestamp, message, k1, k2, k3
		t.Fatalf("expected 5 headers, got %d: %v", len(headers), headers)
	}
	if headers[0] != hdrTimestamp || headers[1] != hdrMessage {
		t.Fatalf("expected first headers timestamp,message; got %v", headers[:2])
	}
	// order of custom keys is sorted
	custom := headers[2:]
	want := []string{"k1", "k2", "k3"}
	if strings.Join(custom, ",") != strings.Join(want, ",") {
		t.Fatalf("expected custom keys %v, got %v", want, custom)
	}
}

func TestSnapshot_And_Interactive_UseCanonicalHeaders(t *testing.T) {
	// Construct columns with extra fields that should be ignored
	cols := []appinsights.Column{{Name: "timestamp"}, {Name: "message"}, {Name: "customDimensions"}, {Name: "ignored"}}
	rows := [][]interface{}{
		{"2025-01-01T00:00:00Z", "hello", `{"x":1}`, "zzz"},
		{"2025-01-01T00:01:00Z", "world", `{"y":2}`, 123},
	}
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	// store results
	m2Any, _ := m.Update(kqlResultMsg{tableName: "PrimaryResult", columns: cols, rows: rows})
	m2 := m2Any.(model)
	if len(m2.lastDisplayHeaders) != 4 { // timestamp, message, x, y
		t.Fatalf("expected 4 canonical headers; got %d: %v", len(m2.lastDisplayHeaders), m2.lastDisplayHeaders)
	}
	// Render snapshot: should not contain "ignored"
	snap := m2.renderSnapshot(cols, rows)
	if strings.Contains(snap, "ignored") {
		t.Fatalf("snapshot should not include non-target column 'ignored'")
	}
	if !strings.Contains(snap, hdrTimestamp) || !strings.Contains(snap, hdrMessage) {
		t.Fatalf("snapshot should include timestamp and message headers: %q", snap)
	}

	// Open interactive and check the header row view contains only canonical headers
	m3Any, _ := m2.Update(tea.KeyMsg{Type: tea.KeyF6})
	m3 := m3Any.(model)
	view := m3.tbl.View()
	if strings.Contains(view, "ignored") {
		t.Fatalf("interactive view should not include non-target column 'ignored'")
	}
	// Ensure at least one custom key header is present
	if !strings.Contains(view, "x") || !strings.Contains(view, "y") {
		t.Fatalf("interactive header should include discovered custom keys: %q", view)
	}
}

// When width is constrained, we show as many canonical headers as fit and add an ellipsis column (+N)
// where N equals the count of hidden canonical headers. This test forces a narrow width.
func TestSnapshot_EllipsisCount_FromCanonicalHeaders(t *testing.T) {
	cols := []appinsights.Column{{Name: "timestamp"}, {Name: "message"}, {Name: "customDimensions"}}
	rows := [][]interface{}{
		{"2025-01-01T00:00:00Z", "m0", `{"a":1,"b":2,"c":3,"d":4}`},
		{"2025-01-01T00:01:00Z", "m1", `{"a":11,"c":33,"e":5}`},
	}
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	// Shrink viewport width to force truncation. minColWidth is 14 => width 42 fits 3 columns max.
	// With ellipsis, dataCols = visibleCols-1 = 2, leaving (+N) as third.
	m.vp.Width = 42

	m2Any, _ := m.Update(kqlResultMsg{tableName: "PrimaryResult", columns: cols, rows: rows})
	m2 := m2Any.(model)
	// Canonical headers are timestamp, message, and union of {a,b,c,d,e} => 7 total
	if len(m2.lastDisplayHeaders) != 7 {
		t.Fatalf("expected 7 headers; got %d: %v", len(m2.lastDisplayHeaders), m2.lastDisplayHeaders)
	}
	snap := m2.renderSnapshot(cols, rows)
	// With width=42, visibleCols = 3, dataCols=2, so hidden = 7-2 = 5
	if !strings.Contains(snap, "(+5)") {
		t.Fatalf("expected snapshot ellipsis '(+5)' reflecting hidden canonical headers; got: %q", snap)
	}
}

func TestInteractive_EllipsisCount_FromCanonicalHeaders(t *testing.T) {
	cols := []appinsights.Column{{Name: "timestamp"}, {Name: "message"}, {Name: "customDimensions"}}
	rows := [][]interface{}{
		{"2025-01-01T00:00:00Z", "m0", `{"a":1,"b":2,"c":3,"d":4}`},
		{"2025-01-01T00:01:00Z", "m1", `{"a":11,"c":33,"e":5}`},
	}
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.vp.Width = 42
	// Produce results and then open interactive (F6)
	m2Any, _ := m.Update(kqlResultMsg{tableName: "PrimaryResult", columns: cols, rows: rows})
	m2 := m2Any.(model)
	m3Any, _ := m2.Update(tea.KeyMsg{Type: tea.KeyF6})
	m3 := m3Any.(model)
	view := m3.tbl.View()
	// Same calculation: 7 total headers, dataCols=2 => hidden = 5
	if !strings.Contains(view, "(+5)") {
		t.Fatalf("expected interactive ellipsis '(+5)' reflecting hidden canonical headers; got: %q", view)
	}
}

// Ensure we de-duplicate custom keys case-insensitively and prefer first-seen casing,
// while mapping values regardless of later rows using different case.
func TestHeaders_CaseInsensitive_Dedup_And_ValueMapping(t *testing.T) {
	cols := []appinsights.Column{{Name: "timestamp"}, {Name: "message"}, {Name: "customDimensions"}}
	rows := [][]interface{}{
		{"2025-01-01T00:00:00Z", "m0", `{"Foo": "A", "bar": 1}`},
		{"2025-01-01T00:01:00Z", "m1", `{"foo": "B", "Bar": 2}`},
	}
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.vp.Width = 200
	m2Any, _ := m.Update(kqlResultMsg{tableName: "PrimaryResult", columns: cols, rows: rows})
	m2 := m2Any.(model)

	headers := m2.lastDisplayHeaders
	// Verify consolidated headers contain 'bar' and 'foo' once each, case-insensitively
	custom := headers[2:]
	seenLower := map[string]bool{}
	for _, h := range custom {
		seenLower[strings.ToLower(h)] = true
	}
	if !seenLower["bar"] || !seenLower["foo"] {
		t.Fatalf("expected consolidated headers to include 'bar' and 'foo' (any casing); got %v", headers)
	}
	// Ensure there are no duplicates case-insensitively
	if len(seenLower) != len(custom) {
		t.Fatalf("expected no duplicate custom headers (case-insensitive); got %v", headers)
	}

	// Render snapshot and ensure values appear irrespective of case in row
	out := m2.renderSnapshot(cols, rows)
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("expected both values A and B for key 'foo/ Foo' to appear; got: %q", out)
	}
}
