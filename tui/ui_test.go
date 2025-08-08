package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/FBakkensen/bc-insights-tui/auth"
)

// newTestModel creates a minimal model suitable for unit tests without hitting external services.
func newTestModel() model {
	ta := textarea.New()
	ta.Placeholder = ""
	ta.Prompt = "> "
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.SetWidth(80)
	ta.KeyMap.InsertNewline.SetEnabled(false) // single-line

	vp := viewport.New(80, 10)
	vp.SetContent("")

	m := model{
		vp:              vp,
		ta:              ta,
		authenticator:   nil, // do not call real auth in unit tests
		authState:       auth.AuthStateUnknown,
		maxContentWidth: 120,
		followTail:      true,
	}
	return m
}

func TestUI_PreAuth_LoginFlow_SyntheticMsgs(t *testing.T) {
	m := newTestModel()

	// User types "login" and presses Enter (pre-auth state)
	m.ta.SetValue("login")
	m2Any, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)

	if m2.authState != auth.AuthStateInProgress {
		t.Fatalf("expected auth state InProgress after 'login', got %v", m2.authState)
	}
	if cmd == nil {
		t.Fatalf("expected a command to start device flow, got nil")
	}
	if !strings.Contains(m2.content, "> login") {
		t.Fatalf("expected input echo appended to content; got: %q", m2.content)
	}

	// Simulate device code message (no network)
	dmsg := deviceCodeMsg{resp: &auth.DeviceCodeResponse{
		DeviceCode:              "dev-code",
		UserCode:                "USER-CODE",
		VerificationURI:         "https://example.test/verify",
		VerificationURIComplete: "https://example.test/verify?user_code=USER-CODE",
		Interval:                1,
	}}
	m3Any, _ := m2.Update(dmsg)
	m3 := m3Any.(model)

	// Verify instructions appended
	if !strings.Contains(m3.content, "Open https://example.test/verify and enter code USER-CODE.") {
		t.Fatalf("expected verification instructions in content; got: %q", m3.content)
	}
	if !strings.Contains(m3.content, "Waiting for verification…") {
		t.Fatalf("expected waiting message in content; got: %q", m3.content)
	}

	// Simulate successful auth completion
	m4Any, _ := m3.Update(authSuccessMsg{})
	m4 := m4Any.(model)
	if m4.authState != auth.AuthStateCompleted {
		t.Fatalf("expected auth state Completed, got %v", m4.authState)
	}
	if !strings.Contains(m4.content, "Authentication successful.") {
		t.Fatalf("expected success message appended; got: %q", m4.content)
	}
}

func TestUI_PostAuth_HelpAndQuit(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted

	// help command
	m.ta.SetValue("help")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	if !strings.Contains(m2.content, "Commands: help, login, quit") {
		t.Fatalf("expected help text in content; got: %q", m2.content)
	}

	// quit command
	m2.ta.SetValue("quit")
	m3Any, cmd := m2.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := m3Any.(model)
	if !m3.quitting {
		t.Fatalf("expected quitting flag true after quit command")
	}
	if cmd == nil {
		t.Fatalf("expected a quit command, got nil")
	}
	// Execute cmd to verify it returns a QuitMsg
	if msg := cmd(); msg == nil {
		t.Fatalf("expected tea.Quit command to return a message")
	} else {
		if _, ok := (msg).(tea.QuitMsg); !ok {
			t.Fatalf("expected tea.QuitMsg, got %T", msg)
		}
	}
}

func TestUI_Resize_LayoutDimensions(t *testing.T) {
	m := newTestModel()
	// Send a WindowSizeMsg and verify widths/heights computed
	width, height := 120, 40
	m2, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	got := m2.(model)

	// inner width subtracts 2 for border when capped by maxContentWidth
	expectedContentWidth := width
	if expectedContentWidth > got.maxContentWidth {
		expectedContentWidth = got.maxContentWidth
	}
	expectedInner := expectedContentWidth - 2
	if expectedInner < 10 {
		expectedInner = 10
	}
	if got.ta.Width() <= 0 || got.ta.Width() > expectedInner {
		t.Fatalf("textarea width = %d, want in (0, %d]", got.ta.Width(), expectedInner)
	}
	if got.vp.Width != expectedInner {
		t.Fatalf("viewport width = %d, want %d", got.vp.Width, expectedInner)
	}
	// vpHeight = height - ta.Height() - 1 (spacer) - 2 (border)
	expectedVPHeight := height - got.ta.Height() - 1 - 2
	if expectedVPHeight < 3 {
		expectedVPHeight = 3
	}
	if got.vp.Height != expectedVPHeight {
		t.Fatalf("viewport height = %d, want %d", got.vp.Height, expectedVPHeight)
	}
}

func TestUI_View_QuittingIsEmpty(t *testing.T) {
	m := newTestModel()
	m.quitting = true
	if v := m.View(); v != "" {
		t.Fatalf("expected empty view when quitting, got: %q", v)
	}
}
