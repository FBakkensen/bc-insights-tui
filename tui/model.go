package tui

// Model for Bubble Tea TUI

import (
	"fmt"

	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/config"
	tea "github.com/charmbracelet/bubbletea"
)

const AppVersion = "v1.0.0"

type Model struct {
	WelcomeMsg      string
	HelpText        string
	Config          config.Config
	CommandPalette  bool   // Command palette is open
	WindowWidth     int    // Terminal width
	WindowHeight    int    // Terminal height
	CommandInput    string // Current command input
	FeedbackMessage string // Feedback from last command
	FeedbackIsError bool   // Whether feedback is an error message

	// Authentication state
	AuthState     auth.AuthState
	DeviceCode    *auth.DeviceCodeResponse
	AuthError     error
	Authenticator *auth.Authenticator

	// KQL Editor components
	KQLEditorMode    bool           // Whether KQL editor is active
	KQLEditor        *KQLEditor     // KQL editor component
	QueryExecutor    *QueryExecutor // Query execution engine
	ResultsTable     *ResultsTable  // Results display table
	QueryHistory     *QueryHistory  // Query history management
	FocusedComponent string         // "editor" or "results"
}

func InitialModel(cfg config.Config) Model {
	// Initialize query history
	queryHistory := NewQueryHistory(cfg.QueryHistoryMaxEntries, cfg.QueryHistoryFile)

	// Initialize KQL editor components (but not visible initially)
	kqlEditor := NewKQLEditor(queryHistory, 80, 20) // Will be resized on first render
	resultsTable := NewResultsTable(80, 20)         // Will be resized on first render
	queryExecutor := NewQueryExecutor(auth.NewAuthenticator(cfg.OAuth2), cfg.ApplicationInsightsKey)

	// Blur components initially since we start in main view
	kqlEditor.Blur()
	resultsTable.Blur()

	return Model{
		WelcomeMsg:      "Welcome to bc-insights-tui!",
		HelpText:        fmt.Sprintf("Press Ctrl+Q to quit, Ctrl+P for command palette. Log fetch size: %d", cfg.LogFetchSize),
		Config:          cfg,
		CommandPalette:  false,
		WindowWidth:     80, // Default width
		WindowHeight:    24, // Default height
		CommandInput:    "",
		FeedbackMessage: "",
		FeedbackIsError: false,

		// Initialize authentication
		AuthState:     auth.AuthStateUnknown,
		DeviceCode:    nil,
		AuthError:     nil,
		Authenticator: auth.NewAuthenticator(cfg.OAuth2),

		// Initialize KQL Editor components
		KQLEditorMode:    false,
		KQLEditor:        kqlEditor,
		QueryExecutor:    queryExecutor,
		ResultsTable:     resultsTable,
		QueryHistory:     queryHistory,
		FocusedComponent: focusEditor, // Default focus when entering KQL mode
	}
}

// Init implements tea.Model interface
func (m Model) Init() tea.Cmd {
	// Check authentication status on startup
	return checkAuthStatus(m.Authenticator)
}
