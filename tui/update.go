package tui

// Update logic for Bubble Tea TUI

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FBakkensen/bc-insights-tui/auth"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/oauth2"
)

const (
	// Key combinations
	quitKeyCtrlC = "ctrl+c"
	quitKeyCtrlQ = "ctrl+q"

	// Component focus constants
	focusEditor  = "editor"
	focusResults = "results"

	// Key names
	keyEsc = "esc"
)

// Message types for authentication flow
type authCheckMsg struct {
	hasValidToken bool
}

type deviceCodeMsg struct {
	deviceCode *auth.DeviceCodeResponse
	err        error
}

type authCompleteMsg struct {
	token *oauth2.Token
	err   error
}

type authInitiateMsg struct{}

// Commands for authentication flow
func checkAuthStatus(authenticator *auth.Authenticator) tea.Cmd {
	return func() tea.Msg {
		hasValid := authenticator.HasValidToken()
		return authCheckMsg{hasValidToken: hasValid}
	}
}

func initiateDeviceFlow(authenticator *auth.Authenticator) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		deviceCode, err := authenticator.InitiateDeviceFlow(ctx)
		return deviceCodeMsg{deviceCode: deviceCode, err: err}
	}
}

func pollForToken(authenticator *auth.Authenticator, deviceCode string, interval int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		token, err := authenticator.PollForToken(ctx, deviceCode, interval)
		return authCompleteMsg{token: token, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle authentication messages
	case authCheckMsg, authInitiateMsg, deviceCodeMsg, authCompleteMsg:
		return m.handleAuthMessages(msg)

	// Handle KQL editor messages
	case ExecuteQueryMsg:
		return m.handleExecuteQueryMsg(msg)
	case QueryResultMsg:
		return m.handleQueryResultMsg(msg)

	case tea.KeyMsg:
		return m.handleKeyMessages(msg)

	case tea.WindowSizeMsg:
		// Handle terminal resize
		m.WindowWidth = msg.Width
		m.WindowHeight = msg.Height

		// Update KQL editor component sizes
		if m.KQLEditorMode {
			m.updateKQLEditorSizes()
		}

		return m, nil
	}
	return m, nil
}

// handleAuthMessages processes authentication-related messages
func (m Model) handleAuthMessages(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case authCheckMsg:
		if msg.hasValidToken {
			m.AuthState = auth.AuthStateCompleted
		} else {
			m.AuthState = auth.AuthStateRequired
		}
		return m, nil

	case authInitiateMsg:
		m.AuthState = auth.AuthStateInProgress
		return m, initiateDeviceFlow(m.Authenticator)

	case deviceCodeMsg:
		if msg.err != nil {
			m.AuthState = auth.AuthStateFailed
			m.AuthError = msg.err
			return m, nil
		}
		m.DeviceCode = msg.deviceCode
		return m, pollForToken(m.Authenticator, msg.deviceCode.DeviceCode, msg.deviceCode.Interval)

	case authCompleteMsg:
		if msg.err != nil {
			m.AuthState = auth.AuthStateFailed
			m.AuthError = msg.err
			return m, nil
		}

		// Save token securely
		if err := m.Authenticator.SaveTokenSecurely(msg.token); err != nil {
			m.AuthState = auth.AuthStateFailed
			m.AuthError = fmt.Errorf("failed to save token: %w", err)
			return m, nil
		}

		m.AuthState = auth.AuthStateCompleted
		m.DeviceCode = nil
		m.AuthError = nil
		return m, nil
	}
	return m, nil
}

// handleKeyMessages processes keyboard input
func (m Model) handleKeyMessages(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check for global quit commands first - but only Ctrl+Q and Ctrl+C
	if msg.String() == quitKeyCtrlQ || msg.String() == quitKeyCtrlC {
		return m, tea.Quit
	}

	// Handle command palette input first (highest priority when active)
	if m.CommandPalette {
		return m.handleCommandPaletteKeys(msg)
	}

	// Handle KQL editor input when in KQL editor mode
	if m.KQLEditorMode {
		return m.handleKQLEditorKeys(msg)
	}

	// Handle authentication state key inputs (only when not authenticated)
	if m.AuthState != auth.AuthStateCompleted {
		return m.handleAuthStateKeys(msg)
	}

	// Handle main application input (only when authenticated)
	return m.handleMainAppKeys(msg)
}

// handleAuthStateKeys handles key input during authentication states
func (m Model) handleAuthStateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.AuthState {
	case auth.AuthStateRequired:
		// Any key starts authentication (quit is handled globally)
		return m, tea.Cmd(func() tea.Msg { return authInitiateMsg{} })

	case auth.AuthStateInProgress:
		// Only quit allowed (handled globally), ignore other keys
		return m, nil

	case auth.AuthStateFailed:
		switch msg.String() {
		case "r":
			// Retry authentication
			m.AuthError = nil
			return m, tea.Cmd(func() tea.Msg { return authInitiateMsg{} })
		}
		return m, nil
	}
	return m, nil
}

// handleCommandPaletteKeys handles key input when command palette is active
func (m Model) handleCommandPaletteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		// Close command palette
		m.CommandPalette = false
		m.CommandInput = ""
		return m, nil
	case "enter":
		// Process command
		m.CommandPalette = false
		cmd := strings.TrimSpace(m.CommandInput)
		m.CommandInput = ""

		// Process the command
		if cmd != "" {
			(&m).processCommand(cmd)
		}
		return m, nil
	case "backspace":
		// Remove last character from command input
		if len(m.CommandInput) > 0 {
			m.CommandInput = m.CommandInput[:len(m.CommandInput)-1]
		}
		return m, nil
	default:
		// Add character to command input
		if len(msg.String()) == 1 {
			m.CommandInput += msg.String()
		}
		return m, nil
	}
}

// handleMainAppKeys handles key input for the main application
func (m Model) handleMainAppKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+p":
		// Only allow command palette when authenticated
		if m.AuthState == auth.AuthStateCompleted {
			m.CommandPalette = true
			m.CommandInput = ""
		}
		return m, nil
	case keyEsc:
		// Close any open modals (currently just command palette)
		if m.CommandPalette {
			m.CommandPalette = false
			m.CommandInput = ""
		}
		return m, nil
	}
	return m, nil
}

// processCommand handles command execution from the command palette
func (m *Model) processCommand(cmd string) {
	// Clear previous feedback
	m.FeedbackMessage = ""
	m.FeedbackIsError = false

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "set":
		m.handleSetCommand(parts[1:])
	case "auth":
		m.handleAuthCommand(parts[1:])
	case "query":
		m.handleQueryCommand(parts[1:])
	default:
		m.FeedbackMessage = fmt.Sprintf("Unknown command: %s", parts[0])
		m.FeedbackIsError = true
	}
}

// handleAuthCommand processes authentication-related commands
func (m *Model) handleAuthCommand(args []string) {
	if len(args) == 0 {
		// Show auth status
		switch m.AuthState {
		case auth.AuthStateCompleted:
			m.FeedbackMessage = "✓ Authentication: Active and valid"
		case auth.AuthStateRequired:
			m.FeedbackMessage = "⚠ Authentication: Required"
		case auth.AuthStateInProgress:
			m.FeedbackMessage = "⏳ Authentication: In progress"
		case auth.AuthStateFailed:
			m.FeedbackMessage = "❌ Authentication: Failed"
		default:
			m.FeedbackMessage = "❓ Authentication: Unknown status"
		}
		return
	}

	switch args[0] {
	case "logout", "clear":
		if err := m.Authenticator.ClearToken(); err != nil {
			m.FeedbackMessage = fmt.Sprintf("Failed to clear authentication: %v", err)
			m.FeedbackIsError = true
		} else {
			m.AuthState = auth.AuthStateRequired
			m.FeedbackMessage = "✓ Authentication cleared. You will need to re-authenticate."
		}
	case "refresh":
		// This would normally be handled automatically, but allow manual refresh
		m.FeedbackMessage = "Token refresh is handled automatically when needed"
	default:
		m.FeedbackMessage = fmt.Sprintf("Unknown auth command: %s. Available: logout, clear, refresh", args[0])
		m.FeedbackIsError = true
	}
}

// handleSetCommand processes the "set" command
func (m *Model) handleSetCommand(args []string) {
	if len(args) == 0 {
		// List all settings
		settings := m.Config.ListAllSettings()
		var settingsList []string
		for name, value := range settings {
			settingsList = append(settingsList, fmt.Sprintf("%s=%s", name, value))
		}
		m.FeedbackMessage = fmt.Sprintf("Current settings: %s", strings.Join(settingsList, ", "))
		return
	}

	// Parse setting=value format
	arg := strings.Join(args, " ")
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		m.FeedbackMessage = "Usage: set <setting>=<value> or just 'set' to list all settings"
		m.FeedbackIsError = true
		return
	}

	setting := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	if err := m.Config.ValidateAndUpdateSetting(setting, value); err != nil {
		m.FeedbackMessage = err.Error()
		m.FeedbackIsError = true
		return
	}

	// Success feedback
	m.FeedbackMessage = fmt.Sprintf("✓ %s set to: %s", setting, value)

	// Update help text if LogFetchSize changed
	if setting == "fetchSize" {
		m.HelpText = fmt.Sprintf("Press Ctrl+Q to quit, Ctrl+P for command palette. Log fetch size: %d", m.Config.LogFetchSize)
	}
}

// handleQueryCommand processes query-related commands
func (m *Model) handleQueryCommand(args []string) {
	// Check authentication first
	if m.AuthState != auth.AuthStateCompleted {
		m.FeedbackMessage = "❌ Authentication required for query operations"
		m.FeedbackIsError = true
		return
	}

	if len(args) == 0 {
		// Open KQL editor mode
		m.KQLEditorMode = true
		m.FocusedComponent = focusEditor
		m.updateKQLEditorSizes()
		m.KQLEditor.Focus()
		m.ResultsTable.Blur()
		m.FeedbackMessage = "✓ KQL Editor opened. Press F5 to execute queries, Esc to exit"
		return
	}

	switch args[0] {
	case "run":
		if !m.KQLEditorMode {
			m.FeedbackMessage = "❌ KQL editor is not open. Use 'query' command to open it first"
			m.FeedbackIsError = true
			return
		}
		// Execute current query (same as F5)
		query := m.KQLEditor.GetContent()
		if strings.TrimSpace(query) == "" {
			m.FeedbackMessage = "❌ No query to execute"
			m.FeedbackIsError = true
			return
		}
		// This will be handled by the handleExecuteQueryMsg function
	case "clear":
		if !m.KQLEditorMode {
			m.FeedbackMessage = "❌ KQL editor is not open. Use 'query' command to open it first"
			m.FeedbackIsError = true
			return
		}
		m.KQLEditor.SetContent("")
		m.FeedbackMessage = "✓ Query editor cleared"
	case "history":
		count := m.QueryHistory.Count()
		if count == 0 {
			m.FeedbackMessage = "No queries in history"
		} else {
			m.FeedbackMessage = fmt.Sprintf("Query history: %d queries. Use Ctrl+↑/↓ in editor to browse", count)
		}
	default:
		m.FeedbackMessage = fmt.Sprintf("Unknown query command: %s. Available: run, clear, history", args[0])
		m.FeedbackIsError = true
	}
}

// handleKQLEditorKeys handles key input when KQL editor is active
func (m Model) handleKQLEditorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		// Exit KQL editor mode
		m.KQLEditorMode = false
		m.KQLEditor.Blur()
		m.ResultsTable.Blur()
		return m, nil
	case "tab":
		// Switch focus between editor and results
		if m.FocusedComponent == focusEditor && m.ResultsTable.HasResults() {
			m.FocusedComponent = focusResults
			m.KQLEditor.Blur()
			m.ResultsTable.Focus()
		} else {
			m.FocusedComponent = focusEditor
			m.KQLEditor.Focus()
			m.ResultsTable.Blur()
		}
		return m, nil
	}

	// Forward other keys to the focused component
	var cmd tea.Cmd
	switch m.FocusedComponent {
	case focusEditor:
		m.KQLEditor, cmd = m.KQLEditor.Update(msg)
	case focusResults:
		m.ResultsTable, cmd = m.ResultsTable.Update(msg)
	}

	return m, cmd
}

// handleExecuteQueryMsg handles query execution requests
func (m Model) handleExecuteQueryMsg(msg ExecuteQueryMsg) (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(msg.Query)
	if query == "" {
		m.KQLEditor.SetError("Query cannot be empty")
		return m, nil
	}

	// Clear any previous errors
	m.KQLEditor.ClearError()
	m.KQLEditor.SetExecuting(true)

	// Execute query with configured timeout
	timeout := time.Duration(m.Config.QueryTimeoutSeconds) * time.Second
	return m, m.QueryExecutor.ExecuteQuery(query, timeout)
}

// handleQueryResultMsg handles query execution results
func (m Model) handleQueryResultMsg(msg QueryResultMsg) (tea.Model, tea.Cmd) {
	m.KQLEditor.SetExecuting(false)

	if msg.Error != nil {
		// Display error
		errorMsg := formatQueryError(msg.Error)
		m.KQLEditor.SetError(errorMsg)
		m.ResultsTable.ClearResults()
	} else {
		// Display results
		m.KQLEditor.ClearError()
		m.ResultsTable.SetResults(msg.Results)

		// Add to history if successful
		if msg.Results != nil {
			m.QueryHistory.AddEntry(msg.Query, true, msg.Results.RowCount, msg.Duration)
		}

		// Switch focus to results if we have data
		if msg.Results != nil && msg.Results.RowCount > 0 {
			m.FocusedComponent = focusResults
			m.KQLEditor.Blur()
			m.ResultsTable.Focus()
		}
	}

	return m, nil
}

// updateKQLEditorSizes updates the sizes of KQL editor components based on window size
func (m *Model) updateKQLEditorSizes() {
	if m.WindowWidth <= 0 || m.WindowHeight <= 0 {
		return
	}

	// Calculate panel heights based on configured ratio
	totalHeight := m.WindowHeight - 4 // Account for header, footer, borders
	editorHeight := int(float32(totalHeight) * m.Config.EditorPanelRatio)
	resultsHeight := totalHeight - editorHeight - 2 // Account for borders between panels

	// Ensure minimum heights
	if editorHeight < 5 {
		editorHeight = 5
	}
	if resultsHeight < 5 {
		resultsHeight = 5
	}

	// Update component sizes
	m.KQLEditor.SetSize(m.WindowWidth-4, editorHeight)
	m.ResultsTable.SetSize(m.WindowWidth-4, resultsHeight)
}
