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

	case tea.KeyMsg:
		// Handle authentication state key inputs
		switch m.AuthState {
		case auth.AuthStateRequired:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			default:
				// Any other key starts authentication
				return m, tea.Cmd(func() tea.Msg { return authInitiateMsg{} })
			}

		case auth.AuthStateInProgress:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil

		case auth.AuthStateFailed:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "r":
				// Retry authentication
				m.AuthError = nil
				return m, tea.Cmd(func() tea.Msg { return authInitiateMsg{} })
			}
			return m, nil
		}

		// Handle command palette input first (only in main view)
		if m.CommandPalette {
			switch msg.String() {
			case "esc":
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
				if cmd == "" {
					return m, nil
				}

				m.processCommand(cmd)
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

		// Handle main application input (only when authenticated)
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "ctrl+p":
			// Only allow command palette when authenticated
			if m.AuthState == auth.AuthStateCompleted {
				m.CommandPalette = true
				m.CommandInput = ""
			}
			return m, nil
		case "esc":
			// Close any open modals (currently just command palette)
			if m.CommandPalette {
				m.CommandPalette = false
				m.CommandInput = ""
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		// Handle terminal resize
		m.WindowWidth = msg.Width
		m.WindowHeight = msg.Height
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
		m.HelpText = fmt.Sprintf("Press q to quit, Ctrl+P for command palette. Log fetch size: %d", m.Config.LogFetchSize)
	}
}
