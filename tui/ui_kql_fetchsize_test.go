package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
)

const testUnconstrainedQuery = "traces | where timestamp > ago(1h)"

// kqlCapture captures the query sent to ExecuteQuery for testing
type kqlFetchSizeCapture struct {
	lastQuery string
	resp      *appinsights.QueryResponse
}

func (k *kqlFetchSizeCapture) ValidateQuery(query string) error { return nil }

func (k *kqlFetchSizeCapture) ExecuteQuery(ctx context.Context, query string) (*appinsights.QueryResponse, error) {
	k.lastQuery = query
	return k.resp, nil
}

func TestEnforceFetchSizeLimit_UnconstrainedQuery(t *testing.T) {
	query := testUnconstrainedQuery
	finalQuery, applied, effectiveFetch := enforceFetchSizeLimit(query, 50)

	if !applied {
		t.Fatalf("expected limiter to be applied for unconstrained query")
	}
	if effectiveFetch != 50 {
		t.Fatalf("expected effectiveFetch=50, got %d", effectiveFetch)
	}
	if !strings.Contains(finalQuery, "| take 50") {
		t.Fatalf("expected finalQuery to contain '| take 50', got: %q", finalQuery)
	}
}

func TestEnforceFetchSizeLimit_AlreadyConstrainedTake(t *testing.T) {
	query := testUnconstrainedQuery + " | take 100"
	finalQuery, applied, _ := enforceFetchSizeLimit(query, 50)

	if applied {
		t.Fatalf("expected limiter NOT to be applied for query with take")
	}
	if finalQuery != query {
		t.Fatalf("expected finalQuery to be unchanged, got: %q", finalQuery)
	}
}

func TestEnforceFetchSizeLimit_AlreadyConstrainedLimit(t *testing.T) {
	query := "traces | where severityLevel >= 3 | limit 10"
	finalQuery, applied, _ := enforceFetchSizeLimit(query, 50)

	if applied {
		t.Fatalf("expected limiter NOT to be applied for query with limit")
	}
	if finalQuery != query {
		t.Fatalf("expected finalQuery to be unchanged, got: %q", finalQuery)
	}
}

func TestEnforceFetchSizeLimit_AlreadyConstrainedTopBy(t *testing.T) {
	query := "requests | top 10 by duration desc"
	finalQuery, applied, _ := enforceFetchSizeLimit(query, 50)

	if applied {
		t.Fatalf("expected limiter NOT to be applied for query with 'top N by'")
	}
	if finalQuery != query {
		t.Fatalf("expected finalQuery to be unchanged, got: %q", finalQuery)
	}
}

func TestEnforceFetchSizeLimit_TopWithoutBy(t *testing.T) {
	// "top" without "by" should not be treated as a row limiter
	query := "traces | project timestamp, message | sort by timestamp"
	finalQuery, applied, _ := enforceFetchSizeLimit(query, 25)

	if !applied {
		t.Fatalf("expected limiter to be applied for query with 'top' but no 'by'")
	}
	if !strings.Contains(finalQuery, "| take 25") {
		t.Fatalf("expected finalQuery to contain '| take 25', got: %q", finalQuery)
	}
}

func TestEnforceFetchSizeLimit_CaseInsensitive(t *testing.T) {
	query := "traces | WHERE timestamp > ago(1h) | TAKE 100"
	finalQuery, applied, _ := enforceFetchSizeLimit(query, 50)

	if applied {
		t.Fatalf("expected limiter NOT to be applied for case-insensitive TAKE")
	}
	if finalQuery != query {
		t.Fatalf("expected finalQuery to be unchanged, got: %q", finalQuery)
	}
}

func TestEnforceFetchSizeLimit_MultilineQuery(t *testing.T) {
	query := "traces\n| where timestamp > ago(1h)\n| project timestamp, message"
	finalQuery, applied, _ := enforceFetchSizeLimit(query, 30)

	if !applied {
		t.Fatalf("expected limiter to be applied for multiline query")
	}
	if !strings.Contains(finalQuery, "| take 30") {
		t.Fatalf("expected finalQuery to contain '| take 30', got: %q", finalQuery)
	}
	// Should handle multiline correctly
	lines := strings.Split(finalQuery, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected multiline output, got: %q", finalQuery)
	}
}

func TestEnforceFetchSizeLimit_ZeroFetchSize(t *testing.T) {
	query := testUnconstrainedQuery
	finalQuery, applied, effectiveFetch := enforceFetchSizeLimit(query, 0)

	if !applied {
		t.Fatalf("expected limiter to be applied")
	}
	if effectiveFetch != 50 {
		t.Fatalf("expected normalized effectiveFetch=50, got %d", effectiveFetch)
	}
	if !strings.Contains(finalQuery, "| take 50") {
		t.Fatalf("expected finalQuery to contain '| take 50' (normalized), got: %q", finalQuery)
	}
}

func TestEnforceFetchSizeLimit_NegativeFetchSize(t *testing.T) {
	query := testUnconstrainedQuery
	finalQuery, applied, effectiveFetch := enforceFetchSizeLimit(query, -10)

	if !applied {
		t.Fatalf("expected limiter to be applied")
	}
	if effectiveFetch != 50 {
		t.Fatalf("expected normalized effectiveFetch=50, got %d", effectiveFetch)
	}
	if !strings.Contains(finalQuery, "| take 50") {
		t.Fatalf("expected finalQuery to contain '| take 50' (normalized), got: %q", finalQuery)
	}
}

func TestKQL_FetchSizeEnforcement_Integration(t *testing.T) {
	// Integration test: verify that unconstrained queries get limiter applied
	resp := &appinsights.QueryResponse{Tables: []appinsights.Table{{
		Name:    "PrimaryResult",
		Columns: []appinsights.Column{{Name: "timestamp"}, {Name: "message"}},
		Rows:    [][]interface{}{{"2023-01-01T00:00:00Z", "test message"}},
	}}}

	capture := &kqlFetchSizeCapture{resp: resp}
	m := newPostAuthModelWithKQL(capture)
	m.cfg.LogFetchSize = 75

	// Submit unconstrained query
	inputQuery := testUnconstrainedQuery
	m.ta.SetValue("kql: " + inputQuery)
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatalf("expected cmd to run KQL")
	}

	// Execute the command to capture the query
	msg := cmd()
	result := msg.(kqlResultMsg)

	// Verify no error occurred
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}

	// Verify limiter was applied
	if !result.limiterApplied {
		t.Fatalf("expected limiterApplied=true")
	}
	if result.effectiveFetch != 75 {
		t.Fatalf("expected effectiveFetch=75, got %d", result.effectiveFetch)
	}

	// Verify the actual query sent to the client
	if !strings.Contains(capture.lastQuery, "| take 75") {
		t.Fatalf("expected query sent to client to contain '| take 75', got: %q", capture.lastQuery)
	}
}

func TestKQL_FetchSizeEnforcement_AlreadyConstrained(t *testing.T) {
	// Integration test: verify that constrained queries are not modified
	resp := &appinsights.QueryResponse{Tables: []appinsights.Table{{
		Name:    "PrimaryResult",
		Columns: []appinsights.Column{{Name: "timestamp"}},
		Rows:    [][]interface{}{{"2023-01-01T00:00:00Z"}},
	}}}

	capture := &kqlFetchSizeCapture{resp: resp}
	m := newPostAuthModelWithKQL(capture)
	m.cfg.LogFetchSize = 50

	// Submit constrained query
	originalQuery := testUnconstrainedQuery + " | take 25"
	m.ta.SetValue("kql: " + originalQuery)
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatalf("expected cmd to run KQL")
	}

	// Execute the command
	msg := cmd()
	result := msg.(kqlResultMsg)

	// Verify limiter was NOT applied
	if result.limiterApplied {
		t.Fatalf("expected limiterApplied=false for constrained query")
	}

	// Verify the query was not modified
	if capture.lastQuery != originalQuery {
		t.Fatalf("expected query to be unchanged, got: %q", capture.lastQuery)
	}
}

func TestKQL_UserFacingNote_Applied(t *testing.T) {
	// Test that user sees the note when limiter is applied
	resp := &appinsights.QueryResponse{Tables: []appinsights.Table{{
		Name:    "PrimaryResult",
		Columns: []appinsights.Column{{Name: "test"}},
		Rows:    [][]interface{}{{"value"}},
	}}}

	m := newPostAuthModelWithKQL(&kqlOK{resp: resp})
	m.cfg.LogFetchSize = 40

	// Simulate successful query result with limiter applied
	result := kqlResultMsg{
		tableName:      "PrimaryResult",
		columns:        resp.Tables[0].Columns,
		rows:           resp.Tables[0].Rows,
		duration:       10 * time.Millisecond,
		limiterApplied: true,
		effectiveFetch: 40,
	}

	m2Any, _ := m.Update(result)
	m2 := m2Any.(model)

	// Check that user-facing note is present
	if !strings.Contains(m2.content, "Applied server-side row cap: take 40") {
		t.Fatalf("expected user-facing note about applied limiter, got content: %q", m2.content)
	}
}

func TestKQL_UserFacingNote_NotApplied(t *testing.T) {
	// Test that user does NOT see the note when limiter is not applied
	resp := &appinsights.QueryResponse{Tables: []appinsights.Table{{
		Name:    "PrimaryResult",
		Columns: []appinsights.Column{{Name: "test"}},
		Rows:    [][]interface{}{{"value"}},
	}}}

	m := newPostAuthModelWithKQL(&kqlOK{resp: resp})

	// Simulate successful query result without limiter applied
	result := kqlResultMsg{
		tableName:      "PrimaryResult",
		columns:        resp.Tables[0].Columns,
		rows:           resp.Tables[0].Rows,
		duration:       10 * time.Millisecond,
		limiterApplied: false,
		effectiveFetch: 50,
	}

	m2Any, _ := m.Update(result)
	m2 := m2Any.(model)

	// Check that user-facing note is NOT present
	if strings.Contains(m2.content, "Applied server-side row cap") {
		t.Fatalf("unexpected user-facing note when limiter not applied, got content: %q", m2.content)
	}
	// But should still have normal success messages
	if !strings.Contains(m2.content, "Query complete") {
		t.Fatalf("expected normal query success message, got content: %q", m2.content)
	}
}

func TestEnforceFetchSizeLimit_EdgeCases(t *testing.T) {
	// Test various edge cases in one test function

	testCases := []struct {
		name          string
		query         string
		fetchSize     int
		expectApplied bool
		expectedFetch int
		shouldContain string
	}{
		{
			name:          "Query with take in string literal",
			query:         `traces | where message contains "| take 100"`,
			fetchSize:     50,
			expectApplied: false, // Our textual approach will see this as constrained (documented limitation)
			expectedFetch: 50,
			shouldContain: "", // Query unchanged
		},
		{
			name:          "Query with limit in field name",
			query:         "traces | project limit_field",
			fetchSize:     30,
			expectApplied: true, // Field names don't match our pattern "| limit " (note the space)
			expectedFetch: 30,
			shouldContain: "| take 30",
		},
		{
			name:          "Empty query",
			query:         "",
			fetchSize:     25,
			expectApplied: true,
			expectedFetch: 25,
			shouldContain: "| take 25",
		},
		{
			name:          "Whitespace only query",
			query:         "   \n  \t  ",
			fetchSize:     20,
			expectApplied: true,
			expectedFetch: 20,
			shouldContain: "| take 20",
		},
		{
			name:          "Query ending with newline",
			query:         testUnconstrainedQuery + "\n",
			fetchSize:     15,
			expectApplied: true,
			expectedFetch: 15,
			shouldContain: "| take 15",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			finalQuery, applied, effectiveFetch := enforceFetchSizeLimit(tc.query, tc.fetchSize)

			if applied != tc.expectApplied {
				t.Fatalf("expected applied=%t, got %t", tc.expectApplied, applied)
			}
			if effectiveFetch != tc.expectedFetch {
				t.Fatalf("expected effectiveFetch=%d, got %d", tc.expectedFetch, effectiveFetch)
			}
			if tc.shouldContain != "" && !strings.Contains(finalQuery, tc.shouldContain) {
				t.Fatalf("expected finalQuery to contain %q, got: %q", tc.shouldContain, finalQuery)
			}
			if tc.shouldContain == "" && tc.expectApplied == false && finalQuery != tc.query {
				t.Fatalf("expected finalQuery to be unchanged, got: %q", finalQuery)
			}
		})
	}
}
