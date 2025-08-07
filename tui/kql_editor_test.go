package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKQLEditor_BasicFunctionality(t *testing.T) {
	history := NewQueryHistory(10, "")
	editor := NewKQLEditor(history, 80, 20)

	// Test initial state
	if !editor.IsFocused() {
		t.Error("Expected editor to be focused initially")
	}

	if editor.GetContent() != "" {
		t.Error("Expected empty content initially")
	}

	if editor.IsExecuting() {
		t.Error("Expected not executing initially")
	}

	// Test setting content
	testQuery := "traces | limit 10"
	editor.SetContent(testQuery)
	if editor.GetContent() != testQuery {
		t.Errorf("Expected content %q, got %q", testQuery, editor.GetContent())
	}

	// Test setting error
	testError := "test error message"
	editor.SetError(testError)
	if editor.errorMsg != testError {
		t.Errorf("Expected error message %q, got %q", testError, editor.errorMsg)
	}

	// Test clearing error
	editor.ClearError()
	if editor.errorMsg != "" {
		t.Errorf("Expected empty error message after clear, got %q", editor.errorMsg)
	}

	// Test executing state
	editor.SetExecuting(true)
	if !editor.IsExecuting() {
		t.Error("Expected executing state to be true")
	}

	editor.SetExecuting(false)
	if editor.IsExecuting() {
		t.Error("Expected executing state to be false")
	}
}

func TestKQLEditor_FocusManagement(t *testing.T) {
	history := NewQueryHistory(10, "")
	editor := NewKQLEditor(history, 80, 20)

	// Test initial focus
	if !editor.IsFocused() {
		t.Error("Expected editor to be focused initially")
	}

	// Test blur
	editor.Blur()
	if editor.IsFocused() {
		t.Error("Expected editor to be blurred")
	}

	// Test focus
	editor.Focus()
	if !editor.IsFocused() {
		t.Error("Expected editor to be focused")
	}
}

func TestKQLEditor_SizeManagement(t *testing.T) {
	history := NewQueryHistory(10, "")
	editor := NewKQLEditor(history, 80, 20)

	// Test initial size
	if editor.width != 80 || editor.height != 20 {
		t.Errorf("Expected size 80x20, got %dx%d", editor.width, editor.height)
	}

	// Test resize
	editor.SetSize(100, 30)
	if editor.width != 100 || editor.height != 30 {
		t.Errorf("Expected size 100x30 after resize, got %dx%d", editor.width, editor.height)
	}
}

func TestKQLEditor_HistoryNavigation(t *testing.T) {
	history := NewQueryHistory(10, "")

	// Add some test queries to history
	history.AddEntry("query1", true, 10, time.Second)
	history.AddEntry("query2", true, 20, time.Second)
	history.AddEntry("query3", true, 30, time.Second)

	editor := NewKQLEditor(history, 80, 20)

	// Test direct navigation methods
	editor = editor.navigateHistory(-1) // Go to most recent (index 0)
	if editor.historyIdx != 0 {
		t.Errorf("Expected history index 0, got %d", editor.historyIdx)
	}

	editor = editor.navigateHistory(-1) // Go to older (index 1)
	if editor.historyIdx != 1 {
		t.Errorf("Expected history index 1, got %d", editor.historyIdx)
	}

	editor = editor.navigateHistory(1) // Go back to newer (index 0)
	if editor.historyIdx != 0 {
		t.Errorf("Expected history index 0, got %d", editor.historyIdx)
	}

	editor = editor.navigateHistory(1) // Go back to current (-1)
	if editor.historyIdx != -1 {
		t.Errorf("Expected history index -1, got %d", editor.historyIdx)
	}
}

func TestKQLEditor_KeyHandling(t *testing.T) {
	history := NewQueryHistory(10, "")
	editor := NewKQLEditor(history, 80, 20)

	// Test F5 key (execute query)
	editor.SetContent("traces | limit 10")
	_, cmd := editor.Update(tea.KeyMsg{Type: tea.KeyF5})

	if cmd == nil {
		t.Error("Expected F5 to return a command")
	}

	// Test Ctrl+K (clear content)
	editor.SetContent("some content")
	_, _ = editor.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	if editor.GetContent() != "" {
		t.Error("Expected Ctrl+K to clear content")
	}
}

func TestExecuteQueryMsg(t *testing.T) {
	msg := ExecuteQueryMsg{Query: "test query"}
	if msg.Query != "test query" {
		t.Errorf("Expected query 'test query', got %q", msg.Query)
	}
}
