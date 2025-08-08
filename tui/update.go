package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FBakkensen/bc-insights-tui/auth"
)

// Init starts device flow automatically if no valid token exists.
func (m model) Init() tea.Cmd {
	if m.authenticator != nil && !m.authenticator.HasValidToken() {
		// Start device flow immediately when auth is required
		return m.startAuthCmd()
	}
	return nil
}

// Update handles messages and key events.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)
	case tea.KeyMsg:
		// Let textarea consume keys first so typing works
		var cmd tea.Cmd
		m.ta, cmd = m.ta.Update(msg)
		// Then handle global keys and Enter submission
		m2, cmd2 := m.handleKey(msg)
		return m2, tea.Batch(cmd, cmd2)
	case deviceCodeMsg:
		return m.handleDeviceCode(msg)
	case authSuccessMsg:
		return m.handleAuthSuccess()
	case authErrorMsg:
		return m.handleAuthError(msg)
	}
	// Let child components update
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	// Track followTail based on whether user is at bottom before update
	wasAtBottom := m.vp.AtBottom()
	m.vp, _ = m.vp.Update(msg)
	// If user scrolled up, stop following; if they reached bottom, resume
	if wasAtBottom && m.vp.AtBottom() {
		m.followTail = true
	} else if !m.vp.AtBottom() {
		m.followTail = false
	}
	return m, cmd
}

func (m model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width, m.height = msg.Width, msg.Height
	// Desired content width: cap at maxContentWidth; center horizontally via container padding
	contentWidth := m.width
	if m.maxContentWidth > 0 && contentWidth > m.maxContentWidth {
		contentWidth = m.maxContentWidth
	}
	// Borders take 2 cols; textarea/view should subtract borders from inner widths
	innerWidth := contentWidth - 2
	if innerWidth < 10 {
		innerWidth = 10
	}
	m.ta.SetWidth(innerWidth)
	// Leave one line spacer between viewport and textarea
	vpHeight := m.height - m.ta.Height() - 1 - 2 // -2 for viewport border
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.vp.Width = innerWidth
	m.vp.Height = vpHeight
	if m.followTail || m.vp.AtBottom() {
		m.vp.GotoBottom()
		m.followTail = true
	}

	// Center container horizontally
	sidePad := (m.width - (innerWidth + 2)) / 2 // +2 borders
	if sidePad < 0 {
		sidePad = 0
	}
	m.containerStyle = lipgloss.NewStyle().PaddingLeft(sidePad).PaddingRight(sidePad)
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		// Only submit when we're in single-line mode (InsertNewline disabled)
		if m.ta.KeyMap.InsertNewline.Enabled() {
			return m, nil
		}
		input := strings.TrimSpace(m.ta.Value())
		m.ta.Reset()
		if input == "" {
			return m, nil
		}
		m.append("> " + input)
		if m.authState != auth.AuthStateCompleted {
			return m.handlePreAuthCommand(input)
		}
		return m.handlePostAuthCommand(input)
	}
	return m, nil
}

func (m model) handlePreAuthCommand(input string) (tea.Model, tea.Cmd) {
	if input == "login" {
		if m.authState != auth.AuthStateInProgress {
			m.authState = auth.AuthStateInProgress
			return m, m.startAuthCmd()
		}
		m.append("Login already in progress…")
		return m, nil
	}
	m.append("Authentication required. Type 'login' to start device flow.")
	return m, nil
}

func (m model) handlePostAuthCommand(input string) (tea.Model, tea.Cmd) {
	switch input {
	case "help", "?":
		m.append("Commands: help, login, quit")
	case "quit", "exit":
		m.quitting = true
		return m, tea.Quit
	default:
		m.append("Unknown command. Type 'help'.")
	}
	return m, nil
}

func (m model) handleDeviceCode(msg deviceCodeMsg) (tea.Model, tea.Cmd) {
	// store context and device response
	m.deviceResp = msg.resp
	m.authCtx = msg.ctx
	m.cancelAuth = msg.cancel
	m.authState = auth.AuthStateInProgress
	// Show instructions
	if m.deviceResp.VerificationURI != "" && m.deviceResp.UserCode != "" {
		m.append("Open " + m.deviceResp.VerificationURI + " and enter code " + m.deviceResp.UserCode + ".")
	}
	if m.deviceResp.VerificationURIComplete != "" {
		m.append("Or open: " + m.deviceResp.VerificationURIComplete)
	}
	m.append("Waiting for verification…")
	return m, m.pollForTokenCmd()
}

func (m model) handleAuthSuccess() (tea.Model, tea.Cmd) {
	if m.cancelAuth != nil {
		m.cancelAuth()
		m.cancelAuth = nil
	}
	m.authState = auth.AuthStateCompleted
	m.append("Authentication successful. Token received and saved.")
	return m, nil
}

func (m model) handleAuthError(msg authErrorMsg) (tea.Model, tea.Cmd) {
	if m.cancelAuth != nil {
		m.cancelAuth()
		m.cancelAuth = nil
	}
	m.authState = auth.AuthStateFailed
	m.append("Authentication failed: " + msg.err.Error())
	m.append("Tips: verify Tenant and Client ID, network access, and permissions in Azure Portal: https://portal.azure.com")
	m.append("Type 'login' to try again or 'help'.")
	return m, nil
}
