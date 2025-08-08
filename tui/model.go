package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/config"
)

// model implements the chat-first UI with a top viewport (scrollback) and bottom textarea (input).
// It wires Step 1: Azure OAuth2 Device Flow login.

type model struct {
	width  int
	height int

	vp viewport.Model
	ta textarea.Model

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

	m := model{
		vp:              vp,
		ta:              ta,
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
)

// startAuthCmd begins the device flow.
func (m model) startAuthCmd() tea.Cmd {
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
func (m model) pollForTokenCmd() tea.Cmd {
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
