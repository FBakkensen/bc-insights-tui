package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/logging"
)

// model implements the chat-first UI with a top viewport (scrollback) and bottom textarea (input).
// It wires Step 1: Azure OAuth2 Device Flow login.

type model struct {
	width  int
	height int

	vp viewport.Model
	ta textarea.Model

	// top panel alternative components
	list list.Model
	mode uiMode
	cfg  config.Config

	// chat content
	content string

	// when true we auto-scroll to bottom on new content; toggled off when
	// the user scrolls up, and back on when they return to bottom.
	followTail bool

	// auth
	authenticator *auth.Authenticator
	authState     auth.AuthState
	deviceResp    *auth.DeviceCodeResponse
	authCtx       context.Context
	cancelAuth    context.CancelFunc

	quitting bool

	// styling/layout
	maxContentWidth int
	vpStyle         lipgloss.Style
	containerStyle  lipgloss.Style
}

type uiMode int

const (
	modeChat uiMode = iota
	modeListSubscriptions
)

// config keys used in TUI (mirror of config.settingAzureSubscriptionID)
const keyAzureSubscriptionID = "azure.subscriptionId"

// Run starts the Bubble Tea program with the chat-first model.
func Run(cfg config.Config) error {
	m := initialModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialModel(cfg config.Config) model {
	ta := textarea.New()
	ta.Placeholder = "Type 'login' to authenticate or 'help'"
	ta.ShowLineNumbers = false
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.SetWidth(80)
	ta.CursorEnd()
	ta.KeyMap.InsertNewline.SetEnabled(false) // single-line behavior for chat

	vp := viewport.New(80, 20)
	vp.SetContent("")
	vpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63"))

	a := auth.NewAuthenticator(cfg.OAuth2)

	// list setup (will be sized on WindowSizeMsg)
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Azure Subscription"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	m := model{
		vp:              vp,
		ta:              ta,
		list:            l,
		mode:            modeChat,
		cfg:             cfg,
		authenticator:   a,
		authState:       auth.AuthStateUnknown,
		maxContentWidth: 120,
		vpStyle:         vpStyle,
		containerStyle:  lipgloss.NewStyle(),
		followTail:      true,
	}
	m.append("Welcome to bc-insights-tui (chat-first).")
	m.append("Step 1: Login using Azure Device Flow.")
	// If a valid token exists, mark completed; else prompt user to 'login'.
	if a.HasValidToken() {
		m.authState = auth.AuthStateCompleted
		m.append("You're already authenticated.")
	} else {
		m.append("Type 'login' and press Enter to authenticate.")
	}
	return m
}

// append adds a line to the scrollback and moves viewport to bottom.
func (m *model) append(line string) {
	if m.content == "" {
		m.content = line
	} else {
		m.content += "\n" + line
	}
	// Update viewport content. Auto-scroll only when following the tail.
	m.vp.SetContent(m.content)
	if m.followTail {
		m.vp.GotoBottom()
	}
}

// msgs used by the update loop

type (
	deviceCodeMsg struct {
		resp   *auth.DeviceCodeResponse
		ctx    context.Context
		cancel context.CancelFunc
	}
	authSuccessMsg struct{}
	authErrorMsg   struct{ err error }
	subsLoadedMsg  struct {
		items []list.Item
		err   error
	}
)

// startAuthCmd begins the device flow.
func (m *model) startAuthCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		resp, err := m.authenticator.InitiateDeviceFlow(ctx)
		if err != nil {
			cancel()
			return authErrorMsg{err: err}
		}
		return deviceCodeMsg{resp: resp, ctx: ctx, cancel: cancel}
	}
}

// bottom-aligning for short content intentionally removed for Option 1.

// pollForTokenCmd runs the blocking PollForToken and signals when done.
func (m *model) pollForTokenCmd() tea.Cmd {
	deviceCode := m.deviceResp.DeviceCode
	interval := m.deviceResp.Interval
	ctx := m.authCtx
	return func() tea.Msg {
		token, err := m.authenticator.PollForToken(ctx, deviceCode, interval)
		if err != nil {
			return authErrorMsg{err: err}
		}
		if err := m.authenticator.SaveTokenSecurely(token); err != nil {
			return authErrorMsg{err: err}
		}
		return authSuccessMsg{}
	}
}

// waitTickCmd emits ticks while waiting during auth.
// (no waiting ticker needed for now)

// loadSubscriptionsCmd loads Azure subscriptions using the Azure client
func (m *model) loadSubscriptionsCmd() tea.Cmd {
	return func() tea.Msg {
		logging.Debug("Starting subscription loading command")

		// Ensure authenticator is present and can provide ARM-scoped tokens
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		logging.Debug("Creating Azure client with authenticator")
		client, err := appinsights.NewAzureClientWithAuthenticator(m.authenticator)
		if err != nil {
			logging.Error("Failed to create Azure client", "error", err.Error())
			return subsLoadedMsg{err: fmt.Errorf("failed to create Azure client: %w", err)}
		}

		logging.Debug("Calling ListSubscriptions")
		subs, err := client.ListSubscriptions(ctx)
		if err != nil {
			logging.Error("Failed to list subscriptions", "error", err.Error())
			return subsLoadedMsg{err: err}
		}

		logging.Info("Successfully retrieved subscriptions", "count", fmt.Sprintf("%d", len(subs)))
		for i, s := range subs {
			logging.Debug("Subscription found", "index", fmt.Sprintf("%d", i), "id", s.ID, "name", s.DisplayName, "state", s.State)
		}

		items := make([]list.Item, 0, len(subs))
		for _, s := range subs {
			// Wrap into list item adapter
			s := s
			items = append(items, subscriptionItem{s: s})
		}

		logging.Debug("Returning subscription items", "itemCount", fmt.Sprintf("%d", len(items)))
		return subsLoadedMsg{items: items}
	}
}

// subscriptionItem adapts AzureSubscription to list.Item
type subscriptionItem struct{ s appinsights.AzureSubscription }

func (i subscriptionItem) FilterValue() string { return i.s.DisplayName }
func (i subscriptionItem) Title() string       { return i.s.DisplayName }
func (i subscriptionItem) Description() string {
	return fmt.Sprintf("ID: %s | State: %s", i.s.ID, i.s.State)
}
