package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/logging"
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
		return m.handleKeyMessage(msg)
	case deviceCodeMsg:
		return m.handleDeviceCode(msg)
	case authSuccessMsg:
		return m.handleAuthSuccess()
	case authErrorMsg:
		return m.handleAuthError(msg)
	case subsLoadedMsg:
		return m.handleSubsLoaded(msg)
	case insightsLoadedMsg:
		return m.handleInsightsLoaded(msg)
	}
	// Let child components update
	return m.handleComponentUpdate(msg)
}

func (m model) handleKeyMessage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When in list mode, handle Esc and selection differently
	if m.mode == modeListSubscriptions || m.mode == modeListInsightsResources {
		return m.handleListKey(msg)
	}
	// In chat mode: Let textarea consume keys first so typing works
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	// Then handle global keys and Enter submission
	m2, cmd2 := m.handleKey(msg)
	return m2, tea.Batch(cmd, cmd2)
}

func (m model) handleSubsLoaded(msg subsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		logging.Error("Failed to load subscriptions", "error", msg.err.Error())
		m.append("Failed to load subscriptions: " + msg.err.Error())
		m.mode = modeChat
		return m, nil
	}
	logging.Debug("Received subsLoadedMsg", "itemCount", fmt.Sprintf("%d", len(msg.items)))
	m.list.SetItems(msg.items)
	if len(msg.items) == 0 {
		m.append("No subscriptions found for this account.")
		m.mode = modeChat
		return m, nil
	}
	logging.Debug("Set items to list", "listItemCount", fmt.Sprintf("%d", len(m.list.Items())))
	// Stay in list mode and focus list
	return m, nil
}

func (m model) handleInsightsLoaded(msg insightsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		logging.Error("Failed to load Application Insights resources", "error", msg.err.Error())
		m.append("Failed to load Application Insights resources: " + msg.err.Error())
		m.mode = modeChat
		return m, nil
	}
	logging.Debug("Received insightsLoadedMsg", "itemCount", fmt.Sprintf("%d", len(msg.items)))
	m.list.SetItems(msg.items)
	if len(msg.items) == 0 {
		m.append("No Application Insights resources found in the selected subscription.")
		m.mode = modeChat
		return m, nil
	}
	logging.Debug("Set Application Insights items to list", "listItemCount", fmt.Sprintf("%d", len(m.list.Items())))
	// Stay in list mode and focus list
	return m, nil
}

func (m model) handleComponentUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	// size list to same as viewport when active
	m.list.SetSize(innerWidth, vpHeight)
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
		m.append("Commands: help, subs, resources, config, login, quit")
	case "quit", "exit":
		m.quitting = true
		return m, tea.Quit
	case "subs":
		// Open subscriptions list panel and load items
		m.mode = modeListSubscriptions
		m.list.SetItems(nil)
		m.list.Title = titleSelectSubscription
		m.append("Loading subscriptions…")
		return m, m.loadSubscriptionsCmd()
	case "resources":
		// Open Application Insights resources list panel and load items
		m.mode = modeListInsightsResources
		m.list.SetItems(nil)
		m.list.Title = titleSelectInsights
		m.append("Loading Application Insights resources…")
		return m, m.loadInsightsResourcesCmd()
	case "config":
		// Show current configuration
		m.showConfig()
		return m, nil
	default:
		m.append("Unknown command. Type 'help'.")
	}
	return m, nil
}

// handleListKey processes keys while in list modes (subscriptions or insights resources)
func (m model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		switch m.mode {
		case modeListSubscriptions:
			m.append("Closed subscriptions panel.")
		case modeListInsightsResources:
			m.append("Closed Application Insights resources panel.")
		}
		m.mode = modeChat
		return m, nil
	case "enter":
		switch m.mode {
		case modeListSubscriptions:
			if sel, ok := m.list.SelectedItem().(subscriptionItem); ok {
				if err := m.cfg.ValidateAndUpdateSetting(keyAzureSubscriptionID, sel.s.ID); err != nil {
					m.append("Failed to set subscription: " + err.Error())
				} else {
					m.append("Subscription selected: " + sel.s.DisplayName + " (" + sel.s.ID + ")")
				}
				m.mode = modeChat
			}
		case modeListInsightsResources:
			if sel, ok := m.list.SelectedItem().(insightsResourceItem); ok {
				if err := m.cfg.ValidateAndUpdateSetting("applicationInsightsAppId", sel.r.ApplicationID); err != nil {
					m.append("Failed to set Application Insights resource: " + err.Error())
				} else {
					m.append("Application Insights resource selected: " + sel.r.Name + " (App ID: " + sel.r.ApplicationID + ")")
				}
				m.mode = modeChat
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
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
