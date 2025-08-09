package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
)

// Minimal fake implementing KQLClient for tests

type kqlOK struct{ resp *appinsights.QueryResponse }

func (k *kqlOK) ValidateQuery(query string) error { return nil }
func (k *kqlOK) ExecuteQuery(ctx context.Context, query string) (*appinsights.QueryResponse, error) {
	return k.resp, nil
}

type kqlValidateErr struct{ err error }

func (k *kqlValidateErr) ValidateQuery(query string) error { return k.err }
func (k *kqlValidateErr) ExecuteQuery(ctx context.Context, query string) (*appinsights.QueryResponse, error) {
	return nil, nil
}

// (no exec error fake needed for these tests)

// helper to create a post-auth model with injected KQL client
func newPostAuthModelWithKQL(k KQLClient) model {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.kqlClient = k
	return m
}

func TestKQL_ParseAndDispatch_EmptyError(t *testing.T) {
	m := newPostAuthModelWithKQL(&kqlOK{})
	m.ta.SetValue("kql:   ")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	if !strings.Contains(m2.content, "Query cannot be empty") {
		t.Fatalf("expected empty query error, got: %q", m2.content)
	}
}

func TestKQL_ParseAndDispatch_RunsAndShowsRunning(t *testing.T) {
	resp := &appinsights.QueryResponse{Tables: []appinsights.Table{{
		Name:    "PrimaryResult",
		Columns: []appinsights.Column{{Name: "a"}},
		Rows:    [][]interface{}{{"1"}},
	}}}
	m := newPostAuthModelWithKQL(&kqlOK{resp: resp})
	// Allow run without real authenticator by injecting client; we won't assert HasValidToken in tests
	m.ta.SetValue("kql: traces | take 1")
	m2Any, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	if cmd == nil {
		t.Fatalf("expected cmd to run KQL")
	}
	if !strings.Contains(m2.content, "> kql:") || !strings.Contains(m2.content, "Running query…") {
		t.Fatalf("expected echo and running message; got: %q", m2.content)
	}
}

func TestKQL_ValidationError_Message(t *testing.T) {
	m := newPostAuthModelWithKQL(&kqlValidateErr{err: fmt.Errorf("bad query")})
	m.ta.SetValue("kql: bad")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Simulate command completion with validation error
	m3Any, _ := m2.Update(kqlResultMsg{err: fmt.Errorf("bad query")})
	m3 := m3Any.(model)
	if !strings.Contains(m3.content, "bad query") {
		t.Fatalf("expected validation error message; got: %q", m3.content)
	}
}

func TestKQL_ZeroTables_NoResults(t *testing.T) {
	m := newPostAuthModelWithKQL(&kqlOK{resp: &appinsights.QueryResponse{Tables: []appinsights.Table{}}})
	// Directly send result msg
	m2Any, _ := m.Update(kqlResultMsg{columns: nil, rows: nil, duration: 10 * time.Millisecond})
	m2 := m2Any.(model)
	if !strings.Contains(m2.content, "No results.") {
		t.Fatalf("expected 'No results.'; got: %q", m2.content)
	}
}

func TestKQL_SnapshotAndOpenInteractively(t *testing.T) {
	cols := []appinsights.Column{{Name: "a"}, {Name: "b"}}
	rows := [][]interface{}{{"1", "x"}, {"2", "y"}}
	m := newPostAuthModelWithKQL(&kqlOK{resp: &appinsights.QueryResponse{}})
	m2Any, _ := m.Update(kqlResultMsg{tableName: "PrimaryResult", columns: cols, rows: rows, duration: 10 * time.Millisecond})
	m2 := m2Any.(model)
	if !strings.Contains(m2.content, "Press F6 to open interactively.") {
		t.Fatalf("expected hint to open interactively; got: %q", m2.content)
	}
	// Press F6 to open interactively
	m3Any, _ := m2.Update(tea.KeyMsg{Type: tea.KeyF6})
	m3 := m3Any.(model)
	if m3.mode != modeTableResults {
		t.Fatalf("expected modeTableResults after F6; got %v", m3.mode)
	}
	// Verify returnMode was set
	if m3.returnMode != modeChat {
		t.Fatalf("expected returnMode to be modeChat; got %v", m3.returnMode)
	}
	// Press Esc to return
	m4Any, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := m4Any.(model)
	if m4.mode != modeChat {
		t.Fatalf("expected to return to chat after Esc; got %v", m4.mode)
	}
	if !strings.Contains(m4.content, "Closed results table.") {
		t.Fatalf("expected closed message; got: %q", m4.content)
	}
	// Verify returnMode was cleared
	if m4.returnMode != 0 {
		t.Fatalf("expected returnMode to be cleared after close; got %v", m4.returnMode)
	}
}

func TestChatMode_F6_WithResults(t *testing.T) {
	cols := []appinsights.Column{{Name: "test"}}
	rows := [][]interface{}{{"value"}}
	m := newPostAuthModelWithKQL(&kqlOK{resp: &appinsights.QueryResponse{}})
	// Set up results
	m.lastColumns = cols
	m.lastRows = rows
	m.lastTable = "TestTable"
	m.haveResults = true

	// Press F6 in chat mode
	m2Any, _ := m.Update(tea.KeyMsg{Type: tea.KeyF6})
	m2 := m2Any.(model)

	// Should switch to table mode
	if m2.mode != modeTableResults {
		t.Fatalf("expected modeTableResults after F6; got %v", m2.mode)
	}
	// Should store chat mode for return
	if m2.returnMode != modeChat {
		t.Fatalf("expected returnMode to be modeChat; got %v", m2.returnMode)
	}
	// Should show opened message
	if !strings.Contains(m2.content, "Opened results table.") {
		t.Fatalf("expected opened message; got: %q", m2.content)
	}
}

func TestChatMode_F6_WithoutResults(t *testing.T) {
	m := newPostAuthModelWithKQL(&kqlOK{resp: &appinsights.QueryResponse{}})
	// No results
	m.haveResults = false

	// Press F6 in chat mode
	m2Any, _ := m.Update(tea.KeyMsg{Type: tea.KeyF6})
	m2 := m2Any.(model)

	// Should remain in chat mode
	if m2.mode != modeChat {
		t.Fatalf("expected to remain in modeChat; got %v", m2.mode)
	}
	// Should show no results message
	if !strings.Contains(m2.content, "No results to open.") {
		t.Fatalf("expected no results message; got: %q", m2.content)
	}
}
