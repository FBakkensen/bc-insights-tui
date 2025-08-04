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
}

func InitialModel(cfg config.Config) Model {
	return Model{
		WelcomeMsg:      "Welcome to bc-insights-tui!",
		HelpText:        fmt.Sprintf("Press q to quit, Ctrl+P for command palette. Log fetch size: %d", cfg.LogFetchSize),
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
	}
}

// Init implements tea.Model interface
func (m Model) Init() tea.Cmd {
	// Check authentication status on startup
	return checkAuthStatus(m.Authenticator)
}
