package tui

// KQL Editor text component with multi-line editing and KQL features

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// KQLEditor represents the multi-line text editor for KQL queries
type KQLEditor struct {
	textArea   textarea.Model
	focused    bool
	content    string
	history    *QueryHistory
	historyIdx int // Current position in history (-1 = not browsing history)
	errorMsg   string
	executing  bool
	width      int
	height     int
}

// NewKQLEditor creates a new KQL editor instance
func NewKQLEditor(history *QueryHistory, width, height int) *KQLEditor {
	ta := textarea.New()
	ta.Placeholder = "Enter your KQL query here...\nExample:\ntraces\n| where timestamp >= ago(1h)\n| where customDimensions.eventId == \"RT0019\"\n| project timestamp, severityLevel, message\n| order by timestamp desc\n| limit 100"
	ta.ShowLineNumbers = true
	ta.Focus()

	editor := &KQLEditor{
		textArea:   ta,
		focused:    true,
		content:    "",
		history:    history,
		historyIdx: -1,
		errorMsg:   "",
		executing:  false,
		width:      width,
		height:     height,
	}

	editor.updateSize()
	return editor
}

// Focus sets the editor as focused
func (e *KQLEditor) Focus() {
	e.focused = true
	e.textArea.Focus()
}

// Blur removes focus from the editor
func (e *KQLEditor) Blur() {
	e.focused = false
	e.textArea.Blur()
}

// IsFocused returns whether the editor is focused
func (e *KQLEditor) IsFocused() bool {
	return e.focused
}

// SetSize updates the editor dimensions
func (e *KQLEditor) SetSize(width, height int) {
	e.width = width
	e.height = height
	e.updateSize()
}

// updateSize applies the current width/height to the textarea
func (e *KQLEditor) updateSize() {
	// Leave some space for borders and line numbers
	e.textArea.SetWidth(e.width - 4)
	e.textArea.SetHeight(e.height - 2)
}

// GetContent returns the current editor content
func (e *KQLEditor) GetContent() string {
	return e.textArea.Value()
}

// SetContent sets the editor content
func (e *KQLEditor) SetContent(content string) {
	e.textArea.SetValue(content)
	e.content = content
}

// SetError sets an error message to display
func (e *KQLEditor) SetError(msg string) {
	e.errorMsg = msg
}

// ClearError clears any error message
func (e *KQLEditor) ClearError() {
	e.errorMsg = ""
}

// SetExecuting sets the executing state
func (e *KQLEditor) SetExecuting(executing bool) {
	e.executing = executing
}

// IsExecuting returns whether a query is currently executing
func (e *KQLEditor) IsExecuting() bool {
	return e.executing
}

// Update handles bubble tea messages for the editor
func (e *KQLEditor) Update(msg tea.Msg) (*KQLEditor, tea.Cmd) {
	if !e.focused {
		return e, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlK:
			// Clear editor content
			e.SetContent("")
			e.historyIdx = -1
			return e, nil
		case tea.KeyCtrlUp:
			// Navigate to previous query in history
			return e.navigateHistory(-1), nil
		case tea.KeyCtrlDown:
			// Navigate to next query in history
			return e.navigateHistory(1), nil
		case tea.KeyF5:
			// Execute query - this will be handled by the parent component
			e.content = e.textArea.Value()
			return e, tea.Cmd(func() tea.Msg {
				return ExecuteQueryMsg{Query: e.content}
			})
		}
	}

	// Handle regular textarea updates
	var cmd tea.Cmd
	e.textArea, cmd = e.textArea.Update(msg)

	// Update content and reset history navigation when user types
	if e.textArea.Value() != e.content {
		e.content = e.textArea.Value()
		e.historyIdx = -1 // Reset history navigation when user modifies content
	}

	return e, cmd
}

// navigateHistory moves through query history
func (e *KQLEditor) navigateHistory(direction int) *KQLEditor {
	if e.history.Count() == 0 {
		return e
	}

	// Calculate new history index
	// -1 direction = go back in history (from current -1 to 0, 1, 2...)
	// +1 direction = go forward in history (from 2, 1, 0 to -1)
	newIdx := e.historyIdx - direction

	// Clamp to valid range
	if newIdx < -1 {
		newIdx = -1
	} else if newIdx >= e.history.Count() {
		newIdx = e.history.Count() - 1
	}

	e.historyIdx = newIdx

	// Set content based on history index
	if e.historyIdx == -1 {
		// Back to current editing content (empty for new query)
		e.textArea.SetValue("")
	} else {
		if entry, ok := e.history.GetQuery(e.historyIdx); ok {
			e.textArea.SetValue(entry.Query)
		}
	}

	// Update the content field to match
	e.content = e.textArea.Value()

	return e
}

// View renders the KQL editor
func (e *KQLEditor) View() string {
	var (
		titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true)

		errorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")).
				Bold(true)

		statusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

		borderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("24"))

		executingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226")).
				Bold(true)
	)

	var content strings.Builder

	// Title and status line
	title := "KQL Query Editor"
	if e.executing {
		title += " " + executingStyle.Render("[Executing...]")
	}
	content.WriteString(titleStyle.Render(title))

	// Status line with shortcuts
	statusLine := "Press F5 to Execute | Ctrl+↑/↓ for History | Ctrl+K to Clear"
	if e.history.Count() > 0 {
		if e.historyIdx >= 0 {
			statusLine += " " + statusStyle.Render(fmt.Sprintf("(History %d/%d)", e.historyIdx+1, e.history.Count()))
		} else {
			statusLine += " " + statusStyle.Render(fmt.Sprintf("(%d queries in history)", e.history.Count()))
		}
	}
	content.WriteString(" | " + statusLine)
	content.WriteString("\n")

	// Error message if present
	if e.errorMsg != "" {
		content.WriteString(errorStyle.Render("Error: " + e.errorMsg))
		content.WriteString("\n")
	}

	// Text area
	textAreaView := e.textArea.View()

	// Apply border to the entire editor
	editorContent := content.String() + textAreaView
	return borderStyle.Render(editorContent)
}

// ExecuteQueryMsg is sent when F5 is pressed to execute a query
type ExecuteQueryMsg struct {
	Query string
}
