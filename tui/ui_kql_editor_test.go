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

func TestEditor_Submit_RunsAndStaysInEditor(t *testing.T) {
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
	if m3.mode != modeKQLEditor {
		t.Fatalf("expected to stay in editor mode on submit; got %v", m3.mode)
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
	if !strings.Contains(m4.content, "Press Esc to exit editor") {
		t.Fatalf("expected interactive hint after success; got: %q", m4.content)
	}
}

func TestEditor_Submit_WithPortableShortcut(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	cap := &kqlCapture{}
	m.kqlClient = cap

	// Enter editor
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Type query and press Ctrl+R (reliable) to submit
	m2.ta.SetValue("traces | take 1")
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if cmd == nil {
		t.Fatalf("expected submit command on ctrl+r")
	}
}

func TestEditor_Submit_WithF5(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	cap := &kqlCapture{}
	m.kqlClient = cap

	// Enter editor
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Type query and press F5 to submit
	m2.ta.SetValue("requests | take 1")
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyF5})
	if cmd == nil {
		t.Fatalf("expected submit command on F5")
	}
}

func TestEditor_Submit_NormalizesCRLF(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	cap := &kqlCapture{}
	m.kqlClient = cap

	// Enter editor
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Use CRLF in buffer
	m2.ta.SetValue("traces\r\n| take 1\r\n")
	// Submit via internal message
	m3Any, cmd := m2.Update(submitEditorMsg{})
	_ = m3Any
	if cmd == nil {
		t.Fatalf("expected KQL run command")
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

func TestEditor_Resize_TinyTerminalStillValid(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	// Enter editor
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	// Send very small resize
	m3Any, _ := m2.Update(tea.WindowSizeMsg{Width: 20, Height: 6})
	m3 := m3Any.(model)
	if m3.ta.Height() < minEditorHeight {
		t.Fatalf("textarea height below minEditorHeight: %d", m3.ta.Height())
	}
	if m3.vp.Height < minViewportHeight {
		t.Fatalf("viewport height below minViewportHeight: %d", m3.vp.Height)
	}
}

func TestEditorMode_F6_WithResults(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	cap := &kqlCapture{}
	m.kqlClient = cap

	// Enter editor mode
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)

	// Add a multi-line query to the editor
	m2.ta.SetValue("traces\n| take 1\n| project timestamp")

	// Set up results
	cols := []appinsights.Column{{Name: "timestamp"}}
	rows := [][]interface{}{{"2023-01-01T00:00:00Z"}}
	m2.lastColumns = cols
	m2.lastRows = rows
	m2.lastTable = "PrimaryResult"
	m2.haveResults = true

	// Press F6 in editor mode
	m3Any, _ := m2.Update(tea.KeyMsg{Type: tea.KeyF6})
	m3 := m3Any.(model)

	// Should switch to table mode
	if m3.mode != modeTableResults {
		t.Fatalf("expected modeTableResults after F6; got %v", m3.mode)
	}
	// Should store editor mode for return
	if m3.returnMode != modeKQLEditor {
		t.Fatalf("expected returnMode to be modeKQLEditor; got %v", m3.returnMode)
	}
	// Should show opened message
	if !strings.Contains(m3.content, "Opened results table.") {
		t.Fatalf("expected opened message; got: %q", m3.content)
	}

	// Press Esc to return to editor
	m4Any, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := m4Any.(model)

	// Should return to editor mode
	if m4.mode != modeKQLEditor {
		t.Fatalf("expected to return to modeKQLEditor after Esc; got %v", m4.mode)
	}
	// Should preserve editor content
	if !strings.Contains(m4.ta.Value(), "traces") {
		t.Fatalf("expected editor content to be preserved; got: %q", m4.ta.Value())
	}
	// Should still have InsertNewline enabled
	if !m4.ta.KeyMap.InsertNewline.Enabled() {
		t.Fatalf("expected InsertNewline to remain enabled after return to editor")
	}
	// Should show closed message
	if !strings.Contains(m4.content, "Closed results table.") {
		t.Fatalf("expected closed message; got: %q", m4.content)
	}
	// Verify returnMode was cleared
	if m4.returnMode != 0 {
		t.Fatalf("expected returnMode to be cleared after close; got %v", m4.returnMode)
	}
}

func TestEditorMode_F6_WithoutResults(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted

	// Enter editor mode
	m.ta.SetValue("edit")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)

	// Add content to editor
	m2.ta.SetValue("traces | take 5")
	// No results
	m2.haveResults = false

	// Press F6 in editor mode
	m3Any, _ := m2.Update(tea.KeyMsg{Type: tea.KeyF6})
	m3 := m3Any.(model)

	// Should remain in editor mode
	if m3.mode != modeKQLEditor {
		t.Fatalf("expected to remain in modeKQLEditor; got %v", m3.mode)
	}
	// Should preserve editor content
	if !strings.Contains(m3.ta.Value(), "traces | take 5") {
		t.Fatalf("expected editor content to be preserved; got: %q", m3.ta.Value())
	}
	// Should show no results message
	if !strings.Contains(m3.content, "No results to open.") {
		t.Fatalf("expected no results message; got: %q", m3.content)
	}
}
