package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
)

func TestDetails_OpenAndClose_FromTable(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted

	// Seed last results with a customDimensions column and one row
	cols := []appinsights.Column{{Name: "timestamp"}, {Name: "message"}, {Name: "customDimensions"}}
	row := []interface{}{"2025-03-03T00:00:00Z", "hello world", map[string]interface{}{"foo": "bar", "nested": map[string]interface{}{"x": 1}}}
	m.lastColumns = cols
	m.lastRows = [][]interface{}{row}
	m.lastTable = "PrimaryResult"
	m.haveResults = true

	// Open table interactively via F6
	m2Any, _ := m.Update(tea.KeyMsg{Type: tea.KeyF6})
	m2 := m2Any.(model)
	if m2.mode != modeTableResults {
		t.Fatalf("expected table mode after F6; got %v", m2.mode)
	}

	// Press Enter to open details for selected row 0
	m3Any, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := m3Any.(model)
	if m3.mode != modeDetails {
		t.Fatalf("expected modeDetails after Enter; got %v", m3.mode)
	}
	view := m3.detailsVP.View()
	// Basic header marker and sections present
	if !strings.Contains(view, "Details — PrimaryResult · row 0") {
		t.Fatalf("expected header with table and row index; got: %q", view)
	}
	if !strings.Contains(view, "timestamp: 2025-03-03T00:00:00Z") {
		t.Fatalf("expected timestamp line; got: %q", view)
	}
	if !strings.Contains(view, "message: hello world") {
		t.Fatalf("expected message line; got: %q", view)
	}
	if !strings.Contains(view, "customDimensions:") {
		t.Fatalf("expected customDimensions section; got: %q", view)
	}
	// At least one flattened key
	if !strings.Contains(view, "foo: bar") {
		t.Fatalf("expected flattened key foo: bar; got: %q", view)
	}

	// Close details with Esc back to table
	m4Any, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := m4Any.(model)
	if m4.mode != modeTableResults {
		t.Fatalf("expected to return to table after Esc; got %v", m4.mode)
	}
}
