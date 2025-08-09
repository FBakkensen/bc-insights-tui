package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
)

// fake KQL client for editor tests
type kqlCapture struct{ last string }

func (k *kqlCapture) ValidateQuery(query string) error { return nil }
func (k *kqlCapture) ExecuteQuery(ctx context.Context, query string) (*appinsights.QueryResponse, error) {
	k.last = query
	// Minimal success response
	return &appinsights.QueryResponse{Tables: []appinsights.Table{{
		Name:    "PrimaryResult",
		Columns: []appinsights.Column{{Name: "x"}},
		Rows:    [][]interface{}{{"ok"}},
	}}}, nil
}

func TestEditor_EnterMode_FromChat(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	if m2.mode != modeKQLEditor {
		t.Fatalf("expected modeKQLEditor, got %v", m2.mode)
	}
	if !m2.ta.KeyMap.InsertNewline.Enabled() {
		t.Fatalf("expected InsertNewline enabled in editor mode")
	}
	if !strings.Contains(m2.content, "Ctrl+Enter") {
		t.Fatalf("expected helper hint appended; got: %q", m2.content)
	}
}

func TestEditor_CancelWithEsc(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	// Enter editor mode
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Press Esc
	m3Any, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := m3Any.(model)
	if m3.mode != modeChat {
		t.Fatalf("expected to return to chat mode after Esc; got %v", m3.mode)
	}
	if !strings.Contains(m3.content, "Canceled edit.") {
		t.Fatalf("expected 'Canceled edit.' in content; got: %q", m3.content)
	}
}

func TestEditor_NewlineInsertion(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	// Enter editor mode
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Ensure textarea is focused for key handling
	_ = m2.ta.Focus()
	// Type text and press Enter (should insert newline)
	m2.ta.SetValue("traces | take 1")
	m3Any, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := m3Any.(model)
	if !strings.Contains(m3.ta.Value(), "\n") {
		t.Fatalf("expected newline to be inserted in editor textarea; got: %q", m3.ta.Value())
	}
}

func TestEditor_Submit_EmptyShowsError(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	// Enter editor
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Submit with empty buffer via internal message
	m3Any, _ := m2.Update(submitEditorMsg{})
	m3 := m3Any.(model)
	if !strings.Contains(m3.content, "Query cannot be empty.") {
		t.Fatalf("expected empty query error; got: %q", m3.content)
	}
	if m3.mode != modeKQLEditor {
		t.Fatalf("expected to remain in editor mode on empty submit; got %v", m3.mode)
	}
}

func TestEditor_Submit_RunsAndExitsEditor(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	cap := &kqlCapture{}
	m.kqlClient = cap

	// Enter editor
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Type multi-line query
	m2.ta.SetValue("traces\n| take 1")
	// Submit via internal message
	m3Any, cmd := m2.Update(submitEditorMsg{})
	m3 := m3Any.(model)
	if m3.mode != modeChat {
		t.Fatalf("expected to exit editor mode on submit; got %v", m3.mode)
	}
	if !strings.Contains(m3.content, "> traces …") || !strings.Contains(m3.content, "Running…") {
		t.Fatalf("expected echo and Running…; got: %q", m3.content)
	}
	if cmd == nil {
		t.Fatalf("expected KQL run command")
	}
	// Bypass auth preflight by injecting a synthetic success result
	success := kqlResultMsg{tableName: "PrimaryResult", columns: []appinsights.Column{{Name: "x"}}, rows: [][]interface{}{{"ok"}}, duration: 0}
	m4Any, _ := m3.Update(success)
	m4 := m4Any.(model)
	if !strings.Contains(m4.content, "Press Enter to open interactively.") {
		t.Fatalf("expected interactive hint after success; got: %q", m4.content)
	}
}

func TestEditor_Resize_AdjustsHeights(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	// Enter editor
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Send resize
	width, height := 100, 30
	m3Any, _ := m2.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m3 := m3Any.(model)
	if m3.ta.Height() < 3 || m3.vp.Height <= 0 {
		t.Fatalf("expected positive heights for editor textarea and viewport; got ta=%d vp=%d", m3.ta.Height(), m3.vp.Height)
	}
}
